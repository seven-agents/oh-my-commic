package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// sessionCookie is the name of the cookie carrying the session token.
const sessionCookie = "session"

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

// Handler serves the authentication HTTP endpoints. It is mountable as an
// http.Handler and also exposes its individual routes for reuse.
type Handler struct {
	svc *Service
}

// NewHandler returns an http.Handler exposing the auth routes (mounted under the
// caller's /api/v1 group):
//
//	POST /register
//	POST /login
//	POST /logout
//
// The session store is taken from svc, so both share the same token map.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
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
// RequireUser so h.Me can rely on a valid user ID in the request context. The
// effective path is /api/v1/me.
//
//	GET /me
func (h *Handler) MountProtected(r chi.Router) {
	r.Get("/me", h.Me)
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
			writeError(w, http.StatusForbidden, "邀请码不正确")
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
