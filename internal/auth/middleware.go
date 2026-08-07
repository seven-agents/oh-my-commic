package auth

import (
	"context"
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
			cookie, err := r.Cookie(cookieName)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			userID, ok := sess.UserID(cookie.Value)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserID returns the authenticated user's ID from ctx, or 0 if the request was
// not processed by RequireUser (or carried no user). Callers use the returned ID
// to scope data access to the current user.
func UserID(ctx context.Context) int64 {
	id, _ := ctx.Value(userIDKey).(int64)
	return id
}
