package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func setupSessions(t *testing.T) (*SessionsRepo, *UsersRepo, int64) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Close(db) })
	if err := Apply(db); err != nil {
		t.Fatal(err)
	}
	users := NewUsersRepo(db)
	u, err := users.Create(context.Background(), "admin@local", "a-decent-password", RoleAdmin, true)
	if err != nil {
		t.Fatal(err)
	}
	return NewSessionsRepo(db), users, u.ID
}

func TestSessions_CreateAndLookup(t *testing.T) {
	r, _, uid := setupSessions(t)
	ctx := context.Background()

	tok, exp, err := r.Create(ctx, uid, time.Hour, "agent-x", "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" {
		t.Fatal("empty token")
	}
	if !exp.After(time.Now()) {
		t.Fatalf("expires_at not in the future: %v", exp)
	}

	s, err := r.Lookup(ctx, tok)
	if err != nil {
		t.Fatal(err)
	}
	if s.UserID != uid {
		t.Fatalf("session userID %d != %d", s.UserID, uid)
	}
	if s.User.Email != "admin@local" {
		t.Fatalf("session user email %q", s.User.Email)
	}
	if s.UserAgent != "agent-x" {
		t.Fatalf("user_agent not persisted: %q", s.UserAgent)
	}
	if s.IP != "1.2.3.4" {
		t.Fatalf("ip not persisted: %q", s.IP)
	}
}

func TestSessions_Lookup_UnknownToken(t *testing.T) {
	r, _, _ := setupSessions(t)
	_, err := r.Lookup(context.Background(), "definitely-not-a-real-token")
	if !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("want ErrSessionInvalid, got %v", err)
	}
}

func TestSessions_Lookup_EmptyToken(t *testing.T) {
	r, _, _ := setupSessions(t)
	_, err := r.Lookup(context.Background(), "")
	if !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("want ErrSessionInvalid, got %v", err)
	}
}

func TestSessions_Lookup_Expired(t *testing.T) {
	r, _, uid := setupSessions(t)
	ctx := context.Background()
	tok, _, err := r.Create(ctx, uid, -time.Minute, "ua", "ip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Lookup(ctx, tok); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("expired session must surface ErrSessionInvalid, got %v", err)
	}
}

func TestSessions_Revoke(t *testing.T) {
	r, _, uid := setupSessions(t)
	ctx := context.Background()
	tok, _, _ := r.Create(ctx, uid, time.Hour, "ua", "ip")

	if err := r.Revoke(ctx, tok); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Lookup(ctx, tok); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("revoked session must surface ErrSessionInvalid, got %v", err)
	}
	// Idempotent: second revoke is fine.
	if err := r.Revoke(ctx, tok); err != nil {
		t.Fatal(err)
	}
}

func TestSessions_CascadeDeleteWithUser(t *testing.T) {
	r, _, uid := setupSessions(t)
	ctx := context.Background()
	tok, _, _ := r.Create(ctx, uid, time.Hour, "ua", "ip")

	if _, err := r.DB.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, uid); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Lookup(ctx, tok); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("session must be gone after user deletion, got %v", err)
	}
}

func TestSessions_DeleteExpired(t *testing.T) {
	r, _, uid := setupSessions(t)
	ctx := context.Background()

	// One already-expired, one live.
	_, _, _ = r.Create(ctx, uid, -time.Minute, "ua", "ip")
	live, _, _ := r.Create(ctx, uid, time.Hour, "ua", "ip")

	n, err := r.DeleteExpired(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 deletion, got %d", n)
	}
	if _, err := r.Lookup(ctx, live); err != nil {
		t.Fatalf("live session unexpectedly removed: %v", err)
	}
}
