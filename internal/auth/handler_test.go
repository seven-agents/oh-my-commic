package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestHandler builds a Handler over a fresh service and returns it together
// with the seeded invite code (required by the register endpoint).
func newTestHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	svc, code := newTestService(t, 100)
	return NewHandler(svc), code
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

func TestHandlerBadInviteReturns403(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := post(t, h, "/api/v1/register", registerJSON("badinvite", "wrong-code"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("bad invite status = %d, want 403; body=%s", rec.Code, rec.Body.String())
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
