package community

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/auth"
)

// Handler serves the community HTTP endpoints on top of a Service.
type Handler struct {
	svc *Service
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// MountPublic registers the public (OptionalUser) endpoints:
//
//	GET  /community/books
//	GET  /community/books/{id}
//	POST /community/books/{id}/view
func (h *Handler) MountPublic(r chi.Router) {
	r.Get("/community/books", h.List)
	r.Get("/community/books/{id}", h.Detail)
	r.Post("/community/books/{id}/view", h.RecordView)
}

// MountAuthed registers the authenticated endpoints (behind RequireUser):
//
//	POST   /community/books/{id}/like
//	DELETE /community/books/{id}/like
func (h *Handler) MountAuthed(r chi.Router) {
	r.Post("/community/books/{id}/like", h.Like)
	r.Delete("/community/books/{id}/like", h.Unlike)
}

// viewerKey builds the like/view identity from the (optional) authenticated user
// and a client-provided anonymous id. Logged-in users key by user id; anonymous
// users key by "anon:{clientId}"; a blank client id falls back to "anon:".
func viewerKeyFor(userID int64, clientID string) string {
	if userID != 0 {
		return "u:" + strconv.FormatInt(userID, 10)
	}
	return "anon:" + clientID
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	vk := ""
	if userID != 0 {
		vk = "u:" + strconv.FormatInt(userID, 10)
	}
	sort := r.URL.Query().Get("sort")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	list, err := h.svc.ListPublic(vk, sort, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取社区列表失败")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	vk := ""
	if userID != 0 {
		vk = "u:" + strconv.FormatInt(userID, 10)
	}
	bookID, ok := parseID(w, r)
	if !ok {
		return
	}
	d, err := h.svc.GetPublicDetail(vk, bookID)
	if err != nil {
		writeCommunityError(w, err, "获取内容失败")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

type viewInput struct {
	ClientID string `json:"clientId"`
}

func (h *Handler) RecordView(w http.ResponseWriter, r *http.Request) {
	bookID, ok := parseID(w, r)
	if !ok {
		return
	}
	var in viewInput
	// body 可选：空 body 允许（匿名无 clientId 时退化为 "anon:"）。
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&in)
	}
	vk := viewerKeyFor(auth.UserID(r.Context()), in.ClientID)
	if err := h.svc.RecordView(bookID, vk); err != nil {
		writeCommunityError(w, err, "记录浏览失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) Like(w http.ResponseWriter, r *http.Request)   { h.like(w, r, true) }
func (h *Handler) Unlike(w http.ResponseWriter, r *http.Request) { h.like(w, r, false) }

func (h *Handler) like(w http.ResponseWriter, r *http.Request, like bool) {
	userID := auth.UserID(r.Context())
	bookID, ok := parseID(w, r)
	if !ok {
		return
	}
	var res LikeResult
	var err error
	if like {
		res, err = h.svc.Like(userID, bookID)
	} else {
		res, err = h.svc.Unlike(userID, bookID)
	}
	if err != nil {
		writeCommunityError(w, err, "操作失败")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// parseID reads the {id} path param; on a bad id writes 400 and returns ok=false.
func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效的 ID")
		return 0, false
	}
	return id, true
}

func writeCommunityError(w http.ResponseWriter, err error, fallback string) {
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "找不到这个内容")
		return
	}
	writeError(w, http.StatusInternalServerError, fallback)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
