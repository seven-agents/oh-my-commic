package auth

import (
	"context"
	"encoding/json"
	"net/http"
)

// cookieName is the HTTP cookie carrying the opaque session token. It must match
// the name set by the login handler.
const cookieName = "session"

// ctxKey is an unexported type used for context keys defined in this package.
// Using a private type prevents collisions with keys from other packages, since
// no external code can construct a value of this type.
type ctxKey int

// userIDKey is the context key under which the authenticated user's ID is stored.
const userIDKey ctxKey = iota

// RequireUser returns middleware that authenticates requests against sess.
//
// It reads the session cookie, resolves the token to a user ID, and on success
// injects that ID into the request context before invoking next. If the cookie
// is missing or the token is unknown, it responds 401 Unauthorized and does not
// call next. This is the enforcement point for per-user data isolation.
func RequireUser(sess *Session) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := resolveUser(sess, r)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// resolveUser reads the session cookie from r and resolves it to a user ID via
// sess. It returns ok=false when the cookie is missing or the token is unknown,
// so callers can uniformly answer 401. It is the shared token-parsing logic for
// both RequireUser and RequireAdmin.
func resolveUser(sess *Session, r *http.Request) (int64, bool) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return 0, false
	}
	return sess.UserID(cookie.Value)
}

// RequireAdmin returns middleware that authenticates the request (same session
// resolution as RequireUser) and then requires the resolved user to have the
// "admin" role. A missing or unknown session yields 401 Unauthorized; an
// authenticated non-admin (or a user that can no longer be loaded) yields 403
// Forbidden with a JSON body {"error":"需要管理员权限"}. On success the user ID is
// injected into the request context, exactly as RequireUser does.
func RequireAdmin(sess *Session, repo *UserRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := resolveUser(sess, r)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			user, err := repo.ByID(userID)
			if err != nil || user.Role != "admin" {
				writeForbidden(w)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeForbidden responds 403 with the admin-only JSON error. Encoding a fixed
// struct never fails in practice; any error is swallowed after the status and
// content type are already committed.
func writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "需要管理员权限"})
}

// UserID returns the authenticated user's ID from ctx, or 0 if the request was
// not processed by RequireUser (or carried no user). Callers use the returned ID
// to scope data access to the current user.
func UserID(ctx context.Context) int64 {
	id, _ := ctx.Value(userIDKey).(int64)
	return id
}

// WithUserID returns a copy of ctx carrying userID under the same key
// RequireUser uses. It lets other packages (and their tests) construct a request
// context as if it had passed through RequireUser, without depending on the
// unexported key.
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}
