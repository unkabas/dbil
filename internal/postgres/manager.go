package postgres

import (
	"context"
	"fmt"
	"sync"

	"github.com/unkabas/dbil/internal/policy"
	"github.com/unkabas/dbil/internal/sqlcheck"
	"github.com/unkabas/dbil/internal/store"
)

// Manager owns one pgx pool per registered connection id. Pools are cached
// lazily on first use and released via CloseConn or Shutdown.
type Manager struct {
	driver Driver
	repo   *store.ConnectionsRepo
	audit  *store.AuditRepo
	mu     sync.Mutex
	pools  map[int64]Pool
}

// NewManager wires a Manager to a Driver, ConnectionsRepo, and AuditRepo.
// The audit repo is required for query.execute / query.blocked entries.
func NewManager(d Driver, repo *store.ConnectionsRepo, audit *store.AuditRepo) *Manager {
	return &Manager{driver: d, repo: repo, audit: audit, pools: make(map[int64]Pool)}
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

// ExecuteParams is the input to Manager.Execute.
type ExecuteParams struct {
	ConnID     int64
	Passphrase string
	SQL        string
	Confirm    bool   // mirror of the X-Confirm header
	UserEmail  string // for audit attribution
}

// Execute applies the connection's tag policy, runs the statement when
// permitted, and writes an audit entry. Returns BlockedError when the
// statement is disallowed outright, ConfirmationRequiredError when the
// caller must re-send with Confirm=true, store.ErrConnectionNotFound /
// ErrPassphraseRequired / ErrInvalidPassphrase from the repo, or the
// underlying driver error.
func (m *Manager) Execute(ctx context.Context, p ExecuteParams) (*Result, error) {
	conn, err := m.repo.Get(ctx, p.ConnID)
	if err != nil {
		return nil, err
	}
	pol := policy.PolicyFor(conn.Tag)
	class := sqlcheck.Classify(p.SQL)
	dangerous := sqlcheck.IsDangerous(p.SQL)

	allowed := true
	needsConfirm := false
	reason := ""

	switch {
	case dangerous && !pol.DangerousAllowed:
		allowed = false
		reason = fmt.Sprintf("dangerous statement (DELETE/UPDATE without WHERE) not allowed on tag %s", conn.Tag)
	case class == sqlcheck.ClassDDL && !pol.DDLAllowed:
		allowed = false
		reason = fmt.Sprintf("ddl not allowed on tag %s", conn.Tag)
	case class == sqlcheck.ClassDML && !pol.DMLAllowed:
		allowed = false
		reason = fmt.Sprintf("dml not allowed on tag %s", conn.Tag)
	case dangerous && pol.DangerousRequiresConfirm:
		needsConfirm = true
		reason = "dangerous statement requires X-Confirm: yes"
	case class == sqlcheck.ClassDDL && pol.DDLRequiresConfirm:
		needsConfirm = true
		reason = fmt.Sprintf("ddl requires X-Confirm: yes on tag %s", conn.Tag)
	case class == sqlcheck.ClassDML && pol.DMLRequiresConfirm:
		needsConfirm = true
		reason = fmt.Sprintf("dml requires X-Confirm: yes on tag %s", conn.Tag)
	}

	if !allowed {
		m.auditBlocked(ctx, p, conn, class, dangerous, reason)
		return nil, &BlockedError{Reason: reason}
	}
	if needsConfirm && !p.Confirm {
		return nil, &ConfirmationRequiredError{Reason: reason}
	}

	pool, err := m.OpenByID(ctx, p.ConnID, p.Passphrase)
	if err != nil {
		return nil, err
	}

	qctx, cancel := context.WithTimeout(ctx, pol.Timeout)
	defer cancel()

	res, execErr := pool.Execute(qctx, p.SQL)
	m.auditExecuted(ctx, p, conn, class, dangerous, res, execErr)
	if execErr != nil {
		return nil, execErr
	}
	return res, nil
}

func (m *Manager) auditBlocked(ctx context.Context, p ExecuteParams, conn store.Connection, class sqlcheck.Class, dangerous bool, reason string) {
	if m.audit == nil {
		return
	}
	_, _ = m.audit.Append(ctx, p.UserEmail, "query.blocked", fmt.Sprintf("conn:%d", p.ConnID), map[string]any{
		"alias":     conn.Alias,
		"tag":       conn.Tag,
		"class":     class.String(),
		"dangerous": dangerous,
		"sql_len":   len(p.SQL),
		"reason":    reason,
	})
}

func (m *Manager) auditExecuted(ctx context.Context, p ExecuteParams, conn store.Connection, class sqlcheck.Class, dangerous bool, res *Result, execErr error) {
	if m.audit == nil {
		return
	}
	details := map[string]any{
		"alias":     conn.Alias,
		"tag":       conn.Tag,
		"class":     class.String(),
		"dangerous": dangerous,
		"sql_len":   len(p.SQL),
	}
	action := "query.execute"
	if res != nil {
		details["rows"] = len(res.Rows)
		details["rows_affected"] = res.RowsAffected
		details["duration_ms"] = res.Duration.Milliseconds()
		details["truncated"] = res.Truncated
		details["command_tag"] = res.CommandTag
	}
	if execErr != nil {
		details["error"] = execErr.Error()
		action = "query.failed"
	}
	_, _ = m.audit.Append(ctx, p.UserEmail, action, fmt.Sprintf("conn:%d", p.ConnID), details)
}
