package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/unkabas/dbil/internal/auth"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// LoginHandler handles POST /api/auth/login. No auth required (allow-listed
// in scripts/lint-auth).
func LoginHandler(d auth.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Email == "" || req.Password == "" {
			jsonError(w, http.StatusBadRequest, "email and password are required")
			return
		}
		res, err := auth.Login(r.Context(), d, req.Email, req.Password,
			r.Header.Get("User-Agent"), auth.RemoteIP(r))
		if err != nil {
			if errors.Is(err, auth.ErrInvalidCredentials) {
				jsonError(w, http.StatusUnauthorized, "invalid credentials")
				return
			}
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(loginResponse{
			Token:     res.Token,
			ExpiresAt: res.ExpiresAt.Unix(),
		})
	}
}

// LogoutHandler handles POST /api/auth/logout. Mounted under RequireAuth.
func LogoutHandler(d auth.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.UserFromContext(r.Context())
		if !ok {
			jsonError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		token, _ := auth.TokenFromContext(r.Context())
		if err := auth.Logout(r.Context(), d, user, token); err != nil {
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// MeHandler handles GET /api/me. Mounted under RequireAuth. Returns the
// authenticated user's profile (no password material).
func MeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.UserFromContext(r.Context())
		if !ok {
			jsonError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":          user.ID,
			"email":       user.Email,
			"role":        user.Role,
			"must_rotate": user.MustRotate,
		})
	}
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
