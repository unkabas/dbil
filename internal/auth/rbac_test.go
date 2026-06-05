package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/unkabas/dbil/internal/store"
)

func TestCanWrite(t *testing.T) {
	cases := map[string]bool{
		store.RoleAdmin:  true,
		store.RoleMember: true,
		store.RoleViewer: false,
		"unknown":        false,
	}
	for role, want := range cases {
		if got := CanWrite(role); got != want {
			t.Errorf("CanWrite(%q) = %v, want %v", role, got, want)
		}
	}
}

func withUser(r *http.Request, role string) *http.Request {
	ctx := context.WithValue(r.Context(), userCtxKey, store.User{ID: 1, Email: "u@local", Role: role})
	return r.WithContext(ctx)
}

func TestRequireRole_AllowsMatchingRole(t *testing.T) {
	var ran bool
	h := RequireRole(store.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ran = true
		w.WriteHeader(http.StatusOK)
	}))
	r := withUser(httptest.NewRequest(http.MethodGet, "/api/users", nil), store.RoleAdmin)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if !ran || w.Code != http.StatusOK {
		t.Fatalf("admin should pass: ran=%v code=%d", ran, w.Code)
	}
}

func TestRequireRole_DeniesOtherRole(t *testing.T) {
	h := RequireRole(store.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run for non-admin")
	}))
	for _, role := range []string{store.RoleMember, store.RoleViewer} {
		r := withUser(httptest.NewRequest(http.MethodGet, "/api/users", nil), role)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("role %q: want 403, got %d", role, w.Code)
		}
	}
}

func TestRequireRole_NoUserReturns401(t *testing.T) {
	h := RequireRole(store.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run without a user in context")
	}))
	r := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without user, got %d", w.Code)
	}
}

func TestGeneratePassword_UniqueAndStrong(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		pw, err := GeneratePassword()
		if err != nil {
			t.Fatal(err)
		}
		if len(pw) < 20 {
			t.Fatalf("password too short: %q (%d)", pw, len(pw))
		}
		if seen[pw] {
			t.Fatalf("duplicate password generated: %q", pw)
		}
		seen[pw] = true
	}
}
