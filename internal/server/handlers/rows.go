package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/unkabas/dbil/internal/pg"
)

// RowsHandler — GET /api/connections/{id}/table/{schema}/{name}/rows
//
// Query params:
//
//	page       — zero-based page index (default 0)
//	page_size  — rows per page (default 50, max 200)
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

type tableSearchReq struct {
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Filters  []pg.TableFilter `json:"filters"`
}

func SearchRowsHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, schema, name, ok := parseTableRoute(w, r)
		if !ok {
			return
		}
		var req tableSearchReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		pool, err := d.Manager.OpenByID(r.Context(), id, r.Header.Get("X-Connection-Passphrase"))
		if err != nil {
			respondPoolOpenError(w, err)
			return
		}
		resp, err := pg.SearchRows(r.Context(), pool, schema, name, pg.SearchRowsRequest{
			Page: req.Page, PageSize: req.PageSize, Filters: req.Filters,
		})
		if err != nil {
			respondTableDataError(w, "search rows", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

type valuesReq struct {
	Filters []pg.TableFilter `json:"filters"`
}

func DistinctValuesHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, schema, name, ok := parseTableRoute(w, r)
		if !ok {
			return
		}
		column := chi.URLParam(r, "column")
		if column == "" {
			jsonError(w, http.StatusBadRequest, "column is required")
			return
		}
		var req valuesReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		pool, err := d.Manager.OpenByID(r.Context(), id, r.Header.Get("X-Connection-Passphrase"))
		if err != nil {
			respondPoolOpenError(w, err)
			return
		}
		resp, err := pg.DistinctValues(r.Context(), pool, schema, name, column, req.Filters)
		if err != nil {
			respondTableDataError(w, "fetch distinct values", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

type exportReq struct {
	Format  string           `json:"format"`
	Scope   string           `json:"scope"`
	Filters []pg.TableFilter `json:"filters"`
}

func ExportTableHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, schema, name, ok := parseTableRoute(w, r)
		if !ok {
			return
		}
		var req exportReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		format := strings.ToLower(strings.TrimSpace(req.Format))
		scope := strings.ToLower(strings.TrimSpace(req.Scope))
		if scope == "" {
			scope = "filtered"
		}
		if scope != "filtered" && scope != "all" {
			jsonError(w, http.StatusBadRequest, "invalid export scope")
			return
		}
		var contentType, ext string
		switch format {
		case "csv":
			contentType, ext = "text/csv; charset=utf-8", "csv"
		case "json":
			contentType, ext = "application/json; charset=utf-8", "json"
		case "xlsx":
			contentType, ext = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx"
		default:
			jsonError(w, http.StatusBadRequest, "invalid export format")
			return
		}
		pool, err := d.Manager.OpenByID(r.Context(), id, r.Header.Get("X-Connection-Passphrase"))
		if err != nil {
			respondPoolOpenError(w, err)
			return
		}
		data, err := pg.ExportRows(r.Context(), pool, schema, name, req.Filters, scope == "filtered")
		if err != nil {
			respondTableDataError(w, "export rows", err)
			return
		}

		var body []byte
		switch format {
		case "csv":
			body, err = renderCSV(data)
		case "json":
			body, err = renderJSON(data)
		case "xlsx":
			body, err = renderXLSX(data)
		}
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "render export failed: "+err.Error())
			return
		}
		filename := safeExportFilename(schema, name, ext)
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		if data.Truncated {
			w.Header().Set("X-DBIL-Export-Truncated", "true")
			w.Header().Set("X-DBIL-Export-Limit", strconv.Itoa(pg.ExportRowCap))
		}
		_, _ = w.Write(body)
	}
}

func parseTableRoute(w http.ResponseWriter, r *http.Request) (int64, string, string, bool) {
	id, ok := parseID(w, r)
	if !ok {
		return 0, "", "", false
	}
	schema := chi.URLParam(r, "schema")
	name := chi.URLParam(r, "name")
	if schema == "" || name == "" {
		jsonError(w, http.StatusBadRequest, "schema and name are required")
		return 0, "", "", false
	}
	return id, schema, name, true
}

func respondTableDataError(w http.ResponseWriter, op string, err error) {
	if errors.Is(err, pg.ErrInvalidIdentifier) {
		jsonError(w, http.StatusBadRequest, "invalid identifier")
		return
	}
	jsonError(w, http.StatusBadGateway, op+" failed: "+err.Error())
}

func safeExportFilename(schema, name, ext string) string {
	safe := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
				b.WriteRune(r)
			default:
				b.WriteByte('_')
			}
		}
		return b.String()
	}
	return safe(schema) + "." + safe(name) + "." + ext
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
