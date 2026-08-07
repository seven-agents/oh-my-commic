package chapter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/db"
	"github.com/seven-agents/oh-my-commic/internal/models"
)

// handlerTestEnv bundles the Handler under test with the book.Repo used to seed
// owned books and the router that mounts the chapter routes.
type handlerTestEnv struct {
	books  *book.Repo
	router chi.Router
}

// newHandlerTestEnv opens an in-memory DB, seeds two users, and wires a full
// chapter Handler onto a chi router. The routes are mounted WITHOUT RequireUser
// (matching production, where the group middleware injects the user); tests set
// the user id on the request context directly.
func newHandlerTestEnv(t *testing.T) *handlerTestEnv {
	t.Helper()

	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	seedUsers(t, d, 2)

	books := book.NewRepo(d)
	svc := NewService(NewRepo(d), books)
	h := NewHandler(svc)

	r := chi.NewRouter()
	h.Mount(r)
	return &handlerTestEnv{books: books, router: r}
}

// do issues a request as userID against the mounted router and returns the
// recorder.
func (e *handlerTestEnv) do(userID int64, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			panic(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req = req.WithContext(auth.WithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// TestHTTPCreateListGet exercises the create, list, and get endpoints end to end.
func TestHTTPCreateListGet(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, err := env.books.Create(1, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	base := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/chapters"

	rec := env.do(1, http.MethodPost, base, titleRequest{Title: "第一章"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create expected 201, got %d", rec.Code)
	}
	var created models.Chapter
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.Order != 1 || created.Status != "draft" {
		t.Fatalf("unexpected created chapter: %+v", created)
	}

	rec = env.do(1, http.MethodGet, base, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d", rec.Code)
	}
	var list []models.Chapter
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 chapter, got %d", len(list))
	}

	rec = env.do(1, http.MethodGet, "/api/chapters/"+strconv.FormatInt(created.ID, 10), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get expected 200, got %d", rec.Code)
	}
}

// TestHTTPSetStatusFlowAndIllegal covers a legal transition (200) and an illegal
// one (400).
func TestHTTPSetStatusFlowAndIllegal(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, err := env.books.Create(1, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	create := env.do(1, http.MethodPost, "/api/books/"+strconv.FormatInt(b.ID, 10)+"/chapters", titleRequest{Title: "章"})
	var c models.Chapter
	if err := json.Unmarshal(create.Body.Bytes(), &c); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	statusPath := "/api/chapters/" + strconv.FormatInt(c.ID, 10) + "/status"

	rec := env.do(1, http.MethodPut, statusPath, statusRequest{Status: "storyboarding"})
	if rec.Code != http.StatusOK {
		t.Fatalf("legal transition expected 200, got %d", rec.Code)
	}

	rec = env.do(1, http.MethodPut, statusPath, statusRequest{Status: "done"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("illegal transition expected 400, got %d", rec.Code)
	}
}

// TestHTTPCrossUser404 verifies cross-user access is reported as 404 across all
// endpoints, never leaking existence.
func TestHTTPCrossUser404(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, err := env.books.Create(1, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	base := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/chapters"
	create := env.do(1, http.MethodPost, base, titleRequest{Title: "章"})
	var c models.Chapter
	if err := json.Unmarshal(create.Body.Bytes(), &c); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	chID := strconv.FormatInt(c.ID, 10)

	cases := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, base, nil},
		{http.MethodPost, base, titleRequest{Title: "越权"}},
		{http.MethodGet, "/api/chapters/" + chID, nil},
		{http.MethodPut, "/api/chapters/" + chID + "/status", statusRequest{Status: "storyboarding"}},
	}
	for _, tc := range cases {
		rec := env.do(2, tc.method, tc.path, tc.body)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s cross-user expected 404, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

// TestHTTPBadInput covers 400 paths: malformed IDs and bad JSON.
func TestHTTPBadInput(t *testing.T) {
	env := newHandlerTestEnv(t)

	if rec := env.do(1, http.MethodGet, "/api/books/abc/chapters", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad bookId expected 400, got %d", rec.Code)
	}
	if rec := env.do(1, http.MethodGet, "/api/chapters/abc", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad chapter id expected 400, got %d", rec.Code)
	}

	b, err := env.books.Create(1, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/books/"+strconv.FormatInt(b.ID, 10)+"/chapters", bytes.NewBufferString("{bad json"))
	req = req.WithContext(auth.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad json expected 400, got %d", rec.Code)
	}
}
