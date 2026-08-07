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

// credentials is the JSON body accepted by register and login.
type credentials struct {
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

// Handler serves the authentication HTTP endpoints. It is mountable as an
// http.Handler and also exposes its individual routes for reuse.
type Handler struct {
	svc *Service
}

// NewHandler returns an http.Handler exposing the auth routes:
//
//	POST /api/register
//	POST /api/login
//	POST /api/logout
//
// The session store is taken from svc, so both share the same token map.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers the auth endpoints onto r using their absolute paths. These
// routes are public (no authentication middleware) so the enclosing router must
// not wrap them with auth.RequireUser.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/api/register", h.Register)
	r.Post("/api/login", h.Login)
	r.Post("/api/logout", h.Logout)
}

// Routes builds a chi router mounting the auth endpoints. It is separated from
// ServeHTTP so callers can mount these routes into a larger router later.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	h.Mount(r)
	return r
}

// ServeHTTP lets Handler be used directly as an http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Routes().ServeHTTP(w, r)
}

// Register handles POST /api/register.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	creds, ok := decodeCredentials(w, r)
	if !ok {
		return
	}

	u, err := h.svc.Register(creds.Nickname, creds.Password)
	if err != nil {
		if errors.Is(err, ErrNicknameTaken) {
			writeError(w, http.StatusConflict, "昵称已被占用")
			return
		}
		writeError(w, http.StatusInternalServerError, "注册失败")
		return
	}

	writeJSON(w, http.StatusCreated, u)
}

// Login handles POST /api/login. On success it sets an HttpOnly session cookie
// and returns the user as JSON.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	creds, ok := decodeCredentials(w, r)
	if !ok {
		return
	}

	token, u, err := h.svc.Login(creds.Nickname, creds.Password)
	if err != nil {
		// Both unknown-nickname and wrong-password map here; do not distinguish.
		writeError(w, http.StatusUnauthorized, "昵称或密码错误")
		return
	}

	setSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, u)
}

// Logout handles POST /api/logout. It revokes the current session (if any) and
// clears the cookie. Logging out without a session is treated as success.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		h.svc.Sessions().Revoke(c.Value)
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// decodeCredentials parses and validates the request body. On failure it writes
// a 400 response and returns ok=false.
func decodeCredentials(w http.ResponseWriter, r *http.Request) (credentials, bool) {
	var c credentials
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return credentials{}, false
	}

	c.Nickname = strings.TrimSpace(c.Nickname)
	if c.Nickname == "" || c.Password == "" {
		writeError(w, http.StatusBadRequest, "昵称和密码不能为空")
		return credentials{}, false
	}
	return c, true
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
