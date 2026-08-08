package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/seven-agents/oh-my-commic/internal/db"
)

func TestRequireUser(t *testing.T) {
	sess := NewSession(nil)
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

func TestOptionalUser(t *testing.T) {
	sess := NewSession(nil)
	token := sess.Issue(7)

	var seen int64
	h := OptionalUser(sess)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = UserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// 无 cookie：放行，userID=0。
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusOK || seen != 0 {
		t.Fatalf("anon: code=%d seen=%d", rec.Code, seen)
	}

	// 有效 cookie：放行，userID=7。
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || seen != 7 {
		t.Fatalf("authed: code=%d seen=%d", rec.Code, seen)
	}

	// 无效 token：放行，userID=0（不 401）。
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "bogus-token"})
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || seen != 0 {
		t.Fatalf("invalid token: code=%d seen=%d", rec.Code, seen)
	}
}

func TestRequireUserInvalidToken(t *testing.T) {
	sess := NewSession(nil)
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

// adminUser / regularUser build NewUser rows for RequireAdmin tests.
func makeUser(username, role string) NewUser {
	return NewUser{
		Username:     username,
		Email:        username + "@example.com",
		PasswordHash: "hash",
		Nickname:     username,
		Role:         role,
		Credits:      0,
	}
}

func TestRequireAdmin(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	repo := NewUserRepo(d)
	admin, err := repo.Create(makeUser("boss", "admin"))
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	user, err := repo.Create(makeUser("kid", "user"))
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	sess := NewSession(nil)
	adminTok := sess.Issue(admin.ID)
	userTok := sess.Issue(user.ID)

	r := chi.NewRouter()
	r.With(RequireAdmin(sess, repo)).Get("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})

	do := func(tok string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/", nil)
		if tok != "" {
			req.AddCookie(&http.Cookie{Name: "session", Value: tok})
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	if rec := do(adminTok); rec.Code != 200 {
		t.Fatalf("admin 应 200, got %d", rec.Code)
	}
	if rec := do(userTok); rec.Code != 403 {
		t.Fatalf("普通用户应 403, got %d", rec.Code)
	} else if body := rec.Body.String(); !strings.Contains(body, "需要管理员权限") {
		t.Fatalf("403 body 应含中文提示, got %q", body)
	}
	if rec := do(""); rec.Code != 401 {
		t.Fatalf("无 session 应 401, got %d", rec.Code)
	}
}
