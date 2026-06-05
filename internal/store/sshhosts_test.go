package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/unkabas/dbil/internal/crypto"
)

func setupSSH(t *testing.T) (*SSHHostsRepo, *ConnectionsRepo) {
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
	return NewSSHHostsRepo(db, mk), NewConnectionsRepo(db, mk)
}

func TestSSHHosts_CreateAndReveal(t *testing.T) {
	repo, _ := setupSSH(t)
	ctx := context.Background()
	h, err := repo.Create(ctx, CreateSSHHostParams{
		Alias: "bastion", Host: "10.0.0.1", Port: 22, Username: "deploy",
		AuthMethod: SSHAuthKey, Secret: "-----BEGIN KEY-----\nabc\n-----END KEY-----",
		KeyPassphrase: "keypass",
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.RequiresPassphrase {
		t.Fatal("no user passphrase => RequiresPassphrase must be false")
	}
	rev, err := repo.Reveal(ctx, h.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if rev.Secret == "" || rev.KeyPassphrase != "keypass" {
		t.Fatalf("reveal mismatch: secret=%q keypass=%q", rev.Secret, rev.KeyPassphrase)
	}
}

func TestSSHHosts_PassphraseWrapped(t *testing.T) {
	repo, _ := setupSSH(t)
	ctx := context.Background()
	h, err := repo.Create(ctx, CreateSSHHostParams{
		Alias: "prod-bastion", Host: "h", Port: 22, Username: "u",
		AuthMethod: SSHAuthPassword, Secret: "s3cret", Passphrase: "unlock-me",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !h.RequiresPassphrase {
		t.Fatal("passphrase-wrapped host must set RequiresPassphrase")
	}
	if _, err := repo.Reveal(ctx, h.ID, ""); !errors.Is(err, ErrSSHPassphraseRequired) {
		t.Fatalf("want ErrSSHPassphraseRequired, got %v", err)
	}
	if _, err := repo.Reveal(ctx, h.ID, "wrong"); !errors.Is(err, ErrSSHInvalidPassphrase) {
		t.Fatalf("want ErrSSHInvalidPassphrase, got %v", err)
	}
	rev, err := repo.Reveal(ctx, h.ID, "unlock-me")
	if err != nil {
		t.Fatal(err)
	}
	if rev.Secret != "s3cret" {
		t.Fatalf("secret mismatch: %q", rev.Secret)
	}
}

func TestSSHHosts_DeleteReferenced(t *testing.T) {
	repo, conns := setupSSH(t)
	ctx := context.Background()
	h, err := repo.Create(ctx, CreateSSHHostParams{
		Alias: "b", Host: "h", Port: 22, Username: "u", AuthMethod: SSHAuthPassword, Secret: "p",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conns.Create(ctx, CreateConnectionParams{
		Alias: "c", Host: "127.0.0.1", Port: 5432, Tag: TagLocal, TLSMode: TLSDisable,
		Username: "u", Password: "p", Database: "d", SSHHostID: &h.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, h.ID); !errors.Is(err, ErrSSHHostReferenced) {
		t.Fatalf("want ErrSSHHostReferenced, got %v", err)
	}
}

func TestSSHHosts_SetFingerprint(t *testing.T) {
	repo, _ := setupSSH(t)
	ctx := context.Background()
	h, err := repo.Create(ctx, CreateSSHHostParams{
		Alias: "b", Host: "h", Port: 22, Username: "u", AuthMethod: SSHAuthPassword, Secret: "p",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetFingerprint(ctx, h.ID, "SHA256:abcdef"); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.HostKeyFingerprint != "SHA256:abcdef" {
		t.Fatalf("fingerprint not pinned: %q", got.HostKeyFingerprint)
	}
}
