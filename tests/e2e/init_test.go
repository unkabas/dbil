// Package e2e runs the built dbil binary as a subprocess and asserts that
// `dbil init` produces the expected on-disk artifacts and DB state.
package e2e

import (
	"bytes"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "dbil")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "../../cmd/dbil")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build: %v\n%s", err, stderr.String())
	}
	return bin
}

func runInit(t *testing.T, bin, dataDir string) (stdout, stderr string, exitErr error) {
	t.Helper()
	cmd := exec.Command(bin, "init")
	// inherit cwd but force DBIL_DATA_DIR
	env := []string{"DBIL_DATA_DIR=" + dataDir, "PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	cmd.Env = env
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	exitErr = cmd.Run()
	return so.String(), se.String(), exitErr
}

func TestE2E_InitArtifactsAndDBContents(t *testing.T) {
	bin := buildBinary(t)
	dataDir := t.TempDir()

	stdout, stderr, err := runInit(t, bin, dataDir)
	if err != nil {
		t.Fatalf("dbil init exit=%v\n--stdout--\n%s\n--stderr--\n%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "init complete: admin admin@local created") {
		t.Fatalf("expected success line in stdout, got:\n%s", stdout)
	}

	mk, err := os.Stat(filepath.Join(dataDir, "master.key"))
	if err != nil {
		t.Fatalf("master.key missing: %v", err)
	}
	if mk.Mode().Perm() != 0o400 {
		t.Fatalf("master.key mode want 0400, got %#o", mk.Mode().Perm())
	}
	if mk.Size() != 32 {
		t.Fatalf("master.key size want 32, got %d", mk.Size())
	}

	cred, err := os.Stat(filepath.Join(dataDir, "initial-credentials.txt"))
	if err != nil {
		t.Fatalf("initial-credentials.txt missing: %v", err)
	}
	if cred.Mode().Perm() != 0o600 {
		t.Fatalf("initial-credentials.txt mode want 0600, got %#o", cred.Mode().Perm())
	}
	body, _ := os.ReadFile(filepath.Join(dataDir, "initial-credentials.txt"))
	if !strings.Contains(string(body), "email=admin@local") {
		t.Fatalf("creds missing email line: %s", body)
	}
	if !strings.Contains(string(body), "password=") {
		t.Fatalf("creds missing password line: %s", body)
	}

	dbPath := filepath.Join(dataDir, "dbil.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("dbil.db missing: %v", err)
	}

	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var users int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&users); err != nil {
		t.Fatal(err)
	}
	if users != 1 {
		t.Fatalf("want 1 user row, got %d", users)
	}

	var role string
	if err := db.QueryRow("SELECT role FROM users LIMIT 1").Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != "admin" {
		t.Fatalf("want role admin, got %q", role)
	}

	var audits int
	if err := db.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 1 {
		t.Fatalf("want 1 audit row, got %d", audits)
	}

	var action string
	if err := db.QueryRow("SELECT action FROM audit_log WHERE id = 1").Scan(&action); err != nil {
		t.Fatal(err)
	}
	if action != "bootstrap.init" {
		t.Fatalf("want action bootstrap.init, got %q", action)
	}
}

func TestE2E_Idempotent(t *testing.T) {
	bin := buildBinary(t)
	dataDir := t.TempDir()

	if _, _, err := runInit(t, bin, dataDir); err != nil {
		t.Fatalf("first init: %v", err)
	}
	stdout, _, err := runInit(t, bin, dataDir)
	if err != nil {
		t.Fatalf("second init: %v", err)
	}
	if !strings.Contains(stdout, "already initialized") {
		t.Fatalf("second init stdout missing 'already initialized':\n%s", stdout)
	}

	db, _ := sql.Open("sqlite", "file:"+filepath.Join(dataDir, "dbil.db"))
	defer db.Close()

	var users int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&users); err != nil {
		t.Fatal(err)
	}
	if users != 1 {
		t.Fatalf("want 1 user after re-init, got %d", users)
	}

	var audits int
	if err := db.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 1 {
		t.Fatalf("want 1 audit row after re-init, got %d", audits)
	}
}
