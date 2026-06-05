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
// lazily on first use and released via CloseConn or Shutdown. When a
// connection references an SSH host, an ssh tunnel is opened alongside the
// pool and torn down with it.
type Manager struct {
	driver  Driver
	repo    *store.ConnectionsRepo
	sshRepo *store.SSHHostsRepo
	audit   *store.AuditRepo
	mu      sync.Mutex
	pools   map[int64]Pool
	tunnels map[int64]*sshTunnel
}

// NewManager wires a Manager to a Driver, ConnectionsRepo, and AuditRepo.
// The audit repo is required for query.execute / query.blocked entries.
func NewManager(d Driver, repo *store.ConnectionsRepo, audit *store.AuditRepo) *Manager {
	return &Manager{
		driver:  d,
		repo:    repo,
		audit:   audit,
		pools:   make(map[int64]Pool),
		tunnels: make(map[int64]*sshTunnel),
	}
}

// SetSSHHosts enables SSH tunnelling by giving the Manager access to the
// ssh_hosts repository. Called once during server wiring; leaving it unset
// disables tunnel support (direct connections only).
func (m *Manager) SetSSHHosts(repo *store.SSHHostsRepo) { m.sshRepo = repo }

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

	// When the connection tunnels through a bastion, open the SSH client and
	// route the Postgres dial through it. The same passphrase unlocks both the
	// connection and the SSH host secret.
	var tunnel *sshTunnel
	if rev.SSHHostID != nil && m.sshRepo != nil {
		tunnel, err = m.openTunnelFor(ctx, *rev.SSHHostID, passphrase)
		if err != nil {
			return nil, fmt.Errorf("manager tunnel id=%d: %w", id, err)
		}
		conn.DialContext = tunnel.DialContext
	}

	pool, err := m.driver.Open(ctx, conn)
	if err != nil {
		_ = tunnel.Close()
		return nil, fmt.Errorf("manager open id=%d: %w", id, err)
	}

	m.mu.Lock()
	if existing, ok := m.pools[id]; ok {
		m.mu.Unlock()
		m.driver.Close(pool)
		_ = tunnel.Close()
		return existing, nil
	}
	m.pools[id] = pool
	if tunnel != nil {
		m.tunnels[id] = tunnel
	}
	m.mu.Unlock()
	return pool, nil
}

// TestSSHHost reveals an SSH host, opens a tunnel to validate reachability and
// authentication, pins the host-key fingerprint on first connect, and tears
// the tunnel down. Returns the observed fingerprint. Distinct from a Postgres
// probe: it exercises only the bastion.
func (m *Manager) TestSSHHost(ctx context.Context, sshHostID int64, passphrase string) (string, error) {
	if m.sshRepo == nil {
		return "", fmt.Errorf("ssh tunnelling is not configured")
	}
	rev, err := m.sshRepo.Reveal(ctx, sshHostID, passphrase)
	if err != nil {
		return "", err
	}
	tunnel, fp, err := openTunnel(ctx, rev)
	if err != nil {
		return fp, err
	}
	defer func() { _ = tunnel.Close() }()
	if rev.HostKeyFingerprint == "" && fp != "" {
		_ = m.sshRepo.SetFingerprint(ctx, sshHostID, fp)
	}
	return fp, nil
}

// openTunnelFor reveals an SSH host and opens a tunnel, pinning the host-key
// fingerprint on first successful connect (trust on first use).
func (m *Manager) openTunnelFor(ctx context.Context, sshHostID int64, passphrase string) (*sshTunnel, error) {
	rev, err := m.sshRepo.Reveal(ctx, sshHostID, passphrase)
	if err != nil {
		return nil, err
	}
	tunnel, fp, err := openTunnel(ctx, rev)
	if err != nil {
		return nil, err
	}
	if rev.HostKeyFingerprint == "" && fp != "" {
		// Best-effort pin; a failure here doesn't break an otherwise-good tunnel.
		_ = m.sshRepo.SetFingerprint(ctx, sshHostID, fp)
	}
	return tunnel, nil
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

// CloseConn releases the cached pool for id and its tunnel (no-op if none).
func (m *Manager) CloseConn(id int64) {
	m.mu.Lock()
	pool, ok := m.pools[id]
	delete(m.pools, id)
	tunnel := m.tunnels[id]
	delete(m.tunnels, id)
	m.mu.Unlock()
	if ok {
		m.driver.Close(pool)
	}
	_ = tunnel.Close()
}

// Shutdown closes every cached pool and tunnel. Called from `dbil serve` on
// shutdown.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	pools := m.pools
	tunnels := m.tunnels
	m.pools = make(map[int64]Pool)
	m.tunnels = make(map[int64]*sshTunnel)
	m.mu.Unlock()
	for _, p := range pools {
		m.driver.Close(p)
	}
	for _, t := range tunnels {
		_ = t.Close()
	}
}

