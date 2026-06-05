package pg

import (
	"context"
	"errors"
	"testing"

	"github.com/unkabas/dbil/internal/postgres"
)

func shape() TableShape {
	return TableShape{
		PrimaryKey: []string{"id"},
		Columns:    map[string]bool{"id": true, "name": true, "status": true},
	}
}

func TestBuildMutations_Update(t *testing.T) {
	stmts, err := BuildMutations("public", "t", shape(), []RowChange{
		{Op: "update", PK: map[string]any{"id": 12}, Set: map[string]any{"name": "bob", "status": nil}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `UPDATE "public"."t" SET "name" = 'bob', "status" = NULL WHERE "id" = '12'`
	if stmts[0] != want {
		t.Fatalf("update SQL mismatch:\n got: %s\nwant: %s", stmts[0], want)
	}
}

func TestBuildMutations_DeleteAndInsert(t *testing.T) {
	stmts, err := BuildMutations("public", "t", shape(), []RowChange{
		{Op: "delete", PK: map[string]any{"id": 7}},
		{Op: "insert", Values: map[string]any{"name": "carol", "id": 9}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stmts[0] != `DELETE FROM "public"."t" WHERE "id" = '7'` {
		t.Fatalf("delete SQL: %s", stmts[0])
	}
	if stmts[1] != `INSERT INTO "public"."t" ("id", "name") VALUES ('9', 'carol')` {
		t.Fatalf("insert SQL: %s", stmts[1])
	}
}

func TestBuildMutations_QuoteEscaping(t *testing.T) {
	stmts, err := BuildMutations("public", "t", shape(), []RowChange{
		{Op: "update", PK: map[string]any{"id": 1}, Set: map[string]any{"name": "O'Brien"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stmts[0] != `UPDATE "public"."t" SET "name" = 'O''Brien' WHERE "id" = '1'` {
		t.Fatalf("escaping SQL: %s", stmts[0])
	}
}

func TestBuildMutations_UnknownColumn(t *testing.T) {
	_, err := BuildMutations("public", "t", shape(), []RowChange{
		{Op: "update", PK: map[string]any{"id": 1}, Set: map[string]any{"nope": "x"}},
	})
	if !errors.Is(err, ErrUnknownColumn) {
		t.Fatalf("want ErrUnknownColumn, got %v", err)
	}
}

func TestBuildMutations_MissingPK(t *testing.T) {
	_, err := BuildMutations("public", "t", shape(), []RowChange{
		{Op: "update", PK: map[string]any{}, Set: map[string]any{"name": "x"}},
	})
	if !errors.Is(err, ErrInvalidChange) {
		t.Fatalf("want ErrInvalidChange, got %v", err)
	}
}

func TestBuildMutations_UnknownOp(t *testing.T) {
	_, err := BuildMutations("public", "t", shape(), []RowChange{{Op: "truncate"}})
	if !errors.Is(err, ErrInvalidChange) {
		t.Fatalf("want ErrInvalidChange, got %v", err)
	}
}

func TestBuildMutations_EmptyUpdate(t *testing.T) {
	_, err := BuildMutations("public", "t", shape(), []RowChange{
		{Op: "update", PK: map[string]any{"id": 1}, Set: map[string]any{}},
	})
	if !errors.Is(err, ErrInvalidChange) {
		t.Fatalf("want ErrInvalidChange for empty set, got %v", err)
	}
}

func TestIntrospectTable_PK(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{
		"\nSELECT a.attname": {Rows: [][]any{
			{"id", true},
			{"name", false},
		}},
	}}
	shape, err := IntrospectTable(context.Background(), pool, "public", "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(shape.PrimaryKey) != 1 || shape.PrimaryKey[0] != "id" {
		t.Fatalf("pk: %v", shape.PrimaryKey)
	}
	if !shape.Columns["name"] {
		t.Fatal("columns should include name")
	}
}

func TestIntrospectTable_NoPK(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{
		"\nSELECT a.attname": {Rows: [][]any{
			{"a", false},
			{"b", false},
		}},
	}}
	_, err := IntrospectTable(context.Background(), pool, "public", "t")
	if !errors.Is(err, ErrNoPrimaryKey) {
		t.Fatalf("want ErrNoPrimaryKey, got %v", err)
	}
}
