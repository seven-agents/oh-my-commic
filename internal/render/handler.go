package render

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/auth"
)

// Handler serves the single-panel render endpoint on top of a Service. It
// resolves the authenticated user from the request context (populated upstream
// by auth.RequireUser) and never trusts a client-supplied user ID.
type Handler struct {
	svc *Service
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers the render endpoint onto r using its absolute path.
//
//	POST /api/panels/{id}/render
//
// Like the sibling handlers, it does not attach auth.RequireUser: the caller
// mounts this inside a group already wrapped with RequireUser.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/api/panels/{id}/render", h.Render)
}

// Render handles POST /api/panels/{id}/render. It renders the panel synchronously
// and returns the updated panel JSON. ErrNotFound maps to 404; any generation or
// download failure maps to a generic 502 so upstream/API detail never leaks.
func (h *Handler) Render(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	panelID, ok := parseID(w, r)
	if !ok {
		return
	}

	updated, err := h.svc.RenderPanel(r.Context(), userID, panelID)
	if err != nil {
		writeRenderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// parseID reads the positive integer panel id path parameter. On a missing or
// non-positive value it writes a 400 response and returns ok=false.
func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效的分镜 ID")
		return 0, false
	}
	return id, true
}

// writeRenderError maps a service error to an HTTP response: ErrNotFound becomes
// 404, and any other error (generation or download failure) becomes a 502 with a
// generic message so the API key and raw upstream body are never leaked.
func writeRenderError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "资源不存在")
	default:
		writeError(w, http.StatusBadGateway, "生成图片失败，请稍后再试")
	}
}

// writeJSON encodes v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error envelope. The message must never contain
// sensitive internal detail.
func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
