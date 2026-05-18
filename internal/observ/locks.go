package observ

import (
	"context"
	"errors"
	"fmt"

	"github.com/unkabas/dbil/internal/postgres"
)

// ErrSelfTerminate is returned by TerminateBackend when the caller passes
// a non-positive pid or one matching pg_backend_pid() (the dbil
// connection itself). The latter check is enforced server-side in the SQL.
var ErrSelfTerminate = errors.New("refusing to terminate own backend or invalid pid")

// TerminateBackend asks Postgres to send a SIGTERM to the session
// identified by pid (pg_terminate_backend). Returns:
//   - (true,  nil) if the function returned true (signal delivered).
//   - (false, nil) if the function returned false (process already gone
//     or insufficient privilege).
//   - (false, ErrSelfTerminate) if pid is invalid or matches our own pid.
//   - (false, err) on any driver-level failure.
func TerminateBackend(ctx context.Context, pool postgres.Pool, pid int) (bool, error) {
	if pid <= 0 {
		return false, ErrSelfTerminate
	}
	// The SQL refuses to kill our own backend defensively, in case the
	// caller forgets the guard above.
	sql := fmt.Sprintf(
		"SELECT CASE WHEN %d = pg_backend_pid() THEN false ELSE pg_terminate_backend(%d) END",
		pid, pid,
	)
	res, err := pool.Execute(ctx, sql)
	if err != nil {
		return false, fmt.Errorf("terminate: %w", err)
	}
	if len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
		return false, fmt.Errorf("terminate: empty result")
	}
	if b, ok := res.Rows[0][0].(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("terminate: unexpected result type %T", res.Rows[0][0])
}

// ListLockChains returns the live lock graph for pool: a list of "chains"
// where each chain has a holder (a session that is blocking others) and
// the sessions waiting on it. The query is fast (a single pg_stat_activity
// scan with pg_blocking_pids); chains are assembled in Go.
//
// We deliberately do NOT persist locks — they are inspected on demand from
// the UI, so storing a 10-second-resolution history would burn space with
// no obvious win. Plan 7+ may add lock-event audit if needed.
func ListLockChains(ctx context.Context, pool postgres.Pool) ([]LockChain, error) {
	res, err := pool.Execute(ctx, listLocksQuery)
	if err != nil {
		return nil, err
	}

	sessions := make(map[int]LockSession, len(res.Rows))
	for _, r := range res.Rows {
		if len(r) < 6 {
			continue
		}
		s := LockSession{
			PID:       asInt(r[0]),
			User:      asString(r[1]),
			Query:     asString(r[2]),
			AgeMs:     asInt64(r[3]),
			State:     asString(r[4]),
			BlockedBy: asIntArray(r[5]),
		}
		sessions[s.PID] = s
	}

	// Compute holder set: any pid that appears in a session's BlockedBy is
	// itself a holder. Build one chain per holder.
	holderSet := make(map[int]bool)
	for _, s := range sessions {
		for _, b := range s.BlockedBy {
			holderSet[b] = true
		}
	}

	var chains []LockChain
	for hpid := range holderSet {
		holder, ok := sessions[hpid]
		if !ok {
			// The holder finished its transaction between rows arriving and
			// chain assembly. Synthesise a minimal entry so the UI still has
			// something to render.
			holder = LockSession{PID: hpid, User: "?", Query: "(no longer in pg_stat_activity)"}
		}
		var blocked []LockSession
		for _, s := range sessions {
			for _, b := range s.BlockedBy {
				if b == hpid {
					blocked = append(blocked, s)
					break
				}
			}
		}
		chains = append(chains, LockChain{Holder: holder, Blocked: blocked})
	}
	return chains, nil
}

const listLocksQuery = `SELECT
    pid::int                                                                          AS pid,
    COALESCE(usename, '')                                                             AS user,
    COALESCE(LEFT(query, 200), '')                                                    AS query,
    COALESCE((EXTRACT(EPOCH FROM (now() - xact_start)) * 1000)::int, 0)               AS age_ms,
    COALESCE(state, '')                                                               AS state,
    pg_blocking_pids(pid)                                                             AS blocked_by
FROM pg_stat_activity
WHERE pid <> pg_backend_pid()
  AND (state IS DISTINCT FROM 'idle' OR cardinality(pg_blocking_pids(pid)) > 0)`
