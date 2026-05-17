package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/unkabas/dbil/internal/pg"
)

// SchemaHandler — GET /api/connections/{id}/schema
// Runs pg_catalog introspection against the connection. Reuses the pool
// cached by the postgres.Manager so the call is cheap on the warm path.
// Optional X-Connection-Passphrase header for passphrase-protected
// connections (same as the locks endpoint).
func SchemaHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		passphrase := r.Header.Get("X-Connection-Passphrase")
		pool, err := d.Manager.OpenByID(r.Context(), id, passphrase)
		if err != nil {
			respondPoolOpenError(w, err)
			return
		}
		doc, err := pg.ListSchema(r.Context(), pool)
		if err != nil {
			jsonError(w, http.StatusBadGateway, "introspect failed: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}
}
