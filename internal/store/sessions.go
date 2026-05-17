package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/unkabas/dbil/internal/crypto"
)

// ErrSessionInvalid is returned by Lookup for unknown, expired, or revoked
// sessions. The error intentionally does not distinguish between cases to
// avoid leaking session state to attackers.
var ErrSessionInvalid = errors.New("session invalid")

// Session is a sessions row joined with the owning user.
type Session struct {
	ID        string // sha256(token) hex
	UserID    int64
	User      User
	CreatedAt time.Time
	ExpiresAt time.Time
	Revoked   bool
	UserAgent string
	IP        string
}

// SessionsRepo manages the sessions table.
type SessionsRepo struct {
	DB *sql.DB
}

// NewSessionsRepo binds a SessionsRepo to db.
func NewSessionsRepo(db *sql.DB) *SessionsRepo {
	return &SessionsRepo{DB: db}
}

// Create generates a fresh token, stores its sha256 hash + metadata, and
// returns the raw token to the caller. The raw token never touches the DB.
func (r *SessionsRepo) Create(ctx context.Context, userID int64, ttl time.Duration, userAgent, ip string) (token string, expiresAt time.Time, err error) {
	raw, err := crypto.Random(32)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sessions create: random: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	id := hashToken(token)
	now := time.Now()
	expiresAt = now.Add(ttl)
	_, err = r.DB.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, created_at, expires_at, user_agent, ip)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, userID, now.UnixNano(), expiresAt.UnixNano(), userAgent, ip,
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sessions create: insert: %w", err)
	}
	return token, expiresAt, nil
}

// Lookup returns the session and joined user for a valid token. Expired or
// revoked sessions return ErrSessionInvalid, indistinguishable from unknown.
func (r *SessionsRepo) Lookup(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, ErrSessionInvalid
	}
	id := hashToken(token)
	var (
		s              Session
		revokedNS      sql.NullInt64
		createdNS      int64
		expiresNS      int64
		userCreatedNS  int64
		userUpdatedNS  int64
		userMustRotate int
	)
	err := r.DB.QueryRowContext(ctx, `
		SELECT s.id, s.user_id, s.created_at, s.expires_at, s.revoked_at,
		       s.user_agent, s.ip,
		       u.id, u.email, u.role, u.must_rotate, u.created_at, u.updated_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = ?`, id).Scan(
		&s.ID, &s.UserID, &createdNS, &expiresNS, &revokedNS,
		&s.UserAgent, &s.IP,
		&s.User.ID, &s.User.Email, &s.User.Role, &userMustRotate,
		&userCreatedNS, &userUpdatedNS,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionInvalid
	}
	if err != nil {
		return Session{}, fmt.Errorf("sessions lookup: %w", err)
	}
	s.CreatedAt = time.Unix(0, createdNS)
	s.ExpiresAt = time.Unix(0, expiresNS)
	s.Revoked = revokedNS.Valid
	s.User.MustRotate = userMustRotate != 0
	s.User.CreatedAt = time.Unix(0, userCreatedNS)
	s.User.UpdatedAt = time.Unix(0, userUpdatedNS)

	if s.Revoked || !time.Now().Before(s.ExpiresAt) {
		return Session{}, ErrSessionInvalid
	}
	return s, nil
}

// Revoke marks the session as revoked. Idempotent: no error if the session
// does not exist or is already revoked.
func (r *SessionsRepo) Revoke(ctx context.Context, token string) error {
	id := hashToken(token)
	_, err := r.DB.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = ?
		WHERE id = ? AND revoked_at IS NULL`,
		time.Now().UnixNano(), id,
	)
	if err != nil {
		return fmt.Errorf("sessions revoke: %w", err)
	}
	return nil
}

// DeleteExpired removes sessions whose expires_at is before now. Intended for
// a periodic cleanup goroutine; not called in Plan 2.
func (r *SessionsRepo) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, now.UnixNano())
	if err != nil {
		return 0, fmt.Errorf("sessions delete expired: %w", err)
	}
	return res.RowsAffected()
}

// hashToken returns the hex-encoded sha256 of a raw token, used as the row id.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
