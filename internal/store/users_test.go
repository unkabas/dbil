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

func TestUsers_ListAndGetByID(t *testing.T) {
	r := setupUsers(t)
	ctx := context.Background()
	a, err := r.Create(ctx, "admin@local", "admin-password", RoleAdmin, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Create(ctx, "member@local", "member-password", RoleMember, false); err != nil {
		t.Fatal(err)
	}
	users, err := r.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("want 2 users, got %d", len(users))
	}
	got, err := r.GetByID(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "admin@local" {
		t.Fatalf("GetByID email mismatch: %q", got.Email)
	}
	if _, err := r.GetByID(ctx, 99999); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("want ErrUserNotFound, got %v", err)
	}
}

func TestUsers_UpdateRole_LastAdminGuard(t *testing.T) {
	r := setupUsers(t)
	ctx := context.Background()
	admin, err := r.Create(ctx, "admin@local", "admin-password", RoleAdmin, false)
	if err != nil {
		t.Fatal(err)
	}
	// Demoting the only admin must fail.
	if _, err := r.UpdateRole(ctx, admin.ID, RoleViewer); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("want ErrLastAdmin demoting last admin, got %v", err)
	}
	// With a second admin present, demotion is allowed.
	second, err := r.Create(ctx, "admin2@local", "admin2-password", RoleAdmin, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpdateRole(ctx, second.ID, RoleMember); err != nil {
		t.Fatalf("demoting non-last admin should succeed, got %v", err)
	}
}

func TestUsers_Delete_LastAdminGuard(t *testing.T) {
	r := setupUsers(t)
	ctx := context.Background()
	admin, err := r.Create(ctx, "admin@local", "admin-password", RoleAdmin, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, admin.ID); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("want ErrLastAdmin deleting last admin, got %v", err)
	}
	viewer, err := r.Create(ctx, "viewer@local", "viewer-password", RoleViewer, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, viewer.ID); err != nil {
		t.Fatalf("deleting a viewer should succeed, got %v", err)
	}
	if _, err := r.GetByID(ctx, viewer.ID); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("deleted user should be gone, got %v", err)
	}
}

func TestUsers_SetPassword_ClearsMustRotate(t *testing.T) {
	r := setupUsers(t)
	ctx := context.Background()
	u, err := r.Create(ctx, "u@local", "initial-password", RoleMember, true)
	if err != nil {
		t.Fatal(err)
	}
	if !u.MustRotate {
		t.Fatal("auto-generated user should require rotation")
	}
	if err := r.SetPassword(ctx, u.ID, "a-brand-new-password", false); err != nil {
		t.Fatal(err)
	}
	ua, err := r.GetUserAuthByEmail(ctx, "u@local")
	if err != nil {
		t.Fatal(err)
	}
	if ua.MustRotate {
		t.Fatal("self-set password must clear must_rotate")
	}
	// Old password must no longer verify against the new hash.
	got, err := r.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.MustRotate {
		t.Fatal("GetByID should reflect cleared must_rotate")
	}
}
