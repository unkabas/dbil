package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAuth_NoHeader_Returns401(t *testing.T) {
	d, _ := setupAuth(t)
	h := RequireAuth(d)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler must not run without auth")
	}))
	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestRequireAuth_BadHeader_Returns401(t *testing.T) {
	d, _ := setupAuth(t)
	h := RequireAuth(d)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler must not run with bad header")
	}))
	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r.Header.Set("Authorization", "not-bearer xyz")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestRequireAuth_InvalidToken_Returns401(t *testing.T) {
	d, _ := setupAuth(t)
	h := RequireAuth(d)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler must not run with invalid token")
	}))
	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r.Header.Set("Authorization", "Bearer not-a-real-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestRequireAuth_ValidTokenInjectsUser(t *testing.T) {
	d, u := setupAuth(t)
	ctx := context.Background()
	res, err := Login(ctx, d, u.Email, testPassword, "ua", "ip")
	if err != nil {
		t.Fatal(err)
	}

	var seen bool
	h := RequireAuth(d)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = true
		got, ok := UserFromContext(r.Context())
		if !ok {
			t.Error("user not in context")
		}
		if got.ID != u.ID {
			t.Errorf("user id mismatch: want %d got %d", u.ID, got.ID)
		}
		tok, ok := TokenFromContext(r.Context())
		if !ok {
			t.Error("token not in context")
		}
		if tok != res.Token {
			t.Error("token mismatch in context")
		}
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r.Header.Set("Authorization", "Bearer "+res.Token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if !seen {
		t.Fatal("inner handler did not run")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestRemoteIP(t *testing.T) {
	cases := map[string]struct {
		fwd, remote, want string
	}{
		"xff single":    {"1.2.3.4", "10.0.0.1:5555", "1.2.3.4"},
		"xff multiple":  {"1.2.3.4, 10.0.0.1", "", "1.2.3.4"},
		"remote only":   {"", "10.0.0.1:5555", "10.0.0.1"},
		"remote no port":{"", "10.0.0.1", "10.0.0.1"},
		"empty":         {"", "", ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if c.fwd != "" {
				r.Header.Set("X-Forwarded-For", c.fwd)
			}
			r.RemoteAddr = c.remote
			if got := RemoteIP(r); got != c.want {
				t.Fatalf("want %q, got %q", c.want, got)
			}
		})
	}
}
