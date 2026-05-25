package pg

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/unkabas/dbil/internal/postgres"
)

const (
	MaxDistinctValues = 200
	ExportRowCap      = 100000
)

type TableFilter struct {
	Column string `json:"column"`
	Values []any  `json:"values"`
}

type SearchRowsRequest struct {
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Filters  []TableFilter `json:"filters"`
}

type SearchRowsResponse struct {
	Columns       []ColumnRef `json:"columns"`
	Rows          [][]any     `json:"rows"`
	FilteredTotal int64       `json:"filtered_total"`
	Truncated     bool        `json:"truncated"`
}

type DistinctValue struct {
	Value any   `json:"value"`
	Count int64 `json:"count"`
}

type DistinctValuesResponse struct {
	Values    []DistinctValue `json:"values"`
	Truncated bool            `json:"truncated"`
}

type ExportRowsResult struct {
	Columns   []ColumnRef
	Rows      [][]any
	Truncated bool
}

func SearchRows(ctx context.Context, pool postgres.Pool, schema, name string, req SearchRowsRequest) (*SearchRowsResponse, error) {
	qSchema, qName, err := quoteTable(schema, name)
	if err != nil {
		return nil, err
	}
	where, err := buildWhere(req.Filters, "")
	if err != nil {
		return nil, err
	}
	page, pageSize := normalizePage(req.Page, req.PageSize)
	offset := page * pageSize

	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s%s", qSchema, qName, where)
	countRes, err := pool.Execute(ctx, countSQL)
	if err != nil {
		return nil, fmt.Errorf("count filtered rows: %w", err)
	}
	filteredTotal := int64(0)
	if len(countRes.Rows) > 0 && len(countRes.Rows[0]) > 0 {
		filteredTotal = asInt64(countRes.Rows[0][0])
	}

	rowsSQL := fmt.Sprintf("SELECT * FROM %s.%s%s LIMIT %d OFFSET %d", qSchema, qName, where, pageSize, offset)
	res, err := pool.Execute(ctx, rowsSQL)
	if err != nil {
		return nil, fmt.Errorf("fetch filtered rows: %w", err)
	}
	cols := resultColumns(res)
	rows := res.Rows
	if rows == nil {
		rows = [][]any{}
	}
	return &SearchRowsResponse{Columns: cols, Rows: rows, FilteredTotal: filteredTotal, Truncated: res.Truncated}, nil
}

func DistinctValues(ctx context.Context, pool postgres.Pool, schema, name, column string, filters []TableFilter) (*DistinctValuesResponse, error) {
	qSchema, qName, err := quoteTable(schema, name)
	if err != nil {
		return nil, err
	}
	qColumn, err := quoteIdent(column)
	if err != nil {
		return nil, err
	}
	where, err := buildWhere(filters, column)
	if err != nil {
		return nil, err
	}
	sql := fmt.Sprintf(
		"SELECT %s IS NULL AS is_null, %s::text AS value, COUNT(*) AS count FROM %s.%s%s "+
			"GROUP BY 1, 2 ORDER BY 3 DESC, 2 ASC LIMIT %d",
		qColumn, qColumn, qSchema, qName, where, MaxDistinctValues+1,
	)
	res, err := pool.Execute(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("fetch distinct values: %w", err)
	}
	values := make([]DistinctValue, 0, len(res.Rows))
	truncated := false
	for i, row := range res.Rows {
		if i >= MaxDistinctValues {
			truncated = true
			break
		}
		if len(row) < 3 {
			continue
		}
		value := any(asString(row[1]))
		if asBool(row[0]) {
			value = nil
		}
		values = append(values, DistinctValue{Value: value, Count: asInt64(row[2])})
	}
	return &DistinctValuesResponse{Values: values, Truncated: truncated || res.Truncated}, nil
}

func ExportRows(ctx context.Context, pool postgres.Pool, schema, name string, filters []TableFilter, includeFilters bool) (*ExportRowsResult, error) {
	qSchema, qName, err := quoteTable(schema, name)
	if err != nil {
		return nil, err
	}
	where := ""
	if includeFilters {
		where, err = buildWhere(filters, "")
		if err != nil {
			return nil, err
		}
	}
	sql := fmt.Sprintf("SELECT * FROM %s.%s%s LIMIT %d", qSchema, qName, where, ExportRowCap+1)
	res, err := postgres.ExecuteWithLimit(ctx, pool, sql, ExportRowCap+1)
	if err != nil {
		return nil, fmt.Errorf("export rows: %w", err)
	}
	rows := res.Rows
	truncated := res.Truncated
	if len(rows) > ExportRowCap {
		rows = rows[:ExportRowCap]
		truncated = true
	}
	if rows == nil {
		rows = [][]any{}
	}
	return &ExportRowsResult{Columns: resultColumns(res), Rows: rows, Truncated: truncated}, nil
}

