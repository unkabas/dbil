package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/unkabas/dbil/internal/auth"
	"github.com/unkabas/dbil/internal/store"
)

// userJSON renders a user for API responses. Never includes password material.
func userJSON(u store.User) map[string]any {
	return map[string]any{
		"id":          u.ID,
		"email":       u.Email,
		"role":        u.Role,
		"must_rotate": u.MustRotate,
		"created_at":  u.CreatedAt.Unix(),
		"updated_at":  u.UpdatedAt.Unix(),
	}
}

// ListUsers handles GET /api/users. Admin-only (RequireRole at the route).
func ListUsers(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := d.Auth.Users.List(r.Context())
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		out := make([]map[string]any, 0, len(users))
		for _, u := range users {
			out = append(out, userJSON(u))
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": out})
	}
}

type createUserReq struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// CreateUser handles POST /api/users. Admin-only. Generates a random initial
// password (same strength as the bootstrap admin), forces rotation on first
// login, and returns the password exactly once.
func CreateUser(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createUserReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		if req.Email == "" || !strings.Contains(req.Email, "@") {
			jsonError(w, http.StatusBadRequest, "a valid email is required")
			return
		}
		switch req.Role {
		case store.RoleAdmin, store.RoleMember, store.RoleViewer:
		default:
			jsonError(w, http.StatusBadRequest, "role must be admin, member, or viewer")
			return
		}
		password, err := auth.GeneratePassword()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		u, err := d.Auth.Users.Create(r.Context(), req.Email, password, req.Role, true)
		if err != nil {
			if errors.Is(err, store.ErrUserExists) {
				jsonError(w, http.StatusConflict, "a user with that email already exists")
				return
			}
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		auditAppend(r, d, "user.create", fmt.Sprintf("user:%d", u.ID), map[string]any{
			"email": u.Email,
			"role":  u.Role,
		})
		resp := userJSON(u)
		resp["password"] = password
		writeJSON(w, http.StatusCreated, resp)
	}
}

type updateRoleReq struct {
	Role string `json:"role"`
}

// UpdateUserRole handles PATCH /api/users/{id}. Admin-only.
func UpdateUserRole(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		var req updateRoleReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		u, err := d.Auth.Users.UpdateRole(r.Context(), id, req.Role)
		if err != nil {
			switch {
			case errors.Is(err, store.ErrUserNotFound):
				jsonError(w, http.StatusNotFound, "user not found")
			case errors.Is(err, store.ErrLastAdmin):
				jsonError(w, http.StatusConflict, "cannot demote the last admin")
			default:
				jsonError(w, http.StatusBadRequest, "invalid role")
			}
			return
		}
		auditAppend(r, d, "user.role", fmt.Sprintf("user:%d", id), map[string]any{
			"email": u.Email,
			"role":  u.Role,
		})
		writeJSON(w, http.StatusOK, userJSON(u))
	}
}

// DeleteUser handles DELETE /api/users/{id}. Admin-only. Refuses to delete the
// requesting admin's own account or the last remaining admin.
func DeleteUser(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		if self, ok := auth.UserFromContext(r.Context()); ok && self.ID == id {
			jsonError(w, http.StatusConflict, "you cannot delete your own account")
			return
		}
		if err := d.Auth.Users.Delete(r.Context(), id); err != nil {
			switch {
			case errors.Is(err, store.ErrUserNotFound):
				jsonError(w, http.StatusNotFound, "user not found")
			case errors.Is(err, store.ErrLastAdmin):
				jsonError(w, http.StatusConflict, "cannot delete the last admin")
			default:
				jsonError(w, http.StatusInternalServerError, "internal error")
			}
			return
		}
		auditAppend(r, d, "user.delete", fmt.Sprintf("user:%d", id), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

// ResetUserPassword handles POST /api/users/{id}/reset-password. Admin-only.
// Generates a fresh random password, forces rotation, returns it once.
func ResetUserPassword(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		if _, err := d.Auth.Users.GetByID(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				jsonError(w, http.StatusNotFound, "user not found")
				return
			}
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		password, err := auth.GeneratePassword()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if err := d.Auth.Users.SetPassword(r.Context(), id, password, true); err != nil {
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		auditAppend(r, d, "user.password.reset", fmt.Sprintf("user:%d", id), nil)
		writeJSON(w, http.StatusOK, map[string]any{"password": password})
	}
}

type changePasswordReq struct {
	Current string `json:"current"`
	New     string `json:"new"`
}

// ChangeOwnPassword handles POST /api/me/password for any authenticated user.
// Verifies the current password, sets the new one, and clears must_rotate so
// the forced-rotation gate is satisfied.
func ChangeOwnPassword(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		self, ok := auth.UserFromContext(r.Context())
		if !ok {
			jsonError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		var req changePasswordReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if len(req.New) < 12 {
			jsonError(w, http.StatusBadRequest, "new password must be at least 12 characters")
			return
		}
		if _, err := auth.VerifyPassword(r.Context(), d.Auth.Users, self.Email, req.Current); err != nil {
			jsonError(w, http.StatusUnauthorized, "current password is incorrect")
			return
		}
		if err := d.Auth.Users.SetPassword(r.Context(), self.ID, req.New, false); err != nil {
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		auditAppend(r, d, "user.password.change", fmt.Sprintf("user:%d", self.ID), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}
