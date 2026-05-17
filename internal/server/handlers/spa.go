package handlers

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/unkabas/dbil/web"
)

// spaFS wraps an embedded FS so that requests for routes that don't map to a
// real file fall back to index.html. That keeps client-side routes (/query,
// /data/:schema/:name, etc.) reload-safe.
type spaFS struct{ fsys fs.FS }

func (s spaFS) Open(name string) (fs.File, error) {
	f, err := s.fsys.Open(name)
	if err == nil {
		return f, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	// Real 404 — serve index.html so React Router can decide what to render.
	return s.fsys.Open("index.html")
}

// SPAHandler returns an http.Handler that serves the embedded React bundle.
// Paths under /api/ and /healthz are NOT handled here — they get a JSON 404
// so SPA HTML doesn't pollute API responses.
//
// Response headers set on every successful response:
//   - X-Content-Type-Options: nosniff
//   - Referrer-Policy: no-referrer
//   - Content-Security-Policy with same-origin defaults + 'unsafe-inline'
//     for styles (CodeMirror injects style attributes)
func SPAHandler() http.Handler {
	sub, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":"frontend bundle missing"}`, http.StatusInternalServerError)
		})
	}
	fileServer := http.FileServer(http.FS(spaFS{fsys: sub}))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API and health-check paths must never be answered by the SPA.
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}

		setSecurityHeaders(w)
		// Cache hashed Vite assets aggressively; HTML always re-fetched.
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		// http.FileServer expects URL paths starting with "/" — pass through.
		fileServer.ServeHTTP(w, r)
	})
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set(
		"Content-Security-Policy",
		// 'unsafe-inline' for style-src is a known concession: CodeMirror
		// emits inline styles. Scripts are strict same-origin only.
		"default-src 'self'; "+
			"script-src 'self'; "+
			"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
			"font-src 'self' https://fonts.gstatic.com; "+
			"img-src 'self' data:; "+
			"connect-src 'self'; "+
			"frame-ancestors 'none'; "+
			"base-uri 'self'",
	)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// resolveSPAPath is exposed for tests; it returns the file the SPA would
// serve for a given URL path (so the test does not need a live server).
func resolveSPAPath(p string) string {
	clean := path.Clean("/" + strings.TrimPrefix(p, "/"))
	if clean == "/" {
		return "index.html"
	}
	return strings.TrimPrefix(clean, "/")
}
