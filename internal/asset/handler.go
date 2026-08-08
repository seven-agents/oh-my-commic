package asset

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/ai"
	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/comicify"
	"github.com/seven-agents/oh-my-commic/internal/models"
	"github.com/seven-agents/oh-my-commic/internal/storage"
)

// mediaPrefix is the URL prefix of a locally stored upload. Only such URLs are
// candidates for comic-ification; an empty or external URL is persisted as-is.
const mediaPrefix = "/media/"

// maxUploadBytes caps the size of an uploaded asset (5 MiB). The limit is
// enforced both on the request body (MaxBytesReader) and the multipart parser.
const maxUploadBytes = 5 << 20

// sniffLen is the number of leading bytes inspected by http.DetectContentType.
const sniffLen = 512

// contentTypeExt maps the image content types we accept to the file extension
// handed to storage. image/jpeg is normalized to the .jpg extension.
var contentTypeExt = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

// Handler serves the asset HTTP endpoints (image upload plus character and scene
// CRUD) on top of a Service and a storage backend. It resolves the authenticated
// user from the request context (populated upstream by auth.RequireUser) and
// never trusts a user ID from the request. Uploads are additionally gated by
// book ownership so a user can only write files into a book they own.
type Handler struct {
	svc   *Service
	store storage.Local
	comic *comicify.Service
}

// NewHandler returns a Handler backed by svc, store, and the comicify service
// used to redraw a freshly uploaded asset image into its locked stylized form
// before the asset is persisted.
func NewHandler(svc *Service, store storage.Local, comic *comicify.Service) *Handler {
	return &Handler{svc: svc, store: store, comic: comic}
}

// isLocalUpload reports whether url points at a locally stored upload (and is
// therefore a candidate for comic-ification).
func isLocalUpload(url string) bool {
	return strings.HasPrefix(url, mediaPrefix)
}

// belongsToBook reports whether a local media url lives under the given book's
// directory (/media/{bookID}/...). It ties a client-supplied source image to the
// caller's own, ownership-checked book so a user cannot point comicify at another
// user's upload (/media/{otherBookID}/...) and launder it into their book.
func belongsToBook(url string, bookID int64) bool {
	return strings.HasPrefix(url, fmt.Sprintf("%s%d/", mediaPrefix, bookID))
}

// Mount registers the asset endpoints onto r using their absolute paths.
//
//	POST   /api/books/{bookId}/upload
//	GET    /api/books/{bookId}/characters
//	POST   /api/books/{bookId}/characters
//	PUT    /api/books/{bookId}/characters/{id}
//	DELETE /api/books/{bookId}/characters/{id}
//	GET    /api/books/{bookId}/scenes
//	POST   /api/books/{bookId}/scenes
//	PUT    /api/books/{bookId}/scenes/{id}
//	DELETE /api/books/{bookId}/scenes/{id}
//
// It deliberately does NOT attach auth.RequireUser: the caller mounts these
// routes inside a group already wrapped with RequireUser, so every handler can
// rely on a valid user ID being present in the context.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/api/books/{bookId}/upload", h.Upload)

	r.Get("/api/books/{bookId}/characters", h.ListCharacters)
	r.Post("/api/books/{bookId}/characters", h.CreateCharacter)
	r.Put("/api/books/{bookId}/characters/{id}", h.UpdateCharacter)
	r.Delete("/api/books/{bookId}/characters/{id}", h.DeleteCharacter)

	r.Get("/api/books/{bookId}/scenes", h.ListScenes)
	r.Post("/api/books/{bookId}/scenes", h.CreateScene)
	r.Put("/api/books/{bookId}/scenes/{id}", h.UpdateScene)
	r.Delete("/api/books/{bookId}/scenes/{id}", h.DeleteScene)
}

