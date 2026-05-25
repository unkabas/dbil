package pg

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/unkabas/dbil/internal/postgres"
)

// fakePool returns canned Results matched by SQL prefix.
type fakePool struct {
	executed []string
	results  map[string]*postgres.Result
	err      error
}

func (p *fakePool) Ping(_ context.Context) error { return nil }
func (p *fakePool) Close()                       {}
func (p *fakePool) Execute(_ context.Context, sql string) (*postgres.Result, error) {
	p.executed = append(p.executed, sql)
	if p.err != nil {
		return nil, p.err
	}
	for prefix, r := range p.results {
		if strings.HasPrefix(sql, prefix) {
			return r, nil
		}
	}
	return &postgres.Result{}, nil
}

// row helper: keeps the introspection-row layout in one place so tests stay
// readable when the SQL gains columns.
func mkRow(schema, table string, rows int64, size int64, kind string, attnum int, col, typ string, nullable, pk, uniq bool, fk, fkDel, fkUpd, def, comment string) []any {
	return []any{
		schema, table,
		rows, size,
		kind,  // relkind
		true,  // has_index (not asserted in most tests)
		"",    // last_analyze
		attnum,
		col, typ,
		nullable, pk, uniq,
		fk, fkDel, fkUpd,
		def, comment,
	}
}

func TestListSchema_GroupsTablesAndColumns(t *testing.T) {
	// Two tables under "public" with PK + FK + nullable columns.
	pool := &fakePool{results: map[string]*postgres.Result{
		"\nWITH": {
			Rows: [][]any{
				// users (id pk, email unique, age nullable). 1500 rows is small
				// enough that exactRows will follow up with a COUNT(*).
				mkRow("public", "users", 1500, 8192*4, "r", 1, "id", "bigint", false, true, false, "", "", "", "", ""),
				mkRow("public", "users", 1500, 8192*4, "r", 2, "email", "text", false, false, true, "", "", "", "", ""),
				mkRow("public", "users", 1500, 8192*4, "r", 3, "age", "integer", true, false, false, "", "", "", "", ""),
				// orders (id pk, user_id fk → users.id). 20000 rows skips exact COUNT.
				mkRow("public", "orders", 20000, 8192*10, "r", 1, "id", "bigint", false, true, false, "", "", "", "", ""),
				mkRow("public", "orders", 20000, 8192*10, "r", 2, "user_id", "bigint", false, false, false, "users.id", "c", "a", "", ""),
			},
		},
		`SELECT COUNT(*) FROM "public"."users"`: {Rows: [][]any{{int64(1500)}}},
	}}

	doc, err := ListSchema(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Schemas) != 1 || doc.Schemas[0].Name != "public" {
		t.Fatalf("schemas: %+v", doc.Schemas)
	}
	if len(doc.Schemas[0].Tables) != 2 {
		t.Fatalf("tables: %+v", doc.Schemas[0].Tables)
	}
	users := doc.Schemas[0].Tables[0]
	if users.Name != "users" || users.Rows != 1500 || len(users.Columns) != 3 {
		t.Fatalf("users: %+v", users)
	}
	if users.RowsEstimated {
		t.Fatalf("users.Rows should be exact after small-table COUNT pass: %+v", users)
	}
	if users.RowsExact == nil || *users.RowsExact != 1500 {
		t.Fatalf("users.RowsExact should be set to 1500: %+v", users.RowsExact)
	}
	if !users.Columns[0].PK || users.Columns[0].Nullable {
		t.Fatalf("users.id should be pk + not null: %+v", users.Columns[0])
	}
	if !users.Columns[1].Unique {
		t.Fatalf("users.email should be unique: %+v", users.Columns[1])
	}
	if !users.Columns[2].Nullable {
		t.Fatalf("users.age should be nullable: %+v", users.Columns[2])
	}
	orders := doc.Schemas[0].Tables[1]
	if !orders.RowsEstimated {
		t.Fatalf("orders is large; Rows should remain estimated: %+v", orders)
	}
	fk := orders.Columns[1].FK
	if fk == nil || fk.Table != "users" || fk.Column != "id" {
		t.Fatalf("orders.user_id fk: %+v", orders.Columns[1])
	}
	if fk.OnDelete != "CASCADE" || fk.OnUpdate != "NO ACTION" {
		t.Fatalf("orders.user_id fk action codes: %+v", fk)
	}
}

func TestListSchema_EmptyDatabase(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{"\nWITH": {Rows: [][]any{}}}}
	doc, err := ListSchema(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Schemas == nil {
		t.Fatal("doc.Schemas should be non-nil empty slice for JSON shape")
	}
	if len(doc.Schemas) != 0 {
		t.Fatalf("want zero schemas, got %d", len(doc.Schemas))
	}
}

