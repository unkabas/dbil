package pg

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/unkabas/dbil/internal/postgres"
)

func TestFetchRows_HappyPath(t *testing.T) {
	// A page that's filled to pageSize doesn't pin the total — we report
	// the reltuples estimate (and EstimatedTotalExact stays false).
	rows := make([][]any, 50)
	for i := range rows {
		rows[i] = []any{int64(i + 1), "u@x"}
	}
	pool := &fakePool{results: map[string]*postgres.Result{
		"SELECT c.reltuples": {Rows: [][]any{{int64(12345)}}},
		`SELECT * FROM "public"."users"`: {
			Columns: []postgres.ColumnDef{
				{Name: "id", TypeName: "int8"},
				{Name: "email", TypeName: "text"},
			},
			Rows: rows,
		},
	}}

	resp, err := FetchRows(context.Background(), pool, "public", "users", 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if resp.EstimatedTotal != 12345 || resp.EstimatedTotalExact {
		t.Fatalf("estimated_total: %d exact:%v (want 12345, !exact)", resp.EstimatedTotal, resp.EstimatedTotalExact)
	}
	if len(resp.Rows) != 50 || len(resp.Columns) != 2 {
		t.Fatalf("payload: %d rows %d cols", len(resp.Rows), len(resp.Columns))
	}
	if resp.Columns[0].Name != "id" || resp.Columns[0].TypeName != "int8" {
		t.Fatalf("col: %+v", resp.Columns[0])
	}
}

func TestFetchRows_RejectsInvalidIdentifier(t *testing.T) {
	pool := &fakePool{}
	cases := [][2]string{
		{"public; DROP TABLE x", "users"},
		{"public", "users;--"},
		{"", "users"},
		{"public", ""},
		{"public", "Users mixed"}, // space
	}
	for _, c := range cases {
		_, err := FetchRows(context.Background(), pool, c[0], c[1], 0, 50)
		if !errors.Is(err, ErrInvalidIdentifier) {
			t.Fatalf("schema=%q name=%q: expected ErrInvalidIdentifier, got %v", c[0], c[1], err)
		}
	}
}

func TestFetchRows_PageSizeClampedAndOffset(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{
		"SELECT c.reltuples":             {Rows: [][]any{{int64(0)}}},
		`SELECT * FROM "public"."users"`: {Rows: [][]any{}},
	}}
	if _, err := FetchRows(context.Background(), pool, "public", "users", 2, 10_000); err != nil {
		t.Fatal(err)
	}
	if got := lastMatching(pool.executed, `SELECT * FROM "public"."users"`); !strings.Contains(got, "LIMIT 200 OFFSET 400") {
		t.Fatalf("page-size clamp failed; got SQL: %s", got)
	}
}

func TestFetchRows_NegativePageBecomesZero(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{
		"SELECT c.reltuples":             {Rows: [][]any{{int64(0)}}},
		`SELECT * FROM "public"."users"`: {Rows: [][]any{}},
	}}
	if _, err := FetchRows(context.Background(), pool, "public", "users", -3, 0); err != nil {
		t.Fatal(err)
	}
	got := lastMatching(pool.executed, `SELECT * FROM "public"."users"`)
	if !strings.Contains(got, "OFFSET 0") {
		t.Fatalf("offset clamp failed; got SQL: %s", got)
	}
	if !strings.Contains(got, "LIMIT 50") {
		t.Fatalf("default page size should be 50; got SQL: %s", got)
	}
}

