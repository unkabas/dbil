package observ

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unkabas/dbil/internal/crypto"
	"github.com/unkabas/dbil/internal/postgres"
	"github.com/unkabas/dbil/internal/store"
)

// fakePool implements postgres.Pool for collector tests. Each Execute is
// matched by SQL prefix to a canned Result so different tests don't have
// to wire a full mock harness.
type fakePool struct {
	mu       sync.Mutex
	executed []string
	results  map[string]*postgres.Result
	err      error
}

func (p *fakePool) Ping(_ context.Context) error              { return nil }
func (p *fakePool) Close()                                    {}
func (p *fakePool) Execute(_ context.Context, sql string) (*postgres.Result, error) {
	p.mu.Lock()
	p.executed = append(p.executed, sql)
	p.mu.Unlock()
	if p.err != nil {
		return nil, p.err
	}
	for prefix, r := range p.results {
		if len(sql) >= len(prefix) && sql[:len(prefix)] == prefix {
			return r, nil
		}
	}
	return &postgres.Result{}, nil
}

func newObservRepo(t *testing.T) (*store.ObservabilityRepo, int64) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close(db) })
	if err := store.Apply(db); err != nil {
		t.Fatal(err)
	}
	mkB, _ := crypto.Random(crypto.KeySize)
	mk, _ := crypto.NewMasterKey(mkB)
	conns := store.NewConnectionsRepo(db, mk)
	c, err := conns.Create(context.Background(), store.CreateConnectionParams{
		Alias: "p", Host: "h", Port: 5432, Tag: store.TagLocal, TLSMode: store.TLSDisable,
		Username: "u", Password: "pw", Database: "d",
	})
	if err != nil {
		t.Fatal(err)
	}
	return store.NewObservabilityRepo(db), c.ID
}

