// Package observ collects pg_stat_* metrics from registered Postgres
// connections and exposes live lock-chain queries. Storage of historical
// samples lives in internal/store; this package is the glue between live
// pgx pools and the repo.
package observ

import "time"

// LockSession is one entry in pg_stat_activity with its blocking PIDs.
type LockSession struct {
	PID       int    `json:"pid"`
	User      string `json:"user"`
	Query     string `json:"query"`
	AgeMs     int64  `json:"age_ms"`
	State     string `json:"state"`
	BlockedBy []int  `json:"blocked_by"`
}

// LockChain groups a holder session with the sessions it is blocking.
type LockChain struct {
	Holder  LockSession   `json:"holder"`
	Blocked []LockSession `json:"blocked"`
}

// CollectorName identifies a collector when logging or rotating its state.
type CollectorName string

const (
	CollectorOverview CollectorName = "overview"
	CollectorSlow     CollectorName = "slow_queries"
)

// runStats are written by the collector goroutine for debugging.
type runStats struct {
	lastTick time.Time
	lastErr  error
	errCount int
}
