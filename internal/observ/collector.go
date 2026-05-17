package observ

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/unkabas/dbil/internal/postgres"
	"github.com/unkabas/dbil/internal/store"
)

// Collector is one periodic snapshot taker. Implementations may keep
// internal state across ticks (Overview tracks the previous xact counter
// to compute TPS, for example).
type Collector interface {
	Name() CollectorName
	Tick(ctx context.Context, pool postgres.Pool) error
}

// PoolFn opens (or returns the cached) Postgres pool for a connection.
// In production this is wired to *postgres.Manager.OpenByID with an empty
// passphrase — collectors only run for connections that have already
// been unlocked by the user during normal use.
type PoolFn func(ctx context.Context, connID int64) (postgres.Pool, error)

// CollectorFactory builds the collectors a Manager runs against one
// connection per tick.
type CollectorFactory func(connID int64, repo *store.ObservabilityRepo) []Collector

// DefaultFactory returns the production collector set: overview + slow_queries.
// Locks are queried on demand via ListLockChains, not on a poll loop.
func DefaultFactory(connID int64, repo *store.ObservabilityRepo) []Collector {
	return []Collector{
		&OverviewCollector{ConnID: connID, Repo: repo},
		&SlowQueriesCollector{ConnID: connID, Repo: repo},
	}
}

// Manager runs one goroutine per registered connection that ticks the
// supplied collectors at the connection's tag-driven cadence.
type Manager struct {
	repo    *store.ObservabilityRepo
	pool    PoolFn
	factory CollectorFactory

	mu   sync.Mutex
	runs map[int64]*runner
}

type runner struct {
	connID   int64
	cancel   context.CancelFunc
	done     chan struct{}
	interval time.Duration
}

// NewManager wires the observability collector manager. pool resolves
// a connection id to a live pgx pool; factory chooses which collectors
// to run per connection.
func NewManager(repo *store.ObservabilityRepo, pool PoolFn, factory CollectorFactory) *Manager {
	if factory == nil {
		factory = DefaultFactory
	}
	return &Manager{
		repo:    repo,
		pool:    pool,
		factory: factory,
		runs:    make(map[int64]*runner),
	}
}

// Start kicks off the collector loop for connID. Calling Start twice with
// the same id is a no-op (the existing runner continues).
func (m *Manager) Start(connID int64, interval time.Duration) {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	m.mu.Lock()
	if _, ok := m.runs[connID]; ok {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &runner{connID: connID, cancel: cancel, done: make(chan struct{}), interval: interval}
	m.runs[connID] = r
	m.mu.Unlock()

	go m.loop(ctx, r)
}

func (m *Manager) loop(ctx context.Context, r *runner) {
	defer close(r.done)
	collectors := m.factory(r.connID, m.repo)
	t := time.NewTicker(r.interval)
	defer t.Stop()

	// Run an immediate tick so the UI does not stare at zero on first load.
	m.tickAll(ctx, r.connID, collectors)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.tickAll(ctx, r.connID, collectors)
		}
	}
}

func (m *Manager) tickAll(ctx context.Context, connID int64, cs []Collector) {
	pool, err := m.pool(ctx, connID)
	if err != nil {
		slog.Warn("observ: open pool failed", "conn_id", connID, "err", err)
		return
	}
	for _, c := range cs {
		m.runOne(ctx, connID, c, pool)
	}
}

func (m *Manager) runOne(ctx context.Context, connID int64, c Collector, pool postgres.Pool) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("observ: collector panic",
				"conn_id", connID, "collector", string(c.Name()), "panic", fmt.Sprintf("%v", rec))
		}
	}()
	tctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := c.Tick(tctx, pool); err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Warn("observ: collector tick failed",
				"conn_id", connID, "collector", string(c.Name()), "err", err)
		}
	}
}

// Stop cancels the runner for connID and waits for the goroutine to exit.
// Safe to call when no runner exists.
func (m *Manager) Stop(connID int64) {
	m.mu.Lock()
	r, ok := m.runs[connID]
	delete(m.runs, connID)
	m.mu.Unlock()
	if !ok {
		return
	}
	r.cancel()
	<-r.done
}

// Shutdown cancels every runner and waits for all goroutines.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	runs := m.runs
	m.runs = make(map[int64]*runner)
	m.mu.Unlock()
	for _, r := range runs {
		r.cancel()
	}
	for _, r := range runs {
		<-r.done
	}
}
