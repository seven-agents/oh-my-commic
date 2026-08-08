package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/models"
	"github.com/seven-agents/oh-my-commic/internal/storage"
)

// newTestHandler builds a Handler over a fresh service and returns it together
// with the seeded invite code (required by the register endpoint). The avatar
// store is rooted at a per-test temp dir so uploads never touch the repo tree.
func newTestHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	svc, code := newTestService(t, 100)
	return NewHandler(svc, storage.Local{Root: t.TempDir()}), code
}

// pngBytes is a minimal but valid PNG header so http.DetectContentType reports
// image/png. The trailing bytes are padding; content is never decoded.
var pngBytes = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
}

// registerAndLogin registers a fresh user and returns the session cookie value
// from the login response, so protected endpoints can be exercised.
func registerAndLogin(t *testing.T, h http.Handler, username, code string) string {
	t.Helper()
	if rec := post(t, h, "/api/v1/register", registerJSON(username, code)); rec.Code != http.StatusCreated {
		t.Fatalf("register %q: status %d; body=%s", username, rec.Code, rec.Body.String())
	}
	rec := post(t, h, "/api/v1/login", fmt.Sprintf(`{"username":%q,"password":"pw123456"}`, username))
	if rec.Code != http.StatusOK {
		t.Fatalf("login %q: status %d; body=%s", username, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			return c.Value
		}
	}
	t.Fatalf("login %q did not set a session cookie", username)
	return ""
}