// ExecuteParams is the input to Manager.Execute.
type ExecuteParams struct {
	ConnID     int64
	Passphrase string
	SQL        string
	Confirm    bool   // mirror of the X-Confirm header
	UserEmail  string // for audit attribution
	// ReadOnly, when true, blocks any statement that is not a read (DML/DDL).
	// Set by handlers for viewer-role callers; the zero value preserves the
	// pre-RBAC behaviour for internal callers.
	ReadOnly bool
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

	// Read-only callers (viewer role) may run reads only; any write class is
	// rejected before the tag policy is even consulted.
	if p.ReadOnly && (class == sqlcheck.ClassDML || class == sqlcheck.ClassDDL) {
		reason := "read-only role: data writes are not permitted"
		m.auditBlocked(ctx, p, conn, class, dangerous, reason)
		return nil, &BlockedError{Reason: reason}
	}

	allowed, needsConfirm, reason := decidePolicy(pol, conn.Tag, class, dangerous)

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

// decidePolicy applies a tag policy to one classified statement, returning
// whether it is allowed outright, whether it requires X-Confirm, and a
// human-readable reason. Shared by Execute (single statement) and
// ExecuteBatch (every statement in the batch).
func decidePolicy(pol policy.Policy, tag string, class sqlcheck.Class, dangerous bool) (allowed, needsConfirm bool, reason string) {
	switch {
	case dangerous && !pol.DangerousAllowed:
		return false, false, fmt.Sprintf("dangerous statement (DELETE/UPDATE without WHERE) not allowed on tag %s", tag)
	case class == sqlcheck.ClassDDL && !pol.DDLAllowed:
		return false, false, fmt.Sprintf("ddl not allowed on tag %s", tag)
	case class == sqlcheck.ClassDML && !pol.DMLAllowed:
		return false, false, fmt.Sprintf("dml not allowed on tag %s", tag)
	case dangerous && pol.DangerousRequiresConfirm:
		return true, true, "dangerous statement requires X-Confirm: yes"
	case class == sqlcheck.ClassDDL && pol.DDLRequiresConfirm:
		return true, true, fmt.Sprintf("ddl requires X-Confirm: yes on tag %s", tag)
	case class == sqlcheck.ClassDML && pol.DMLRequiresConfirm:
		return true, true, fmt.Sprintf("dml requires X-Confirm: yes on tag %s", tag)
	}
	return true, false, ""
}

// BatchParams is the input to Manager.ExecuteBatch. Stmts are pre-built by the
// caller (pg.BuildMutations) from a validated set of typed edits.
type BatchParams struct {
	ConnID     int64
	Passphrase string
	Stmts      []string
	Confirm    bool
	UserEmail  string
	ReadOnly   bool
}

// BatchResult summarises a committed batch.
type BatchResult struct {
	RowsAffected int64
	Statements   int
}

// ExecuteBatch applies the connection's tag policy to every statement, then
// runs the whole set in a single transaction (all-or-nothing) under the tag's
// timeout, and writes one audit entry. A single blocked statement blocks the
// whole batch; if any statement needs confirmation the batch is held until the
// caller re-sends with Confirm=true. Statements are expected to be the typed,
// PK-scoped UPDATE/DELETE/INSERT built by pg.BuildMutations.
func (m *Manager) ExecuteBatch(ctx context.Context, p BatchParams) (*BatchResult, error) {
	if len(p.Stmts) == 0 {
		return &BatchResult{}, nil
	}
	conn, err := m.repo.Get(ctx, p.ConnID)
	if err != nil {
		return nil, err
	}
	pol := policy.PolicyFor(conn.Tag)

	needsConfirm := false
	for _, sql := range p.Stmts {
		class := sqlcheck.Classify(sql)
		dangerous := sqlcheck.IsDangerous(sql)
		if p.ReadOnly && (class == sqlcheck.ClassDML || class == sqlcheck.ClassDDL) {
			reason := "read-only role: data writes are not permitted"
			m.auditBatchBlocked(ctx, p, conn, reason)
			return nil, &BlockedError{Reason: reason}
		}
		allowed, confirm, reason := decidePolicy(pol, conn.Tag, class, dangerous)
		if !allowed {
			m.auditBatchBlocked(ctx, p, conn, reason)
			return nil, &BlockedError{Reason: reason}
		}
		if confirm {
			needsConfirm = true
		}
	}
	if needsConfirm && !p.Confirm {
		return nil, &ConfirmationRequiredError{Reason: "batch touches a tag that requires X-Confirm: yes"}
	}

	pool, err := m.OpenByID(ctx, p.ConnID, p.Passphrase)
	if err != nil {
		return nil, err
	}
	qctx, cancel := context.WithTimeout(ctx, pol.Timeout)
	defer cancel()

	affected, execErr := ExecuteTx(qctx, pool, p.Stmts)
	m.auditBatchExecuted(ctx, p, conn, affected, execErr)
	if execErr != nil {
		return nil, execErr
	}
	return &BatchResult{RowsAffected: affected, Statements: len(p.Stmts)}, nil
}

func (m *Manager) auditBatchBlocked(ctx context.Context, p BatchParams, conn store.Connection, reason string) {
	if m.audit == nil {
		return
	}
	_, _ = m.audit.Append(ctx, p.UserEmail, "table.rows.blocked", fmt.Sprintf("conn:%d", p.ConnID), map[string]any{
		"alias":      conn.Alias,
		"tag":        conn.Tag,
		"statements": len(p.Stmts),
		"reason":     reason,
	})
}

func (m *Manager) auditBatchExecuted(ctx context.Context, p BatchParams, conn store.Connection, affected int64, execErr error) {
	if m.audit == nil {
		return
	}
	details := map[string]any{
		"alias":      conn.Alias,
		"tag":        conn.Tag,
		"statements": len(p.Stmts),
	}
	action := "table.rows.mutate"
	if execErr != nil {
		details["error"] = execErr.Error()
		action = "table.rows.failed"
	} else {
		details["rows_affected"] = affected
	}
	_, _ = m.audit.Append(ctx, p.UserEmail, action, fmt.Sprintf("conn:%d", p.ConnID), details)
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
