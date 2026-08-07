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
	// Story mounts the per-chapter AI storyboard routes (converse + generate,
	// behind RequireUser). Optional: nil disables story routes.
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
// Route groups:
//   - GET /api/health                          public
//   - /api/register, /api/login, /api/logout   public (auth)
//   - /api/books*                              protected by RequireUser
//   - /media/*                                 static asset serving
func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()

	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	// Public auth routes.
	deps.Auth.Mount(r)

	// Protected book routes: everything in this group requires a valid session.
	r.Group(func(pr chi.Router) {
		pr.Use(auth.RequireUser(deps.Session))
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
