package postgres

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/unkabas/dbil/internal/crypto"
	"github.com/unkabas/dbil/internal/store"
)

// mockPool is a no-op Pool for Manager tests.
type mockPool struct {
	pingErr error
	closed  atomic.Bool
}

func (m *mockPool) Ping(_ context.Context) error { return m.pingErr }
func (m *mockPool) Close()                       { m.closed.Store(true) }

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

func setupManager(t *testing.T) (*Manager, *mockDriver, int64) {
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

	c, err := repo.Create(context.Background(), store.CreateConnectionParams{
		Alias: "p", Host: "h", Port: 5432, Tag: store.TagLocal, TLSMode: store.TLSDisable,
		Username: "u", Password: "pw", Database: "d",
	})
	if err != nil {
		t.Fatal(err)
	}
	mp := &mockPool{}
	md := &mockDriver{pool: mp, probe: Probe{Version: "PostgreSQL 16.test"}}
	return NewManager(md, repo), md, c.ID
}

func TestManager_OpenByID_CachesPool(t *testing.T) {
	mgr, drv, id := setupManager(t)

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
		t.Fatalf("Open should be called exactly once, got %d", got)
	}
}

func TestManager_Ping(t *testing.T) {
	mgr, _, id := setupManager(t)
	if err := mgr.Ping(context.Background(), id, ""); err != nil {
		t.Fatal(err)
	}
}

func TestManager_Probe(t *testing.T) {
	mgr, _, id := setupManager(t)
	p, err := mgr.Probe(context.Background(), id, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != "PostgreSQL 16.test" {
		t.Fatalf("probe version mismatch: %q", p.Version)
	}
}

func TestManager_OpenError(t *testing.T) {
	mgr, drv, id := setupManager(t)
	drv.openErr = errors.New("nope")
	if _, err := mgr.OpenByID(context.Background(), id, ""); err == nil {
		t.Fatal("expected open error")
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
	c, err := repo.Create(context.Background(), store.CreateConnectionParams{
		Alias: "prod", Host: "h", Port: 5432, Tag: store.TagProduction, TLSMode: store.TLSRequire,
		Username: "u", Password: "pw", Database: "d", Passphrase: "the-passphrase",
	})
	if err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(&mockDriver{pool: &mockPool{}}, repo)
	if _, err := mgr.OpenByID(context.Background(), c.ID, ""); !errors.Is(err, store.ErrPassphraseRequired) {
		t.Fatalf("want ErrPassphraseRequired, got %v", err)
	}
}

func TestManager_CloseConn(t *testing.T) {
	mgr, drv, id := setupManager(t)
	if _, err := mgr.OpenByID(context.Background(), id, ""); err != nil {
		t.Fatal(err)
	}
	mgr.CloseConn(id)
	if got := drv.closeCalls.Load(); got != 1 {
		t.Fatalf("Close should be called once, got %d", got)
	}
	// Opening again should call Open once more (cache was cleared).
	if _, err := mgr.OpenByID(context.Background(), id, ""); err != nil {
		t.Fatal(err)
	}
	if got := drv.openCalls.Load(); got != 2 {
		t.Fatalf("Open should now be called twice, got %d", got)
	}
}

func TestManager_Shutdown(t *testing.T) {
	mgr, drv, id := setupManager(t)
	if _, err := mgr.OpenByID(context.Background(), id, ""); err != nil {
		t.Fatal(err)
	}
	mgr.Shutdown()
	if got := drv.closeCalls.Load(); got != 1 {
		t.Fatalf("Shutdown should Close one pool, got %d", got)
	}
}
