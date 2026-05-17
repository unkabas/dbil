package observ

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/unkabas/dbil/internal/postgres"
	"github.com/unkabas/dbil/internal/store"
)

// OverviewCollector snapshots pg_stat_database + pg_stat_activity +
// pg_stat_replication per tick. TPS is computed from the delta of
// (xact_commit + xact_rollback) across ticks; the first tick records
// the baseline and produces TPS=0.
type OverviewCollector struct {
	ConnID int64
	Repo   *store.ObservabilityRepo

	mu   sync.Mutex
	prev *overviewPrev
}

type overviewPrev struct {
	xacts int64
	at    time.Time
}

const overviewQuery = `SELECT
    (SELECT COALESCE(SUM(xact_commit + xact_rollback), 0)::bigint
        FROM pg_stat_database)                                       AS xacts,
    (SELECT COALESCE(SUM(blks_hit)::float8 / NULLIF(SUM(blks_hit + blks_read), 0), 1.0)
        FROM pg_stat_database)                                       AS cache_hit,
    (SELECT count(*)::int FROM pg_stat_activity WHERE state = 'active') AS active,
    (SELECT count(*)::int FROM pg_stat_activity WHERE state = 'idle')   AS idle,
    (SELECT (EXTRACT(EPOCH FROM MAX(replay_lag)) * 1000)::int
        FROM pg_stat_replication)                                    AS rep_lag_ms`

// Name implements Collector.
func (c *OverviewCollector) Name() CollectorName { return CollectorOverview }

// Tick implements Collector.
func (c *OverviewCollector) Tick(ctx context.Context, pool postgres.Pool) error {
	res, err := pool.Execute(ctx, overviewQuery)
	if err != nil {
		return err
	}
	if len(res.Rows) != 1 {
		return errors.New("overview collector: expected exactly one row")
	}
	row := res.Rows[0]
	if len(row) < 5 {
		return errors.New("overview collector: short row")
	}

	xacts := asInt64(row[0])
	cacheHit := asFloat64(row[1])
	active := asInt(row[2])
	idle := asInt(row[3])
	var repLag sql.NullInt64
	if row[4] != nil {
		repLag.Valid = true
		repLag.Int64 = asInt64(row[4])
	}

	now := time.Now()

	c.mu.Lock()
	var tps float64
	if c.prev != nil {
		dt := now.Sub(c.prev.at).Seconds()
		if dt > 0 {
			diff := xacts - c.prev.xacts
			if diff >= 0 {
				tps = float64(diff) / dt
			}
		}
	}
	c.prev = &overviewPrev{xacts: xacts, at: now}
	c.mu.Unlock()

	return c.Repo.RecordOverview(ctx, store.OverviewSample{
		ConnID:      c.ConnID,
		TS:          now,
		TPS:         tps,
		CacheHit:    cacheHit,
		ActiveConns: active,
		IdleConns:   idle,
		RepLagMs:    repLag,
	})
}
