// Package postgres bridges DBil's connection records to the live PostgreSQL
// engine via pgx. The Driver interface is the seam for tests (mocks) and
// future engines; the production implementation is NewPGX().
package postgres

import (
	"context"
	"net"
	"time"
)

// Conn is the connection material needed to dial a PostgreSQL instance.
// Always built from store.ConnectionsRepo.Reveal; never persisted.
type Conn struct {
	Alias    string
	Host     string
	Port     int
	Username string
	Password string
	Database string
	TLSMode  string
	// DialContext, when non-nil, replaces the default network dialer — used to
	// route the Postgres connection through an SSH tunnel. addr is the DB
	// address as seen from the tunnel endpoint (i.e. Host:Port above).
	DialContext func(ctx context.Context, network, addr string) (net.Conn, error)
}

// Probe is the result of querying a freshly opened connection. The capability
// flags are consulted later (Plan 6) by the observability collector.
type Probe struct {
	Version             string
	SuperuserOK         bool
	HasPgStatStatements bool
}

// ColumnDef describes one column in a query Result.
type ColumnDef struct {
	Name     string `json:"name"`
	TypeName string `json:"type_name"` // pgtype name when known (e.g. "int4", "text"); empty otherwise.
}

// Result is the engine-neutral shape returned by Pool.Execute.
type Result struct {
	Columns      []ColumnDef
	Rows         [][]any
	RowsAffected int64
	CommandTag   string
	Duration     time.Duration
	Truncated    bool
}

// Pool is the minimal surface DBil needs from a Postgres connection pool.
type Pool interface {
	Ping(ctx context.Context) error
	Close()
	Execute(ctx context.Context, sql string) (*Result, error)
}

type rowLimitPool interface {
	ExecuteWithLimit(ctx context.Context, sql string, rowCap int) (*Result, error)
}

type txPool interface {
	ExecuteTx(ctx context.Context, stmts []string) (int64, error)
}

// ExecuteTx runs stmts as a single atomic transaction when the concrete pool
// supports it (the production pgx pool does), rolling back on the first error.
// Test doubles that do not implement txPool fall back to sequential,
// non-atomic Execute — acceptable for unit tests, never for real writes.
func ExecuteTx(ctx context.Context, pool Pool, stmts []string) (int64, error) {
	if p, ok := pool.(txPool); ok {
		return p.ExecuteTx(ctx, stmts)
	}
	var total int64
	for _, s := range stmts {
		res, err := pool.Execute(ctx, s)
		if err != nil {
			return total, err
		}
		if res != nil {
			total += res.RowsAffected
		}
	}
	return total, nil
}

// ExecuteWithLimit runs sql with a caller-supplied row cap when the concrete
// pool supports it, falling back to Execute for older test doubles.
func ExecuteWithLimit(ctx context.Context, pool Pool, sql string, rowCap int) (*Result, error) {
	if p, ok := pool.(rowLimitPool); ok {
		return p.ExecuteWithLimit(ctx, sql, rowCap)
	}
	return pool.Execute(ctx, sql)
}

// Driver opens, probes, and closes pools.
type Driver interface {
	Open(ctx context.Context, conn Conn) (Pool, error)
	Probe(ctx context.Context, pool Pool) (Probe, error)
	Close(pool Pool)
}
