package handlers

import (
	"context"
	"sync"
	"time"

	"github.com/unkabas/dbil/internal/postgres"
)

// Capabilities lists optional server features that observability cares about.
// Empty / unknown values mean "could not detect" — the UI then falls back to
// generic hints instead of misleading specifics.
type Capabilities struct {
	ReplicationConfigured bool   `json:"replication_configured"`
	ReplicationReason     string `json:"replication_reason,omitempty"`
	PgStatStatements      bool   `json:"pg_stat_statements_installed"`
	PgStatStatementsKnown bool   `json:"pg_stat_statements_known"`
	PgStatStatementsHint  string `json:"pg_stat_statements_hint,omitempty"`
}

type capCacheEntry struct {
	caps Capabilities
	exp  time.Time
}

var (
	capCache    sync.Map // connID -> *capCacheEntry
	capCacheTTL = 30 * time.Second
)

// fetchCapabilities probes the target Postgres for the bits used in the
// observability UI. Results are cached per (connID, pool) for capCacheTTL —
// the polled overview endpoint runs this on every tick and we don't want a
// fresh capability query every 5 seconds on production.
//
// On any error the returned Capabilities is zero-valued; the caller treats
// that as "unknown" and the UI falls back to a generic install message.
func fetchCapabilities(ctx context.Context, connID int64, pool postgres.Pool) Capabilities {
	if entry, ok := capCache.Load(connID); ok {
		e := entry.(*capCacheEntry)
		if time.Now().Before(e.exp) {
			return e.caps
		}
	}
	caps := probeCapabilities(ctx, pool)
	capCache.Store(connID, &capCacheEntry{caps: caps, exp: time.Now().Add(capCacheTTL)})
	return caps
}

func probeCapabilities(ctx context.Context, pool postgres.Pool) Capabilities {
	var caps Capabilities

	// Replication: any row in pg_stat_replication means at least one standby
	// is connected. This is cheaper than a join and works on every PG 9+.
	if res, err := pool.Execute(ctx, "SELECT count(*) FROM pg_stat_replication"); err == nil &&
		len(res.Rows) > 0 && len(res.Rows[0]) > 0 {
		n := asCapInt64(res.Rows[0][0])
		if n > 0 {
			caps.ReplicationConfigured = true
		} else {
			caps.ReplicationReason = "no standby connected"
		}
	} else {
		caps.ReplicationReason = "could not query pg_stat_replication"
	}

	// pg_stat_statements: try the boolean from pg_extension first (cheap, no
	// permission issues). pg_available_extensions is best-effort because
	// managed services (RDS, Heroku) often restrict it.
	if res, err := pool.Execute(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements')",
	); err == nil && len(res.Rows) > 0 && len(res.Rows[0]) > 0 {
		caps.PgStatStatementsKnown = true
		if b, ok := res.Rows[0][0].(bool); ok && b {
			caps.PgStatStatements = true
		}
	}

	if !caps.PgStatStatements {
		// Try to detect if the extension is at least available on disk.
		avail := false
		if res, err := pool.Execute(ctx,
			"SELECT EXISTS(SELECT 1 FROM pg_available_extensions WHERE name = 'pg_stat_statements')",
		); err == nil && len(res.Rows) > 0 && len(res.Rows[0]) > 0 {
			if b, ok := res.Rows[0][0].(bool); ok && b {
				avail = true
			}
		}
		if avail {
			caps.PgStatStatementsHint = "Run: CREATE EXTENSION pg_stat_statements; " +
				"(needs shared_preload_libraries — restart required)"
		} else {
			caps.PgStatStatementsHint = "pg_stat_statements is not available; ask your DBA to enable it."
		}
	}

	return caps
}

func asCapInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int32:
		return int64(t)
	case int:
		return int64(t)
	}
	return 0
}

