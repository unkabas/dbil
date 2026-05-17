package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unkabas/dbil/internal/auth"
	"github.com/unkabas/dbil/internal/crypto"
	"github.com/unkabas/dbil/internal/postgres"
	"github.com/unkabas/dbil/internal/store"
)

// fakePool is the smallest Pool implementation for handler tests. Returns
// a canned single-row "SELECT 1" Result from Execute.
type fakePool struct{}

func (fakePool) Ping(context.Context) error { return nil }
func (fakePool) Close()                     {}
func (fakePool) Execute(context.Context, string) (*postgres.Result, error) {
	return &postgres.Result{
		Columns:    []postgres.ColumnDef{{Name: "n", TypeName: "int4"}},
		Rows:       [][]any{{int64(1)}},
		CommandTag: "SELECT 1",
	}, nil
}

// fakeDriver is a Driver that does not talk to a real Postgres; it returns
// canned Probes / errors. Lives in the handler test file so it can't be
// confused with the production driver.
type fakeDriver struct {
	openErr  error
	probeErr error
	probe    postgres.Probe
}

func (d *fakeDriver) Open(context.Context, postgres.Conn) (postgres.Pool, error) {
	if d.openErr != nil {
		return nil, d.openErr
	}
	return fakePool{}, nil
}

func (d *fakeDriver) Probe(context.Context, postgres.Pool) (postgres.Probe, error) {
	if d.probeErr != nil {
		return postgres.Probe{}, d.probeErr
	}
	return d.probe, nil
}

func (d *fakeDriver) Close(postgres.Pool) {}

func setupConns(t *testing.T) (Deps, http.Handler, string) {
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
	users := store.NewUsersRepo(db)
	sessions := store.NewSessionsRepo(db)
	auditRepo := store.NewAuditRepo(db, mk)
	if _, err := users.Create(context.Background(), "admin@local", adminPassword, store.RoleAdmin, false); err != nil {
		t.Fatal(err)
	}
	ad := auth.Deps{Users: users, Sessions: sessions, Audit: auditRepo}
	conns := store.NewConnectionsRepo(db, mk)
	drv := &fakeDriver{probe: postgres.Probe{Version: "PostgreSQL 16.fake", HasPgStatStatements: true}}
	mgr := postgres.NewManager(drv, conns, auditRepo)
	d := Deps{Auth: ad, Conns: conns, Manager: mgr, Version: "test"}

	res, err := auth.Login(context.Background(), ad, "admin@local", adminPassword, "ua", "ip")
	if err != nil {
		t.Fatal(err)
	}
	return d, Mount(d), res.Token
}

func authed(method, path, token string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

func TestConnections_RequiresAuth(t *testing.T) {
	_, h, _ := setupConns(t)
	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/connections"},
		{http.MethodGet, "/api/connections/1"},
		{http.MethodPost, "/api/connections/1/test"},
		{http.MethodDelete, "/api/connections/1"},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(c.method, c.path, nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s without auth: want 401, got %d", c.method, c.path, w.Code)
		}
	}
}

