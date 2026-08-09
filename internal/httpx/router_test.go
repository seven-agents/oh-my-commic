package httpx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/db"
	"github.com/seven-agents/oh-my-commic/internal/models"
	"github.com/seven-agents/oh-my-commic/internal/storage"
)

// testEnv bundles the router with the handles a test needs to seed data or
// promote a user to admin out of band.
type testEnv struct {
	router http.Handler
	svc    *auth.Service
	repo   *auth.UserRepo
	code   string
}

// newTestRouter wires a real in-memory database and the real handlers into the
// application router, seeding an invite code so register works end to end.
func newTestRouter(t *testing.T) testEnv {
	t.Helper()

	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	sess := auth.NewSession(nil)
	repo := auth.NewUserRepo(d)
	invites := auth.NewInviteRepo(d)
	code, err := invites.Seed("invite01")
	if err != nil {
		t.Fatalf("seed invite: %v", err)
	}
	svc := auth.NewService(repo, invites, sess, 100, 0)
	authHandler := auth.NewHandler(svc, storage.Local{Root: t.TempDir()})
	bookHandler := book.NewHandler(book.NewService(book.NewRepo(d)))

	router := NewRouter(Deps{
		Session:  sess,
		UserRepo: repo,
		Auth:     authHandler,
		Book:     bookHandler,
		Media:    nil,
	})
	return testEnv{router: router, svc: svc, repo: repo, code: code}
}

// registerBody builds a valid register JSON body for the given username.
func registerBody(username, code string) string {
	return fmt.Sprintf(
		`{"username":%q,"password":"pw123456","email":%q,"inviteCode":%q}`,
		username, username+"@example.com", code,
	)
}

func TestHealth(t *testing.T) {
	env := newTestRouter(t)
	srv := httptest.NewServer(env.router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body["ok"] {
		t.Fatalf("want ok=true, got %v", body)
	}
}

// TestBooksRequireAuth verifies the protected group rejects unauthenticated
// requests to book routes.
func TestBooksRequireAuth(t *testing.T) {
	env := newTestRouter(t)
	srv := httptest.NewServer(env.router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/books")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/books without session: want 401, got %d", resp.StatusCode)
	}
}

// TestBookFlow exercises the full wiring: register -> login (capture cookie) ->
// create a book with that cookie -> list it back.
func TestBookFlow(t *testing.T) {
	env := newTestRouter(t)
	srv := httptest.NewServer(env.router)
	defer srv.Close()

	// Register.
	regResp := post(t, srv.URL+"/api/v1/register", "", registerBody("xiaoming", env.code))
	regResp.Body.Close()
	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("register: want 201, got %d", regResp.StatusCode)
	}

	// Login and capture the session cookie.
	loginResp := post(t, srv.URL+"/api/v1/login", "", `{"username":"xiaoming","password":"pw123456"}`)
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login: want 200, got %d", loginResp.StatusCode)
	}
	cookie := sessionCookie(t, loginResp)

	// Create a book with the session cookie.
	createResp := post(t, srv.URL+"/api/v1/books", cookie, `{"title":"星星的故事"}`)
	var created models.Book
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create book: want 201, got %d", createResp.StatusCode)
	}
	if created.Title != "星星的故事" {
		t.Fatalf("create book: unexpected title %q", created.Title)
	}

	// List books; the created one must be present.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/books", nil)
	req.Header.Set("Cookie", cookie)
	listResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list books: want 200, got %d", listResp.StatusCode)
	}
	var books []models.Book
	if err := json.NewDecoder(listResp.Body).Decode(&books); err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 || books[0].ID != created.ID {
		t.Fatalf("list books: want the created book, got %+v", books)
	}
}

// TestMeRequiresAuth verifies GET /api/v1/me is behind RequireUser.
func TestMeRequiresAuth(t *testing.T) {
	env := newTestRouter(t)
	srv := httptest.NewServer(env.router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/me")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/me without session: want 401, got %d", resp.StatusCode)
	}
}

// TestMeReturnsCredits verifies an authenticated GET /api/v1/me returns the
// user's credit balance and never leaks the password hash.
func TestMeReturnsCredits(t *testing.T) {
	env := newTestRouter(t)
	srv := httptest.NewServer(env.router)
	defer srv.Close()

	cookie := registerLogin(t, srv.URL, "jifen", env.code)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/me", nil)
	req.Header.Set("Cookie", cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/me: want 200, got %d", resp.StatusCode)
	}
	var u models.User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		t.Fatal(err)
	}
	if u.Credits != 100 {
		t.Fatalf("me: want 100 credits, got %d", u.Credits)
	}
	if u.PasswordHash != "" {
		t.Fatalf("me: password hash must never be serialized, got %q", u.PasswordHash)
	}
}

