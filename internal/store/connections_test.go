package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/unkabas/dbil/internal/crypto"
)

func setupConns(t *testing.T) *ConnectionsRepo {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close(db) })
	if err := Apply(db); err != nil {
		t.Fatal(err)
	}
	mkB, _ := crypto.Random(crypto.KeySize)
	mk, _ := crypto.NewMasterKey(mkB)
	return NewConnectionsRepo(db, mk)
}

func validParams() CreateConnectionParams {
	return CreateConnectionParams{
		Alias:    "local-pg",
		Host:     "127.0.0.1",
		Port:     5432,
		Tag:      TagLocal,
		TLSMode:  TLSDisable,
		Username: "postgres",
		Password: "secret",
		Database: "appdb",
	}
}

func TestConnections_CreateListGet(t *testing.T) {
	r := setupConns(t)
	ctx := context.Background()

	c, err := r.Create(ctx, validParams())
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == 0 {
		t.Fatal("expected id > 0")
	}
	if c.RequiresPassphrase {
		t.Fatal("expected RequiresPassphrase=false")
	}

	all, err := r.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("want 1 row, got %d", len(all))
	}

	got, err := r.Get(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Alias != c.Alias {
		t.Fatalf("alias mismatch")
	}
}

func TestConnections_RevealRoundTrip(t *testing.T) {
	r := setupConns(t)
	ctx := context.Background()
	c, err := r.Create(ctx, validParams())
	if err != nil {
		t.Fatal(err)
	}
	rev, err := r.Reveal(ctx, c.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if rev.Username != "postgres" {
		t.Fatalf("username %q", rev.Username)
	}
	if rev.Password != "secret" {
		t.Fatalf("password %q", rev.Password)
	}
	if rev.Database != "appdb" {
		t.Fatalf("database %q", rev.Database)
	}
}

func TestConnections_PassphraseRoundTrip(t *testing.T) {
	r := setupConns(t)
	ctx := context.Background()
	p := validParams()
	p.Alias = "prod-pg"
	p.Tag = TagProduction
	p.TLSMode = TLSRequire
	p.Passphrase = "correct-horse-battery"

	c, err := r.Create(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if !c.RequiresPassphrase {
		t.Fatal("expected RequiresPassphrase=true")
	}

	// Empty passphrase
	if _, err := r.Reveal(ctx, c.ID, ""); !errors.Is(err, ErrPassphraseRequired) {
		t.Fatalf("want ErrPassphraseRequired, got %v", err)
	}

	// Wrong passphrase
	if _, err := r.Reveal(ctx, c.ID, "wrong-passphrase"); !errors.Is(err, ErrInvalidPassphrase) {
		t.Fatalf("want ErrInvalidPassphrase, got %v", err)
	}

	// Correct passphrase
	rev, err := r.Reveal(ctx, c.ID, "correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	if rev.Password != "secret" {
		t.Fatalf("password decrypt mismatch: %q", rev.Password)
	}
}

func TestConnections_DuplicateAlias(t *testing.T) {
	r := setupConns(t)
	ctx := context.Background()
	if _, err := r.Create(ctx, validParams()); err != nil {
		t.Fatal(err)
	}
	_, err := r.Create(ctx, validParams())
	if !errors.Is(err, ErrConnectionExists) {
		t.Fatalf("want ErrConnectionExists, got %v", err)
	}
}

func TestConnections_GetMissing(t *testing.T) {
	r := setupConns(t)
	_, err := r.Get(context.Background(), 999)
	if !errors.Is(err, ErrConnectionNotFound) {
		t.Fatalf("want ErrConnectionNotFound, got %v", err)
	}
}

func TestConnections_DeleteAndRevealAfter(t *testing.T) {
	r := setupConns(t)
	ctx := context.Background()
	c, _ := r.Create(ctx, validParams())
	if err := r.Delete(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, c.ID); !errors.Is(err, ErrConnectionNotFound) {
		t.Fatalf("second delete should be ErrConnectionNotFound, got %v", err)
	}
	if _, err := r.Reveal(ctx, c.ID, ""); !errors.Is(err, ErrConnectionNotFound) {
		t.Fatalf("Reveal after delete should be ErrConnectionNotFound, got %v", err)
	}
}

func TestConnections_InvalidTag(t *testing.T) {
	r := setupConns(t)
	p := validParams()
	p.Tag = "nope"
	if _, err := r.Create(context.Background(), p); err == nil {
		t.Fatal("expected invalid tag error")
	}
}

func TestConnections_InvalidTLSMode(t *testing.T) {
	r := setupConns(t)
	p := validParams()
	p.TLSMode = "nope"
	if _, err := r.Create(context.Background(), p); err == nil {
		t.Fatal("expected invalid tls_mode error")
	}
}

func TestConnections_InvalidPort(t *testing.T) {
	r := setupConns(t)
	p := validParams()
	p.Port = 0
	if _, err := r.Create(context.Background(), p); err == nil {
		t.Fatal("expected invalid port error")
	}
}

func TestConnections_EmptyAlias(t *testing.T) {
	r := setupConns(t)
	p := validParams()
	p.Alias = ""
	if _, err := r.Create(context.Background(), p); err == nil {
		t.Fatal("expected empty alias error")
	}
}