func TestOverviewCollector_TickRecords(t *testing.T) {
	repo, connID := newObservRepo(t)
	pool := &fakePool{results: map[string]*postgres.Result{
		"SELECT\n    (SELECT": {
			Rows: [][]any{{int64(42), 0.99, int(3), int(5), int64(15)}},
		},
	}}
	c := &OverviewCollector{ConnID: connID, Repo: repo}

	if err := c.Tick(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	samples, err := repo.ListOverviewSince(context.Background(), connID, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 {
		t.Fatalf("want 1 sample, got %d", len(samples))
	}
	if samples[0].CacheHit != 0.99 || samples[0].ActiveConns != 3 || samples[0].IdleConns != 5 {
		t.Fatalf("sample mismatch: %+v", samples[0])
	}
	// First tick must produce TPS = 0 (no previous baseline).
	if samples[0].TPS != 0 {
		t.Fatalf("first-tick TPS should be 0, got %v", samples[0].TPS)
	}
}

func TestOverviewCollector_TPSAcrossTicks(t *testing.T) {
	repo, connID := newObservRepo(t)
	// First tick: 100 xacts. Second tick (after sleep): 1000 xacts.
	pool := &fakePool{results: map[string]*postgres.Result{
		"SELECT\n    (SELECT": {
			Rows: [][]any{{int64(100), 0.98, int(2), int(4), nil}},
		},
	}}
	c := &OverviewCollector{ConnID: connID, Repo: repo}
	if err := c.Tick(context.Background(), pool); err != nil {
		t.Fatal(err)
	}

	time.Sleep(30 * time.Millisecond)
	pool.results["SELECT\n    (SELECT"] = &postgres.Result{
		Rows: [][]any{{int64(1000), 0.97, int(2), int(4), nil}},
	}
	if err := c.Tick(context.Background(), pool); err != nil {
		t.Fatal(err)
	}

	samples, _ := repo.ListOverviewSince(context.Background(), connID, time.Time{})
	if len(samples) != 2 {
		t.Fatalf("want 2 samples, got %d", len(samples))
	}
	if samples[1].TPS <= 0 {
		t.Fatalf("second-tick TPS should be > 0, got %v", samples[1].TPS)
	}
}

func TestSlowQueriesCollector_MissingExtensionSkips(t *testing.T) {
	repo, connID := newObservRepo(t)
	pool := &fakePool{results: map[string]*postgres.Result{
		"SELECT EXISTS": {Rows: [][]any{{false}}},
	}}
	c := &SlowQueriesCollector{ConnID: connID, Repo: repo}
	if err := c.Tick(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	_, rows, _ := repo.LatestSlow(context.Background(), connID)
	if len(rows) != 0 {
		t.Fatalf("expected no slow rows when extension is missing, got %d", len(rows))
	}
	// Calling Tick twice must not double-log; we don't assert log output, just
	// that no panic / error occurs on the second call.
	if err := c.Tick(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
}

func TestSlowQueriesCollector_Snapshot(t *testing.T) {
	repo, connID := newObservRepo(t)
	pool := &fakePool{results: map[string]*postgres.Result{
		"SELECT EXISTS": {Rows: [][]any{{true}}},
		"SELECT\n    queryid": {Rows: [][]any{
			{"42", "SELECT 1", 12.5, 4.2, int64(100), 1250.0, 1.0},
			{"43", "INSERT INTO x", 3.1, 0.8, int64(200), 620.0, 1.0},
		}},
	}}
	c := &SlowQueriesCollector{ConnID: connID, Repo: repo}
	if err := c.Tick(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	_, rows, _ := repo.LatestSlow(context.Background(), connID)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	// Order is total_ms desc → SELECT 1 (1250) first.
	if rows[0].Preview != "SELECT 1" {
		t.Fatalf("ordering: %+v", rows)
	}
	if rows[0].MeanMs != 12.5 || rows[0].Calls != 100 {
		t.Fatalf("first row payload: %+v", rows[0])
	}
}

func TestManager_StartTickStop(t *testing.T) {
	repo, connID := newObservRepo(t)
	pool := &fakePool{results: map[string]*postgres.Result{
		"SELECT\n    (SELECT": {
			Rows: [][]any{{int64(0), 1.0, int(0), int(0), nil}},
		},
		"SELECT EXISTS": {Rows: [][]any{{false}}},
	}}
	var poolCalls atomic.Int64
	poolFn := func(_ context.Context, id int64) (postgres.Pool, error) {
		poolCalls.Add(1)
		if id != connID {
			return nil, errors.New("wrong id")
		}
		return pool, nil
	}
	m := NewManager(repo, poolFn, DefaultFactory)

	m.Start(connID, 25*time.Millisecond)
	time.Sleep(120 * time.Millisecond)
	m.Stop(connID)

	if poolCalls.Load() < 2 {
		t.Fatalf("expected at least 2 pool resolves, got %d", poolCalls.Load())
	}
	// After Stop, no new samples should be recorded; sleeping more should
	// keep the count stable.
	stableSamples, _ := repo.ListOverviewSince(context.Background(), connID, time.Time{})
	time.Sleep(60 * time.Millisecond)
	after, _ := repo.ListOverviewSince(context.Background(), connID, time.Time{})
	if len(after) != len(stableSamples) {
		t.Fatalf("samples kept growing after Stop: %d -> %d", len(stableSamples), len(after))
	}
}

func TestManager_StartTwiceNoOp(t *testing.T) {
	repo, connID := newObservRepo(t)
	pool := &fakePool{}
	poolFn := func(_ context.Context, _ int64) (postgres.Pool, error) { return pool, nil }
	m := NewManager(repo, poolFn, func(_ int64, _ *store.ObservabilityRepo) []Collector { return nil })

	m.Start(connID, 100*time.Millisecond)
	m.Start(connID, 100*time.Millisecond) // second call is a no-op
	m.Shutdown()
}

func TestManager_ShutdownStopsAll(t *testing.T) {
	repo, _ := newObservRepo(t)
	pool := &fakePool{}
	poolFn := func(_ context.Context, _ int64) (postgres.Pool, error) { return pool, nil }
	m := NewManager(repo, poolFn, func(_ int64, _ *store.ObservabilityRepo) []Collector { return nil })

	m.Start(1, 50*time.Millisecond)
	m.Start(2, 50*time.Millisecond)
	m.Start(3, 50*time.Millisecond)
	m.Shutdown() // must return promptly even with three runners
}