// TestProfileRouteUpdatesNickname verifies PUT /api/v1/me/profile is wired into
// the protected group and updates the user.
func TestProfileRouteUpdatesNickname(t *testing.T) {
	env := newTestRouter(t)
	srv := httptest.NewServer(env.router)
	defer srv.Close()

	cookie := registerLogin(t, srv.URL, "profile", env.code)

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/me/profile",
		bytes.NewBufferString(`{"nickname":"新名","age":9,"gender":"男"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /me/profile: want 200, got %d", resp.StatusCode)
	}
	var u models.User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		t.Fatal(err)
	}
	if u.Nickname != "新名" || u.Age != 9 || u.Gender != "男" {
		t.Fatalf("profile not updated: %+v", u)
	}
}

// TestAvatarRouteStoresAndSetsURL verifies POST /api/v1/me/avatar is wired in and
// returns an avatarUrl under the users/ namespace.
func TestAvatarRouteStoresAndSetsURL(t *testing.T) {
	env := newTestRouter(t)
	srv := httptest.NewServer(env.router)
	defer srv.Close()

	cookie := registerLogin(t, srv.URL, "avatar", env.code)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "a.png")
	// Minimal PNG magic so DetectContentType reports image/png.
	fw.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	mw.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/me/avatar", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Cookie", cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /me/avatar: want 200, got %d", resp.StatusCode)
	}
	var u models.User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		t.Fatal(err)
	}
	if u.AvatarURL == "" {
		t.Fatal("avatarUrl not set")
	}
}

// TestAdminInviteRouteForbiddenForRegularUser verifies a regular user hitting the
// admin invite route gets 403.
func TestAdminInviteRouteForbiddenForRegularUser(t *testing.T) {
	env := newTestRouter(t)
	srv := httptest.NewServer(env.router)
	defer srv.Close()

	cookie := registerLogin(t, srv.URL, "putong", env.code)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/admin/invite-code", nil)
	req.Header.Set("Cookie", cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("regular user admin route: want 403, got %d", resp.StatusCode)
	}
}

// TestAdminInviteRouteReturnsCode verifies an admin can read the invite code via
// the router-mounted admin group.
func TestAdminInviteRouteReturnsCode(t *testing.T) {
	env := newTestRouter(t)
	srv := httptest.NewServer(env.router)
	defer srv.Close()

	admin, err := env.repo.Create(auth.NewUser{
		Username: "boss", Email: "boss@example.com", PasswordHash: "x",
		Nickname: "boss", Role: "admin", Credits: 0,
	})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	token := env.svc.Sessions().Issue(admin.ID)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/admin/invite-code", nil)
	req.Header.Set("Cookie", "session="+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin GET invite: want 200, got %d", resp.StatusCode)
	}
	var body struct {
		InviteCode string `json:"inviteCode"`
		Used       int    `json:"used"`
		Limit      int    `json:"limit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.InviteCode != env.code {
		t.Fatalf("invite code = %q, want %q", body.InviteCode, env.code)
	}
}

// registerLogin registers a user and returns its session cookie header value.
func registerLogin(t *testing.T, baseURL, username, code string) string {
	t.Helper()
	regResp := post(t, baseURL+"/api/v1/register", "", registerBody(username, code))
	regResp.Body.Close()
	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("register %q: want 201, got %d", username, regResp.StatusCode)
	}
	loginResp := post(t, baseURL+"/api/v1/login", "",
		fmt.Sprintf(`{"username":%q,"password":"pw123456"}`, username))
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login %q: want 200, got %d", username, loginResp.StatusCode)
	}
	return sessionCookie(t, loginResp)
}

// post issues a JSON POST, optionally carrying a Cookie header.
func post(t *testing.T, url, cookie, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// sessionCookie extracts the session cookie from a login response as a
// "name=value" string suitable for a Cookie request header.
func sessionCookie(t *testing.T, resp *http.Response) string {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == "session" && c.Value != "" {
			return c.Name + "=" + c.Value
		}
	}
	t.Fatal("login response did not set a session cookie")
	return ""
}
