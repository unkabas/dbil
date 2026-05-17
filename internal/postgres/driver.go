// Package postgres bridges DBil's connection records to the live PostgreSQL
// engine via pgx. The Driver interface is the seam for tests (mocks) and
// future engines; the production implementation is NewPGX().
package postgres

import "context"

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
}

// Probe is the result of querying a freshly opened connection. The capability
// flags are consulted later (Plan 6) by the observability collector.
type Probe struct {
	Version             string
	SuperuserOK         bool
	HasPgStatStatements bool
}

// Pool is the minimal surface Plan 3 needs. Plan 4 extends this with Query
// and Exec so the query executor mounts on top.
type Pool interface {
	Ping(ctx context.Context) error
	Close()
}

// Driver opens, probes, and closes pools.
type Driver interface {
	Open(ctx context.Context, conn Conn) (Pool, error)
	Probe(ctx context.Context, pool Pool) (Probe, error)
	Close(pool Pool)
}
