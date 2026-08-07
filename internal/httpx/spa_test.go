package httpx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/db"
)

// newSPATestRouter wires the real auth/book handlers plus a SPA handler rooted
// at a temp dist directory, so the SPA fallback and API routing can be tested
// together.
func newSPATestRouter(t *testing.T, dist string) http.Handler {
	t.Helper()

	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	sess := auth.NewSession(nil)
	authHandler := auth.NewHandler(auth.NewService(auth.NewUserRepo(d), sess))
	bookHandler := book.NewHandler(book.NewService(book.NewRepo(d)))

	spa, err := NewSPAHandler(dist)
	if err != nil {
		t.Fatalf("new spa handler: %v", err)
	}

	return NewRouter(Deps{
		Session: sess,
		Auth:    authHandler,
		Book:    bookHandler,
		Static:  spa,
	})
}

// writeDist creates a minimal built-frontend directory: an index.html shell and
// one JS asset under assets/.
func writeDist(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	const indexHTML = "<!doctype html><title>oh-my-commic</title><div id=root></div>"
	if err := os.WriteFile(filepath.Join(dir, indexFile), []byte(indexHTML), 0o644); err != nil {
		t.Fatal(err)
	}
	assets := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "x.js"), []byte("export const x = 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNewSPAHandlerMissingIndex(t *testing.T) {
	if _, err := NewSPAHandler(t.TempDir()); err == nil {
		t.Fatal("want error for dist without index.html, got nil")
	}
}

func TestSPARoutes(t *testing.T) {
	dist := writeDist(t)
	srv := httptest.NewServer(newSPATestRouter(t, dist))
	defer srv.Close()

	get := func(path string) (*http.Response, string) {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp, string(b)
	}

	// Root serves index.html.
	resp, body := get("/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /: want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "oh-my-commic") {
		t.Fatalf("GET /: want index.html shell, got %q", body)
	}

	// Unknown client-side route falls back to index.html.
	resp, body = get("/books/1")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /books/1: want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "oh-my-commic") {
		t.Fatalf("GET /books/1: want SPA fallback, got %q", body)
	}

	// A real asset is served with the right content type and body.
	resp, body = get("/assets/x.js")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /assets/x.js: want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "export const x") {
		t.Fatalf("GET /assets/x.js: want asset body, got %q", body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("GET /assets/x.js: want javascript content-type, got %q", ct)
	}

	// Unknown API path must NOT fall through to index.html; it stays a JSON 404.
	resp, body = get("/api/whatever")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /api/whatever: want 404, got %d", resp.StatusCode)
	}
	if strings.Contains(body, "<div id=root>") {
		t.Fatalf("GET /api/whatever: leaked index.html: %q", body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("GET /api/whatever: want json content-type, got %q", ct)
	}

	// Health still works alongside SPA serving.
	resp, body = get("/api/health")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"ok":true`) {
		t.Fatalf("GET /api/health: want 200 ok=true, got %d %q", resp.StatusCode, body)
	}
}
