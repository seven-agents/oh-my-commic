package panel

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/models"
)

// Handler serves the panel HTTP endpoints on top of a Service. It resolves the
// authenticated user from the request context (populated upstream by
// auth.RequireUser) and never trusts a user ID from the request.
type Handler struct {
	svc *Service
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers the panel endpoints onto r using their absolute paths.
//
//	GET /api/v1/chapters/{id}/panels
//	PUT /api/v1/chapters/{id}/panels   (bulk replace)
//	PUT /api/v1/panels/{id}
//
// It deliberately does NOT attach auth.RequireUser: the caller mounts these
// routes inside a group already wrapped with RequireUser, so every handler can
// rely on a valid user ID being present in the context.
func (h *Handler) Mount(r chi.Router) {
	r.Get("/chapters/{id}/panels", h.List)
	r.Put("/chapters/{id}/panels", h.Replace)
	r.Put("/panels/{id}", h.Update)
}

// List handles GET /api/v1/chapters/{id}/panels.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	chapterID, ok := parseID(w, r, "id", "无效的章节 ID")
	if !ok {
		return
	}
	list, err := h.svc.ListPanels(userID, chapterID)
	if err != nil {
		writePanelError(w, err, "获取分镜失败")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// Replace handles PUT /api/v1/chapters/{id}/panels. The body is a JSON array of
// panels; their order is reassigned to 0..n-1 on persistence.
func (h *Handler) Replace(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	chapterID, ok := parseID(w, r, "id", "无效的章节 ID")
	if !ok {
		return
	}
	var panels []models.Panel
	if !decodeJSON(w, r, &panels) {
		return
	}
	out, err := h.svc.ReplacePanels(userID, chapterID, panels)
	if err != nil {
		writePanelError(w, err, "保存分镜失败")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// Update handles PUT /api/v1/panels/{id}. The body carries the editable fields.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	panelID, ok := parseID(w, r, "id", "无效的分镜 ID")
	if !ok {
		return
	}
	var p models.Panel
	if !decodeJSON(w, r, &p) {
		return
	}
	updated, err := h.svc.UpdatePanel(userID, panelID, p)
	if err != nil {
		writePanelError(w, err, "更新分镜失败")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// parseID reads the given positive integer path parameter. On a missing or
// non-positive value it writes a 400 response and returns ok=false.
func parseID(w http.ResponseWriter, r *http.Request, name, msg string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, msg)
		return 0, false
	}
	return id, true
}

// decodeJSON parses the JSON request body into dst. On malformed input it writes
// a 400 response and returns ok=false.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return false
	}
	return true
}

// writePanelError maps a service error to an HTTP response: ErrNotFound becomes
// 404 and anything else becomes a 500 with the given generic fallback message so
// internal detail is never leaked.
func writePanelError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "资源不存在")
	default:
		writeError(w, http.StatusInternalServerError, fallback)
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
