// Package pg reads PostgreSQL schema metadata and rows via pg_catalog and
// engine-neutral SELECTs. Used by the schema/data UI to replace the v0.5
// mock fixtures.
package pg

import (
	"context"
	"fmt"
	"sort"
	"time"

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
	Schema        string   `json:"schema"`
	Name          string   `json:"name"`
	Rows          int64    `json:"rows"`           // best value: exact if known, else reltuples (estimate); -1 = unknown
	RowsEstimated bool     `json:"rows_estimated"` // true when Rows comes from pg_class.reltuples (or its partitioned-table sum)
	RowsExact     *int64   `json:"rows_exact,omitempty"`
	SizeBytes     int64    `json:"size_bytes"`
	Kind          string   `json:"kind"`        // pg_class.relkind: 'r'=table, 'p'=partitioned
	HasIndex      bool     `json:"has_index"`   // pg_class.relhasindex
	LastAnalyze   string   `json:"last_analyze,omitempty"`
	Columns       []Column `json:"columns"`
	Indexes       []Index  `json:"indexes,omitempty"`
}

// Column is one attribute on a Table. fk is nil when there is no foreign
// key targeting this column.
type Column struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"` // format_type(atttypid, atttypmod)
	Nullable bool    `json:"nullable"`
	PK       bool    `json:"pk"`
	Unique   bool    `json:"unique"`
	FK       *FKRef  `json:"fk,omitempty"`
	Default  *string `json:"default,omitempty"`
	Comment  string  `json:"comment,omitempty"`
}

// FKRef points at the table.column a foreign key references. OnDelete/OnUpdate
// are pg_constraint action codes resolved to SQL action names.
type FKRef struct {
	Table    string `json:"table"`
	Column   string `json:"column"`
	OnDelete string `json:"on_delete,omitempty"`
	OnUpdate string `json:"on_update,omitempty"`
}

// Index describes one index on a table.
type Index struct {
	Name      string   `json:"name"`
	Columns   []string `json:"columns"`
	Unique    bool     `json:"unique"`
	Primary   bool     `json:"primary"`
	SizeBytes int64    `json:"size_bytes"`
	Method    string   `json:"method"` // btree, gin, gist, brin, hash, …
}

// Tunables for ListSchema's exact-rows pass.
const (
	exactCountSizeMax = int64(8 * 1024 * 1024) // 8 MiB heap+index
	exactCountRowMax  = int64(10_000)
	exactCountMax     = 30
	exactCountBudget  = 5 * time.Second
)

