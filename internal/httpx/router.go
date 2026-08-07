package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/book"
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
	// Media serves stored assets under /media/*.
	Media http.Handler
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
	})

	if deps.Media != nil {
		r.Handle("/media/*", deps.Media)
	}

	return r
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
