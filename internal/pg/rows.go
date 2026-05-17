package pg

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/unkabas/dbil/internal/postgres"
)

// RowsResponse is the wire shape returned by the rows endpoint.
type RowsResponse struct {
	Columns        []ColumnRef `json:"columns"`
	Rows           [][]any     `json:"rows"`
	EstimatedTotal int64       `json:"estimated_total"`
}

// ColumnRef carries minimal column info for the data grid header.
type ColumnRef struct {
	Name     string `json:"name"`
	TypeName string `json:"type_name"`
}

// ErrInvalidIdentifier is returned when schema or table name contains
// characters outside the conservative `[A-Za-z_][A-Za-z0-9_]*` set.
var ErrInvalidIdentifier = errors.New("invalid identifier")

// MaxPageSize caps the rows returned per page. Keeps the JSON payload and
// the React grid responsive even on wide tables.
const MaxPageSize = 200

var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z_0-9]*$`)

// quoteIdent returns the SQL-quoted form of an identifier, validating that
// it matches identRe. Double quotes inside the input are rejected by the
// regex above; we don't need to escape them.
func quoteIdent(s string) (string, error) {
	if !identRe.MatchString(s) {
		return "", fmt.Errorf("%w: %q", ErrInvalidIdentifier, s)
	}
	return `"` + s + `"`, nil
}

// FetchRows returns one page of rows from <schema>.<name>. pageSize is
// clamped to MaxPageSize; page is zero-based. EstimatedTotal is read from
// pg_class.reltuples (returns -1 if the relation is missing).
func FetchRows(ctx context.Context, pool postgres.Pool, schema, name string, page, pageSize int) (*RowsResponse, error) {
	qSchema, err := quoteIdent(schema)
	if err != nil {
		return nil, err
	}
	qName, err := quoteIdent(name)
	if err != nil {
		return nil, err
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	if page < 0 {
		page = 0
	}
	offset := page * pageSize

	// Estimated total via pg_class. Schema + table are bound via parameters
	// would be ideal — but postgres.Pool.Execute is text-only today, so we
	// embed validated literals using PostgreSQL's standard quoting.
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

	cols := make([]ColumnRef, len(res.Columns))
	for i, c := range res.Columns {
		cols[i] = ColumnRef{Name: c.Name, TypeName: c.TypeName}
	}
	rows := res.Rows
	if rows == nil {
		rows = [][]any{}
	}
	return &RowsResponse{Columns: cols, Rows: rows, EstimatedTotal: estimated}, nil
}

// sqlLiteral escapes a string for use as a single-quoted SQL literal.
// We only embed values that have passed identRe, so the only character
// we need to escape is the single quote (defense-in-depth).
func sqlLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