// listSchemaSQL returns one row per (table, column). For partitioned parents
// (relkind='p'), rows_estimate is the sum of children's reltuples via
// pg_inherits — the parent itself has reltuples=0.
const listSchemaSQL = `
WITH part_totals AS (
    SELECT pi.inhparent AS oid,
           SUM(GREATEST(pc.reltuples, 0))::bigint AS rows_total
    FROM pg_inherits pi
    JOIN pg_class pc ON pc.oid = pi.inhrelid
    GROUP BY pi.inhparent
)
SELECT
    n.nspname AS schema_name,
    c.relname AS table_name,
    CASE
        WHEN c.relkind = 'p' THEN COALESCE(pt.rows_total, 0)
        ELSE c.reltuples::bigint
    END AS rows_estimate,
    pg_total_relation_size(c.oid)::bigint AS size_bytes,
    c.relkind::text AS relkind,
    c.relhasindex AS has_index,
    COALESCE(GREATEST(
        pg_stat_get_last_analyze_time(c.oid),
        pg_stat_get_last_autoanalyze_time(c.oid)
    )::text, '') AS last_analyze,
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
    COALESCE((
        SELECT cf.relname || '.' || af.attname
        FROM pg_constraint con
        JOIN pg_class cf ON cf.oid = con.confrelid
        JOIN pg_attribute af ON af.attrelid = con.confrelid AND af.attnum = con.confkey[1]
        WHERE con.conrelid = c.oid
          AND con.contype = 'f'
          AND a.attnum = ANY(con.conkey)
        LIMIT 1
    ), '') AS fk_target,
    COALESCE((
        SELECT con.confdeltype::text
        FROM pg_constraint con
        WHERE con.conrelid = c.oid
          AND con.contype = 'f'
          AND a.attnum = ANY(con.conkey)
        LIMIT 1
    ), '') AS fk_on_delete,
    COALESCE((
        SELECT con.confupdtype::text
        FROM pg_constraint con
        WHERE con.conrelid = c.oid
          AND con.contype = 'f'
          AND a.attnum = ANY(con.conkey)
        LIMIT 1
    ), '') AS fk_on_update,
    COALESCE(pg_get_expr(ad.adbin, ad.adrelid), '') AS col_default,
    COALESCE(pg_catalog.col_description(c.oid, a.attnum), '') AS col_comment
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = c.oid
LEFT JOIN pg_attrdef ad ON ad.adrelid = c.oid AND ad.adnum = a.attnum
LEFT JOIN part_totals pt ON pt.oid = c.oid
WHERE c.relkind IN ('r','p')
  AND n.nspname NOT IN ('pg_catalog','information_schema','pg_toast')
  AND n.nspname NOT LIKE 'pg_temp_%'
  AND n.nspname NOT LIKE 'pg_toast_temp_%'
  AND a.attnum > 0
  AND NOT a.attisdropped
ORDER BY n.nspname, c.relname, a.attnum
`

// listIndexesSQL returns one row per index on a user-visible table. Index
// columns are aggregated as a Postgres text array (rendered "{a,b}").
const listIndexesSQL = `
SELECT
    n.nspname AS schema_name,
    c.relname AS table_name,
    ic.relname AS index_name,
    i.indisunique AS is_unique,
    i.indisprimary AS is_primary,
    pg_relation_size(ic.oid)::bigint AS size_bytes,
    am.amname::text AS method,
    (
        SELECT string_agg(att.attname, ',' ORDER BY ord.n)
        FROM unnest(i.indkey::int[]) WITH ORDINALITY AS ord(attnum, n)
        LEFT JOIN pg_attribute att ON att.attrelid = c.oid AND att.attnum = ord.attnum
        WHERE ord.attnum <> 0
    ) AS column_names
FROM pg_index i
JOIN pg_class c ON c.oid = i.indrelid
JOIN pg_class ic ON ic.oid = i.indexrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_am am ON am.oid = ic.relam
WHERE c.relkind IN ('r','p')
  AND n.nspname NOT IN ('pg_catalog','information_schema','pg_toast')
  AND n.nspname NOT LIKE 'pg_temp_%'
  AND n.nspname NOT LIKE 'pg_toast_temp_%'
ORDER BY n.nspname, c.relname, ic.relname
`

type tableKey struct{ schema, table string }

