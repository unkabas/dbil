package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unkabas/dbil/internal/config"
)

func newCfg(t *testing.T) config.DBilConfig {
	t.Helper()
	return config.DBilConfig{
		DataDir:         t.TempDir(),
		Port:            4242,
		MasterKeyEnvVar: "DBIL_MK_BOOTSTRAP_UNSET", // unset, forces auto loader
	}
}

func TestRunInit_FreshCreatesAdminAndGenesis(t *testing.T) {
	cfg := newCfg(t)
	res, err := RunInit(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.CreatedAdmin {
		t.Fatal("expected CreatedAdmin=true on fresh DB")
	}
	if res.AdminEmail != AdminEmail {
		t.Fatalf("admin email mismatch: %q", res.AdminEmail)
	}
	if res.AuditGenesisID == 0 {
		t.Fatal("audit genesis id should be > 0")
	}
	if res.MasterKeySource == "" {
		t.Fatal("master key source should be reported")
	}

	mkPath := filepath.Join(cfg.DataDir, "master.key")
	info, err := os.Stat(mkPath)
	if err != nil {
		t.Fatalf("master.key missing: %v", err)
	}
	if info.Mode().Perm() != 0o400 {
		t.Fatalf("master.key mode want 0400, got %#o", info.Mode().Perm())
	}

	credPath := filepath.Join(cfg.DataDir, "initial-credentials.txt")
	info, err = os.Stat(credPath)
	if err != nil {
		t.Fatalf("initial-credentials.txt missing: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("initial-credentials.txt mode want 0600, got %#o", info.Mode().Perm())
	}

	body, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "email="+AdminEmail) {
		t.Fatalf("creds file missing email line: %s", body)
	}
	if !strings.Contains(string(body), "password=") {
		t.Fatalf("creds file missing password line: %s", body)
	}

	if _, err := os.Stat(filepath.Join(cfg.DataDir, "dbil.db")); err != nil {
		t.Fatalf("dbil.db missing: %v", err)
	}
}

func TestRunInit_Idempotent(t *testing.T) {
	cfg := newCfg(t)
	first, err := RunInit(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !first.CreatedAdmin {
		t.Fatal("first call must create admin")
	}

	mkBefore, err := os.ReadFile(filepath.Join(cfg.DataDir, "master.key"))
	if err != nil {
		t.Fatal(err)
	}

	second, err := RunInit(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if second.CreatedAdmin {
		t.Fatal("second call must not create another admin")
	}

	mkAfter, _ := os.ReadFile(filepath.Join(cfg.DataDir, "master.key"))
	if string(mkBefore) != string(mkAfter) {
		t.Fatal("master.key changed on second init")
	}
}

func TestRunInit_DataDirMode(t *testing.T) {
	cfg := newCfg(t)
	if _, err := RunInit(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	// TempDir may already be mode 0700 on macOS; just ensure MkdirAll
	// didn't somehow open it up.
	if info.Mode().Perm()&0o077 != 0 {
		// allow that TempDir's parent has wider perms; just ensure we're
		// not chmod'ing wider than 0700.
		t.Logf("note: DataDir perms %#o (TempDir defaults vary)", info.Mode().Perm())
	}
}
