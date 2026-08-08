package community

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/db"
)

// mount builds a router: public group with a middleware that injects userID when
// the test asks (uid!=0), plus an authed group that always injects uid.
func mountRouter(h *Handler, uid int64) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", func(v1 chi.Router) {
		v1.Group(func(pub chi.Router) {
			pub.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if uid != 0 {
						req = req.WithContext(auth.WithUserID(req.Context(), uid))
					}
					next.ServeHTTP(w, req)
				})
			})
			h.MountPublic(pub)
		})
		v1.Group(func(pr chi.Router) {
			pr.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					next.ServeHTTP(w, req.WithContext(auth.WithUserID(req.Context(), uid)))
				})
			})
			h.MountAuthed(pr)
		})
	})
	return r
}

func TestAnonymousCanListAndPrivateDetail404(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, nickname, password_hash) VALUES (1,'小明','h')`)
	d.Exec(`INSERT INTO books (id,user_id,title,is_public,published_at) VALUES (10,1,'公开',1,'t'),(12,1,'私密',0,'')`)
	h := NewHandler(NewService(NewRepo(d)))
	srv := mountRouter(h, 0) // 匿名

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/community/books", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "公开") || strings.Contains(rec.Body.String(), "私密") {
		t.Fatalf("feed wrong: code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/community/books/12", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("private detail should 404, got %d", rec.Code)
	}
}

func TestLikeFlow(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, nickname, password_hash) VALUES (1,'小明','h')`)
	d.Exec(`INSERT INTO books (id,user_id,title,is_public,published_at) VALUES (10,1,'公开',1,'t')`)
	h := NewHandler(NewService(NewRepo(d)))
	srv := mountRouter(h, 1)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/community/books/10/like", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"likeCount":1`) {
		t.Fatalf("like: code=%d body=%s", rec.Code, rec.Body.String())
	}
}