// Upload handles POST /api/books/{bookId}/upload. It accepts a multipart form
// with a single file field named "file", validates that the payload is an
// allowed image type (by sniffing its content, not trusting the client), stores
// it under the book, and responds {"imageUrl": "/media/..."}.
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())

	bookID, ok := parseBookID(w, r)
	if !ok {
		return
	}

	if err := h.svc.VerifyBook(userID, bookID); err != nil {
		writeAssetError(w, err, "上传失败")
		return
	}

	// Cap the whole request body before parsing so an oversized upload cannot
	// exhaust memory. ParseMultipartForm is also bounded as defense in depth.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "文件过大或格式错误")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "缺少文件字段 file")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "读取文件失败")
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "文件为空")
		return
	}

	sniff := data
	if len(sniff) > sniffLen {
		sniff = sniff[:sniffLen]
	}
	ext, ok := contentTypeExt[http.DetectContentType(sniff)]
	if !ok {
		writeError(w, http.StatusBadRequest, "仅支持 PNG/JPG/WEBP 图片")
		return
	}

	url, err := h.store.SaveBytes(bookID, ext, data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存文件失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"imageUrl": url})
}

// ListCharacters handles GET /api/books/{bookId}/characters.
func (h *Handler) ListCharacters(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	bookID, ok := parseBookID(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListCharacters(userID, bookID)
	if err != nil {
		writeAssetError(w, err, "获取角色失败")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// CreateCharacter handles POST /api/books/{bookId}/characters.
func (h *Handler) CreateCharacter(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	bookID, ok := parseBookID(w, r)
	if !ok {
		return
	}
	var c models.Character
	if !decodeJSON(w, r, &c) {
		return
	}

	// A freshly uploaded local image is comic-ified into its locked stylized
	// form before persistence; an empty or external URL is kept as-is.
	if isLocalUpload(c.ImageURL) {
		// The source image must live under THIS book (server-resolved bookID),
		// never another user's /media/{otherBook}/... upload.
		if !belongsToBook(c.ImageURL, bookID) {
			writeError(w, http.StatusBadRequest, "图片不合法")
			return
		}
		// Fail fast (404) on an unowned book before spending an API call.
		if err := h.svc.VerifyBook(userID, bookID); err != nil {
			writeAssetError(w, err, "创建角色失败")
			return
		}
		stylized, err := h.comic.Character(r.Context(), userID, bookID, c, c.ImageURL)
		if err != nil {
			writeComicifyError(w, err)
			return
		}
		c.ImageURL = stylized
	}

	created, err := h.svc.CreateCharacter(userID, bookID, c)
	if err != nil {
		writeAssetError(w, err, "创建角色失败")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// UpdateCharacter handles PUT /api/books/{bookId}/characters/{id}.
func (h *Handler) UpdateCharacter(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	id, ok := parseAssetID(w, r)
	if !ok {
		return
	}
	var c models.Character
	if !decodeJSON(w, r, &c) {
		return
	}

	// Load the stored asset so we can (a) resolve the owning book for comicify
	// and (b) compare images: only a NEW local upload triggers re-comic-ification.
	existing, err := h.svc.GetCharacter(userID, id)
	if err != nil {
		writeAssetError(w, err, "更新角色失败")
		return
	}
	if isLocalUpload(c.ImageURL) && c.ImageURL != existing.ImageURL {
		// A changed local image must live under the asset's own book.
		if !belongsToBook(c.ImageURL, existing.BookID) {
			writeError(w, http.StatusBadRequest, "图片不合法")
			return
		}
		stylized, err := h.comic.Character(r.Context(), userID, existing.BookID, c, c.ImageURL)
		if err != nil {
			writeComicifyError(w, err)
			return
		}
		c.ImageURL = stylized
	}

	updated, err := h.svc.UpdateCharacter(userID, id, c)
	if err != nil {
		writeAssetError(w, err, "更新角色失败")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteCharacter handles DELETE /api/books/{bookId}/characters/{id}.
func (h *Handler) DeleteCharacter(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	id, ok := parseAssetID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteCharacter(userID, id); err != nil {
		writeAssetError(w, err, "删除角色失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ListScenes handles GET /api/books/{bookId}/scenes.
func (h *Handler) ListScenes(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	bookID, ok := parseBookID(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListScenes(userID, bookID)
	if err != nil {
		writeAssetError(w, err, "获取场景失败")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// CreateScene handles POST /api/books/{bookId}/scenes.
func (h *Handler) CreateScene(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	bookID, ok := parseBookID(w, r)
	if !ok {
		return
	}
	var sc models.Scene
	if !decodeJSON(w, r, &sc) {
		return
	}

	if isLocalUpload(sc.ImageURL) {
		if !belongsToBook(sc.ImageURL, bookID) {
			writeError(w, http.StatusBadRequest, "图片不合法")
			return
		}
		if err := h.svc.VerifyBook(userID, bookID); err != nil {
			writeAssetError(w, err, "创建场景失败")
			return
		}
		stylized, err := h.comic.Scene(r.Context(), userID, bookID, sc, sc.ImageURL)
		if err != nil {
			writeComicifyError(w, err)
			return
		}
		sc.ImageURL = stylized
	}

	created, err := h.svc.CreateScene(userID, bookID, sc)
	if err != nil {
		writeAssetError(w, err, "创建场景失败")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// UpdateScene handles PUT /api/books/{bookId}/scenes/{id}.
func (h *Handler) UpdateScene(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	id, ok := parseAssetID(w, r)
	if !ok {
		return
	}
	var sc models.Scene
	if !decodeJSON(w, r, &sc) {
		return
	}

	existing, err := h.svc.GetScene(userID, id)
	if err != nil {
		writeAssetError(w, err, "更新场景失败")
		return
	}
	if isLocalUpload(sc.ImageURL) && sc.ImageURL != existing.ImageURL {
		if !belongsToBook(sc.ImageURL, existing.BookID) {
			writeError(w, http.StatusBadRequest, "图片不合法")
			return
		}
		stylized, err := h.comic.Scene(r.Context(), userID, existing.BookID, sc, sc.ImageURL)
		if err != nil {
			writeComicifyError(w, err)
			return
		}
		sc.ImageURL = stylized
	}

	updated, err := h.svc.UpdateScene(userID, id, sc)
	if err != nil {
		writeAssetError(w, err, "更新场景失败")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteScene handles DELETE /api/books/{bookId}/scenes/{id}.
func (h *Handler) DeleteScene(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	id, ok := parseAssetID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteScene(userID, id); err != nil {
		writeAssetError(w, err, "删除场景失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// parseBookID reads the {bookId} path parameter. On a missing or non-positive
// id it writes a 400 response and returns ok=false.
func parseBookID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "bookId"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效的书籍 ID")
		return 0, false
	}
	return id, true
}

// parseAssetID reads the {id} path parameter (a character or scene id). On a
// missing or non-positive id it writes a 400 response and returns ok=false.
func parseAssetID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效的资产 ID")
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

// writeAssetError maps a service error to an HTTP response: ErrNotFound becomes
// 404 and anything else becomes a 500 with the given generic fallback message so
// internal detail is never leaked.
func writeAssetError(w http.ResponseWriter, err error, fallback string) {
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "资源不存在")
		return
	}
	writeError(w, http.StatusInternalServerError, fallback)
}

// writeComicifyError responds when the AI comic-ification step fails. An
// insufficient-credit error maps to 402 with a friendly message; any other
// failure maps to a generic 502 that never leaks upstream/API detail.
func writeComicifyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, comicify.ErrInsufficientCredits):
		writeError(w, http.StatusPaymentRequired, "积分不足，无法漫画化")
	case errors.Is(err, ai.ErrRateLimited):
		writeError(w, http.StatusTooManyRequests, "画师有点忙，稍等一下再试～（请求太频繁或额度紧张）")
	case errors.Is(err, ai.ErrUpstreamTimeout):
		writeError(w, http.StatusGatewayTimeout, "这次等太久超时啦，再试一次～")
	case errors.Is(err, ai.ErrUpstreamUnavailable):
		writeError(w, http.StatusBadGateway, "AI 服务暂时不可用，稍后再试～")
	default:
		writeError(w, http.StatusBadGateway, "画失败了，再试一次吧")
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
