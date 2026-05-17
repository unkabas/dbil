package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/unkabas/dbil/internal/store"
)

// createConn is a small helper that POSTs a new connection through the
// handler stack and returns the new id. Keeps the per-test boilerplate light.
func createConn(t *testing.T, h http.Handler, token, alias, tag string, passphrase string) int64 {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, authed(http.MethodPost, "/api/connections", token, createConnReq{
		Alias: alias, Host: "h", Port: 5432, Tag: tag, TLSMode: store.TLSDisable,
		Username: "u", Password: "p", Database: "d", Passphrase: passphrase,
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create %s: want 201, got %d (%s)", alias, w.Code, w.Body.String())
	}
	var c connView
	if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func TestQuery_RequiresAuth(t *testing.T) {
	_, h, _ := setupConns(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/connections/1/query", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestQuery_BadJSON_400(t *testing.T) {
	_, h, token := setupConns(t)
	id := createConn(t, h, token, "local-pg", store.TagLocal, "")
	r := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/connections/%d/query", id),
		bytes.NewBufferString("{not json"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestQuery_EmptySQL_400(t *testing.T) {
	_, h, token := setupConns(t)
	id := createConn(t, h, token, "local-pg", store.TagLocal, "")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, authed(http.MethodPost, fmt.Sprintf("/api/connections/%d/query", id), token, queryReq{SQL: "   "}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestQuery_Missing404(t *testing.T) {
	_, h, token := setupConns(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, authed(http.MethodPost, "/api/connections/9999/query", token, queryReq{SQL: "SELECT 1"}))
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestQuery_LocalSelect200(t *testing.T) {
	_, h, token := setupConns(t)
	id := createConn(t, h, token, "local-pg", store.TagLocal, "")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, authed(http.MethodPost, fmt.Sprintf("/api/connections/%d/query", id), token, queryReq{SQL: "SELECT 1"}))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["command_tag"] != "SELECT 1" {
		t.Fatalf("command_tag: %v", resp["command_tag"])
	}
}

func TestQuery_ProductionDDLBlocked_403(t *testing.T) {
	_, h, token := setupConns(t)
	id := createConn(t, h, token, "prod-pg", store.TagProduction, "")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, authed(http.MethodPost, fmt.Sprintf("/api/connections/%d/query", id), token, queryReq{SQL: "DROP TABLE x"}))
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestQuery_StagingDMLConfirmation_428_then_200(t *testing.T) {
	_, h, token := setupConns(t)
	id := createConn(t, h, token, "staging-pg", store.TagStaging, "")

	// No X-Confirm header -> 428
	w := httptest.NewRecorder()
	h.ServeHTTP(w, authed(http.MethodPost, fmt.Sprintf("/api/connections/%d/query", id), token, queryReq{SQL: "INSERT INTO t VALUES (1)"}))
	if w.Code != http.StatusPreconditionRequired {
		t.Fatalf("want 428, got %d", w.Code)
	}

	// With X-Confirm: yes -> 200 (the fake driver always succeeds)
	r := authed(http.MethodPost, fmt.Sprintf("/api/connections/%d/query", id), token, queryReq{SQL: "INSERT INTO t VALUES (1)"})
	r.Header.Set("X-Confirm", "yes")
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, r)
	if w2.Code != http.StatusOK {
		t.Fatalf("want 200 with confirm, got %d (%s)", w2.Code, w2.Body.String())
	}
}

func TestQuery_ProductionDangerousBlocked_403(t *testing.T) {
	_, h, token := setupConns(t)
	id := createConn(t, h, token, "prod-pg", store.TagProduction, "")
	r := authed(http.MethodPost, fmt.Sprintf("/api/connections/%d/query", id), token, queryReq{SQL: "UPDATE users SET x=1"})
	r.Header.Set("X-Confirm", "yes")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}

func TestQuery_PassphraseRequired_428(t *testing.T) {
	_, h, token := setupConns(t)
	id := createConn(t, h, token, "prod-pg", store.TagProduction, "the-passphrase")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, authed(http.MethodPost, fmt.Sprintf("/api/connections/%d/query", id), token, queryReq{SQL: "SELECT 1"}))
	if w.Code != http.StatusPreconditionRequired {
		t.Fatalf("want 428, got %d", w.Code)
	}
}
