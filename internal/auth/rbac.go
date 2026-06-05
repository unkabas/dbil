package auth

import (
	"encoding/base32"
	"encoding/json"
	"net/http"

	"github.com/unkabas/dbil/internal/crypto"
	"github.com/unkabas/dbil/internal/store"
)

// CanWrite reports whether a role may perform data-mutating operations: DML in
// the SQL editor and inline cell edits in the data grid. admin and member may
// write; viewer is strictly read-only.
func CanWrite(role string) bool {
	return role == store.RoleAdmin || role == store.RoleMember
}

// RequireRole admits only authenticated users whose role is in the allowed
// set. It MUST be mounted inside a RequireAuth group so the user is already in
// the request context; an authenticated user with a disallowed role receives
// 403. This composes with — it does not replace — the RequireAuth gate that
// scripts/lint-auth enforces.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			if !allowed[user.Role] {
				writeJSONStatus(w, http.StatusForbidden, map[string]string{
					"error":  "forbidden",
					"reason": "insufficient role",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GeneratePassword returns a 24-character base32 string from 15 random bytes
// (120 bits of entropy). Shared by bootstrap (initial admin) and the user
// management handlers (newly created users and password resets) so every
// auto-generated credential has identical strength.
func GeneratePassword() (string, error) {
	b, err := crypto.Random(15)
	if err != nil {
		return "", err
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return enc.EncodeToString(b), nil
}

func writeJSONStatus(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
