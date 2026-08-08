package httpx

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/asset"
	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/chapter"
	"github.com/seven-agents/oh-my-commic/internal/panel"
	"github.com/seven-agents/oh-my-commic/internal/render"
	"github.com/seven-agents/oh-my-commic/internal/story"
)

// Deps aggregates the dependencies NewRouter needs to compose the HTTP surface.
// The caller (cmd/server) constructs the concrete handlers, session store, and
// media file server, then hands them here so routing stays free of wiring
// concerns.
type Deps struct {
	// Session backs the RequireUser middleware protecting book routes.
	Session *auth.Session
	// Auth mounts the public authentication routes.
	Auth *auth.Handler
	// Book mounts the per-user book routes (behind RequireUser).
	Book *book.Handler
	// Asset mounts the per-book asset routes (upload + character/scene CRUD,
	// behind RequireUser). Optional: nil disables asset routes.
	Asset *asset.Handler
	// Chapter mounts the per-book chapter routes (CRUD + status state machine,
	// behind RequireUser). Optional: nil disables chapter routes.
	Chapter *chapter.Handler
	// Panel mounts the per-chapter panel routes (list + bulk replace + edit,
	// behind RequireUser). Optional: nil disables panel routes.
	Panel *panel.Handler
	// Story mounts the per-chapter conversational storyboard route
	// (storyboard-chat, behind RequireUser). Optional: nil disables story routes.
	Story *story.Handler
	// Render mounts the per-panel AI image render route (behind RequireUser).
	// Optional: nil disables the render route.
	Render *render.Handler
	// Media serves stored assets under /media/*.
	Media http.Handler
	// Static serves the built single-page frontend for any non-API, non-media
	// path, falling back to index.html for client-side routes. Optional: nil
	// disables SPA serving entirely (the server then behaves as an API-only
	// backend, as in tests). Build one with NewSPAHandler.
	Static http.Handler
}

// NewRouter builds the application router from deps.
//
// All business endpoints live under the versioned /api/v1 prefix (mounted via a
// single chi.Route group so a future /api/v2 can be added alongside without
// touching the individual handlers). The health probe is intentionally left
// unversioned so ops tooling (docker-compose, CI) can rely on a stable path.
//
// Route groups:
//   - GET /api/health                                   public (unversioned probe)
//   - /api/v1/register, /api/v1/login, /api/v1/logout   public (auth)
//   - /api/v1/me, /api/v1/books*, ...                   protected by RequireUser
//   - /media/*                                          static asset serving
func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()

	// Log every request first so 404s and errors from any downstream handler
	// (including the SPA/static catch-all) are visible in the server log.
	r.Use(requestLogger)

	// Unversioned health probe (docker-compose + CI depend on this stable path).
	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	// All business endpoints are versioned under /api/v1. Handlers mount using
	// resource-relative paths (e.g. "/books"), so the effective path becomes
	// "/api/v1/books".
	r.Route("/api/v1", func(v1 chi.Router) {
		// Public auth routes.
		deps.Auth.Mount(v1)

		// Protected routes: everything in this group requires a valid session.
		v1.Group(func(pr chi.Router) {
			pr.Use(auth.RequireUser(deps.Session))
			deps.Auth.MountProtected(pr)
			deps.Book.Mount(pr)
			if deps.Asset != nil {
				deps.Asset.Mount(pr)
			}
			if deps.Chapter != nil {
				deps.Chapter.Mount(pr)
			}
			if deps.Panel != nil {
				deps.Panel.Mount(pr)
			}
			if deps.Story != nil {
				deps.Story.Mount(pr)
			}
			if deps.Render != nil {
				deps.Render.Mount(pr)
			}
		})
	})

	if deps.Media != nil {
		r.Handle("/media/*", deps.Media)
	}

	// SPA serving. When a static handler is configured, mount it as the
	// catch-all for everything that did not match an /api/* or /media/* route
	// above. Unknown /api/* paths are explicitly short-circuited to a JSON 404
	// so client-side routing never masks a real API mistake with index.html.
	if deps.Static != nil {
		r.NotFound(func(w http.ResponseWriter, req *http.Request) {
			if strings.HasPrefix(req.URL.Path, "/api/") {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			deps.Static.ServeHTTP(w, req)
		})
	}

	return r
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
