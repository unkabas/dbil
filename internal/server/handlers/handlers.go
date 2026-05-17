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
	"github.com/unkabas/dbil/internal/observ"
	"github.com/unkabas/dbil/internal/postgres"
	"github.com/unkabas/dbil/internal/store"
)

// Deps bundles everything HTTP handlers need from the storage and postgres
// layers. Constructed once at server start, passed to Mount.
type Deps struct {
	Auth      auth.Deps
	Conns     *store.ConnectionsRepo
	Manager   *postgres.Manager
	Observ    *store.ObservabilityRepo
	ObservMgr *observ.Manager
	Version   string
}

// Mount composes all DBil routes onto a fresh chi.Router and returns it.
func Mount(d Deps) chi.Router {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(slogRequestLogger())

	// --- Unauthed (allowlisted in scripts/lint-auth) ---
	r.Get("/healthz", Healthz(d.Version))
	r.Post("/api/auth/login", LoginHandler(d.Auth))

	// --- Authed: every handler under this group is protected by RequireAuth ---
	r.Group(func(p chi.Router) {
		p.Use(auth.RequireAuth(d.Auth))
		p.Post("/api/auth/logout", LogoutHandler(d.Auth))
		p.Get("/api/me", MeHandler())

		p.Get("/api/connections", ListConnections(d))
		p.Post("/api/connections", CreateConnection(d))
		p.Get("/api/connections/{id}", GetConnection(d))
		p.Delete("/api/connections/{id}", DeleteConnection(d))
		p.Post("/api/connections/{id}/test", TestConnection(d))
		p.Post("/api/connections/{id}/query", QueryHandler(d))

		p.Get("/api/connections/{id}/observ/overview", OverviewHandler(d))
		p.Get("/api/connections/{id}/observ/slow", SlowQueriesHandler(d))
		p.Get("/api/connections/{id}/observ/locks", LocksHandler(d))

		p.Get("/api/connections/{id}/schema", SchemaHandler(d))
		p.Get("/api/connections/{id}/table/{schema}/{name}/rows", RowsHandler(d))
	})

	// --- Static SPA (embedded React bundle) ---
	// chi's NotFound and MethodNotAllowed catch anything the router above
	// didn't match. SPAHandler rejects /api/* and /healthz with a JSON 404 so
	// API misses don't return HTML; everything else falls back to index.html
	// so client-side React Router routes survive reloads.
	r.NotFound(SPAHandler().ServeHTTP)
	r.MethodNotAllowed(SPAHandler().ServeHTTP)

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
