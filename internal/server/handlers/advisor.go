package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/unkabas/dbil/internal/observ"
)

// AdvisorHandler — GET /api/connections/{id}/observ/advisor
//
// Runs the live missing-index + unused-index heuristics against the
// target Postgres. Identical pool plumbing to LocksHandler; honours the
// X-Connection-Passphrase header for passphrase-protected connections.
func AdvisorHandler(d Deps) http.HandlerFunc {
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
		rep, err := observ.RunAdvisor(r.Context(), pool)
		if err != nil {
			jsonError(w, http.StatusBadGateway, "advisor failed: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rep)
	}
}
