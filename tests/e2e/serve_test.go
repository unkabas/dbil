package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestE2E_ServeFullLoginFlow exercises the whole production flow:
//   `dbil init`   -> creates admin + writes initial-credentials.txt
//   `dbil serve`  -> spawned as a background process on a free port
//   POST /api/auth/login           -> 200, token
//   GET  /api/me  with token       -> 200, admin user
//   POST /api/auth/logout w/ token -> 204
//   GET  /api/me  with same token  -> 401
func TestE2E_ServeFullLoginFlow(t *testing.T) {
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
		_, _ = cmd.Process.Wait()
	})

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitHealthz(base, 5*time.Second); err != nil {
		t.Fatalf("server never became healthy: %v\nstderr: %s", err, stderr.String())
	}

	// Login.
	var loginResp struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
	}
	if err := postJSON(base+"/api/auth/login", "", map[string]string{
		"email": "admin@local", "password": password,
	}, &loginResp); err != nil {
		t.Fatalf("login: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatal("login returned empty token")
	}

	// /api/me with valid token.
	var meResp struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := getJSON(base+"/api/me", loginResp.Token, &meResp); err != nil {
		t.Fatalf("me: %v", err)
	}
	if meResp.Email != "admin@local" {
		t.Fatalf("want admin@local, got %q", meResp.Email)
	}
	if meResp.Role != "admin" {
		t.Fatalf("want role admin, got %q", meResp.Role)
	}

	// Logout.
	if status := bearerStatus(t, http.MethodPost, base+"/api/auth/logout", loginResp.Token); status != http.StatusNoContent {
		t.Fatalf("logout status: want 204, got %d", status)
	}

	// /api/me with the now-revoked token must 401.
	if status := bearerStatus(t, http.MethodGet, base+"/api/me", loginResp.Token); status != http.StatusUnauthorized {
		t.Fatalf("post-logout /api/me: want 401, got %d", status)
	}
}

// Helpers — kept in serve_test.go so they live with the e2e they're built for.

func readInitialPassword(t *testing.T, dataDir string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dataDir, "initial-credentials.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "password=") {
			return strings.TrimPrefix(line, "password=")
		}
	}
	t.Fatal("password not found in initial-credentials.txt")
	return ""
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func waitHealthz(base string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("healthz never returned 200 within %s", timeout)
}

func postJSON(url, token string, body any, out any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}
	req, _ := http.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s: %d %s", url, resp.StatusCode, b)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func getJSON(url, token string, out any) error {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s: %d %s", url, resp.StatusCode, b)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func bearerStatus(t *testing.T, method, url, token string) int {
	t.Helper()
	req, _ := http.NewRequest(method, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}
