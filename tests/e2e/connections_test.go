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
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// TestE2E_Connections is the full real-Postgres integration:
// init -> serve -> login -> POST /api/connections (pointing at a
// testcontainer Postgres) -> /test (200, version contains "PostgreSQL")
// -> DELETE -> GET (404).
//
// Skips when Docker is unavailable so contributors without Docker still
// get a clean test run.
func TestE2E_Connections(t *testing.T) {
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
	t.Cleanup(func() {
		_ = pg.Terminate(ctx)
	})

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
		t.Fatalf("parse mapped port %q: %v", mapped.Port(), err)
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

	// Create connection pointing at the Postgres testcontainer.
	var created struct {
		ID int64 `json:"id"`
	}
	if err := postJSON(base+"/api/connections", login.Token, map[string]any{
		"alias":    "tc-pg",
		"host":     pgHost,
		"port":     pgPort,
		"tag":      "local",
		"tls_mode": "disable",
		"username": "testuser",
		"password": "testpass",
		"database": "testdb",
	}, &created); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("created.id is zero")
	}

	// Test connection.
	var probe map[string]any
	if err := postJSON(fmt.Sprintf("%s/api/connections/%d/test", base, created.ID), login.Token, nil, &probe); err != nil {
		t.Fatalf("test connection: %v", err)
	}
	version, _ := probe["version"].(string)
	if !strings.Contains(version, "PostgreSQL") {
		t.Fatalf("probe version mismatch: %v", probe)
	}

	// Delete.
	statusDelete := bearerStatus(t, http.MethodDelete,
		fmt.Sprintf("%s/api/connections/%d", base, created.ID), login.Token)
	if statusDelete != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d", statusDelete)
	}

	// Get after delete -> 404.
	statusGet := bearerStatus(t, http.MethodGet,
		fmt.Sprintf("%s/api/connections/%d", base, created.ID), login.Token)
	if statusGet != http.StatusNotFound {
		t.Fatalf("get after delete: want 404, got %d", statusGet)
	}
}
