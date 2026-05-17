package observ

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/unkabas/dbil/internal/postgres"
	"github.com/unkabas/dbil/internal/store"
)

// SlowQueriesCollector polls pg_stat_statements for the top 100 by total
// execution time. When the extension is not installed, it logs once and
// becomes a no-op until a future tick (operator may install the extension
// without restarting dbil).
type SlowQueriesCollector struct {
	ConnID int64
	Repo   *store.ObservabilityRepo

	missingLogged atomic.Bool
}

// Name implements Collector.
func (c *SlowQueriesCollector) Name() CollectorName { return CollectorSlow }

// Tick implements Collector. Returns nil silently when pg_stat_statements
// is not installed; logs the situation once per process.
func (c *SlowQueriesCollector) Tick(ctx context.Context, pool postgres.Pool) error {
	// Cheap existence check first — avoids a planner error on databases
	// without the extension.
	exists, err := pool.Execute(ctx, `SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements')::bool`)
	if err != nil {
		return err
	}
	if len(exists.Rows) != 1 || !asBool(exists.Rows[0][0]) {
		if c.missingLogged.CompareAndSwap(false, true) {
			slog.Info("observ: pg_stat_statements not installed; slow-queries collector skipping",
				"conn_id", c.ConnID,
				"hint", "CREATE EXTENSION pg_stat_statements after setting shared_preload_libraries")
		}
		return nil
	}
	// Reset the missing flag so a future drop-and-reinstall re-logs the event.
	c.missingLogged.Store(false)

	res, err := pool.Execute(ctx, slowQuery)
	if err != nil {
		return err
	}

	now := time.Now()
	rows := make([]store.SlowRow, 0, len(res.Rows))
	for _, r := range res.Rows {
		if len(r) < 7 {
			continue
		}
		preview := truncate(asString(r[1]), 200)
		mean := asFloat64(r[2])
		std := asFloat64(r[3])
		// Normal-distribution approximation for p95 / p99 — pg_stat_statements
		// does not expose true percentiles before extension v1.10+; we err on
		// the safe side and surface the approximate numbers in the UI.
		p95 := mean + 1.645*std
		p99 := mean + 2.326*std
		rows = append(rows, store.SlowRow{
			ConnID:    c.ConnID,
			TakenAt:   now,
			QueryHash: hashHex(asString(r[0])),
			Preview:   preview,
			MeanMs:    mean,
			P95Ms:     p95,
			P99Ms:     p99,
			Calls:     asInt64(r[4]),
			TotalMs:   asFloat64(r[5]),
			RowsAvg:   asFloat64(r[6]),
		})
	}
	return c.Repo.SnapshotSlow(ctx, rows)
}

const slowQuery = `SELECT
    queryid::text                                                      AS qid,
    query                                                              AS preview,
    mean_exec_time                                                     AS mean_ms,
    stddev_exec_time                                                   AS std_ms,
    calls                                                              AS calls,
    total_exec_time                                                    AS total_ms,
    CASE WHEN calls > 0 THEN rows::float8 / calls ELSE 0 END           AS rows_avg
FROM pg_stat_statements
WHERE calls > 0
ORDER BY total_exec_time DESC
LIMIT 100`

func hashHex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
