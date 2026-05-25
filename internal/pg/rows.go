package pg

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// RowsResponse is the wire shape returned by the rows endpoint. When
// EstimatedTotalExact is true, EstimatedTotal is the actual row count for
// the (filtered) result — not a pg_class.reltuples estimate.
type RowsResponse struct {
	Columns             []ColumnRef `json:"columns"`
	Rows                [][]any     `json:"rows"`
	EstimatedTotal      int64       `json:"estimated_total"`
	EstimatedTotalExact bool        `json:"estimated_total_exact"`
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

// sqlLiteral escapes a string for use as a single-quoted SQL literal.
// We only embed values that have passed identRe, so the only character
// we need to escape is the single quote (defense-in-depth).
func sqlLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
