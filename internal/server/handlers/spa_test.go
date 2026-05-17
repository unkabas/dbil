package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSPA_RootServesIndex(t *testing.T) {
	h := SPAHandler()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	// The committed placeholder index has a "<!doctype html>" preamble (or the
	// placeholder gitkeep when no build has run); either way the response is
	// non-empty and content-type is text/html or set sensibly.
	if w.Body.Len() == 0 {
		t.Fatal("empty response body")
	}
}

func TestSPA_ClientRouteFallsBackToIndex(t *testing.T) {
	h := SPAHandler()
	// React Router uses /data/public/users — chi never registered it. The
	// SPA handler should serve index.html so the browser keeps the route on
	// reload.
	r := httptest.NewRequest(http.MethodGet, "/data/public/users", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for client route, got %d", w.Code)
	}
}

func TestSPA_APIPathReturnsJSON404(t *testing.T) {
	h := SPAHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for /api path, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("want JSON content-type for /api path, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), `"error"`) {
		t.Fatalf("expected JSON error body, got %q", w.Body.String())
	}
}

func TestSPA_HealthzReturnsJSON404FromSPA(t *testing.T) {
	// /healthz is normally answered by Healthz, but if the SPA handler ever
	// receives it (e.g., chi misroute), it must NOT serve HTML.
	h := SPAHandler()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for /healthz fallback, got %d", w.Code)
	}
}

func TestSPA_SecurityHeaders(t *testing.T) {
	h := SPAHandler()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	cases := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"X-Frame-Options":        "DENY",
	}
	for k, want := range cases {
		if got := w.Header().Get(k); got != want {
			t.Errorf("%s: want %q, got %q", k, want, got)
		}
	}
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("CSP missing default-src 'self': %q", csp)
	}
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP missing frame-ancestors 'none': %q", csp)
	}
}

func TestSPA_AssetsCacheImmutable(t *testing.T) {
	h := SPAHandler()
	r := httptest.NewRequest(http.MethodGet, "/assets/index-abcdef.js", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	// We don't have a real bundle in the test environment, but the cache
	// header must already be set even if the file resolves to index.html via
	// the SPA fallback.
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control on /assets/ path must be immutable, got %q", cc)
	}
}

func TestSPA_ResolveSPAPath(t *testing.T) {
	cases := map[string]string{
		"/":                 "index.html",
		"/index.html":       "index.html",
		"/assets/main.js":   "assets/main.js",
		"/data/public/x":    "data/public/x",
		"/../etc/passwd":    "etc/passwd", // path.Clean removes ..
	}
	for input, want := range cases {
		if got := resolveSPAPath(input); got != want {
			t.Errorf("%q: want %q got %q", input, want, got)
		}
	}
}
