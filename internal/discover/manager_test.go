package discover

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unkabas/dbil/internal/crypto"
	"github.com/unkabas/dbil/internal/store"
)

type fakeAudit struct {
	mu      sync.Mutex
	entries []map[string]any
}

func (f *fakeAudit) Append(_ context.Context, _, action, resource string, details map[string]any) (store.AppendResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := map[string]any{"action": action, "resource": resource}
	for k, v := range details {
		d[k] = v
	}
	f.entries = append(f.entries, d)
	return store.AppendResult{}, nil
}

func (f *fakeAudit) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.entries)
}

type stubScanner struct {
	source  Source
	entries []Entry
	err     error
	calls   atomic.Int64
}

func (s *stubScanner) Source() Source { return s.source }
func (s *stubScanner) Scan(_ context.Context) ([]Entry, error) {
	s.calls.Add(1)
	if s.err != nil {
		return nil, s.err
	}
	return s.entries, nil
}

type panicScanner struct{}

func (p *panicScanner) Source() Source                            { return SourceEnv }
func (p *panicScanner) Scan(_ context.Context) ([]Entry, error) { panic("boom") }

func newRepo(t *testing.T) *store.DiscoveredRepo {
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
	return store.NewDiscoveredRepo(db, mk)
}

func TestManager_RunOnce_InsertsThenRefreshes(t *testing.T) {
	repo := newRepo(t)
	audit := &fakeAudit{}
	m, err := NewManager(Config{Mode: ModeOff}, repo, audit, nil)
	if err != nil {
		t.Fatal(err)
	}
	scan := &stubScanner{
		source: SourceEnv,
		entries: []Entry{
			{Source: SourceEnv, Key: "k1", Alias: "a", Host: "h", Port: 5432, Database: "d", Username: "u", Tag: "dev"},
		},
	}
	m.SetScanners(scan)

	if _, err := m.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if audit.count() != 1 {
		t.Fatalf("first run audit count: %d (want 1)", audit.count())
	}

	if _, err := m.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if audit.count() != 1 {
		t.Fatalf("second run should not re-audit same key: %d", audit.count())
	}

	all, _ := repo.List(context.Background())
	if len(all) != 1 || all[0].Alias != "a" {
		t.Fatalf("list: %+v", all)
	}
}

func TestManager_RunOnce_DisappearingMarkedUnreachable(t *testing.T) {
	repo := newRepo(t)
	m, _ := NewManager(Config{Mode: ModeOff}, repo, nil, nil)
	scan := &stubScanner{
		source: SourceEnv,
		entries: []Entry{
			{Source: SourceEnv, Key: "k1", Alias: "a", Host: "h", Port: 5432, Database: "d", Username: "u", Tag: "dev"},
			{Source: SourceEnv, Key: "k2", Alias: "b", Host: "h", Port: 5432, Database: "d", Username: "u", Tag: "dev"},
		},
	}
	m.SetScanners(scan)
	if _, err := m.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	// k1 disappears second tick.
	scan.entries = scan.entries[1:]
	if _, err := m.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	all, _ := repo.List(context.Background())
	statusByKey := map[string]string{}
	for _, d := range all {
		statusByKey[d.SourceKey] = d.Status
	}
	if statusByKey["k1"] != store.DiscoverStatusUnreachable {
		t.Fatalf("k1 should be unreachable: %s", statusByKey["k1"])
	}
	if statusByKey["k2"] != store.DiscoverStatusPending {
		t.Fatalf("k2 should stay pending: %s", statusByKey["k2"])
	}
}

func TestManager_RunOnce_ScanErrorDoesNotKill(t *testing.T) {
	repo := newRepo(t)
	m, _ := NewManager(Config{Mode: ModeOff}, repo, nil, nil)
	good := &stubScanner{
		source: SourceDocker,
		entries: []Entry{
			{Source: SourceDocker, Key: "ok", Alias: "g", Host: "h", Port: 5432, Database: "d", Username: "u", Tag: "dev"},
		},
	}
	bad := &stubScanner{source: SourceEnv, err: errors.New("scan failure")}
	m.SetScanners(bad, good)
	if _, err := m.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	all, _ := repo.List(context.Background())
	if len(all) != 1 || all[0].SourceKey != "ok" {
		t.Fatalf("good scanner still persisted? got %+v", all)
	}
}

func TestManager_StartShutdown(t *testing.T) {
	repo := newRepo(t)
	m, _ := NewManager(Config{Mode: ModeOff}, repo, nil, nil)
	scan := &stubScanner{
		source: SourceEnv,
		entries: []Entry{
			{Source: SourceEnv, Key: "k1", Alias: "a", Host: "h", Port: 5432, Database: "d", Username: "u", Tag: "dev"},
		},
	}
	m.SetScanners(scan)
	m.SetInterval(40 * time.Millisecond)

	m.Start()
	time.Sleep(150 * time.Millisecond)
	m.Shutdown()

	if scan.calls.Load() < 2 {
		t.Fatalf("expected ≥2 scan invocations, got %d", scan.calls.Load())
	}
	// Calling Shutdown twice is a no-op.
	m.Shutdown()
}

func TestManager_PanicInScannerRecovers(t *testing.T) {
	repo := newRepo(t)
	m, _ := NewManager(Config{Mode: ModeOff}, repo, nil, nil)
	m.SetScanners(&panicScanner{})
	m.SetInterval(30 * time.Millisecond)

	m.Start()
	time.Sleep(120 * time.Millisecond)
	m.Shutdown()
	// If we got here without the process exiting, the panic was recovered.
}

func TestNewManager_RejectsUnknownMode(t *testing.T) {
	repo := newRepo(t)
	if _, err := NewManager(Config{Mode: "weird"}, repo, nil, nil); err == nil {
		t.Fatal("expected error for unknown mode")
	}
}
