package story

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/ai"
	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/models"
)

// Handler serves the unified conversational storyboard endpoint on top of a
// Service. It resolves the authenticated user from the request context
// (populated upstream by auth.RequireUser) and never trusts a client user ID.
type Handler struct {
	svc *Service
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers the story endpoints onto r using their absolute paths.
//
//	POST /api/chapters/{id}/storyboard-chat
//	POST /api/panels/{id}/process
//
// Like the sibling handlers, it does not attach auth.RequireUser: the caller
// mounts this inside a group already wrapped with RequireUser.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/api/chapters/{id}/storyboard-chat", h.StoryboardChat)
	r.Post("/api/panels/{id}/process", h.ProcessPanel)
}

// storyboardChatRequest is the body for POST /api/chapters/{id}/storyboard-chat.
// Messages is the {role,content} conversation history. PanelCount is optional:
// when omitted or 0, the prompt uses its default frame range instead of a
// specific target.
type storyboardChatRequest struct {
	Messages   []models.ConversationMsg `json:"messages"`
	PanelCount int                      `json:"panelCount"`
}

// storyboardChatResponse is the body returned by the storyboard-chat endpoint:
// the model's one-line reply plus the persisted structured panels.
type storyboardChatResponse struct {
	Reply  string         `json:"reply"`
	Panels []models.Panel `json:"panels"`
}

// StoryboardChat handles POST /api/chapters/{id}/storyboard-chat.
func (h *Handler) StoryboardChat(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	chapterID, ok := parseID(w, r)
	if !ok {
		return
	}

	var req storyboardChatRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	reply, panels, err := h.svc.StoryboardChat(userID, chapterID, req.Messages, req.PanelCount)
	if err != nil {
		// Log the real cause server-side (never leaked to the client; the
		// wrapped error carries upstream detail but not the API key).
		log.Printf("storyboard-chat chapter %d failed: %v", chapterID, err)
		writeStoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, storyboardChatResponse{Reply: reply, Panels: panels})
}

// ProcessPanel handles POST /api/panels/{id}/process: the stage-2 per-frame
// decomposition. It returns the updated panel with its freshly parsed structured
// fields.
func (h *Handler) ProcessPanel(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	panelID, ok := parseID(w, r)
	if !ok {
		return
	}

	updated, err := h.svc.ProcessPanel(userID, panelID)
	if err != nil {
		log.Printf("process panel %d failed: %v", panelID, err)
		writeStoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// parseID reads the positive integer id path parameter. On a missing or
// non-positive value it writes a 400 response and returns ok=false.
func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效的 ID")
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

// writeStoryError maps a service error to an HTTP response: ErrNotFound becomes
// 404, and any other error (e.g. an upstream AI failure) becomes a 502 with a
// generic message so the API key and raw upstream body are never leaked.
func writeStoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "资源不存在")
	case errors.Is(err, ai.ErrRateLimited):
		writeError(w, http.StatusTooManyRequests, "画师有点忙，稍等一下再试～（请求太频繁或额度紧张）")
	case errors.Is(err, ai.ErrUpstreamTimeout):
		writeError(w, http.StatusGatewayTimeout, "这次等太久超时啦，再试一次～")
	case errors.Is(err, ai.ErrUpstreamUnavailable):
		writeError(w, http.StatusBadGateway, "AI 服务暂时不可用，稍后再试～")
	default:
		writeError(w, http.StatusBadGateway, "AI 服务暂时不可用，请稍后再试")
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
