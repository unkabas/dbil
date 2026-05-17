// Package pg reads PostgreSQL schema metadata and rows via pg_catalog and
// engine-neutral SELECTs. Used by the schema/data UI to replace the v0.5
// mock fixtures.
package pg

import (
	"context"
	"fmt"
	"sort"

	"github.com/unkabas/dbil/internal/postgres"
)

// SchemaDoc is the wire shape returned by /api/connections/{id}/schema.
type SchemaDoc struct {
	Schemas []Schema `json:"schemas"`
}

// Schema groups tables under a single PostgreSQL namespace.
type Schema struct {
	Name   string  `json:"name"`
	Tables []Table `json:"tables"`
}

// Table describes one relation (regular or partitioned) with column metadata.
type Table struct {
	Schema    string   `json:"schema"`
	Name      string   `json:"name"`
	Rows      int64    `json:"rows"`       // pg_class.reltuples (estimate)
	SizeBytes int64    `json:"size_bytes"` // pg_total_relation_size
	Columns   []Column `json:"columns"`
}

// Column is one attribute on a Table. fk is nil when there is no foreign
// key targeting this column.
type Column struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`     // format_type(atttypid, atttypmod)
	Nullable bool    `json:"nullable"`
	PK       bool    `json:"pk"`
	Unique   bool    `json:"unique"`
	FK       *FKRef  `json:"fk,omitempty"`
}

// FKRef points at the table.column a foreign key references. Schema name
// is dropped to keep the payload small; future versions may add it.
type FKRef struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

// listSchemaSQL joins pg_namespace, pg_class, pg_attribute, pg_constraint to
// produce one row per (table, column). User-visible schemas only — pg_*
// internals and information_schema are filtered out. Partitioned tables
// (relkind='p') are included alongside ordinary tables (relkind='r').
const listSchemaSQL = `
SELECT
    n.nspname AS schema_name,
    c.relname AS table_name,
    c.reltuples::bigint AS rows_estimate,
    pg_total_relation_size(c.oid)::bigint AS size_bytes,
    a.attnum::int AS attnum,
    a.attname AS column_name,
    format_type(a.atttypid, a.atttypmod) AS data_type,
    (NOT a.attnotnull) AS nullable,
    EXISTS (
        SELECT 1 FROM pg_constraint con
        WHERE con.conrelid = c.oid
          AND con.contype = 'p'
          AND a.attnum = ANY(con.conkey)
    ) AS is_pk,
    EXISTS (
        SELECT 1 FROM pg_constraint con
        WHERE con.conrelid = c.oid
          AND con.contype = 'u'
          AND a.attnum = ANY(con.conkey)
    ) AS is_unique,
    (
        SELECT cf.relname || '.' || af.attname
        FROM pg_constraint con
        JOIN pg_class cf ON cf.oid = con.confrelid
        JOIN pg_attribute af ON af.attrelid = con.confrelid AND af.attnum = con.confkey[1]
        WHERE con.conrelid = c.oid
          AND con.contype = 'f'
          AND a.attnum = ANY(con.conkey)
        LIMIT 1
    ) AS fk_target
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = c.oid
WHERE c.relkind IN ('r','p')
  AND n.nspname NOT IN ('pg_catalog','information_schema','pg_toast')
  AND n.nspname NOT LIKE 'pg_temp_%'
  AND n.nspname NOT LIKE 'pg_toast_temp_%'
  AND a.attnum > 0
  AND NOT a.attisdropped
ORDER BY n.nspname, c.relname, a.attnum
`

// ListSchema runs a single SQL against pool and folds the (table, column)
// rows into a SchemaDoc. Returns an empty doc if the database has no
// user-visible tables.
func ListSchema(ctx context.Context, pool postgres.Pool) (*SchemaDoc, error) {
	res, err := pool.Execute(ctx, listSchemaSQL)
	if err != nil {
		return nil, fmt.Errorf("introspect: %w", err)
	}
	// Group: schema_name → table_name → Table
	type tkey struct{ schema, table string }
	tables := map[tkey]*Table{}
	order := []tkey{}

	for _, row := range res.Rows {
		if len(row) < 11 {
			continue
		}
		schemaName := asString(row[0])
		tableName := asString(row[1])
		key := tkey{schemaName, tableName}
		t, ok := tables[key]
		if !ok {
			t = &Table{
				Schema:    schemaName,
				Name:      tableName,
				Rows:      asInt64(row[2]),
				SizeBytes: asInt64(row[3]),
			}
			tables[key] = t
			order = append(order, key)
		}
		col := Column{
			Name:     asString(row[5]),
			Type:     asString(row[6]),
			Nullable: asBool(row[7]),
			PK:       asBool(row[8]),
			Unique:   asBool(row[9]),
		}
		if fk := asString(row[10]); fk != "" {
			if tbl, col2 := splitFK(fk); tbl != "" {
				col.FK = &FKRef{Table: tbl, Column: col2}
			}
		}
		t.Columns = append(t.Columns, col)
	}

	// Group tables by schema, preserving SQL order.
	bySchema := map[string]*Schema{}
	var schemaOrder []string
	for _, k := range order {
		s, ok := bySchema[k.schema]
		if !ok {
			s = &Schema{Name: k.schema}
			bySchema[k.schema] = s
			schemaOrder = append(schemaOrder, k.schema)
		}
		s.Tables = append(s.Tables, *tables[k])
	}
	sort.Strings(schemaOrder)
	doc := &SchemaDoc{}
	for _, name := range schemaOrder {
		doc.Schemas = append(doc.Schemas, *bySchema[name])
	}
	if doc.Schemas == nil {
		doc.Schemas = []Schema{}
	}
	return doc, nil
}

func splitFK(s string) (string, string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return s[:i], s[i+1:]
		}
	}
	return "", ""
}