func TestConnections_ListEmpty(t *testing.T) {
	_, h, token := setupConns(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, authed(http.MethodGet, "/api/connections", token, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	body := strings.TrimSpace(w.Body.String())
	if body != "[]" {
		t.Fatalf("want [], got %q", body)
	}
}

func TestConnections_CreateGetList(t *testing.T) {
	_, h, token := setupConns(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, authed(http.MethodPost, "/api/connections", token, createConnReq{
		Alias: "local", Host: "127.0.0.1", Port: 5432,
		Tag: store.TagLocal, TLSMode: store.TLSDisable,
		Username: "u", Password: "pw", Database: "d",
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var c connView
	if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	if c.Alias != "local" || c.Tag != store.TagLocal {
		t.Fatalf("create response wrong: %+v", c)
	}

	// GET
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, authed(http.MethodGet, fmt.Sprintf("/api/connections/%d", c.ID), token, nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("get: want 200, got %d", w2.Code)
	}

	// LIST
	w3 := httptest.NewRecorder()
	h.ServeHTTP(w3, authed(http.MethodGet, "/api/connections", token, nil))
	var list []connView
	_ = json.Unmarshal(w3.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list len %d, want 1", len(list))
	}
}

func TestConnections_CreateDuplicate409(t *testing.T) {
	_, h, token := setupConns(t)
	mk := func() *http.Request {
		return authed(http.MethodPost, "/api/connections", token, createConnReq{
			Alias: "x", Host: "h", Port: 5432, Tag: store.TagLocal,
			TLSMode: store.TLSDisable, Username: "u", Password: "p", Database: "d",
		})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, mk())
	if w.Code != http.StatusCreated {
		t.Fatalf("first create: want 201, got %d", w.Code)
	}
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, mk())
	if w2.Code != http.StatusConflict {
		t.Fatalf("duplicate: want 409, got %d", w2.Code)
	}
}

func TestConnections_CreateBadJSON_400(t *testing.T) {
	_, h, token := setupConns(t)
	r := httptest.NewRequest(http.MethodPost, "/api/connections", bytes.NewBufferString("{not json"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestConnections_CreateInvalidTag_400(t *testing.T) {
	_, h, token := setupConns(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, authed(http.MethodPost, "/api/connections", token, createConnReq{
		Alias: "x", Host: "h", Port: 5432, Tag: "nope",
		TLSMode: store.TLSDisable, Username: "u", Password: "p", Database: "d",
	}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestConnections_GetMissing404(t *testing.T) {
	_, h, token := setupConns(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, authed(http.MethodGet, "/api/connections/999", token, nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestConnections_DeleteFlow(t *testing.T) {
	_, h, token := setupConns(t)
	wCreate := httptest.NewRecorder()
	h.ServeHTTP(wCreate, authed(http.MethodPost, "/api/connections", token, createConnReq{
		Alias: "x", Host: "h", Port: 5432, Tag: store.TagLocal,
		TLSMode: store.TLSDisable, Username: "u", Password: "p", Database: "d",
	}))
	var c connView
	_ = json.Unmarshal(wCreate.Body.Bytes(), &c)

	wDelete := httptest.NewRecorder()
	h.ServeHTTP(wDelete, authed(http.MethodDelete, fmt.Sprintf("/api/connections/%d", c.ID), token, nil))
	if wDelete.Code != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d", wDelete.Code)
	}

	wGet := httptest.NewRecorder()
	h.ServeHTTP(wGet, authed(http.MethodGet, fmt.Sprintf("/api/connections/%d", c.ID), token, nil))
	if wGet.Code != http.StatusNotFound {
		t.Fatalf("get-after-delete: want 404, got %d", wGet.Code)
	}

	wDelete2 := httptest.NewRecorder()
	h.ServeHTTP(wDelete2, authed(http.MethodDelete, fmt.Sprintf("/api/connections/%d", c.ID), token, nil))
	if wDelete2.Code != http.StatusNotFound {
		t.Fatalf("delete-after-delete: want 404, got %d", wDelete2.Code)
	}
}

func TestConnections_TestEndpoint(t *testing.T) {
	_, h, token := setupConns(t)
	wCreate := httptest.NewRecorder()
	h.ServeHTTP(wCreate, authed(http.MethodPost, "/api/connections", token, createConnReq{
		Alias: "x", Host: "h", Port: 5432, Tag: store.TagLocal,
		TLSMode: store.TLSDisable, Username: "u", Password: "p", Database: "d",
	}))
	var c connView
	_ = json.Unmarshal(wCreate.Body.Bytes(), &c)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, authed(http.MethodPost, fmt.Sprintf("/api/connections/%d/test", c.ID), token, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("test: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var probe map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &probe)
	v, _ := probe["version"].(string)
	if !strings.Contains(v, "PostgreSQL") {
		t.Fatalf("probe version mismatch: %v", probe)
	}
}

func TestConnections_TestMissing404(t *testing.T) {
	_, h, token := setupConns(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, authed(http.MethodPost, "/api/connections/999/test", token, nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestConnections_TestPassphraseFlow(t *testing.T) {
	_, h, token := setupConns(t)
	wCreate := httptest.NewRecorder()
	h.ServeHTTP(wCreate, authed(http.MethodPost, "/api/connections", token, createConnReq{
		Alias: "prod", Host: "h", Port: 5432, Tag: store.TagProduction,
		TLSMode: store.TLSRequire, Username: "u", Password: "p", Database: "d",
		Passphrase: "the-passphrase",
	}))
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("create: %d (%s)", wCreate.Code, wCreate.Body.String())
	}
	var c connView
	_ = json.Unmarshal(wCreate.Body.Bytes(), &c)

	// No passphrase header -> 428
	wNone := httptest.NewRecorder()
	h.ServeHTTP(wNone, authed(http.MethodPost, fmt.Sprintf("/api/connections/%d/test", c.ID), token, nil))
	if wNone.Code != http.StatusPreconditionRequired {
		t.Fatalf("no passphrase: want 428, got %d", wNone.Code)
	}

	// Wrong passphrase -> 401
	rWrong := authed(http.MethodPost, fmt.Sprintf("/api/connections/%d/test", c.ID), token, nil)
	rWrong.Header.Set("X-Connection-Passphrase", "wrong")
	wWrong := httptest.NewRecorder()
	h.ServeHTTP(wWrong, rWrong)
	if wWrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong passphrase: want 401, got %d", wWrong.Code)
	}

	// Correct -> 200
	rOK := authed(http.MethodPost, fmt.Sprintf("/api/connections/%d/test", c.ID), token, nil)
	rOK.Header.Set("X-Connection-Passphrase", "the-passphrase")
	wOK := httptest.NewRecorder()
	h.ServeHTTP(wOK, rOK)
	if wOK.Code != http.StatusOK {
		t.Fatalf("correct passphrase: want 200, got %d (%s)", wOK.Code, wOK.Body.String())
	}
}