func TestFetchRows_EstimateMinusOneOnMissingRelation(t *testing.T) {
	// pg_class lookup returns no rows. We received zero rows on the first
	// page, which pins the exact total at 0 — better than -1 in the UI.
	pool := &fakePool{results: map[string]*postgres.Result{
		"SELECT c.reltuples":             {Rows: [][]any{}},
		`SELECT * FROM "public"."users"`: {Rows: [][]any{}},
	}}
	resp, err := FetchRows(context.Background(), pool, "public", "users", 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if resp.EstimatedTotal != 0 || !resp.EstimatedTotalExact {
		t.Fatalf("estimated_total: %d exact:%v (want 0 exact)", resp.EstimatedTotal, resp.EstimatedTotalExact)
	}
}

func TestFetchRows_ShortPageMarksExact(t *testing.T) {
	// reltuples claims 12345 but we only got 3 rows on page 0 → exact total
	// is 3 and we should report that, not the stale estimate.
	pool := &fakePool{results: map[string]*postgres.Result{
		"SELECT c.reltuples": {Rows: [][]any{{int64(12345)}}},
		`SELECT * FROM "public"."tiny"`: {
			Columns: []postgres.ColumnDef{{Name: "id", TypeName: "int8"}},
			Rows:    [][]any{{int64(1)}, {int64(2)}, {int64(3)}},
		},
	}}
	resp, err := FetchRows(context.Background(), pool, "public", "tiny", 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.EstimatedTotalExact {
		t.Fatalf("short page should mark exact: %+v", resp)
	}
}

func TestSearchRows_BuildsExactValueFilters(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{
		`SELECT COUNT(*) FROM "public"."items"`: {Rows: [][]any{{int64(2)}}},
		`SELECT * FROM "public"."items"`: {
			Columns: []postgres.ColumnDef{{Name: "category", TypeName: "text"}},
			Rows:    [][]any{{"books"}, {"food"}},
		},
	}}
	resp, err := SearchRows(context.Background(), pool, "public", "items", SearchRowsRequest{
		Page: 1, PageSize: 25,
		Filters: []TableFilter{{Column: "category", Values: []any{"books", "food"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.FilteredTotal != 2 {
		t.Fatalf("filtered_total: %d", resp.FilteredTotal)
	}
	sql := lastMatching(pool.executed, `SELECT * FROM "public"."items"`)
	if !strings.Contains(sql, `"category"::text IN ('books', 'food')`) {
		t.Fatalf("missing IN filter: %s", sql)
	}
	if !strings.Contains(sql, "LIMIT 25 OFFSET 25") {
		t.Fatalf("missing paging: %s", sql)
	}
}

func TestSearchRows_FilterHandlesNullAndInvalidColumn(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{
		`SELECT COUNT(*) FROM "public"."items"`: {Rows: [][]any{{int64(1)}}},
		`SELECT * FROM "public"."items"`:        {Rows: [][]any{}},
	}}
	_, err := SearchRows(context.Background(), pool, "public", "items", SearchRowsRequest{
		Filters: []TableFilter{{Column: "category", Values: []any{nil, "books"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	sql := lastMatching(pool.executed, `SELECT * FROM "public"."items"`)
	if !strings.Contains(sql, `"category"::text IN ('books') OR "category" IS NULL`) {
		t.Fatalf("missing null filter: %s", sql)
	}
	_, err = SearchRows(context.Background(), pool, "public", "items", SearchRowsRequest{
		Filters: []TableFilter{{Column: "bad column", Values: []any{"x"}}},
	})
	if !errors.Is(err, ErrInvalidIdentifier) {
		t.Fatalf("expected ErrInvalidIdentifier, got %v", err)
	}
}

func TestDistinctValues_ExcludesOwnFilter(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{
		`SELECT "category" IS NULL`: {
			Rows: [][]any{
				{false, "books", int64(2)},
				{true, "", int64(1)},
			},
		},
	}}
	resp, err := DistinctValues(context.Background(), pool, "public", "items", "category", []TableFilter{
		{Column: "category", Values: []any{"books"}},
		{Column: "status", Values: []any{"open"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Values) != 2 || resp.Values[1].Value != nil {
		t.Fatalf("values: %+v", resp.Values)
	}
	sql := lastMatching(pool.executed, `SELECT "category" IS NULL`)
	if strings.Contains(sql, `"category"::text IN`) {
		t.Fatalf("own filter should be excluded from distinct query: %s", sql)
	}
	if !strings.Contains(sql, `"status"::text IN ('open')`) {
		t.Fatalf("other filters should remain: %s", sql)
	}
}

func TestExportRows_CapsAndCanIgnoreFilters(t *testing.T) {
	rows := make([][]any, ExportRowCap+1)
	for i := range rows {
		rows[i] = []any{i}
	}
	pool := &fakePool{results: map[string]*postgres.Result{
		`SELECT * FROM "public"."items"`: {
			Columns: []postgres.ColumnDef{{Name: "id", TypeName: "int8"}},
			Rows:    rows,
		},
	}}
	resp, err := ExportRows(context.Background(), pool, "public", "items", []TableFilter{
		{Column: "category", Values: []any{"books"}},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Truncated || len(resp.Rows) != ExportRowCap {
		t.Fatalf("cap failed: truncated=%v rows=%d", resp.Truncated, len(resp.Rows))
	}
	sql := lastMatching(pool.executed, `SELECT * FROM "public"."items"`)
	if strings.Contains(sql, "WHERE") {
		t.Fatalf("full export should ignore filters: %s", sql)
	}
}

func lastMatching(executed []string, prefix string) string {
	for i := len(executed) - 1; i >= 0; i-- {
		if strings.HasPrefix(executed[i], prefix) {
			return executed[i]
		}
	}
	return ""
}
