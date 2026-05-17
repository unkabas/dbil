package handlers

import (
	"encoding/json"
	"net/http"
)

// Healthz handles GET /healthz. No auth required; allow-listed in
// scripts/lint-auth so a container orchestrator can probe it.
func Healthz(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": version,
		})
	}
}
