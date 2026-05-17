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

func TestListSchema_GroupsTablesAndColumns(t *testing.T) {
	// Two tables under "public" with PK + FK + nullable columns.
	pool := &fakePool{results: map[string]*postgres.Result{
		"\nSELECT": {
			Rows: [][]any{
				// users (id pk, email unique, age nullable)
				{"public", "users", int64(1500), int64(8192 * 4), 1, "id", "bigint", false, true, false, ""},
				{"public", "users", int64(1500), int64(8192 * 4), 2, "email", "text", false, false, true, ""},
				{"public", "users", int64(1500), int64(8192 * 4), 3, "age", "integer", true, false, false, ""},
				// orders (id pk, user_id fk → users.id)
				{"public", "orders", int64(20000), int64(8192 * 10), 1, "id", "bigint", false, true, false, ""},
				{"public", "orders", int64(20000), int64(8192 * 10), 2, "user_id", "bigint", false, false, false, "users.id"},
			},
		},
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
	if orders.Columns[1].FK == nil || orders.Columns[1].FK.Table != "users" || orders.Columns[1].FK.Column != "id" {
		t.Fatalf("orders.user_id fk: %+v", orders.Columns[1])
	}
}

func TestListSchema_EmptyDatabase(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{"\nSELECT": {Rows: [][]any{}}}}
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
		"\nSELECT": {Rows: [][]any{
			{"z_archive", "old", int64(1), int64(8192), 1, "id", "bigint", false, true, false, ""},
			{"public", "live", int64(10), int64(8192), 1, "id", "bigint", false, true, false, ""},
		}},
	}}
	doc, err := ListSchema(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Schemas[0].Name != "public" || doc.Schemas[1].Name != "z_archive" {
		t.Fatalf("sort: %+v", doc.Schemas)
	}
}
