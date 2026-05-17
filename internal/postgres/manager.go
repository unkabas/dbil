package postgres

import (
	"context"
	"fmt"
	"sync"

	"github.com/unkabas/dbil/internal/store"
)

// Manager owns one pgx pool per registered connection id. Pools are cached
// lazily on first use and released via CloseConn or Shutdown.
type Manager struct {
	driver Driver
	repo   *store.ConnectionsRepo
	mu     sync.Mutex
	pools  map[int64]Pool
}

// NewManager wires a Manager to a Driver and ConnectionsRepo.
func NewManager(d Driver, repo *store.ConnectionsRepo) *Manager {
	return &Manager{driver: d, repo: repo, pools: make(map[int64]Pool)}
}

// OpenByID returns the cached Pool for id, opening a fresh one (after
// decrypting credentials via ConnectionsRepo.Reveal) when none is cached.
// Concurrent callers receive the same Pool instance.
func (m *Manager) OpenByID(ctx context.Context, id int64, passphrase string) (Pool, error) {
	m.mu.Lock()
	if p, ok := m.pools[id]; ok {
		m.mu.Unlock()
		return p, nil
	}
	m.mu.Unlock()

	rev, err := m.repo.Reveal(ctx, id, passphrase)
	if err != nil {
		return nil, err
	}
	conn := Conn{
		Alias:    rev.Alias,
		Host:     rev.Host,
		Port:     rev.Port,
		Username: rev.Username,
		Password: rev.Password,
		Database: rev.Database,
		TLSMode:  rev.TLSMode,
	}
	pool, err := m.driver.Open(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("manager open id=%d: %w", id, err)
	}

	m.mu.Lock()
	if existing, ok := m.pools[id]; ok {
		m.mu.Unlock()
		m.driver.Close(pool)
		return existing, nil
	}
	m.pools[id] = pool
	m.mu.Unlock()
	return pool, nil
}

// Ping opens (if needed) and pings the pool for id.
func (m *Manager) Ping(ctx context.Context, id int64, passphrase string) error {
	pool, err := m.OpenByID(ctx, id, passphrase)
	if err != nil {
		return err
	}
	return pool.Ping(ctx)
}

// Probe opens (if needed) and runs the driver-specific probe queries.
func (m *Manager) Probe(ctx context.Context, id int64, passphrase string) (Probe, error) {
	pool, err := m.OpenByID(ctx, id, passphrase)
	if err != nil {
		return Probe{}, err
	}
	return m.driver.Probe(ctx, pool)
}

// CloseConn releases the cached pool for id (no-op if none cached).
func (m *Manager) CloseConn(id int64) {
	m.mu.Lock()
	pool, ok := m.pools[id]
	delete(m.pools, id)
	m.mu.Unlock()
	if ok {
		m.driver.Close(pool)
	}
}

// Shutdown closes every cached pool. Called from `dbil serve` on shutdown.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	pools := m.pools
	m.pools = make(map[int64]Pool)
	m.mu.Unlock()
	for _, p := range pools {
		m.driver.Close(p)
	}
}
