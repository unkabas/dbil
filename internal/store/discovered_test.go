package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/unkabas/dbil/internal/crypto"
	"github.com/unkabas/dbil/internal/store"
)

func newDiscoveredRepo(t *testing.T) *store.DiscoveredRepo {
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

func sampleUpsert() store.DiscoveredUpsert {
	return store.DiscoveredUpsert{
		Source:    store.DiscoverSourceDocker,
		SourceKey: "abc123",
		Alias:     "app",
		Host:      "postgres",
		Port:      5432,
		Database:  "appdb",
		Username:  "app",
		Password:  "s3cret",
		Tag:       store.TagDev,
	}
}

func TestDiscovered_UpsertInsertsThenRefresh(t *testing.T) {
	r := newDiscoveredRepo(t)
	ctx := context.Background()

	d1, isNew, err := r.Upsert(ctx, sampleUpsert())
	if err != nil {
		t.Fatal(err)
	}
	if !isNew || d1.ID == 0 {
		t.Fatalf("first upsert: want new+id, got %+v new=%v", d1, isNew)
	}
	if !d1.HasPassword {
		t.Fatal("password should be stored")
	}

	// Second upsert with refreshed alias updates in place.
	u := sampleUpsert()
	u.Alias = "app-renamed"
	d2, isNew, err := r.Upsert(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	if isNew {
		t.Fatal("second upsert should not create a new row")
	}
	if d2.ID != d1.ID || d2.Alias != "app-renamed" {
		t.Fatalf("expected refresh, got %+v", d2)
	}
}

func TestDiscovered_RevealRoundTrip(t *testing.T) {
	r := newDiscoveredRepo(t)
	ctx := context.Background()
	d, _, _ := r.Upsert(ctx, sampleUpsert())

	pw, err := r.RevealPassword(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pw != "s3cret" {
		t.Fatalf("password mismatch: %q", pw)
	}
}

func TestDiscovered_RejectStays(t *testing.T) {
	r := newDiscoveredRepo(t)
	ctx := context.Background()
	d, _, _ := r.Upsert(ctx, sampleUpsert())

	if err := r.Reject(ctx, d.ID); err != nil {
		t.Fatal(err)
	}

	// Re-upsert with same source_key — should remain rejected.
	d2, _, _ := r.Upsert(ctx, sampleUpsert())
	if d2.Status != store.DiscoverStatusRejected {
		t.Fatalf("status after re-upsert: %s (want rejected)", d2.Status)
	}
}

func TestDiscovered_MarkUnreachable(t *testing.T) {
	r := newDiscoveredRepo(t)
	ctx := context.Background()

	// Two entries from docker source.
	a := sampleUpsert()
	a.SourceKey = "container-a"
	if _, _, err := r.Upsert(ctx, a); err != nil {
		t.Fatal(err)
	}
	b := sampleUpsert()
	b.SourceKey = "container-b"
	if _, _, err := r.Upsert(ctx, b); err != nil {
		t.Fatal(err)
	}

	// Second scan only sees container-a.
	if err := r.MarkUnreachable(ctx, store.DiscoverSourceDocker, []string{"container-a"}); err != nil {
		t.Fatal(err)
	}

	all, _ := r.List(ctx)
	statusByKey := map[string]string{}
	for _, d := range all {
		statusByKey[d.SourceKey] = d.Status
	}
	if statusByKey["container-a"] != store.DiscoverStatusPending {
		t.Fatalf("container-a should stay pending: %s", statusByKey["container-a"])
	}
	if statusByKey["container-b"] != store.DiscoverStatusUnreachable {
		t.Fatalf("container-b should be unreachable: %s", statusByKey["container-b"])
	}
}

func TestDiscovered_RevivePendingFromUnreachable(t *testing.T) {
	r := newDiscoveredRepo(t)
	ctx := context.Background()

	d, _, _ := r.Upsert(ctx, sampleUpsert())
	// Force unreachable
	if err := r.MarkUnreachable(ctx, store.DiscoverSourceDocker, nil); err != nil {
		t.Fatal(err)
	}
	// Re-upsert revives to pending.
	d2, isNew, err := r.Upsert(ctx, sampleUpsert())
	if err != nil {
		t.Fatal(err)
	}
	if isNew || d2.ID != d.ID || d2.Status != store.DiscoverStatusPending {
		t.Fatalf("revive: %+v isNew=%v", d2, isNew)
	}
}

func TestDiscovered_ListFilter(t *testing.T) {
	r := newDiscoveredRepo(t)
	ctx := context.Background()

	a := sampleUpsert()
	a.SourceKey = "k1"
	if _, _, err := r.Upsert(ctx, a); err != nil {
		t.Fatal(err)
	}
	b := sampleUpsert()
	b.SourceKey = "k2"
	dB, _, _ := r.Upsert(ctx, b)
	if err := r.Reject(ctx, dB.ID); err != nil {
		t.Fatal(err)
	}

	only, err := r.List(ctx, store.DiscoverStatusPending)
	if err != nil {
		t.Fatal(err)
	}
	if len(only) != 1 || only[0].SourceKey != "k1" {
		t.Fatalf("filtered list: %+v", only)
	}
}

func TestDiscovered_ValidationErrors(t *testing.T) {
	r := newDiscoveredRepo(t)
	ctx := context.Background()

	bad := sampleUpsert()
	bad.Tag = "weird"
	if _, _, err := r.Upsert(ctx, bad); err == nil {
		t.Fatal("expected tag validation error")
	}

	bad2 := sampleUpsert()
	bad2.Source = "other"
	if _, _, err := r.Upsert(ctx, bad2); err == nil {
		t.Fatal("expected source validation error")
	}

	bad3 := sampleUpsert()
	bad3.Port = 0
	if _, _, err := r.Upsert(ctx, bad3); err == nil {
		t.Fatal("expected port validation error")
	}
}
