package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// TestE2E_Query exercises the full query pipeline against a real Postgres
// testcontainer: SELECT 1, CREATE TABLE, INSERT, SELECT FROM, plus the
// production-tag DROP TABLE block.
//
// Skips when Docker is unavailable.
func TestE2E_Query(t *testing.T) {
	ctx := context.Background()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Skipf("docker not available or postgres container failed: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	pgHost, err := pg.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mapped, err := pg.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}
	pgPort, err := strconv.Atoi(mapped.Port())
	if err != nil {
		t.Fatal(err)
	}

	bin := buildBinary(t)
	dataDir := t.TempDir()
	if _, _, err := runInit(t, bin, dataDir); err != nil {
		t.Fatalf("init: %v", err)
	}
	password := readInitialPassword(t, dataDir)

	port := freePort(t)
	cmd := exec.Command(bin, "serve")
	cmd.Env = []string{
		"DBIL_DATA_DIR=" + dataDir,
		"DBIL_PORT=" + strconv.Itoa(port),
		"PATH=" + os.Getenv("PATH"),
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatalf("start serve: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() { _, _ = cmd.Process.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
		}
	})

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitHealthz(base, 5*time.Second); err != nil {
		t.Fatalf("server never became healthy: %v\nstderr: %s", err, stderr.String())
	}

	// Login.
	var login struct {
		Token string `json:"token"`
	}
	if err := postJSON(base+"/api/auth/login", "", map[string]string{
		"email": "admin@local", "password": password,
	}, &login); err != nil {
		t.Fatalf("login: %v", err)
	}

	// Create local connection pointing at testcontainer.
	var local struct {
		ID int64 `json:"id"`
	}
	if err := postJSON(base+"/api/connections", login.Token, map[string]any{
		"alias":    "local-tc",
		"host":     pgHost,
		"port":     pgPort,
		"tag":      "local",
		"tls_mode": "disable",
		"username": "testuser",
		"password": "testpass",
		"database": "testdb",
	}, &local); err != nil {
		t.Fatalf("create local connection: %v", err)
	}

	// SELECT 1 -> 200 with command_tag "SELECT 1".
	var sel struct {
		Columns    []map[string]any `json:"columns"`
		Rows       [][]any          `json:"rows"`
		CommandTag string           `json:"command_tag"`
	}
	if err := postJSON(fmt.Sprintf("%s/api/connections/%d/query", base, local.ID), login.Token,
		map[string]string{"sql": "SELECT 1 AS n"}, &sel); err != nil {
		t.Fatalf("SELECT 1: %v", err)
	}
	if sel.CommandTag != "SELECT 1" {
		t.Fatalf("command_tag: want 'SELECT 1', got %q", sel.CommandTag)
	}
	if len(sel.Rows) != 1 || len(sel.Rows[0]) != 1 {
		t.Fatalf("rows: %v", sel.Rows)
	}
	if len(sel.Columns) != 1 {
		t.Fatalf("columns: %v", sel.Columns)
	}
	if name, _ := sel.Columns[0]["name"].(string); name != "n" {
		t.Fatalf("column name: %v", sel.Columns[0])
	}

	// CREATE TABLE -> 200.
	var cr struct {
		CommandTag string `json:"command_tag"`
	}
	if err := postJSON(fmt.Sprintf("%s/api/connections/%d/query", base, local.ID), login.Token,
		map[string]string{"sql": "CREATE TABLE t (id INT)"}, &cr); err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}

	// INSERT -> 200 with rows_affected = 1.
	var ins struct {
		CommandTag   string `json:"command_tag"`
		RowsAffected int64  `json:"rows_affected"`
	}
	if err := postJSON(fmt.Sprintf("%s/api/connections/%d/query", base, local.ID), login.Token,
		map[string]string{"sql": "INSERT INTO t (id) VALUES (42)"}, &ins); err != nil {
		t.Fatalf("INSERT: %v", err)
	}
	if ins.RowsAffected != 1 {
		t.Fatalf("INSERT rows_affected: want 1, got %d", ins.RowsAffected)
	}

	// SELECT FROM t -> 200, one row with 42.
	var sel2 struct {
		Rows [][]any `json:"rows"`
	}
	if err := postJSON(fmt.Sprintf("%s/api/connections/%d/query", base, local.ID), login.Token,
		map[string]string{"sql": "SELECT id FROM t"}, &sel2); err != nil {
		t.Fatalf("SELECT FROM t: %v", err)
	}
	if len(sel2.Rows) != 1 {
		t.Fatalf("SELECT result rows: %v", sel2.Rows)
	}
	// JSON unmarshals int4 -> float64 by default; compare loosely.
	if fmt.Sprintf("%v", sel2.Rows[0][0]) != "42" {
		t.Fatalf("row value: %v", sel2.Rows[0])
	}

	// Create a *production* connection at the same testcontainer.
	var prod struct {
		ID int64 `json:"id"`
	}
	if err := postJSON(base+"/api/connections", login.Token, map[string]any{
		"alias":    "prod-tc",
		"host":     pgHost,
		"port":     pgPort,
		"tag":      "production",
		"tls_mode": "disable",
		"username": "testuser",
		"password": "testpass",
		"database": "testdb",
	}, &prod); err != nil {
		t.Fatalf("create production connection: %v", err)
	}

	// DROP TABLE on production must 403 even with X-Confirm: yes.
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/connections/%d/query", base, prod.ID),
		bytes.NewBufferString(`{"sql":"DROP TABLE t"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("X-Confirm", "yes")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("production DROP TABLE: want 403, got %d (%s)", resp.StatusCode, body)
	}

	// Cleanup: drop the table via the local-tagged connection so a re-run
	// against the same image starts clean (testcontainers throws away the
	// container anyway, but this exercises a successful DDL through local).
	if err := postJSON(fmt.Sprintf("%s/api/connections/%d/query", base, local.ID), login.Token,
		map[string]string{"sql": "DROP TABLE t"}, &struct{}{}); err != nil {
		t.Fatalf("DROP TABLE on local: %v", err)
	}
}
