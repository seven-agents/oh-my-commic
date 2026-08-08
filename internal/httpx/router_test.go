package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/db"
	"github.com/seven-agents/oh-my-commic/internal/models"
)

// newTestRouter wires a real in-memory database and the real handlers into the
// application router, returning it alongside the DB handle for cleanup.
func newTestRouter(t *testing.T) http.Handler {
	t.Helper()

	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	sess := auth.NewSession(nil)
	authHandler := auth.NewHandler(auth.NewService(auth.NewUserRepo(d), sess, 100))
	bookHandler := book.NewHandler(book.NewService(book.NewRepo(d)))

	return NewRouter(Deps{
		Session: sess,
		Auth:    authHandler,
		Book:    bookHandler,
		Media:   nil,
	})
}

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(newTestRouter(t))
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
	srv := httptest.NewServer(newTestRouter(t))
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
// create a book with that cookie -> list it back. This proves the public auth
// routes, the RequireUser-protected book group, and per-user scoping all
// compose correctly end to end.
func TestBookFlow(t *testing.T) {
	srv := httptest.NewServer(newTestRouter(t))
	defer srv.Close()

	const creds = `{"nickname":"小明","password":"pw123456"}`

	// Register.
	regResp := post(t, srv.URL+"/api/v1/register", "", creds)
	regResp.Body.Close()
	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("register: want 201, got %d", regResp.StatusCode)
	}

	// Login and capture the session cookie.
	loginResp := post(t, srv.URL+"/api/v1/login", "", creds)
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
	srv := httptest.NewServer(newTestRouter(t))
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

// TestMeReturnsCredits verifies an authenticated GET /api/v1/me returns the user's
// credit balance (100 by default) and never leaks the password hash.
func TestMeReturnsCredits(t *testing.T) {
	srv := httptest.NewServer(newTestRouter(t))
	defer srv.Close()

	const creds = `{"nickname":"积分","password":"pw123456"}`
	post(t, srv.URL+"/api/v1/register", "", creds).Body.Close()
	loginResp := post(t, srv.URL+"/api/v1/login", "", creds)
	loginResp.Body.Close()
	cookie := sessionCookie(t, loginResp)

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
	if u.Nickname != "积分" {
		t.Fatalf("me: unexpected nickname %q", u.Nickname)
	}
	if u.Credits != 100 {
		t.Fatalf("me: want 100 credits, got %d", u.Credits)
	}
	if u.PasswordHash != "" {
		t.Fatalf("me: password hash must never be serialized, got %q", u.PasswordHash)
	}
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
