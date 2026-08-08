package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/storage"
)

// sessionCookie is the name of the cookie carrying the session token.
const sessionCookie = "session"

// maxAvatarBytes caps the size of an uploaded avatar (2 MiB). Enforced on both
// the request body (MaxBytesReader) and the multipart parser.
const maxAvatarBytes = 2 << 20

// avatarSniffLen is the number of leading bytes inspected by
// http.DetectContentType to identify the uploaded image type.
const avatarSniffLen = 512

// avatarContentTypeExt maps the accepted avatar image content types to the file
// extension handed to storage. image/jpeg is normalized to .jpg.
var avatarContentTypeExt = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

// registerBody is the JSON body accepted by register: username/password are the
// login credentials, email is required and unique, inviteCode gates signup, and
// nickname is an optional display name (defaults to username when blank).
type registerBody struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Email      string `json:"email"`
	InviteCode string `json:"inviteCode"`
	Nickname   string `json:"nickname"`
}

// loginBody is the JSON body accepted by login.
type loginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// profileBody is the JSON body accepted by PUT /me/profile. All fields are
// validated by the service layer; nickname falls back to the username when blank.
type profileBody struct {
	Nickname string `json:"nickname"`
	Age      int    `json:"age"`
	Gender   string `json:"gender"`
}

// Handler serves the authentication HTTP endpoints. It is mountable as an
// http.Handler and also exposes its individual routes for reuse.
type Handler struct {
	svc   *Service
	store storage.Local
}

// NewHandler returns an http.Handler exposing the auth routes (mounted under the
// caller's /api/v1 group):
//
//	POST /register
//	POST /login
//	POST /logout
//
// store is the local media backend used to persist uploaded avatars under
// users/{userID}/. The session store is taken from svc, so both share the same
// token map.
func NewHandler(svc *Service, store storage.Local) *Handler {
	return &Handler{svc: svc, store: store}
}

// Mount registers the auth endpoints onto r using resource-relative paths. The
// caller mounts these under the /api/v1 group, so the effective paths are
// /api/v1/register etc. These routes are public (no authentication middleware)
// so the enclosing router must not wrap them with auth.RequireUser.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/logout", h.Logout)
}

// MountProtected registers the auth endpoints that require an authenticated
// session. The caller must mount these inside a group already wrapped with
// RequireUser so the handlers can rely on a valid user ID in the request
// context. Effective paths are /api/v1/me, /api/v1/me/profile, /api/v1/me/avatar.
//
//	GET  /me
//	PUT  /me/profile
//	POST /me/avatar
func (h *Handler) MountProtected(r chi.Router) {
	r.Get("/me", h.Me)
	r.Put("/me/profile", h.UpdateProfile)
	r.Post("/me/avatar", h.UploadAvatar)
}

// MountAdmin registers the admin-only invite-code endpoints. The caller must
// mount these inside a group already wrapped with RequireAdmin so only admins
// can read or rotate the global invite code. Effective paths are
// /api/v1/admin/invite-code and /api/v1/admin/invite-code/rotate.
//
//	GET  /admin/invite-code
//	POST /admin/invite-code/rotate
func (h *Handler) MountAdmin(r chi.Router) {
	r.Get("/admin/invite-code", h.InviteCode)
	r.Post("/admin/invite-code/rotate", h.RotateInvite)
}

// Routes builds a chi router mounting the public auth endpoints under the
// versioned /api/v1 prefix (so ServeHTTP and standalone use match the effective
// production paths). It is separated from ServeHTTP so callers can mount these
// routes into a larger router later.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", func(v1 chi.Router) { h.Mount(v1) })
	return r
}

// ServeHTTP lets Handler be used directly as an http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Routes().ServeHTTP(w, r)
}

// Register handles POST /api/v1/register. On success it sets an HttpOnly session
// cookie (the caller is logged in immediately) and returns the user as JSON.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var body registerBody
	if !decodeBody(w, r, &body) {
		return
	}

	token, u, err := h.svc.Register(RegisterInput{
		Username:   strings.TrimSpace(body.Username),
		Password:   body.Password,
		Email:      body.Email,
		InviteCode: strings.TrimSpace(body.InviteCode),
		Nickname:   body.Nickname,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrBadInvite):
			// A bad invite code is a client input error, not an authorization
			// failure, so it maps to 400 (not 403).
			writeError(w, http.StatusBadRequest, "邀请码不正确")
		case errors.Is(err, ErrUsernameTaken):
			writeError(w, http.StatusConflict, "用户名已被占用")
		case errors.Is(err, ErrEmailTaken):
			writeError(w, http.StatusConflict, "邮箱已注册")
		case errors.Is(err, ErrBadUsername), errors.Is(err, ErrBadPassword),
			errors.Is(err, ErrBadEmail), errors.Is(err, ErrBadNickname):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "注册失败")
		}
		return
	}

	setSessionCookie(w, r, token)
	writeJSON(w, http.StatusCreated, u)
}

