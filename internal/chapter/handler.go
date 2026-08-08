package chapter

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/auth"
)

// Handler serves the chapter HTTP endpoints on top of a Service. It resolves the
// authenticated user from the request context (populated upstream by
// auth.RequireUser) and never trusts a user ID from the request.
type Handler struct {
	svc *Service
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers the chapter endpoints onto r using their absolute paths.
//
//	GET  /api/v1/books/{bookId}/chapters
//	POST /api/v1/books/{bookId}/chapters
//	POST /api/v1/books/{bookId}/cover-chapter
//	GET    /api/v1/chapters/{id}
//	PUT    /api/v1/chapters/{id}/status
//	DELETE /api/v1/chapters/{id}
//
// It deliberately does NOT attach auth.RequireUser: the caller mounts these
// routes inside a group already wrapped with RequireUser, so every handler can
// rely on a valid user ID being present in the context.
func (h *Handler) Mount(r chi.Router) {
	r.Get("/books/{bookId}/chapters", h.List)
	r.Post("/books/{bookId}/chapters", h.Create)
	r.Post("/books/{bookId}/cover-chapter", h.EnsureCover)
	r.Get("/chapters/{id}", h.Get)
	r.Put("/chapters/{id}/status", h.SetStatus)
	r.Delete("/chapters/{id}", h.Delete)
}

// titleRequest is the body of a create-chapter request.
type titleRequest struct {
	Title string `json:"title"`
}

// statusRequest is the body of a set-status request.
type statusRequest struct {
	Status string `json:"status"`
}

// List handles GET /api/v1/books/{bookId}/chapters.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	bookID, ok := parseBookID(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListChapters(userID, bookID)
	if err != nil {
		writeChapterError(w, err, "获取章节失败")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// Create handles POST /api/v1/books/{bookId}/chapters.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	bookID, ok := parseBookID(w, r)
	if !ok {
		return
	}
	var req titleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	created, err := h.svc.CreateChapter(userID, bookID, req.Title)
	if err != nil {
		writeChapterError(w, err, "创建章节失败")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// EnsureCover handles POST /api/v1/books/{bookId}/cover-chapter. It returns the
// book's single cover chapter, creating it on first call. The response is 200
// whether the cover chapter was found or freshly created.
func (h *Handler) EnsureCover(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	bookID, ok := parseBookID(w, r)
	if !ok {
		return
	}
	cover, err := h.svc.EnsureCover(userID, bookID)
	if err != nil {
		writeChapterError(w, err, "创建封面章失败")
		return
	}
	writeJSON(w, http.StatusOK, cover)
}

// Get handles GET /api/v1/chapters/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	id, ok := parseChapterID(w, r)
	if !ok {
		return
	}
	c, err := h.svc.GetChapter(userID, id)
	if err != nil {
		writeChapterError(w, err, "获取章节失败")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// SetStatus handles PUT /api/v1/chapters/{id}/status.
func (h *Handler) SetStatus(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	id, ok := parseChapterID(w, r)
	if !ok {
		return
	}
	var req statusRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	updated, err := h.svc.SetStatus(userID, id, req.Status)
	if err != nil {
		writeChapterError(w, err, "更新章节状态失败")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /api/v1/chapters/{id}. Deleting a chapter cascades to its
// panels at the database level. It returns 200 on success and 404 for a
// cross-user or unknown chapter.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	id, ok := parseChapterID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(userID, id); err != nil {
		writeChapterError(w, err, "删除章节失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// parseBookID reads the {bookId} path parameter. On a missing or non-positive id
// it writes a 400 response and returns ok=false.
func parseBookID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "bookId"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效的书籍 ID")
		return 0, false
	}
	return id, true
}

// parseChapterID reads the {id} path parameter. On a missing or non-positive id
// it writes a 400 response and returns ok=false.
func parseChapterID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效的章节 ID")
		return 0, false
	}
	return id, true
}

// decodeJSON parses the JSON request body into dst. On malformed input it writes
// a 400 response and returns ok=false.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return false
	}
	return true
}

// writeChapterError maps a service error to an HTTP response: ErrNotFound becomes
// 404, ErrInvalidStatus becomes 400, and anything else becomes a 500 with the
// given generic fallback message so internal detail is never leaked.
func writeChapterError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "资源不存在")
	case errors.Is(err, ErrInvalidStatus):
		writeError(w, http.StatusBadRequest, "非法的状态流转")
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
