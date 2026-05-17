package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// OverviewSample is one 10-second tick of pg_stat_database-derived metrics.
type OverviewSample struct {
	ConnID      int64
	TS          time.Time
	TPS         float64
	CacheHit    float64 // 0..1
	ActiveConns int
	IdleConns   int
	RepLagMs    sql.NullInt64 // null when no replica is connected
}

// SlowRow is one row of a pg_stat_statements snapshot. Multiple SlowRows
// share the same TakenAt across a single snapshot.
type SlowRow struct {
	ConnID    int64
	TakenAt   time.Time
	QueryHash string
	Preview   string
	MeanMs    float64
	P95Ms     float64
	P99Ms     float64
	Calls     int64
	TotalMs   float64
	RowsAvg   float64
}

// ObservabilityRepo persists overview samples and slow-query snapshots and
// provides retention helpers (the Janitor sweep).
type ObservabilityRepo struct {
	DB *sql.DB
}

// NewObservabilityRepo binds the repo to db.
func NewObservabilityRepo(db *sql.DB) *ObservabilityRepo {
	return &ObservabilityRepo{DB: db}
}

// RecordOverview inserts a single tick; (conn_id, ts_ns) is the primary key,
// so a re-insertion for the same conn at the same ts (rare clock collision)
// is silently replaced.
func (r *ObservabilityRepo) RecordOverview(ctx context.Context, s OverviewSample) error {
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO overview_samples
			(conn_id, ts_ns, tps, cache_hit, active_conns, idle_conns, rep_lag_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(conn_id, ts_ns) DO UPDATE SET
			tps = excluded.tps,
			cache_hit = excluded.cache_hit,
			active_conns = excluded.active_conns,
			idle_conns = excluded.idle_conns,
			rep_lag_ms = excluded.rep_lag_ms`,
		s.ConnID, s.TS.UnixNano(), s.TPS, s.CacheHit, s.ActiveConns, s.IdleConns, s.RepLagMs,
	)
	if err != nil {
		return fmt.Errorf("observability: record overview: %w", err)
	}
	return nil
}

// ListOverviewSince returns samples for connID with ts >= since, ascending.
func (r *ObservabilityRepo) ListOverviewSince(ctx context.Context, connID int64, since time.Time) ([]OverviewSample, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT conn_id, ts_ns, tps, cache_hit, active_conns, idle_conns, rep_lag_ms
		FROM overview_samples
		WHERE conn_id = ? AND ts_ns >= ?
		ORDER BY ts_ns ASC`,
		connID, since.UnixNano(),
	)
	if err != nil {
		return nil, fmt.Errorf("observability: list overview: %w", err)
	}
	defer rows.Close()
	var out []OverviewSample
	for rows.Next() {
		var (
			s    OverviewSample
			tsNS int64
		)
		if err := rows.Scan(&s.ConnID, &tsNS, &s.TPS, &s.CacheHit, &s.ActiveConns, &s.IdleConns, &s.RepLagMs); err != nil {
			return nil, fmt.Errorf("observability: list overview scan: %w", err)
		}
		s.TS = time.Unix(0, tsNS)
		out = append(out, s)
	}
	return out, rows.Err()
}

// SnapshotSlow inserts a slow-query snapshot in a single transaction. Every
// row must share the same (ConnID, TakenAt) so a query later for "latest
// snapshot" can group by that pair.
func (r *ObservabilityRepo) SnapshotSlow(ctx context.Context, rows []SlowRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("observability: snapshot slow begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, s := range rows {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO slow_query_snaps
				(conn_id, taken_at_ns, query_hash, preview, mean_ms, p95_ms, p99_ms, calls, total_ms, rows_avg)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(conn_id, taken_at_ns, query_hash) DO UPDATE SET
				preview = excluded.preview,
				mean_ms = excluded.mean_ms,
				p95_ms = excluded.p95_ms,
				p99_ms = excluded.p99_ms,
				calls = excluded.calls,
				total_ms = excluded.total_ms,
				rows_avg = excluded.rows_avg`,
			s.ConnID, s.TakenAt.UnixNano(), s.QueryHash, s.Preview,
			s.MeanMs, s.P95Ms, s.P99Ms, s.Calls, s.TotalMs, s.RowsAvg,
		)
		if err != nil {
			return fmt.Errorf("observability: snapshot slow insert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("observability: snapshot slow commit: %w", err)
	}
	return nil
}

// LatestSlow returns the rows from the latest snapshot for connID.
// Returns (zero time, nil, nil) when no snapshots have been recorded yet.
func (r *ObservabilityRepo) LatestSlow(ctx context.Context, connID int64) (time.Time, []SlowRow, error) {
	var latest sql.NullInt64
	if err := r.DB.QueryRowContext(ctx, `
		SELECT MAX(taken_at_ns) FROM slow_query_snaps WHERE conn_id = ?`,
		connID,
	).Scan(&latest); err != nil {
		return time.Time{}, nil, fmt.Errorf("observability: latest slow: %w", err)
	}
	if !latest.Valid {
		return time.Time{}, nil, nil
	}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT conn_id, taken_at_ns, query_hash, preview, mean_ms, p95_ms, p99_ms, calls, total_ms, rows_avg
		FROM slow_query_snaps
		WHERE conn_id = ? AND taken_at_ns = ?
		ORDER BY total_ms DESC`,
		connID, latest.Int64,
	)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("observability: latest slow query: %w", err)
	}
	defer rows.Close()
	var out []SlowRow
	for rows.Next() {
		var s SlowRow
		var takenNS int64
		if err := rows.Scan(&s.ConnID, &takenNS, &s.QueryHash, &s.Preview, &s.MeanMs, &s.P95Ms, &s.P99Ms, &s.Calls, &s.TotalMs, &s.RowsAvg); err != nil {
			return time.Time{}, nil, fmt.Errorf("observability: latest slow scan: %w", err)
		}
		s.TakenAt = time.Unix(0, takenNS)
		out = append(out, s)
	}
	return time.Unix(0, latest.Int64), out, rows.Err()
}

// Janitor sweeps stale rows: overview older than 24 h, slow snaps older
// than 1 h (we only need the latest snapshot). Returns the (overviewDeleted,
// slowDeleted) counts.
func (r *ObservabilityRepo) Janitor(ctx context.Context, now time.Time) (int64, int64, error) {
	ovCutoff := now.Add(-24 * time.Hour).UnixNano()
	slowCutoff := now.Add(-1 * time.Hour).UnixNano()

	ov, err := r.DB.ExecContext(ctx, `DELETE FROM overview_samples WHERE ts_ns < ?`, ovCutoff)
	if err != nil {
		return 0, 0, fmt.Errorf("observability: janitor overview: %w", err)
	}
	slow, err := r.DB.ExecContext(ctx, `DELETE FROM slow_query_snaps WHERE taken_at_ns < ?`, slowCutoff)
	if err != nil {
		return 0, 0, fmt.Errorf("observability: janitor slow: %w", err)
	}
	ovN, _ := ov.RowsAffected()
	slowN, _ := slow.RowsAffected()
	return ovN, slowN, nil
}