// Login handles POST /api/v1/login. On success it sets an HttpOnly session cookie
// and returns the user as JSON.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if !decodeBody(w, r, &body) {
		return
	}
	username := strings.TrimSpace(body.Username)
	if username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "用户名和密码不能为空")
		return
	}

	token, u, err := h.svc.Login(username, body.Password)
	if err != nil {
		// Both unknown-username and wrong-password map here; do not distinguish.
		writeError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	setSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, u)
}

// Me handles GET /api/v1/me. It returns the currently authenticated user (including
// the live credit balance, never the password hash) resolved from the session
// context populated by RequireUser.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID := UserID(r.Context())
	u, err := h.svc.Me(userID)
	if err != nil {
		// A resolved session pointing at a missing user is treated as not found
		// rather than leaking internal detail.
		writeError(w, http.StatusNotFound, "找不到这个内容")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// UpdateProfile handles PUT /api/v1/me/profile. It updates the caller's editable
// profile fields (nickname/age/gender) and returns the refreshed user. Validation
// failures (bad age/gender/nickname) map to 400; a missing user maps to 404.
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := UserID(r.Context())

	var body profileBody
	if !decodeBody(w, r, &body) {
		return
	}

	u, err := h.svc.UpdateProfile(userID, strings.TrimSpace(body.Nickname), body.Age, body.Gender)
	if err != nil {
		switch {
		case errors.Is(err, ErrBadAge), errors.Is(err, ErrBadGender),
			errors.Is(err, ErrBadNickname):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			// A resolved session pointing at a missing user is treated as not
			// found rather than leaking internal detail.
			writeError(w, http.StatusNotFound, "找不到这个内容")
		}
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// UploadAvatar handles POST /api/v1/me/avatar. It accepts a multipart form with a
// single file field named "file", validates that the payload is an allowed image
// type (by sniffing its content, not trusting the client) and is within the size
// cap, stores it under users/{userID}/, records the URL, and returns the
// refreshed user (with avatarUrl populated). Type/size errors map to 400.
func (h *Handler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID := UserID(r.Context())

	// Cap the whole request body before parsing so an oversized upload cannot
	// exhaust memory. ParseMultipartForm is also bounded as defense in depth.
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes)
	if err := r.ParseMultipartForm(maxAvatarBytes); err != nil {
		// Distinguish an oversized upload (MaxBytesReader tripped) from a
		// malformed multipart body so the client gets an actionable message.
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) || strings.Contains(err.Error(), "request body too large") {
			writeError(w, http.StatusBadRequest, "头像太大啦，请选 2MB 以内的图片")
			return
		}
		writeError(w, http.StatusBadRequest, "图片格式不对，请上传 PNG/JPG/WEBP 图片")
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
	if len(sniff) > avatarSniffLen {
		sniff = sniff[:avatarSniffLen]
	}
	ext, ok := avatarContentTypeExt[http.DetectContentType(sniff)]
	if !ok {
		writeError(w, http.StatusBadRequest, "仅支持 PNG/JPG/WEBP 图片")
		return
	}

	url, err := h.store.SaveUserAvatar(userID, ext, data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存文件失败")
		return
	}

	u, err := h.svc.SetAvatar(userID, url)
	if err != nil {
		writeError(w, http.StatusNotFound, "找不到这个内容")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// InviteCode handles GET /api/v1/admin/invite-code (admin only). It returns the
// current global invite code as {"inviteCode": "..."}.
func (h *Handler) InviteCode(w http.ResponseWriter, _ *http.Request) {
	code, err := h.svc.InviteCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取邀请码失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"inviteCode": code})
}

// RotateInvite handles POST /api/v1/admin/invite-code/rotate (admin only). It
// generates and persists a fresh invite code and returns it as
// {"inviteCode": "..."}.
func (h *Handler) RotateInvite(w http.ResponseWriter, _ *http.Request) {
	code, err := h.svc.RotateInvite()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "更新邀请码失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"inviteCode": code})
}

// Logout handles POST /api/v1/logout. It revokes the current session (if any) and
// clears the cookie. Logging out without a session is treated as success.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		h.svc.Sessions().Revoke(c.Value)
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// decodeBody parses the JSON request body into dst, rejecting unknown fields. On
// a decode failure it writes a 400 response and returns false. Field-level
// validation is delegated to the service layer's validators.
func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return false
	}
	return true
}

// setSessionCookie sets the HttpOnly session cookie. Secure is enabled when the
// request arrived over TLS so local HTTP development still works.
func setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}

// clearSessionCookie expires the session cookie on the client.
func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   -1,
	})
}

// writeJSON encodes v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error envelope. The message is caller-controlled and
// must never contain sensitive internal detail.
func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// compile-time assurance that Handler satisfies http.Handler.
var _ http.Handler = (*Handler)(nil)
