package httpx

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// indexFile is the SPA entrypoint served for any client-side route that does
// not map to a real file on disk.
const indexFile = "index.html"

// NewSPAHandler builds an http.Handler that serves the built single-page
// frontend from dir. Requests that map to an existing file (for example
// /assets/app.js) are served directly with the correct content type; every
// other path falls back to dir/index.html so client-side routes like /books/1
// resolve to the SPA shell instead of a 404.
//
// It returns (nil, error) when dir/index.html is missing, so the caller can
// cleanly decide to run in API-only mode.
func NewSPAHandler(dir string) (http.Handler, error) {
	index := filepath.Join(dir, indexFile)
	if _, err := os.Stat(index); err != nil {
		return nil, err
	}

	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve real files (assets, favicon, etc.) directly. For anything that
		// is not a regular file — including the root path and unknown client
		// routes — serve the SPA shell so the frontend router can take over.
		if servable(dir, r.URL.Path) {
			// Vite emits hashed filenames under /assets/, so their content never
			// changes for a given URL — cache them for a year. index.html itself
			// must stay uncached (below) so a new build's asset hashes are picked
			// up immediately.
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fs.ServeHTTP(w, r)
			return
		}
		// The SPA shell must never be cached: it references the current build's
		// hashed assets, so a stale copy would load dead asset URLs after deploy.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	}), nil
}

// servable reports whether urlPath maps to a regular file inside dir. It cleans
// the path and confirms the resolved target stays within dir, guarding against
// traversal even though http.Dir already rejects escapes on serve.
func servable(dir, urlPath string) bool {
	if urlPath == "/" || urlPath == "" {
		return false
	}
	clean := filepath.Clean("/" + urlPath)
	target := filepath.Join(dir, clean)

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	if absTarget != absDir && !hasPathPrefix(absTarget, absDir) {
		return false
	}

	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		return false
	}
	return true
}

// hasPathPrefix reports whether path is prefix or a descendant of prefix,
// comparing on path-segment boundaries.
func hasPathPrefix(path, prefix string) bool {
	if !filepath.IsAbs(path) || !filepath.IsAbs(prefix) {
		return false
	}
	rel, err := filepath.Rel(prefix, path)
	if err != nil {
		return false
	}
	return rel != ".." && !hasDotDotPrefix(rel)
}

// hasDotDotPrefix reports whether rel begins with a ".." segment.
func hasDotDotPrefix(rel string) bool {
	return rel == ".." ||
		len(rel) >= 3 && rel[0] == '.' && rel[1] == '.' && (rel[2] == filepath.Separator)
}
