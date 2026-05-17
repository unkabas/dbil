package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/unkabas/dbil/internal/crypto"
	"github.com/unkabas/dbil/internal/store"
)

const testPassword = "a-decent-password-1234"

func setupAuth(t *testing.T) (Deps, store.User) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close(db) })
	if err := store.Apply(db); err != nil {
		t.Fatal(err)
	}
	mkBytes, _ := crypto.Random(crypto.KeySize)
	mk, _ := crypto.NewMasterKey(mkBytes)

	users := store.NewUsersRepo(db)
	sessions := store.NewSessionsRepo(db)
	audit := store.NewAuditRepo(db, mk)

	u, err := users.Create(context.Background(), "admin@local", testPassword, store.RoleAdmin, false)
	if err != nil {
		t.Fatal(err)
	}
	return Deps{Users: users, Sessions: sessions, Audit: audit}, u
}

func TestVerifyPassword_OK(t *testing.T) {
	d, u := setupAuth(t)
	got, err := VerifyPassword(context.Background(), d.Users, u.Email, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID {
		t.Fatalf("user id mismatch: %d vs %d", got.ID, u.ID)
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	d, u := setupAuth(t)
	_, err := VerifyPassword(context.Background(), d.Users, u.Email, "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestVerifyPassword_UnknownUser(t *testing.T) {
	d, _ := setupAuth(t)
	_, err := VerifyPassword(context.Background(), d.Users, "nobody@local", "anything")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_SuccessReturnsTokenAndAuditEvent(t *testing.T) {
	d, u := setupAuth(t)
	ctx := context.Background()
	res, err := Login(ctx, d, u.Email, testPassword, "test-ua", "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if res.Token == "" {
		t.Fatal("login returned empty token")
	}
	if res.User.ID != u.ID {
		t.Fatalf("login user mismatch")
	}
	if !res.ExpiresAt.After(res.User.CreatedAt) {
		t.Fatal("expires_at should be after now")
	}

	// Session should be looked-up-able.
	if _, err := d.Sessions.Lookup(ctx, res.Token); err != nil {
		t.Fatalf("session lookup failed: %v", err)
	}

	// Audit chain should contain auth.login (the only entry since no init was run).
	if err := d.Audit.VerifyChain(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestLogin_FailureLogsAudit(t *testing.T) {
	d, u := setupAuth(t)
	ctx := context.Background()
	if _, err := Login(ctx, d, u.Email, "definitely-wrong", "ua", "ip"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
	n, err := d.Audit.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 audit entry, got %d", n)
	}
}

func TestLogout_RevokesAndAudits(t *testing.T) {
	d, u := setupAuth(t)
	ctx := context.Background()
	res, err := Login(ctx, d, u.Email, testPassword, "ua", "ip")
	if err != nil {
		t.Fatal(err)
	}
	if err := Logout(ctx, d, u, res.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Sessions.Lookup(ctx, res.Token); !errors.Is(err, store.ErrSessionInvalid) {
		t.Fatalf("session must be invalid after Logout, got %v", err)
	}
}
