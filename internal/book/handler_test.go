package book

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/models"
)

// newTestHandler builds a Handler over an in-memory DB seeded with two users.
func newTestHandler(t *testing.T) (*Handler, *Repo) {
	t.Helper()
	repo := newTestBookRepo(t)
	return NewHandler(NewService(repo)), repo
}

// asUser wraps a request so the handler sees the given authenticated user, the
// same way auth.RequireUser would inject it upstream.
func asUser(r *http.Request, userID int64) *http.Request {
	ctx := auth.WithUserID(r.Context(), userID)
	return r.WithContext(ctx)
}

func do(t *testing.T, h *Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	return rec
}

func TestHandlerCreateAndList(t *testing.T) {
	h, _ := newTestHandler(t)

	body := strings.NewReader(`{"title":"我的书","style":"","summary":"简介"}`)
	req := asUser(httptest.NewRequest(http.MethodPost, "/api/v1/books", body), 1)
	rec := do(t, h, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", rec.Code, rec.Body)
	}
	var created models.Book
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.Style != "ghibli" || created.UserID != 1 {
		t.Fatalf("unexpected created book: %+v", created)
	}

	req = asUser(httptest.NewRequest(http.MethodGet, "/api/v1/books", nil), 1)
	rec = do(t, h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", rec.Code)
	}
	var list []models.Book
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 book, got %d", len(list))
	}
}

func TestHandlerCreateEmptyTitle400(t *testing.T) {
	h, _ := newTestHandler(t)

	body := strings.NewReader(`{"title":"   ","style":"ghibli","summary":""}`)
	req := asUser(httptest.NewRequest(http.MethodPost, "/api/v1/books", body), 1)
	rec := do(t, h, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body)
	}
}

func TestHandlerCrossUserGet404(t *testing.T) {
	h, repo := newTestHandler(t)
	b, _ := repo.Create(1, "user1 的书", "ghibli", "")

	// User 2 requesting user 1's book must get 404, not the book.
	url := "/api/v1/books/" + strconv.FormatInt(b.ID, 10)
	req := asUser(httptest.NewRequest(http.MethodGet, url, nil), 2)
	rec := do(t, h, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-user get status = %d, want 404: %s", rec.Code, rec.Body)
	}

	// Owner gets 200.
	req = asUser(httptest.NewRequest(http.MethodGet, url, nil), 1)
	rec = do(t, h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner get status = %d, want 200", rec.Code)
	}
}

func TestHandlerCrossUserDelete404(t *testing.T) {
	h, repo := newTestHandler(t)
	b, _ := repo.Create(1, "user1 的书", "ghibli", "")

	url := "/api/v1/books/" + strconv.FormatInt(b.ID, 10)
	req := asUser(httptest.NewRequest(http.MethodDelete, url, nil), 2)
	rec := do(t, h, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-user delete status = %d, want 404", rec.Code)
	}

	// The book must still exist for its owner.
	if _, err := repo.Get(1, b.ID); err != nil {
		t.Fatalf("cross-user delete removed the book: %v", err)
	}
}

func TestHandlerUpdateRoundTrip(t *testing.T) {
	h, repo := newTestHandler(t)
	b, _ := repo.Create(1, "旧", "ghibli", "")

	url := "/api/v1/books/" + strconv.FormatInt(b.ID, 10)
	body := strings.NewReader(`{"title":"新","style":"manga","summary":"x"}`)
	req := asUser(httptest.NewRequest(http.MethodPut, url, body), 1)
	rec := do(t, h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200: %s", rec.Code, rec.Body)
	}
	got, _ := repo.Get(1, b.ID)
	if got.Title != "新" || got.Style != "manga" {
		t.Fatalf("update not persisted: %+v", got)
	}
}

func TestHandlerInvalidID400(t *testing.T) {
	h, _ := newTestHandler(t)

	req := asUser(httptest.NewRequest(http.MethodGet, "/api/v1/books/abc", nil), 1)
	rec := do(t, h, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// Ensure the isolation guarantee also holds when the user context is absent
// (UserID == 0): no book should be returned.
func TestHandlerNoUserContext(t *testing.T) {
	h, repo := newTestHandler(t)
	b, _ := repo.Create(1, "私有", "ghibli", "")

	url := "/api/v1/books/" + strconv.FormatInt(b.ID, 10)
	// No asUser wrapper: context carries user id 0.
	req := httptest.NewRequest(http.MethodGet, url, nil).WithContext(context.Background())
	rec := do(t, h, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("anonymous get status = %d, want 404", rec.Code)
	}
	_ = b
}