// doWithCookie issues a request carrying the session cookie and returns the
// recorder.
func doWithCookie(t *testing.T, h http.Handler, req *http.Request, token string) *httptest.ResponseRecorder {
	t.Helper()
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func post(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// registerJSON builds a valid register request body for the given username.
func registerJSON(username, code string) string {
	return fmt.Sprintf(
		`{"username":%q,"password":"pw123456","email":%q,"inviteCode":%q}`,
		username, username+"@example.com", code,
	)
}

func TestHandlerRegisterThenLoginSetsCookie(t *testing.T) {
	h, code := newTestHandler(t)

	rec := post(t, h, "/api/v1/register", registerJSON("xiaogang", code))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "password") || strings.Contains(rec.Body.String(), "hash") {
		t.Fatalf("register response leaked password field: %s", rec.Body.String())
	}

	rec = post(t, h, "/api/v1/login", `{"username":"xiaogang","password":"pw123456"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var token string
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			token = c.Value
			if !c.HttpOnly {
				t.Fatal("session cookie must be HttpOnly")
			}
			if c.Path != "/" {
				t.Fatalf("session cookie path = %q, want /", c.Path)
			}
		}
	}
	if token == "" {
		t.Fatal("login did not set a session cookie")
	}
	if _, ok := h.svc.Sessions().UserID(token); !ok {
		t.Fatal("cookie token not resolvable in session store")
	}
}

func TestHandlerDuplicateUsernameReturns409(t *testing.T) {
	h, code := newTestHandler(t)
	_ = post(t, h, "/api/v1/register", registerJSON("chongfu", code))
	// Same username, different email.
	dup := fmt.Sprintf(
		`{"username":"chongfu","password":"pw123456","email":"other@example.com","inviteCode":%q}`, code)
	rec := post(t, h, "/api/v1/register", dup)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate register status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandlerBadInviteReturns400(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(t, h, "/api/v1/register", registerJSON("badinvite", "wrong-code"))
	// A bad invite code is a client input error, so it maps to 400 (not 403).
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad invite status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandlerBadCredentialsReturns401(t *testing.T) {
	h, code := newTestHandler(t)
	_ = post(t, h, "/api/v1/register", registerJSON("zhangsan", code))
	rec := post(t, h, "/api/v1/login", `{"username":"zhangsan","password":"nope"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d, want 401", rec.Code)
	}
}

func TestHandlerEmptyBodyReturns400(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(t, h, "/api/v1/login", `{"username":"","password":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body status = %d, want 400", rec.Code)
	}
}

func TestHandlerLogoutRevokesSession(t *testing.T) {
	h, code := newTestHandler(t)
	_ = post(t, h, "/api/v1/register", registerJSON("dengchu", code))
	loginRec := post(t, h, "/api/v1/login", `{"username":"dengchu","password":"pw123456"}`)

	var token string
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == sessionCookie {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("expected session cookie from login")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want 200", rec.Code)
	}
	if _, ok := h.svc.Sessions().UserID(token); ok {
		t.Fatal("session should be revoked after logout")
	}
}

// mountFull builds a chi router mirroring the production wiring so the protected
// (/me/profile, /me/avatar) and admin (/admin/invite-code*) routes are reachable
// with their middleware. It returns the router and the handler's service.
func mountFull(t *testing.T, h *Handler) http.Handler {
	t.Helper()
	r := chi.NewRouter()
	r.Route("/api/v1", func(v1 chi.Router) {
		h.Mount(v1)
		v1.Group(func(pr chi.Router) {
			pr.Use(RequireUser(h.svc.Sessions()))
			h.MountProtected(pr)
		})
		v1.Group(func(ar chi.Router) {
			ar.Use(RequireAdmin(h.svc.Sessions(), h.svc.Repo()))
			h.MountAdmin(ar)
		})
	})
	return r
}

func TestUpdateProfileChangesNickname(t *testing.T) {
	h, code := newTestHandler(t)
	r := mountFull(t, h)
	token := registerAndLogin(t, r, "profileok", code)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/me/profile",
		strings.NewReader(`{"nickname":"新昵称","age":8,"gender":"女"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := doWithCookie(t, r, req, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("update profile status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var u models.User
	if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if u.Nickname != "新昵称" || u.Age != 8 || u.Gender != "女" {
		t.Fatalf("profile not updated: %+v", u)
	}
	if u.PasswordHash != "" {
		t.Fatal("profile response leaked password hash")
	}
}

func TestUpdateProfileBadGenderReturns400(t *testing.T) {
	h, code := newTestHandler(t)
	r := mountFull(t, h)
	token := registerAndLogin(t, r, "badgender", code)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/me/profile",
		strings.NewReader(`{"nickname":"x","age":8,"gender":"火星人"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := doWithCookie(t, r, req, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad gender status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

// avatarRequest builds a multipart POST /me/avatar carrying one "file" field.
func avatarRequest(t *testing.T, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/avatar", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestUploadAvatarPNGSetsURL(t *testing.T) {
	h, code := newTestHandler(t)
	r := mountFull(t, h)
	token := registerAndLogin(t, r, "avatarok", code)

	rec := doWithCookie(t, r, avatarRequest(t, "me.png", pngBytes), token)
	if rec.Code != http.StatusOK {
		t.Fatalf("avatar upload status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var u models.User
	if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(u.AvatarURL, "/media/users/") || !strings.HasSuffix(u.AvatarURL, ".png") {
		t.Fatalf("avatarUrl not set correctly: %q", u.AvatarURL)
	}
}

func TestUploadAvatarRejectsNonImage(t *testing.T) {
	h, code := newTestHandler(t)
	r := mountFull(t, h)
	token := registerAndLogin(t, r, "avatarbad", code)

	rec := doWithCookie(t, r, avatarRequest(t, "note.txt", []byte("just plain text, not an image")), token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("txt avatar status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestUploadAvatarTooLargeReturnsOversizedMessage(t *testing.T) {
	h, code := newTestHandler(t)
	r := mountFull(t, h)
	token := registerAndLogin(t, r, "avatarbig", code)

	// A valid PNG header padded just past the 2 MiB cap so MaxBytesReader trips
	// during ParseMultipartForm (the multipart envelope pushes it over as well).
	oversized := make([]byte, maxAvatarBytes+1)
	copy(oversized, pngBytes)
	rec := doWithCookie(t, r, avatarRequest(t, "big.png", oversized), token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversized avatar status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "头像太大啦") {
		t.Fatalf("oversized avatar should use the dedicated too-large message; body=%s", rec.Body.String())
	}
}

// makeAdmin creates a role="admin" account directly in the repo (there is no
// self-service admin endpoint) and issues a session for it, returning the token.
func makeAdmin(t *testing.T, h *Handler, username string) string {
	t.Helper()
	admin, err := h.svc.Repo().Create(NewUser{
		Username:     username,
		Email:        username + "@example.com",
		PasswordHash: "x",
		Nickname:     username,
		Role:         "admin",
		Credits:      0,
	})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	return h.svc.Sessions().Issue(admin.ID)
}

func TestAdminInviteCodeForbiddenForRegularUser(t *testing.T) {
	h, code := newTestHandler(t)
	r := mountFull(t, h)
	token := registerAndLogin(t, r, "regular", code)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/invite-code", nil)
	rec := doWithCookie(t, r, req, token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("regular user admin invite status = %d, want 403", rec.Code)
	}
}

func TestAdminInviteCodeAndRotate(t *testing.T) {
	h, code := newTestHandler(t)
	r := mountFull(t, h)
	token := makeAdmin(t, h, "boss")

	// GET returns the current code.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/invite-code", nil)
	rec := doWithCookie(t, r, req, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin GET invite status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["inviteCode"] != code {
		t.Fatalf("invite code = %q, want %q", got["inviteCode"], code)
	}

	// Rotate changes the code.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/invite-code/rotate", nil)
	rec = doWithCookie(t, r, req, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin rotate status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var rotated map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &rotated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rotated["inviteCode"] == "" || rotated["inviteCode"] == code {
		t.Fatalf("rotate did not change code: old=%q new=%q", code, rotated["inviteCode"])
	}
}

func TestSessionIssueUniqueTokens(t *testing.T) {
	s := NewSession(nil)
	seen := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		tok := s.Issue(int64(i))
		if tok == "" {
			t.Fatal("empty token")
		}
		if _, dup := seen[tok]; dup {
			t.Fatalf("duplicate token issued: %s", tok)
		}
		seen[tok] = struct{}{}
	}
}
