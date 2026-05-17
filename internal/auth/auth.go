// Package auth implements DBil's session-based authentication: password
// verification (Argon2id, timing-equalised), Login/Logout flows with audit
// hooks, and the RequireAuth middleware used by every protected route.
//
// The middleware is intentionally the only auth code path: solo mode and
// team mode share it. A static lint (scripts/lint-auth) enforces that
// every handler outside an explicit allowlist is mounted under
// RequireAuth(...) in server/handlers.Mount.
package auth

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/unkabas/dbil/internal/crypto"
	"github.com/unkabas/dbil/internal/store"
)

// DefaultSessionTTL is the solo-mode session lifetime. Tag-driven TTLs
// (12 h for team, 1 h for production-operation sessions) arrive in Plan 4.
const DefaultSessionTTL = 7 * 24 * time.Hour

// ErrInvalidCredentials surfaces uniformly for unknown-user and wrong-password
// scenarios to prevent user enumeration via timing or error messages.
var ErrInvalidCredentials = errors.New("invalid credentials")

// dummySalt is used to run a full Argon2id cycle on the unknown-user path so
// timing roughly matches the known-user path. Not a secret; the point is
// CPU work, not secrecy.
var dummySalt = func() []byte {
	b := make([]byte, 16)
	for i := range b {
		b[i] = 0x42
	}
	return b
}()

// Deps is what auth flows and middleware need from the store layer.
type Deps struct {
	Users    *store.UsersRepo
	Sessions *store.SessionsRepo
	Audit    *store.AuditRepo
}

// LoginResult is what Login returns on success.
type LoginResult struct {
	Token     string
	User      store.User
	ExpiresAt time.Time
}

// VerifyPassword fetches the user by email and verifies the password using
// Argon2id with the stored salt. On miss it still spends an Argon2id cycle
// with a fixed salt so the unknown-user path takes roughly the same time as
// the wrong-password path.
func VerifyPassword(ctx context.Context, users *store.UsersRepo, email, password string) (store.User, error) {
	ua, err := users.GetUserAuthByEmail(ctx, email)
	if err != nil {
		// Equalise timing — discard the result.
		_, _ = crypto.Argon2id([]byte(password), dummySalt, 32)
		return store.User{}, ErrInvalidCredentials
	}
	decoded, err := base64.RawURLEncoding.DecodeString(ua.PasswordHash)
	if err != nil {
		return store.User{}, ErrInvalidCredentials
	}
	hash, err := crypto.Argon2id([]byte(password), ua.PasswordSalt, 32)
	if err != nil {
		return store.User{}, ErrInvalidCredentials
	}
	if subtle.ConstantTimeCompare(hash, decoded) != 1 {
		return store.User{}, ErrInvalidCredentials
	}
	return ua.User, nil
}

// Login verifies credentials and creates a session. Both success
// (auth.login) and failure (auth.login.failed) emit audit entries.
func Login(ctx context.Context, d Deps, email, password, userAgent, ip string) (LoginResult, error) {
	user, err := VerifyPassword(ctx, d.Users, email, password)
	if err != nil {
		if d.Audit != nil {
			_, _ = d.Audit.Append(ctx, "anonymous", "auth.login.failed", "user", map[string]any{
				"email": email,
				"ip":    ip,
				"ua":    userAgent,
			})
		}
		return LoginResult{}, err
	}
	token, expires, err := d.Sessions.Create(ctx, user.ID, DefaultSessionTTL, userAgent, ip)
	if err != nil {
		return LoginResult{}, fmt.Errorf("auth login: session: %w", err)
	}
	if d.Audit != nil {
		_, _ = d.Audit.Append(ctx, user.Email, "auth.login", fmt.Sprintf("user:%d", user.ID), map[string]any{
			"ip": ip,
			"ua": userAgent,
		})
	}
	return LoginResult{Token: token, User: user, ExpiresAt: expires}, nil
}

// Logout revokes the supplied session token and records auth.logout.
func Logout(ctx context.Context, d Deps, user store.User, token string) error {
	if err := d.Sessions.Revoke(ctx, token); err != nil {
		return fmt.Errorf("auth logout: revoke: %w", err)
	}
	if d.Audit != nil {
		_, _ = d.Audit.Append(ctx, user.Email, "auth.logout", fmt.Sprintf("user:%d", user.ID), map[string]any{})
	}
	return nil
}
