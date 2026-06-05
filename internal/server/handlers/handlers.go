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
	Auth       auth.Deps
	Conns      *store.ConnectionsRepo
	SSHHosts   *store.SSHHostsRepo
	Manager    *postgres.Manager
	Observ     *store.ObservabilityRepo
	ObservMgr  *observ.Manager
	Discovered *store.DiscoveredRepo
	Version    string
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
	r.Get("/api/openapi.yaml", OpenAPIHandler())
	r.Get("/api/docs", SwaggerUIHandler())

	// --- Authed: every handler under this group is protected by RequireAuth ---
	r.Group(func(p chi.Router) {
		p.Use(auth.RequireAuth(d.Auth))
		p.Post("/api/auth/logout", LogoutHandler(d.Auth))
		p.Get("/api/me", MeHandler())
		p.Post("/api/me/password", ChangeOwnPassword(d))

		// User management — admin only (RequireRole composes over RequireAuth).
		p.Group(func(a chi.Router) {
			a.Use(auth.RequireRole(store.RoleAdmin))
			a.Get("/api/users", ListUsers(d))
			a.Post("/api/users", CreateUser(d))
			a.Patch("/api/users/{id}", UpdateUserRole(d))
			a.Delete("/api/users/{id}", DeleteUser(d))
			a.Post("/api/users/{id}/reset-password", ResetUserPassword(d))
		})

		p.Get("/api/connections", ListConnections(d))
		p.Get("/api/connections/{id}", GetConnection(d))
		p.Post("/api/connections/{id}/test", TestConnection(d))
		p.Post("/api/connections/{id}/query", QueryHandler(d))

		p.Get("/api/connections/{id}/observ/overview", OverviewHandler(d))
		p.Get("/api/connections/{id}/observ/slow", SlowQueriesHandler(d))
		p.Get("/api/connections/{id}/observ/locks", LocksHandler(d))
		p.Post("/api/connections/{id}/locks/{pid}/terminate", TerminateBackendHandler(d))
		p.Get("/api/connections/{id}/observ/advisor", AdvisorHandler(d))

		p.Get("/api/connections/{id}/schema", SchemaHandler(d))
		p.Get("/api/connections/{id}/table/{schema}/{name}/rows", RowsHandler(d))
		p.Post("/api/connections/{id}/table/{schema}/{name}/rows/search", SearchRowsHandler(d))
		p.Post("/api/connections/{id}/table/{schema}/{name}/columns/{column}/values", DistinctValuesHandler(d))
		p.Post("/api/connections/{id}/table/{schema}/{name}/export", ExportTableHandler(d))

		// Inline data editing + SSH-host management — writers only (admin,
		// member). viewer is read-only and never reaches these handlers.
		p.Group(func(wr chi.Router) {
			wr.Use(auth.RequireRole(store.RoleAdmin, store.RoleMember))
			wr.Post("/api/connections", CreateConnection(d))
			wr.Delete("/api/connections/{id}", DeleteConnection(d))
			wr.Post("/api/connections/{id}/table/{schema}/{name}/mutations", MutateTableHandler(d))

			wr.Get("/api/ssh-hosts", ListSSHHosts(d))
			wr.Post("/api/ssh-hosts", CreateSSHHost(d))
			wr.Delete("/api/ssh-hosts/{id}", DeleteSSHHost(d))
			wr.Post("/api/ssh-hosts/{id}/test", TestSSHHost(d))
		})

		p.Get("/api/discover", ListDiscoverHandler(d))
		p.Post("/api/discover/{id}/approve", ApproveDiscoverHandler(d))
		p.Post("/api/discover/{id}/reject", RejectDiscoverHandler(d))
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
