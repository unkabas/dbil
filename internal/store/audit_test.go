package store

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/unkabas/dbil/internal/audit"
	"github.com/unkabas/dbil/internal/crypto"
)

func setupAudit(t *testing.T) *AuditRepo {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Close(db) })
	if err := Apply(db); err != nil {
		t.Fatal(err)
	}
	mkBytes, err := crypto.Random(crypto.KeySize)
	if err != nil {
		t.Fatal(err)
	}
	mk, err := crypto.NewMasterKey(mkBytes)
	if err != nil {
		t.Fatal(err)
	}
	return NewAuditRepo(db, mk)
}

func TestAudit_AppendChain(t *testing.T) {
	r := setupAudit(t)
	ctx := context.Background()

	r1, err := r.Append(ctx, "system", "bootstrap.init", "dbil", map[string]any{"version": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if r1.Entry.ID != 1 {
		t.Fatalf("want first id 1, got %d", r1.Entry.ID)
	}
	if r1.Entry.PrevHash != audit.GenesisHash {
		t.Fatal("first entry must reference GenesisHash")
	}

	r2, err := r.Append(ctx, "admin@local", "auth.login", "user:1", map[string]any{"ip": "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if r2.Entry.ID != 2 {
		t.Fatalf("want second id 2, got %d", r2.Entry.ID)
	}
	if r2.Entry.PrevHash != r1.EntryHash {
		t.Fatal("second entry must reference first entry's hash")
	}

	n, err := r.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("want 2 entries, got %d", n)
	}
}

func TestAudit_VerifyChain_OK(t *testing.T) {
	r := setupAudit(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := r.Append(ctx, "u", "a", "r", map[string]any{"i": i}); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.VerifyChain(ctx); err != nil {
		t.Fatalf("VerifyChain on clean DB: %v", err)
	}
}

func TestAudit_VerifyChain_DetectsCiphertextTampering(t *testing.T) {
	r := setupAudit(t)
	ctx := context.Background()
	if _, err := r.Append(ctx, "u", "a", "r", map[string]any{"x": 1}); err != nil {
		t.Fatal(err)
	}
	// Replace the stored ciphertext with garbage of plausible length.
	if _, err := r.DB.ExecContext(ctx, `UPDATE audit_log SET details_enc = ? WHERE id = 1`, bytes.Repeat([]byte{0x42}, 32)); err != nil {
		t.Fatal(err)
	}
	if err := r.VerifyChain(ctx); !errors.Is(err, audit.ErrChainBroken) {
		t.Fatalf("want ErrChainBroken, got %v", err)
	}
}

func TestAudit_VerifyChain_DetectsHashTampering(t *testing.T) {
	r := setupAudit(t)
	ctx := context.Background()
	if _, err := r.Append(ctx, "u", "a", "r", map[string]any{"x": 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.DB.ExecContext(ctx, `UPDATE audit_log SET entry_hash = ? WHERE id = 1`, bytes.Repeat([]byte{0xee}, 32)); err != nil {
		t.Fatal(err)
	}
	if err := r.VerifyChain(ctx); !errors.Is(err, audit.ErrChainBroken) {
		t.Fatalf("want ErrChainBroken, got %v", err)
	}
}

func TestAudit_VerifyChain_DetectsPrevHashTampering(t *testing.T) {
	r := setupAudit(t)
	ctx := context.Background()
	if _, err := r.Append(ctx, "u", "a", "r", map[string]any{"x": 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Append(ctx, "u", "a", "r", map[string]any{"y": 2}); err != nil {
		t.Fatal(err)
	}
	// Smash the prev_hash on the second row so the chain disconnects.
	if _, err := r.DB.ExecContext(ctx, `UPDATE audit_log SET prev_hash = ? WHERE id = 2`, bytes.Repeat([]byte{0xff}, 32)); err != nil {
		t.Fatal(err)
	}
	if err := r.VerifyChain(ctx); !errors.Is(err, audit.ErrChainBroken) {
		t.Fatalf("want ErrChainBroken, got %v", err)
	}
}
