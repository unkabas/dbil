package pg

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/unkabas/dbil/internal/postgres"
)

// ErrNoPrimaryKey is returned when a mutation targets a table that has no
// primary key. Without a PK we cannot address a single row safely, so inline
// editing is disabled for such tables (and views).
var ErrNoPrimaryKey = errors.New("table has no primary key")

// ErrUnknownColumn is returned when a change references a column that does not
// exist on the target table.
var ErrUnknownColumn = errors.New("unknown column")

// ErrInvalidChange is returned when a change is structurally invalid (missing
// primary-key values, empty SET/VALUES, unknown op, etc.).
var ErrInvalidChange = errors.New("invalid change")

// RowChange is one edit in a batch mutation. Op is "update", "delete", or
// "insert". PK identifies the target row for update/delete and must cover all
// primary-key columns. Set holds the new column values for update; Values the
// columns for insert. A nil map value means SQL NULL.
type RowChange struct {
	Op     string         `json:"op"`
	PK     map[string]any `json:"pk,omitempty"`
	Set    map[string]any `json:"set,omitempty"`
	Values map[string]any `json:"values,omitempty"`
}

// TableShape is the minimal introspection a mutation needs: the primary-key
// columns (in order) and the set of all column names on the table.
type TableShape struct {
	PrimaryKey []string
	Columns    map[string]bool
}

// pkColumnsSQL returns one row per column with a primary-key flag, ordered by
// attribute number, for a single relation. sqlLiteral-escaped names guard the
// catalog lookup; the relation itself is matched by name, not interpolated as
// an identifier.
const pkColumnsSQL = `
SELECT a.attname,
       EXISTS (
           SELECT 1 FROM pg_constraint con
           WHERE con.conrelid = c.oid
             AND con.contype = 'p'
             AND a.attnum = ANY(con.conkey)
       ) AS is_pk
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = c.oid
WHERE n.nspname = %s
  AND c.relname = %s
  AND c.relkind IN ('r','p')
  AND a.attnum > 0
  AND NOT a.attisdropped
ORDER BY a.attnum
`

// IntrospectTable reads the primary-key and column set for one table. Returns
// ErrNoPrimaryKey when the relation exists but has no primary key, and
// ErrInvalidIdentifier when schema/name fail identifier validation.
func IntrospectTable(ctx context.Context, pool postgres.Pool, schema, name string) (TableShape, error) {
	if _, _, err := quoteTable(schema, name); err != nil {
		return TableShape{}, err
	}
	sql := fmt.Sprintf(pkColumnsSQL, sqlLiteral(schema), sqlLiteral(name))
	res, err := pool.Execute(ctx, sql)
	if err != nil {
		return TableShape{}, fmt.Errorf("introspect table: %w", err)
	}
	shape := TableShape{Columns: map[string]bool{}}
	for _, row := range res.Rows {
		if len(row) < 2 {
			continue
		}
		col := asString(row[0])
		shape.Columns[col] = true
		if asBool(row[1]) {
			shape.PrimaryKey = append(shape.PrimaryKey, col)
		}
	}
	if len(shape.Columns) == 0 {
		return TableShape{}, fmt.Errorf("introspect table: %w: %s.%s", ErrInvalidIdentifier, schema, name)
	}
	if len(shape.PrimaryKey) == 0 {
		return TableShape{}, ErrNoPrimaryKey
	}
	return shape, nil
}

