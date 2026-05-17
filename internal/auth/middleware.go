package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/unkabas/dbil/internal/store"
)

type ctxKey int

const (
	userCtxKey  ctxKey = iota
	tokenCtxKey                    // raw session token; available to logout handler
)

// UserFromContext returns the user injected by RequireAuth into r.Context().
func UserFromContext(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(userCtxKey).(store.User)
	return u, ok
}

// TokenFromContext returns the raw Bearer token that authenticated the request,
// for handlers that need to revoke their own session (e.g. logout).
func TokenFromContext(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(tokenCtxKey).(string)
	return t, ok
}

// RequireAuth is the single authentication middleware used by every protected
// route in both solo and team modes. Failures emit an auth.token.rejected
// audit entry so noisy attackers are visible in the chain.
func RequireAuth(d Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				respondUnauthorized(w, r, d, "missing bearer token")
				return
			}
			s, err := d.Sessions.Lookup(r.Context(), token)
			if err != nil {
				if !errors.Is(err, store.ErrSessionInvalid) {
					http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
					return
				}
				respondUnauthorized(w, r, d, "invalid session")
				return
			}
			ctx := context.WithValue(r.Context(), userCtxKey, s.User)
			ctx = context.WithValue(ctx, tokenCtxKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bearerToken parses 'Authorization: Bearer <token>' (case-insensitive scheme).
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func respondUnauthorized(w http.ResponseWriter, r *http.Request, d Deps, reason string) {
	if d.Audit != nil {
		_, _ = d.Audit.Append(r.Context(), "anonymous", "auth.token.rejected", "session", map[string]any{
			"ip":     RemoteIP(r),
			"ua":     r.Header.Get("User-Agent"),
			"reason": reason,
			"path":   r.URL.Path,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}

// RemoteIP extracts a best-effort client IP from the request, preferring the
// first X-Forwarded-For entry when present. Exported so handlers can record
// the same value in their own audit entries.
func RemoteIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.Index(v, ","); i > 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	if r.RemoteAddr != "" {
		if i := strings.LastIndex(r.RemoteAddr, ":"); i > 0 {
			return r.RemoteAddr[:i]
		}
		return r.RemoteAddr
	}
	return ""
}
