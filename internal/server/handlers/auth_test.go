package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/unkabas/dbil/internal/auth"
	"github.com/unkabas/dbil/internal/crypto"
	"github.com/unkabas/dbil/internal/postgres"
	"github.com/unkabas/dbil/internal/store"
)

const adminPassword = "a-decent-password-1234"

func setup(t *testing.T) (auth.Deps, http.Handler, store.User) {
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
	u, err := users.Create(context.Background(), "admin@local", adminPassword, store.RoleAdmin, false)
	if err != nil {
		t.Fatal(err)
	}
	ad := auth.Deps{Users: users, Sessions: sessions, Audit: auditRepo}
	conns := store.NewConnectionsRepo(db, mk)
	mgr := postgres.NewManager(postgres.NewPGX(), conns, auditRepo)
	return ad, Mount(Deps{Auth: ad, Conns: conns, Manager: mgr, Version: "test"}), u
}

func doJSON(t *testing.T, method, path string, body any, headers map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestHealthz(t *testing.T) {
	_, h, _ := setup(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status: %v", body)
	}
}

func TestLogin_SuccessReturnsToken(t *testing.T) {
	_, h, _ := setup(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, doJSON(t, http.MethodPost, "/api/auth/login",
		map[string]string{"email": "admin@local", "password": adminPassword}, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	tok, _ := resp["token"].(string)
	if tok == "" {
		t.Fatalf("missing token: %v", resp)
	}
}

func TestLogin_WrongPassword_401(t *testing.T) {
	_, h, _ := setup(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, doJSON(t, http.MethodPost, "/api/auth/login",
		map[string]string{"email": "admin@local", "password": "wrong"}, nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestLogin_UnknownUser_401(t *testing.T) {
	_, h, _ := setup(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, doJSON(t, http.MethodPost, "/api/auth/login",
		map[string]string{"email": "nobody@local", "password": "anything"}, nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestLogin_BadJSON_400(t *testing.T) {
	_, h, _ := setup(t)
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		bytes.NewBufferString("{not json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestLogin_MissingFields_400(t *testing.T) {
	_, h, _ := setup(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, doJSON(t, http.MethodPost, "/api/auth/login",
		map[string]string{"email": "", "password": ""}, nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestMe_RequiresAuth(t *testing.T) {
	_, h, _ := setup(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/me", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestMe_WithValidToken(t *testing.T) {
	d, h, u := setup(t)
	res, err := auth.Login(context.Background(), d, u.Email, adminPassword, "ua", "ip")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r.Header.Set("Authorization", "Bearer "+res.Token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["email"] != "admin@local" {
		t.Fatalf("unexpected user: %v", resp)
	}
	if resp["role"] != "admin" {
		t.Fatalf("unexpected role: %v", resp)
	}
}

func TestLogout_RevokesSession(t *testing.T) {
	d, h, u := setup(t)
	res, err := auth.Login(context.Background(), d, u.Email, adminPassword, "ua", "ip")
	if err != nil {
		t.Fatal(err)
	}
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+res.Token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, logoutReq)
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d (%s)", w.Code, w.Body.String())
	}

	// A subsequent /api/me with the revoked token must 401.
	meReq := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+res.Token)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, meReq)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 after logout, got %d", w2.Code)
	}
}
