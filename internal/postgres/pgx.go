package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPGX returns the production Driver backed by pgxpool.
func NewPGX() Driver { return &pgxDriver{} }

type pgxDriver struct{}

type pgxPool struct{ p *pgxpool.Pool }

// Ping forwards to the underlying pgx pool.
func (p *pgxPool) Ping(ctx context.Context) error { return p.p.Ping(ctx) }

// Close releases the underlying pgx pool.
func (p *pgxPool) Close() { p.p.Close() }

// Open builds a libpq-style DSN, opens a pgxpool.Pool with DBil's default
// limits (MaxConns=4, MinConns=0, MaxConnIdleTime=5m), and returns it.
func (d *pgxDriver) Open(ctx context.Context, c Conn) (Pool, error) {
	cfg, err := pgxpool.ParseConfig(buildDSN(c))
	if err != nil {
		return nil, fmt.Errorf("pgx parse config: %w", err)
	}
	cfg.MaxConns = 4
	cfg.MinConns = 0
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgx new pool: %w", err)
	}
	return &pgxPool{p: pool}, nil
}

// Close releases the pool.
func (d *pgxDriver) Close(p Pool) { p.Close() }

// Probe runs SHOW server_version, checks pg_stat_statements presence, and
// reads is_superuser. All capability queries are best-effort — failures
// leave the corresponding flag false rather than returning a hard error,
// so a tightly-scoped read-only user still gets a useful probe back.
func (d *pgxDriver) Probe(ctx context.Context, p Pool) (Probe, error) {
	pp, ok := p.(*pgxPool)
	if !ok {
		return Probe{}, fmt.Errorf("pgx probe: pool is not *pgxPool")
	}
	pool := pp.p

	var version string
	if err := pool.QueryRow(ctx, `SHOW server_version`).Scan(&version); err != nil {
		return Probe{}, fmt.Errorf("pgx probe version: %w", err)
	}

	var hasStmt bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements')`).Scan(&hasStmt)

	var isSuper bool
	_ = pool.QueryRow(ctx, `SELECT current_setting('is_superuser') = 'on'`).Scan(&isSuper)

	return Probe{
		Version:             "PostgreSQL " + version,
		SuperuserOK:         isSuper,
		HasPgStatStatements: hasStmt,
	}, nil
}

// buildDSN composes a libpq-style DSN from Conn. pgx accepts both URLs and
// key=value strings; key=value is friendlier when the password contains
// characters that would need URL-escaping.
func buildDSN(c Conn) string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.Username, c.Password, c.Database, c.TLSMode,
	)
}
