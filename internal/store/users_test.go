package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func setupUsers(t *testing.T) *UsersRepo {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Close(db) })
	if err := Apply(db); err != nil {
		t.Fatal(err)
	}
	return NewUsersRepo(db)
}

func TestUsers_CreateAndAdminExists(t *testing.T) {
	r := setupUsers(t)
	ctx := context.Background()

	has, err := r.AdminExists(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("AdminExists must be false on empty DB")
	}

	u, err := r.Create(ctx, "admin@local", "a-decent-password", RoleAdmin, true)
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "admin@local" {
		t.Fatalf("email mismatch: %q", u.Email)
	}
	if u.Role != RoleAdmin {
		t.Fatalf("role mismatch: %q", u.Role)
	}
	if !u.MustRotate {
		t.Fatal("auto-generated password must set must_rotate")
	}

	has, _ = r.AdminExists(ctx)
	if !has {
		t.Fatal("AdminExists must be true after creating an admin")
	}
}

func TestUsers_DuplicateEmail(t *testing.T) {
	r := setupUsers(t)
	ctx := context.Background()
	if _, err := r.Create(ctx, "a@b", "first-password", RoleAdmin, false); err != nil {
		t.Fatal(err)
	}
	_, err := r.Create(ctx, "a@b", "second-password", RoleAdmin, false)
	if !errors.Is(err, ErrUserExists) {
		t.Fatalf("want ErrUserExists, got %v", err)
	}
}

func TestUsers_RejectsEmpty(t *testing.T) {
	r := setupUsers(t)
	ctx := context.Background()
	if _, err := r.Create(ctx, "", "pw", RoleAdmin, false); err == nil {
		t.Fatal("expected error for empty email")
	}
	if _, err := r.Create(ctx, "a@b", "", RoleAdmin, false); err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestUsers_RejectsInvalidRole(t *testing.T) {
	r := setupUsers(t)
	if _, err := r.Create(context.Background(), "a@b", "pw-pw-pw", "superadmin", false); err == nil {
		t.Fatal("expected error for invalid role")
	}
}
