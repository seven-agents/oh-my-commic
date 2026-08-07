package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireUser(t *testing.T) {
	sess := NewSession()
	tok := sess.Issue(42)
	var seen int64
	h := RequireUser(sess)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = UserID(r.Context())
		w.WriteHeader(200)
	}))
	// 无 cookie
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 401 {
		t.Fatalf("无 cookie 应 401, got %d", rec.Code)
	}
	// 有 cookie
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: tok})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || seen != 42 {
		t.Fatalf("应注入 userID=42, got %d code=%d", seen, rec.Code)
	}
}

func TestRequireUserInvalidToken(t *testing.T) {
	sess := NewSession()
	called := false
	h := RequireUser(sess)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "not-a-real-token"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("无效 token 应 401, got %d", rec.Code)
	}
	if called {
		t.Fatal("无效 token 时不应调用 next handler")
	}
}

func TestUserIDAbsent(t *testing.T) {
	if got := UserID(context.Background()); got != 0 {
		t.Fatalf("无 userID 时应返回 0, got %d", got)
	}
}
