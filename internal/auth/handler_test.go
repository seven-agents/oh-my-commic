package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	return NewHandler(newTestService(t))
}

func post(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandlerRegisterThenLoginSetsCookie(t *testing.T) {
	h := newTestHandler(t)

	rec := post(t, h, "/api/v1/register", `{"nickname":"小刚","password":"pw123456"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "password") || strings.Contains(rec.Body.String(), "hash") {
		t.Fatalf("register response leaked password field: %s", rec.Body.String())
	}

	rec = post(t, h, "/api/v1/login", `{"nickname":"小刚","password":"pw123456"}`)
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

func TestHandlerDuplicateNicknameReturns409(t *testing.T) {
	h := newTestHandler(t)
	_ = post(t, h, "/api/v1/register", `{"nickname":"重复","password":"pw123456"}`)
	rec := post(t, h, "/api/v1/register", `{"nickname":"重复","password":"other"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate register status = %d, want 409", rec.Code)
	}
}

func TestHandlerBadCredentialsReturns401(t *testing.T) {
	h := newTestHandler(t)
	_ = post(t, h, "/api/v1/register", `{"nickname":"张三","password":"pw123456"}`)
	rec := post(t, h, "/api/v1/login", `{"nickname":"张三","password":"nope"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d, want 401", rec.Code)
	}
}

func TestHandlerEmptyBodyReturns400(t *testing.T) {
	h := newTestHandler(t)
	rec := post(t, h, "/api/v1/register", `{"nickname":"","password":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body status = %d, want 400", rec.Code)
	}
}

func TestHandlerLogoutRevokesSession(t *testing.T) {
	h := newTestHandler(t)
	_ = post(t, h, "/api/v1/register", `{"nickname":"登出","password":"pw123456"}`)
	loginRec := post(t, h, "/api/v1/login", `{"nickname":"登出","password":"pw123456"}`)

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