// BuildMutations validates every change against the table shape and renders
// the corresponding SQL statements in input order. Identifiers are quoted via
// quoteIdent; values are rendered as single-quoted literals (SQL "unknown"
// type) so PostgreSQL coerces them to each column's type on assignment, or as
// the NULL keyword for nil values.
func BuildMutations(schema, name string, shape TableShape, changes []RowChange) ([]string, error) {
	qSchema, qName, err := quoteTable(schema, name)
	if err != nil {
		return nil, err
	}
	table := qSchema + "." + qName
	stmts := make([]string, 0, len(changes))
	for i, ch := range changes {
		var (
			stmt string
			berr error
		)
		switch ch.Op {
		case "update":
			stmt, berr = buildUpdate(table, shape, ch)
		case "delete":
			stmt, berr = buildDelete(table, shape, ch)
		case "insert":
			stmt, berr = buildInsert(table, shape, ch)
		default:
			berr = fmt.Errorf("%w: change %d has unknown op %q", ErrInvalidChange, i, ch.Op)
		}
		if berr != nil {
			return nil, berr
		}
		stmts = append(stmts, stmt)
	}
	return stmts, nil
}

// buildWhereByPK renders "WHERE pk1 = lit AND pk2 = lit" covering every
// primary-key column. Every PK column must be present and non-null.
func buildWhereByPK(shape TableShape, pk map[string]any) (string, error) {
	if len(pk) == 0 {
		return "", fmt.Errorf("%w: missing primary-key values", ErrInvalidChange)
	}
	clauses := make([]string, 0, len(shape.PrimaryKey))
	for _, col := range shape.PrimaryKey {
		v, ok := pk[col]
		if !ok {
			return "", fmt.Errorf("%w: missing primary-key column %q", ErrInvalidChange, col)
		}
		if v == nil {
			return "", fmt.Errorf("%w: primary-key column %q cannot be null", ErrInvalidChange, col)
		}
		qcol, err := quoteIdent(col)
		if err != nil {
			return "", err
		}
		clauses = append(clauses, qcol+" = "+valueLiteral(v))
	}
	return "WHERE " + strings.Join(clauses, " AND "), nil
}

func buildUpdate(table string, shape TableShape, ch RowChange) (string, error) {
	if len(ch.Set) == 0 {
		return "", fmt.Errorf("%w: update with empty set", ErrInvalidChange)
	}
	assigns := make([]string, 0, len(ch.Set))
	for _, col := range sortedKeys(ch.Set) {
		if !shape.Columns[col] {
			return "", fmt.Errorf("%w: %q", ErrUnknownColumn, col)
		}
		qcol, err := quoteIdent(col)
		if err != nil {
			return "", err
		}
		assigns = append(assigns, qcol+" = "+valueLiteral(ch.Set[col]))
	}
	where, err := buildWhereByPK(shape, ch.PK)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("UPDATE %s SET %s %s", table, strings.Join(assigns, ", "), where), nil
}

func buildDelete(table string, shape TableShape, ch RowChange) (string, error) {
	where, err := buildWhereByPK(shape, ch.PK)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("DELETE FROM %s %s", table, where), nil
}

func buildInsert(table string, shape TableShape, ch RowChange) (string, error) {
	if len(ch.Values) == 0 {
		return "", fmt.Errorf("%w: insert with empty values", ErrInvalidChange)
	}
	cols := make([]string, 0, len(ch.Values))
	vals := make([]string, 0, len(ch.Values))
	for _, col := range sortedKeys(ch.Values) {
		if !shape.Columns[col] {
			return "", fmt.Errorf("%w: %q", ErrUnknownColumn, col)
		}
		qcol, err := quoteIdent(col)
		if err != nil {
			return "", err
		}
		cols = append(cols, qcol)
		vals = append(vals, valueLiteral(ch.Values[col]))
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ", "), strings.Join(vals, ", ")), nil
}

// valueLiteral renders a JSON value as a SQL literal. nil → NULL; everything
// else → a single-quoted literal (reusing filterValueText for the text form).
// Quoted literals are PostgreSQL "unknown" type and coerce to the column type
// on assignment, which keeps typed columns (int, bool, timestamp, …) working
// without the handler having to know each column's type.
func valueLiteral(v any) string {
	if v == nil {
		return "NULL"
	}
	return sqlLiteral(filterValueText(v))
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
