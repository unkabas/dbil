package postgres

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unkabas/dbil/internal/crypto"
	"github.com/unkabas/dbil/internal/store"
)

// mockPool is a no-op Pool for Manager tests.
type mockPool struct {
	pingErr    error
	closed     atomic.Bool
	execResult *Result
	execErr    error
	execDelay  time.Duration
}

func (m *mockPool) Ping(_ context.Context) error { return m.pingErr }

func (m *mockPool) Close() { m.closed.Store(true) }

func (m *mockPool) Execute(ctx context.Context, _ string) (*Result, error) {
	if m.execDelay > 0 {
		select {
		case <-time.After(m.execDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.execErr != nil {
		return nil, m.execErr
	}
	return m.execResult, nil
}

// mockDriver records call counts and returns canned results.
type mockDriver struct {
	openCalls  atomic.Int64
	closeCalls atomic.Int64
	openErr    error
	probe      Probe
	pool       Pool
}

func (d *mockDriver) Open(_ context.Context, _ Conn) (Pool, error) {
	d.openCalls.Add(1)
	if d.openErr != nil {
		return nil, d.openErr
	}
	return d.pool, nil
}
func (d *mockDriver) Probe(_ context.Context, _ Pool) (Probe, error) { return d.probe, nil }
func (d *mockDriver) Close(_ Pool)                                   { d.closeCalls.Add(1) }

func setupManager(t *testing.T, tag string) (*Manager, *mockDriver, *mockPool, int64) {
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
	repo := store.NewConnectionsRepo(db, mk)
	auditRepo := store.NewAuditRepo(db, mk)

	c, err := repo.Create(context.Background(), store.CreateConnectionParams{
		Alias: "p", Host: "h", Port: 5432, Tag: tag, TLSMode: store.TLSDisable,
		Username: "u", Password: "pw", Database: "d",
	})
	if err != nil {
		t.Fatal(err)
	}
	mp := &mockPool{
		execResult: &Result{Columns: []ColumnDef{{Name: "n", TypeName: "int4"}}, Rows: [][]any{{int64(1)}}, CommandTag: "SELECT 1"},
	}
	md := &mockDriver{pool: mp, probe: Probe{Version: "PostgreSQL 16.test"}}
	return NewManager(md, repo, auditRepo), md, mp, c.ID
}

func TestManager_OpenByID_CachesPool(t *testing.T) {
	mgr, drv, _, id := setupManager(t, store.TagLocal)
	p1, err := mgr.OpenByID(context.Background(), id, "")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := mgr.OpenByID(context.Background(), id, "")
	if err != nil {
		t.Fatal(err)
	}
	if p1 != p2 {
		t.Fatal("expected cached pool to be reused")
	}
	if got := drv.openCalls.Load(); got != 1 {
		t.Fatalf("Open should be called once, got %d", got)
	}
}

func TestManager_Ping(t *testing.T) {
	mgr, _, _, id := setupManager(t, store.TagLocal)
	if err := mgr.Ping(context.Background(), id, ""); err != nil {
		t.Fatal(err)
	}
}

func TestManager_Probe(t *testing.T) {
	mgr, _, _, id := setupManager(t, store.TagLocal)
	p, err := mgr.Probe(context.Background(), id, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != "PostgreSQL 16.test" {
		t.Fatalf("probe version mismatch: %q", p.Version)
	}
}

func TestManager_Execute_LocalReadOK(t *testing.T) {
	mgr, _, _, id := setupManager(t, store.TagLocal)
	res, err := mgr.Execute(context.Background(), ExecuteParams{
		ConnID: id, SQL: "SELECT 1", UserEmail: "u@x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(res.Rows))
	}
}

func TestManager_Execute_ProductionDDLBlocked(t *testing.T) {
	mgr, _, _, id := setupManager(t, store.TagProduction)
	_, err := mgr.Execute(context.Background(), ExecuteParams{
		ConnID: id, SQL: "DROP TABLE t", UserEmail: "u@x",
	})
	var be *BlockedError
	if !errors.As(err, &be) {
		t.Fatalf("want BlockedError, got %v", err)
	}
}

func TestManager_Execute_StagingDDLNeedsConfirm(t *testing.T) {
	mgr, _, _, id := setupManager(t, store.TagStaging)
	_, err := mgr.Execute(context.Background(), ExecuteParams{
		ConnID: id, SQL: "CREATE TABLE x (id INT)", UserEmail: "u@x",
	})
	var ce *ConfirmationRequiredError
	if !errors.As(err, &ce) {
		t.Fatalf("want ConfirmationRequiredError, got %v", err)
	}
	// With Confirm=true the same statement should run.
	if _, err := mgr.Execute(context.Background(), ExecuteParams{
		ConnID: id, SQL: "CREATE TABLE x (id INT)", Confirm: true, UserEmail: "u@x",
	}); err != nil {
		t.Fatalf("with confirm: %v", err)
	}
}

func TestManager_Execute_ProductionDangerousBlocked(t *testing.T) {
	mgr, _, _, id := setupManager(t, store.TagProduction)
	_, err := mgr.Execute(context.Background(), ExecuteParams{
		ConnID: id, SQL: "UPDATE users SET name='x'", Confirm: true, UserEmail: "u@x",
	})
	var be *BlockedError
	if !errors.As(err, &be) {
		t.Fatalf("want BlockedError for dangerous on production, got %v", err)
	}
}

func TestManager_Execute_TimeoutEnforced(t *testing.T) {
	mgr, _, mp, id := setupManager(t, store.TagProduction)
	mp.execDelay = 200 * time.Millisecond // > prod timeout of 10s? no -- need a smaller policy

	// Production timeout is 10s; we want to verify timeout enforcement, so
	// instead inject a context with a short deadline and confirm we honour it.
	// Manager wraps the caller's ctx with policy.Timeout *and* respects the
	// caller's deadline. Use Local tag here with a small caller deadline.
	mgr2, _, mp2, id2 := setupManager(t, store.TagLocal)
	mp2.execDelay = 200 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := mgr2.Execute(ctx, ExecuteParams{
		ConnID: id2, SQL: "SELECT pg_sleep(1)", UserEmail: "u@x",
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline exceeded, got %v", err)
	}
	_ = mgr // not used in this assertion
	_ = mp
	_ = id
}

func TestManager_Execute_AuditEmitted(t *testing.T) {
	mgr, _, _, id := setupManager(t, store.TagLocal)
	if _, err := mgr.Execute(context.Background(), ExecuteParams{
		ConnID: id, SQL: "SELECT 1", UserEmail: "u@x",
	}); err != nil {
		t.Fatal(err)
	}
	if err := mgr.audit.VerifyChain(context.Background()); err != nil {
		t.Fatalf("audit chain broken: %v", err)
	}
	n, err := mgr.audit.Count(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 audit entry (query.execute), got %d", n)
	}
}

func TestManager_ExecuteBatch_LocalOK(t *testing.T) {
	mgr, _, _, id := setupManager(t, store.TagLocal)
	res, err := mgr.ExecuteBatch(context.Background(), BatchParams{
		ConnID:    id,
		UserEmail: "u@x",
		Stmts: []string{
			`UPDATE "public"."t" SET "name" = 'x' WHERE "id" = '1'`,
			`DELETE FROM "public"."t" WHERE "id" = '2'`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Statements != 2 {
		t.Fatalf("want 2 statements, got %d", res.Statements)
	}
	if err := mgr.audit.VerifyChain(context.Background()); err != nil {
		t.Fatalf("audit chain broken: %v", err)
	}
}

func TestManager_ExecuteBatch_ReadOnlyBlocked(t *testing.T) {
	mgr, _, _, id := setupManager(t, store.TagLocal)
	_, err := mgr.ExecuteBatch(context.Background(), BatchParams{
		ConnID:    id,
		UserEmail: "viewer@x",
		ReadOnly:  true,
		Stmts:     []string{`UPDATE "public"."t" SET "name" = 'x' WHERE "id" = '1'`},
	})
	var be *BlockedError
	if !errors.As(err, &be) {
		t.Fatalf("want BlockedError for read-only batch, got %v", err)
	}
}

func TestManager_ExecuteBatch_StagingNeedsConfirm(t *testing.T) {
	mgr, _, _, id := setupManager(t, store.TagStaging)
	stmts := []string{`UPDATE "public"."t" SET "name" = 'x' WHERE "id" = '1'`}
	_, err := mgr.ExecuteBatch(context.Background(), BatchParams{
		ConnID: id, UserEmail: "u@x", Stmts: stmts,
	})
	var ce *ConfirmationRequiredError
	if !errors.As(err, &ce) {
		t.Fatalf("want ConfirmationRequiredError on staging, got %v", err)
	}
	if _, err := mgr.ExecuteBatch(context.Background(), BatchParams{
		ConnID: id, UserEmail: "u@x", Stmts: stmts, Confirm: true,
	}); err != nil {
		t.Fatalf("with confirm staging batch should run: %v", err)
	}
}

func TestManager_OpenError(t *testing.T) {
	mgr, drv, _, id := setupManager(t, store.TagLocal)
	drv.openErr = errors.New("nope")
	if _, err := mgr.OpenByID(context.Background(), id, ""); err == nil {
		t.Fatal("expected open error")
	}
}

func TestManager_CloseConn(t *testing.T) {
	mgr, drv, _, id := setupManager(t, store.TagLocal)
	if _, err := mgr.OpenByID(context.Background(), id, ""); err != nil {
		t.Fatal(err)
	}
	mgr.CloseConn(id)
	if got := drv.closeCalls.Load(); got != 1 {
		t.Fatalf("Close should be called once, got %d", got)
	}
	if _, err := mgr.OpenByID(context.Background(), id, ""); err != nil {
		t.Fatal(err)
	}
	if got := drv.openCalls.Load(); got != 2 {
		t.Fatalf("Open should now be called twice, got %d", got)
	}
}

func TestManager_Shutdown(t *testing.T) {
	mgr, drv, _, id := setupManager(t, store.TagLocal)
	if _, err := mgr.OpenByID(context.Background(), id, ""); err != nil {
		t.Fatal(err)
	}
	mgr.Shutdown()
	if got := drv.closeCalls.Load(); got != 1 {
		t.Fatalf("Shutdown should Close one pool, got %d", got)
	}
}

func TestManager_PassphraseRequired(t *testing.T) {
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
	repo := store.NewConnectionsRepo(db, mk)
	auditRepo := store.NewAuditRepo(db, mk)
	c, err := repo.Create(context.Background(), store.CreateConnectionParams{
		Alias: "prod", Host: "h", Port: 5432, Tag: store.TagProduction, TLSMode: store.TLSRequire,
		Username: "u", Password: "pw", Database: "d", Passphrase: "the-passphrase",
	})
	if err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(&mockDriver{pool: &mockPool{}}, repo, auditRepo)
	if _, err := mgr.OpenByID(context.Background(), c.ID, ""); !errors.Is(err, store.ErrPassphraseRequired) {
		t.Fatalf("want ErrPassphraseRequired, got %v", err)
	}
}
