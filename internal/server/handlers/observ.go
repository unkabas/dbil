package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/unkabas/dbil/internal/observ"
	"github.com/unkabas/dbil/internal/postgres"
	"github.com/unkabas/dbil/internal/store"
)

// OverviewHandler — GET /api/connections/{id}/observ/overview?since=<unix_ms>
// Returns the recorded overview samples. Without ?since, returns the last
// five minutes.
func OverviewHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		since := time.Now().Add(-5 * time.Minute)
		if raw := r.URL.Query().Get("since"); raw != "" {
			if ms, err := strconv.ParseInt(raw, 10, 64); err == nil {
				since = time.Unix(0, ms*int64(time.Millisecond))
			}
		}
		rows, err := d.Observ.ListOverviewSince(r.Context(), id, since)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		out := make([]map[string]any, len(rows))
		for i, s := range rows {
			entry := map[string]any{
				"ts_ms":        s.TS.UnixMilli(),
				"tps":          s.TPS,
				"cache_hit":    s.CacheHit,
				"active_conns": s.ActiveConns,
				"idle_conns":   s.IdleConns,
			}
			if s.RepLagMs.Valid {
				entry["rep_lag_ms"] = s.RepLagMs.Int64
			}
			out[i] = entry
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"samples": out})
	}
}

// SlowQueriesHandler — GET /api/connections/{id}/observ/slow
// Returns the latest pg_stat_statements snapshot for the connection.
func SlowQueriesHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		takenAt, rows, err := d.Observ.LatestSlow(r.Context(), id)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		out := make([]map[string]any, len(rows))
		for i, s := range rows {
			out[i] = map[string]any{
				"query_hash": s.QueryHash,
				"preview":    s.Preview,
				"mean_ms":    s.MeanMs,
				"p95_ms":     s.P95Ms,
				"p99_ms":     s.P99Ms,
				"calls":      s.Calls,
				"total_ms":   s.TotalMs,
				"rows_avg":   s.RowsAvg,
			}
		}
		var takenAtMs int64
		if !takenAt.IsZero() {
			takenAtMs = takenAt.UnixMilli()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"taken_at_ms": takenAtMs,
			"rows":        out,
		})
	}
}

// LocksHandler — GET /api/connections/{id}/observ/locks
// Runs a live pg_stat_activity + pg_blocking_pids query against the
// target Postgres. Optional X-Connection-Passphrase header for
// passphrase-protected connections.
func LocksHandler(d Deps) http.HandlerFunc {
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
		chains, err := observ.ListLockChains(r.Context(), pool)
		if err != nil {
			jsonError(w, http.StatusBadGateway, "list locks failed: "+err.Error())
			return
		}
		if chains == nil {
			chains = []observ.LockChain{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"chains": chains})
	}
}

func respondPoolOpenError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrConnectionNotFound):
		jsonError(w, http.StatusNotFound, "connection not found")
	case errors.Is(err, store.ErrPassphraseRequired):
		jsonErrorWithReason(w, http.StatusPreconditionRequired, "passphrase required", "")
	case errors.Is(err, store.ErrInvalidPassphrase):
		jsonError(w, http.StatusUnauthorized, "invalid passphrase")
	default:
		jsonError(w, http.StatusBadGateway, "open failed: "+err.Error())
	}
	_ = postgres.NewPGX // ensure import (linter happy when only used for typed errors)
}