func TestListSchema_DriverError(t *testing.T) {
	pool := &fakePool{err: errors.New("boom")}
	if _, err := ListSchema(context.Background(), pool); err == nil {
		t.Fatal("expected error")
	}
}

func TestListSchema_MultipleSchemasSorted(t *testing.T) {
	// "z_archive" returned first by the (mocked) SQL but should be sorted
	// after "public" alphabetically.
	pool := &fakePool{results: map[string]*postgres.Result{
		"\nWITH": {Rows: [][]any{
			mkRow("z_archive", "old", 1, 8192, "r", 1, "id", "bigint", false, true, false, "", "", "", "", ""),
			mkRow("public", "live", 10, 8192, "r", 1, "id", "bigint", false, true, false, "", "", "", "", ""),
		}},
		`SELECT COUNT(*) FROM "z_archive"."old"`: {Rows: [][]any{{int64(1)}}},
		`SELECT COUNT(*) FROM "public"."live"`:   {Rows: [][]any{{int64(10)}}},
	}}
	doc, err := ListSchema(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Schemas[0].Name != "public" || doc.Schemas[1].Name != "z_archive" {
		t.Fatalf("sort: %+v", doc.Schemas)
	}
}

func TestListSchema_PartitionedTablesSkipExactCount(t *testing.T) {
	// Partitioned parents (relkind='p') have reltuples=0 on the parent itself;
	// our SQL aggregates via pg_inherits. We must not run an extra COUNT(*)
	// against a partitioned parent — that would re-scan every partition.
	pool := &fakePool{results: map[string]*postgres.Result{
		"\nWITH": {Rows: [][]any{
			mkRow("public", "events", 5000, 1<<20, "p", 1, "id", "bigint", false, true, false, "", "", "", "", ""),
		}},
	}}
	doc, err := ListSchema(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range pool.executed {
		if strings.HasPrefix(sql, `SELECT COUNT(*) FROM "public"."events"`) {
			t.Fatalf("should not COUNT a partitioned parent: %s", sql)
		}
	}
	events := doc.Schemas[0].Tables[0]
	if !events.RowsEstimated || events.Kind != "p" {
		t.Fatalf("partitioned table: %+v", events)
	}
}

func TestListSchema_CapturesDefaultsAndComments(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{
		"\nWITH": {Rows: [][]any{
			mkRow("public", "users", 0, 8192, "r", 1, "id", "bigint", false, true, false, "", "", "", "nextval('users_id_seq')", ""),
			mkRow("public", "users", 0, 8192, "r", 2, "email", "text", false, false, false, "", "", "", "", "Primary contact"),
		}},
		`SELECT COUNT(*) FROM "public"."users"`: {Rows: [][]any{{int64(0)}}},
	}}
	doc, err := ListSchema(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	cols := doc.Schemas[0].Tables[0].Columns
	if cols[0].Default == nil || *cols[0].Default != "nextval('users_id_seq')" {
		t.Fatalf("id default: %+v", cols[0].Default)
	}
	if cols[1].Comment != "Primary contact" {
		t.Fatalf("email comment: %q", cols[1].Comment)
	}
}

func TestListSchema_FoldsIndexes(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{
		"\nWITH": {Rows: [][]any{
			mkRow("public", "users", 0, 8192, "r", 1, "id", "bigint", false, true, false, "", "", "", "", ""),
		}},
		`SELECT COUNT(*) FROM "public"."users"`: {Rows: [][]any{{int64(0)}}},
		"\nSELECT\n    n.nspname AS schema_name,\n    c.relname AS table_name,\n    ic.relname AS index_name": {
			Rows: [][]any{
				{"public", "users", "users_pkey", true, true, int64(16384), "btree", "id"},
				{"public", "users", "users_email_idx", true, false, int64(8192), "btree", "email,lower(email)"},
			},
		},
	}}
	doc, err := ListSchema(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	idx := doc.Schemas[0].Tables[0].Indexes
	if len(idx) != 2 {
		t.Fatalf("want 2 indexes, got %d", len(idx))
	}
	if idx[0].Name != "users_pkey" || !idx[0].Primary || !idx[0].Unique {
		t.Fatalf("pkey: %+v", idx[0])
	}
	if idx[1].Name != "users_email_idx" || idx[1].Primary || !idx[1].Unique {
		t.Fatalf("email idx: %+v", idx[1])
	}
	if len(idx[1].Columns) != 2 || idx[1].Columns[0] != "email" || idx[1].Columns[1] != "lower(email)" {
		t.Fatalf("email idx columns: %+v", idx[1].Columns)
	}
}
