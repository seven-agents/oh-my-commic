package panel

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
	"github.com/seven-agents/oh-my-commic/internal/chapter"
	"github.com/seven-agents/oh-my-commic/internal/db"
	"github.com/seven-agents/oh-my-commic/internal/models"
)

// handlerTestEnv bundles the panel Handler under test with helpers for seeding
// the ownership chain and the router that mounts the panel routes.
type handlerTestEnv struct {
	books    *book.Repo
	chapters *chapter.Service
	router   chi.Router
}

// newHandlerTestEnv opens an in-memory DB, seeds two users, and wires a full
// panel Handler onto a chi router. The routes are mounted WITHOUT RequireUser
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
	chapterSvc := chapter.NewService(chapter.NewRepo(d), books)
	h := NewHandler(NewService(NewRepo(d), chapterSvc))

	r := chi.NewRouter()
	h.Mount(r)
	return &handlerTestEnv{books: books, chapters: chapterSvc, router: r}
}

// newChapter seeds a book owned by userID plus a chapter under it.
func (e *handlerTestEnv) newChapter(t *testing.T, userID int64) models.Chapter {
	t.Helper()
	b, err := e.books.Create(userID, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	c, err := e.chapters.CreateChapter(userID, b.ID, "章")
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}
	return c
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

// TestHTTPReplaceListUpdate exercises the bulk replace, list, and update
// endpoints end to end, including order reassignment and CharacterIDs round-trip.
func TestHTTPReplaceListUpdate(t *testing.T) {
	env := newHandlerTestEnv(t)
	ch := env.newChapter(t, 1)
	base := "/api/chapters/" + strconv.FormatInt(ch.ID, 10) + "/panels"

	rec := env.do(1, http.MethodPut, base, []models.Panel{
		{Caption: "A", CharacterIDs: []int64{2, 3}},
		{Caption: "B"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("replace expected 200, got %d", rec.Code)
	}
	var out []models.Panel
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode replace: %v", err)
	}
	if len(out) != 2 || out[0].Order != 0 || out[1].Order != 1 {
		t.Fatalf("order 未重排: %+v", out)
	}

	rec = env.do(1, http.MethodGet, base, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d", rec.Code)
	}
	var list []models.Panel
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 2 || len(list[0].CharacterIDs) != 2 || list[0].CharacterIDs[0] != 2 {
		t.Fatalf("CharacterIDs JSON 往返错: %+v", list)
	}

	rec = env.do(1, http.MethodPut, "/api/panels/"+strconv.FormatInt(list[0].ID, 10), models.Panel{Caption: "edited", SceneID: 9})
	if rec.Code != http.StatusOK {
		t.Fatalf("update expected 200, got %d", rec.Code)
	}
	var updated models.Panel
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update: %v", err)
	}
	if updated.Caption != "edited" || updated.SceneID != 9 {
		t.Fatalf("update not persisted: %+v", updated)
	}
}

// TestHTTPCrossUser404 verifies cross-user access is reported as 404 across all
// endpoints, never leaking existence.
func TestHTTPCrossUser404(t *testing.T) {
	env := newHandlerTestEnv(t)
	ch := env.newChapter(t, 1)
	base := "/api/chapters/" + strconv.FormatInt(ch.ID, 10) + "/panels"
	create := env.do(1, http.MethodPut, base, []models.Panel{{Caption: "p"}})
	var out []models.Panel
	if err := json.Unmarshal(create.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	panelID := strconv.FormatInt(out[0].ID, 10)

	cases := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, base, nil},
		{http.MethodPut, base, []models.Panel{{Caption: "越权"}}},
		{http.MethodPut, "/api/panels/" + panelID, models.Panel{Caption: "越权"}},
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

	if rec := env.do(1, http.MethodGet, "/api/chapters/abc/panels", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad chapter id expected 400, got %d", rec.Code)
	}
	if rec := env.do(1, http.MethodPut, "/api/panels/abc", models.Panel{}); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad panel id expected 400, got %d", rec.Code)
	}

	ch := env.newChapter(t, 1)
	req := httptest.NewRequest(http.MethodPut, "/api/chapters/"+strconv.FormatInt(ch.ID, 10)+"/panels", bytes.NewBufferString("{bad json"))
	req = req.WithContext(auth.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad json expected 400, got %d", rec.Code)
	}
}
