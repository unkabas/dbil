package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/unkabas/dbil/internal/pg"
)

// RowsHandler — GET /api/connections/{id}/table/{schema}/{name}/rows
//
// Query params:
//   page       — zero-based page index (default 0)
//   page_size  — rows per page (default 50, max 200)
//
// Identifier validation runs in pg.FetchRows; invalid schema/table
// returns 400. Passphrase header is honored for protected connections.
func RowsHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		schema := chi.URLParam(r, "schema")
		name := chi.URLParam(r, "name")
		if schema == "" || name == "" {
			jsonError(w, http.StatusBadRequest, "schema and name are required")
			return
		}

		page := parsePositiveQuery(r, "page", 0)
		pageSize := parsePositiveQuery(r, "page_size", 50)

		passphrase := r.Header.Get("X-Connection-Passphrase")
		pool, err := d.Manager.OpenByID(r.Context(), id, passphrase)
		if err != nil {
			respondPoolOpenError(w, err)
			return
		}

		resp, err := pg.FetchRows(r.Context(), pool, schema, name, page, pageSize)
		if err != nil {
			if errors.Is(err, pg.ErrInvalidIdentifier) {
				jsonError(w, http.StatusBadRequest, "invalid identifier")
				return
			}
			jsonError(w, http.StatusBadGateway, "fetch rows failed: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func parsePositiveQuery(r *http.Request, key string, dflt int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return dflt
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return dflt
	}
	return n
}