// ListSchema runs introspection against pool, folding the (table, column,
// index) rows into a SchemaDoc. For small tables it follows up with exact
// COUNT(*) queries (capped at exactCountMax with exactCountBudget).
func ListSchema(ctx context.Context, pool postgres.Pool) (*SchemaDoc, error) {
	res, err := pool.Execute(ctx, listSchemaSQL)
	if err != nil {
		return nil, fmt.Errorf("introspect: %w", err)
	}
	tables := map[tableKey]*Table{}
	order := []tableKey{}

	for _, row := range res.Rows {
		if len(row) < 11 {
			continue
		}
		schemaName := asString(row[0])
		tableName := asString(row[1])
		key := tableKey{schemaName, tableName}
		t, ok := tables[key]
		if !ok {
			t = &Table{
				Schema:        schemaName,
				Name:          tableName,
				Rows:          asInt64(row[2]),
				RowsEstimated: true,
				SizeBytes:     asInt64(row[3]),
				Kind:          asString(row[4]),
				HasIndex:      asBool(row[5]),
				LastAnalyze:   asString(row[6]),
			}
			tables[key] = t
			order = append(order, key)
		}
		col := Column{
			Name:     asString(row[8]),
			Type:     asString(row[9]),
			Nullable: asBool(row[10]),
		}
		if len(row) >= 14 {
			col.PK = asBool(row[11])
			col.Unique = asBool(row[12])
			if fk := asString(row[13]); fk != "" {
				if tbl, c := splitFK(fk); tbl != "" {
					ref := &FKRef{Table: tbl, Column: c}
					if len(row) >= 16 {
						ref.OnDelete = fkActionName(asString(row[14]))
						ref.OnUpdate = fkActionName(asString(row[15]))
					}
					col.FK = ref
				}
			}
		}
		if len(row) >= 17 {
			if d := asString(row[16]); d != "" {
				ds := d
				col.Default = &ds
			}
		}
		if len(row) >= 18 {
			col.Comment = asString(row[17])
		}
		t.Columns = append(t.Columns, col)
	}

	// Indexes: best-effort fold. A failure here should not break the page.
	if idxRes, err := pool.Execute(ctx, listIndexesSQL); err == nil {
		for _, row := range idxRes.Rows {
			if len(row) < 8 {
				continue
			}
			key := tableKey{asString(row[0]), asString(row[1])}
			t, ok := tables[key]
			if !ok {
				continue
			}
			idx := Index{
				Name:      asString(row[2]),
				Unique:    asBool(row[3]),
				Primary:   asBool(row[4]),
				SizeBytes: asInt64(row[5]),
				Method:    asString(row[6]),
				Columns:   splitCSV(asString(row[7])),
			}
			t.Indexes = append(t.Indexes, idx)
		}
	}

	// Exact rows for small tables: best-effort, time-budgeted.
	exactRows(ctx, pool, order, tables)

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

// exactRows issues COUNT(*) for tables that look small enough to be cheap,
// updating Table.RowsExact / Table.RowsEstimated in place. Bounded by both
// exactCountMax and exactCountBudget.
func exactRows(ctx context.Context, pool postgres.Pool, order []tableKey, tables map[tableKey]*Table) {
	deadline := time.Now().Add(exactCountBudget)
	done := 0
	for _, k := range order {
		if done >= exactCountMax || time.Now().After(deadline) {
			return
		}
		t := tables[k]
		// Skip partitioned parents: their COUNT(*) is the sum across children
		// which can be expensive; the partition CTE already covers them.
		if t.Kind == "p" {
			continue
		}
		eligible := (t.Rows >= 0 && t.Rows < exactCountRowMax) || t.Rows < 0
		if !eligible {
			continue
		}
		if t.SizeBytes > 0 && t.SizeBytes > exactCountSizeMax {
			continue
		}
		qSchema, qName, err := quoteTable(t.Schema, t.Name)
		if err != nil {
			continue
		}
		cctx, cancel := context.WithDeadline(ctx, deadline)
		res, err := pool.Execute(cctx, fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", qSchema, qName))
		cancel()
		done++
		if err != nil {
			continue
		}
		if len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
			continue
		}
		n := asInt64(res.Rows[0][0])
		t.RowsExact = &n
		t.Rows = n
		t.RowsEstimated = false
	}
}

func splitFK(s string) (string, string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return s[:i], s[i+1:]
		}
	}
	return "", ""
}

// fkActionName maps pg_constraint.confdeltype / confupdtype single-letter
// codes to their SQL action name. Empty input → empty output.
func fkActionName(code string) string {
	if code == "" {
		return ""
	}
	switch code[0] {
	case 'a':
		return "NO ACTION"
	case 'r':
		return "RESTRICT"
	case 'c':
		return "CASCADE"
	case 'n':
		return "SET NULL"
	case 'd':
		return "SET DEFAULT"
	}
	return ""
}

// splitCSV turns "a,b,c" into ["a", "b", "c"]. Empty input → nil.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
