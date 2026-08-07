package book

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/auth"
)

// bookInput is the JSON body accepted by create and update.
type bookInput struct {
	Title   string `json:"title"`
	Style   string `json:"style"`
	Summary string `json:"summary"`
}

// Handler serves the book HTTP endpoints on top of a Service. It resolves the
// authenticated user from the request context (populated upstream by
// auth.RequireUser) and never trusts a user ID from the request body.
type Handler struct {
	svc *Service
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Routes builds a chi subrouter exposing the five book endpoints:
//
//	GET    /api/books
//	POST   /api/books
//	GET    /api/books/{id}
//	PUT    /api/books/{id}
//	DELETE /api/books/{id}
//
// It deliberately does NOT attach auth.RequireUser: the caller mounts this
// subrouter under an /api group already wrapped with RequireUser, so every
// handler can rely on a valid user ID being present in the context.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	h.Mount(r)
	return r
}

// Mount registers the five book endpoints onto r using their absolute paths.
// Like Routes, it does NOT attach auth.RequireUser: the caller must mount these
// routes inside a group already wrapped with RequireUser so a valid user ID is
// present in the request context.
func (h *Handler) Mount(r chi.Router) {
	r.Get("/api/books", h.List)
	r.Post("/api/books", h.Create)
	r.Get("/api/books/{id}", h.Get)
	r.Put("/api/books/{id}", h.Update)
	r.Delete("/api/books/{id}", h.Delete)
}

// List handles GET /api/books.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())

	books, err := h.svc.List(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取书籍失败")
		return
	}
	writeJSON(w, http.StatusOK, books)
}

// Create handles POST /api/books.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())

	in, ok := decodeInput(w, r)
	if !ok {
		return
	}

	b, err := h.svc.Create(userID, in.Title, in.Style, in.Summary)
	if err != nil {
		writeServiceError(w, err, "创建书籍失败")
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

// Get handles GET /api/books/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())

	bookID, ok := parseID(w, r)
	if !ok {
		return
	}

	b, err := h.svc.Get(userID, bookID)
	if err != nil {
		writeServiceError(w, err, "获取书籍失败")
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// Update handles PUT /api/books/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())

	bookID, ok := parseID(w, r)
	if !ok {
		return
	}

	in, ok := decodeInput(w, r)
	if !ok {
		return
	}

	b, err := h.svc.Update(userID, bookID, in.Title, in.Style, in.Summary)
	if err != nil {
		writeServiceError(w, err, "更新书籍失败")
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// Delete handles DELETE /api/books/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())

	bookID, ok := parseID(w, r)
	if !ok {
		return
	}

	if err := h.svc.Delete(userID, bookID); err != nil {
		writeServiceError(w, err, "删除书籍失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// parseID reads the {id} path parameter. On a missing or non-numeric id it
// writes a 400 response and returns ok=false.
func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效的书籍 ID")
		return 0, false
	}
	return id, true
}

// decodeInput parses the JSON request body. On malformed input it writes a 400
// response and returns ok=false.
func decodeInput(w http.ResponseWriter, r *http.Request) (bookInput, bool) {
	var in bookInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return bookInput{}, false
	}
	return in, true
}

// writeServiceError maps a service/repo error to an HTTP response: ErrNotFound
// becomes 404, ErrValidation becomes 400, and anything else becomes a 500 with
// the given generic fallback message so internal detail is never leaked.
func writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "书籍不存在")
	case errors.Is(err, ErrValidation):
		writeError(w, http.StatusBadRequest, validationMessage(err))
	default:
		writeError(w, http.StatusInternalServerError, fallback)
	}
}

// validationMessage extracts the user-facing part of a validation error,
// stripping the internal "validation failed: " sentinel prefix so the client
// sees only the human-readable Chinese message.
func validationMessage(err error) string {
	const prefix = "validation failed: "
	msg := err.Error()
	if strings.HasPrefix(msg, prefix) {
		return msg[len(prefix):]
	}
	return "请求参数无效"
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
