// Package handlers contains DBil's HTTP handlers and the Mount entry point
// that composes them onto a chi router. The Mount function is the single
// place where auth gates are wired; the lint at scripts/lint-auth asserts
// that every handler in this package is either inside the requireAuth group
// or in the explicit unauthed allowlist (Healthz, LoginHandler).
package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/unkabas/dbil/internal/auth"
)

// Mount composes all DBil routes onto a fresh chi.Router and returns it.
func Mount(d auth.Deps, version string) chi.Router {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(slogRequestLogger())

	// --- Unauthed (allowlisted in scripts/lint-auth) ---
	r.Get("/healthz", Healthz(version))
	r.Post("/api/auth/login", LoginHandler(d))

	// --- Authed: every handler under this group is protected by RequireAuth ---
	r.Group(func(p chi.Router) {
		p.Use(auth.RequireAuth(d))
		p.Post("/api/auth/logout", LogoutHandler(d))
		p.Get("/api/me", MeHandler())
	})

	return r
}

// slogRequestLogger emits one structured log line per request.
func slogRequestLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			slog.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"dur_ms", time.Since(start).Milliseconds(),
				"request_id", chimw.GetReqID(r.Context()),
			)
		})
	}
}
