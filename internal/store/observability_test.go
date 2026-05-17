package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/unkabas/dbil/internal/crypto"
)

func setupObservability(t *testing.T) (*ObservabilityRepo, int64, *sql.DB) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close(db) })
	if err := Apply(db); err != nil {
		t.Fatal(err)
	}
	mkB, _ := crypto.Random(crypto.KeySize)
	mk, _ := crypto.NewMasterKey(mkB)
	conns := NewConnectionsRepo(db, mk)
	c, err := conns.Create(context.Background(), CreateConnectionParams{
		Alias: "p", Host: "h", Port: 5432, Tag: TagLocal, TLSMode: TLSDisable,
		Username: "u", Password: "pw", Database: "d",
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewObservabilityRepo(db), c.ID, db
}

func TestObservability_OverviewRoundTrip(t *testing.T) {
	r, connID, _ := setupObservability(t)
	ctx := context.Background()
	now := time.Now()

	for i := 0; i < 5; i++ {
		err := r.RecordOverview(ctx, OverviewSample{
			ConnID: connID, TS: now.Add(time.Duration(i) * 10 * time.Second),
			TPS: float64(100 + i*10), CacheHit: 0.99, ActiveConns: 3, IdleConns: 5,
			RepLagMs: sql.NullInt64{Valid: true, Int64: int64(20 + i)},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	got, err := r.ListOverviewSince(ctx, connID, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("want 5 samples, got %d", len(got))
	}
	if got[0].TPS != 100 || got[4].TPS != 140 {
		t.Fatalf("TPS not preserved: first=%v last=%v", got[0].TPS, got[4].TPS)
	}
	if !got[0].RepLagMs.Valid || got[0].RepLagMs.Int64 != 20 {
		t.Fatalf("rep_lag_ms not preserved: %+v", got[0].RepLagMs)
	}
}

func TestObservability_OverviewSince(t *testing.T) {
	r, connID, _ := setupObservability(t)
	ctx := context.Background()
	now := time.Now()
	for i := 0; i < 6; i++ {
		_ = r.RecordOverview(ctx, OverviewSample{
			ConnID: connID, TS: now.Add(time.Duration(i) * time.Second),
			TPS: 1, CacheHit: 1, ActiveConns: 1, IdleConns: 1,
		})
	}
	got, err := r.ListOverviewSince(ctx, connID, now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 samples >= +3s, got %d", len(got))
	}
}

func TestObservability_SlowSnapshotAndLatest(t *testing.T) {
	r, connID, _ := setupObservability(t)
	ctx := context.Background()
	first := time.Now()

	// First snapshot
	if err := r.SnapshotSlow(ctx, []SlowRow{
		{ConnID: connID, TakenAt: first, QueryHash: "a", Preview: "SELECT 1", MeanMs: 1, P95Ms: 2, P99Ms: 3, Calls: 10, TotalMs: 10, RowsAvg: 1},
		{ConnID: connID, TakenAt: first, QueryHash: "b", Preview: "INSERT INTO x", MeanMs: 4, P95Ms: 7, P99Ms: 9, Calls: 5, TotalMs: 20, RowsAvg: 1},
	}); err != nil {
		t.Fatal(err)
	}

	// Second snapshot 1 minute later — overwrites which one is "latest".
	second := first.Add(1 * time.Minute)
	if err := r.SnapshotSlow(ctx, []SlowRow{
		{ConnID: connID, TakenAt: second, QueryHash: "a", Preview: "SELECT 1", MeanMs: 5, P95Ms: 6, P99Ms: 7, Calls: 12, TotalMs: 60, RowsAvg: 1},
	}); err != nil {
		t.Fatal(err)
	}

	takenAt, rows, err := r.LatestSlow(ctx, connID)
	if err != nil {
		t.Fatal(err)
	}
	if takenAt.UnixNano() != second.UnixNano() {
		t.Fatalf("LatestSlow takenAt: want %v, got %v", second, takenAt)
	}
	if len(rows) != 1 || rows[0].QueryHash != "a" || rows[0].MeanMs != 5 {
		t.Fatalf("LatestSlow rows mismatch: %+v", rows)
	}
}

func TestObservability_LatestSlowEmpty(t *testing.T) {
	r, connID, _ := setupObservability(t)
	takenAt, rows, err := r.LatestSlow(context.Background(), connID)
	if err != nil {
		t.Fatal(err)
	}
	if !takenAt.IsZero() {
		t.Fatalf("want zero time on empty, got %v", takenAt)
	}
	if rows != nil {
		t.Fatalf("want nil rows on empty, got %+v", rows)
	}
}

func TestObservability_Janitor(t *testing.T) {
	r, connID, _ := setupObservability(t)
	ctx := context.Background()
	now := time.Now()

	// Old overview (25 h ago) + new overview (now) → only old deleted.
	_ = r.RecordOverview(ctx, OverviewSample{ConnID: connID, TS: now.Add(-25 * time.Hour), TPS: 1, CacheHit: 1, ActiveConns: 1, IdleConns: 1})
	_ = r.RecordOverview(ctx, OverviewSample{ConnID: connID, TS: now, TPS: 2, CacheHit: 1, ActiveConns: 1, IdleConns: 1})

	// Old slow snap (2 h ago) + new slow snap (now).
	_ = r.SnapshotSlow(ctx, []SlowRow{{ConnID: connID, TakenAt: now.Add(-2 * time.Hour), QueryHash: "old", Preview: "x", Calls: 1, TotalMs: 1, MeanMs: 1, P95Ms: 1, P99Ms: 1, RowsAvg: 1}})
	_ = r.SnapshotSlow(ctx, []SlowRow{{ConnID: connID, TakenAt: now, QueryHash: "new", Preview: "y", Calls: 1, TotalMs: 1, MeanMs: 1, P95Ms: 1, P99Ms: 1, RowsAvg: 1}})

	ovN, slowN, err := r.Janitor(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if ovN != 1 || slowN != 1 {
		t.Fatalf("janitor counts: want (1,1), got (%d,%d)", ovN, slowN)
	}

	// Verify the "new" rows are still present.
	got, _ := r.ListOverviewSince(ctx, connID, now.Add(-1*time.Hour))
	if len(got) != 1 || got[0].TPS != 2 {
		t.Fatalf("janitor wiped fresh data: %+v", got)
	}
}

func TestObservability_CascadeOnConnDelete(t *testing.T) {
	r, connID, db := setupObservability(t)
	ctx := context.Background()
	_ = r.RecordOverview(ctx, OverviewSample{ConnID: connID, TS: time.Now(), TPS: 1, CacheHit: 1, ActiveConns: 1, IdleConns: 1})
	_ = r.SnapshotSlow(ctx, []SlowRow{{ConnID: connID, TakenAt: time.Now(), QueryHash: "a", Preview: "x", Calls: 1, TotalMs: 1, MeanMs: 1, P95Ms: 1, P99Ms: 1, RowsAvg: 1}})

	// Delete the connection directly via raw SQL (mirrors what ConnectionsRepo
	// does); the FK cascade should clean both observability tables.
	if _, err := db.ExecContext(ctx, `DELETE FROM connections WHERE id = ?`, connID); err != nil {
		t.Fatal(err)
	}

	got, _ := r.ListOverviewSince(ctx, connID, time.Time{})
	if len(got) != 0 {
		t.Fatalf("overview not cascaded: %d rows left", len(got))
	}
	_, slow, _ := r.LatestSlow(ctx, connID)
	if len(slow) != 0 {
		t.Fatalf("slow not cascaded: %d rows left", len(slow))
	}
}