// FetchRows returns one page of rows from <schema>.<name>. pageSize is
// clamped to MaxPageSize; page is zero-based. EstimatedTotal is read from
// pg_class.reltuples (returns -1 if the relation is missing).
func FetchRows(ctx context.Context, pool postgres.Pool, schema, name string, page, pageSize int) (*RowsResponse, error) {
	qSchema, qName, err := quoteTable(schema, name)
	if err != nil {
		return nil, err
	}
	page, pageSize = normalizePage(page, pageSize)
	offset := page * pageSize

	estSQL := fmt.Sprintf(
		"SELECT c.reltuples::bigint FROM pg_class c "+
			"JOIN pg_namespace n ON n.oid = c.relnamespace "+
			"WHERE n.nspname = %s AND c.relname = %s",
		sqlLiteral(schema), sqlLiteral(name),
	)
	estRes, err := pool.Execute(ctx, estSQL)
	estimated := int64(-1)
	if err == nil && len(estRes.Rows) > 0 && len(estRes.Rows[0]) > 0 {
		estimated = asInt64(estRes.Rows[0][0])
	}

	rowsSQL := fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d OFFSET %d", qSchema, qName, pageSize, offset)
	res, err := pool.Execute(ctx, rowsSQL)
	if err != nil {
		return nil, fmt.Errorf("fetch rows: %w", err)
	}

	rows := res.Rows
	if rows == nil {
		rows = [][]any{}
	}
	// A short page means we read past the end of the relation. The true
	// total is offset+len(rows), which is more accurate than reltuples (the
	// estimate is often stale, sometimes -1 for never-analyzed tables).
	// The lone exception is an empty page after a non-zero offset: we know
	// total < offset but not the exact value — fall back to the estimate.
	exact := false
	if len(rows) < pageSize && (len(rows) > 0 || offset == 0) {
		estimated = int64(offset + len(rows))
		exact = true
	}
	return &RowsResponse{Columns: resultColumns(res), Rows: rows, EstimatedTotal: estimated, EstimatedTotalExact: exact}, nil
}

func quoteTable(schema, name string) (string, string, error) {
	qSchema, err := quoteIdent(schema)
	if err != nil {
		return "", "", err
	}
	qName, err := quoteIdent(name)
	if err != nil {
		return "", "", err
	}
	return qSchema, qName, nil
}

func normalizePage(page, pageSize int) (int, int) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	if page < 0 {
		page = 0
	}
	return page, pageSize
}

func buildWhere(filters []TableFilter, excludeColumn string) (string, error) {
	clauses := make([]string, 0, len(filters))
	for _, f := range filters {
		if f.Column == "" || f.Column == excludeColumn || len(f.Values) == 0 {
			continue
		}
		qColumn, err := quoteIdent(f.Column)
		if err != nil {
			return "", err
		}
		hasNull := false
		lits := make([]string, 0, len(f.Values))
		for _, v := range f.Values {
			if v == nil {
				hasNull = true
				continue
			}
			lits = append(lits, sqlLiteral(filterValueText(v)))
		}
		parts := make([]string, 0, 2)
		if len(lits) > 0 {
			parts = append(parts, fmt.Sprintf("%s::text IN (%s)", qColumn, strings.Join(lits, ", ")))
		}
		if hasNull {
			parts = append(parts, fmt.Sprintf("%s IS NULL", qColumn))
		}
		if len(parts) > 0 {
			clauses = append(clauses, "("+strings.Join(parts, " OR ")+")")
		}
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), nil
}

func filterValueText(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

func resultColumns(res *postgres.Result) []ColumnRef {
	cols := make([]ColumnRef, len(res.Columns))
	for i, c := range res.Columns {
		cols[i] = ColumnRef{Name: c.Name, TypeName: c.TypeName}
	}
	return cols
}
