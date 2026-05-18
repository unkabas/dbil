package observ

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/unkabas/dbil/internal/postgres"
)

func TestRunAdvisor_HappyPath(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{
		"\nSELECT\n    schemaname": { // missing-index query starts here
			Rows: [][]any{
				{"public", "events", int64(5000), int64(120), int64(2_500_000), int64(800_000_000), int64(120_000)},
				{"public", "users", int64(200), int64(50), int64(1_200_000), int64(420_000_000), int64(3_000)},
			},
		},
	}}
	// Manually wire unused query — fakePool prefix-matches the FIRST matching
	// key. Add a different prefix for the unused query result.
	pool.results["\nSELECT\n    schemaname\n"] = pool.results["\nSELECT\n    schemaname"]
	// Build a fake response specifically for the unused query path.
	pool2 := &fakePool{results: map[string]*postgres.Result{
		"\nSELECT\n    schemaname                                                    AS schema,\n    relname                                                       AS table,\n    seq_scan::bigint": {
			Rows: [][]any{
				{"public", "events", int64(5000), int64(120), int64(2_500_000), int64(800_000_000), int64(120_000)},
			},
		},
		"\nSELECT\n    schemaname                                                    AS schema,\n    relname                                                       AS table,\n    indexrelname": {
			Rows: [][]any{
				{"public", "events", "idx_legacy_session", int64(45_000_000), false},
			},
		},
	}}
	rep, err := RunAdvisor(context.Background(), pool2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.MissingIndexes) != 1 || rep.MissingIndexes[0].Table != "events" {
		t.Fatalf("missing: %+v", rep.MissingIndexes)
	}
	if rep.MissingIndexes[0].SeqScans != 5000 || rep.MissingIndexes[0].SeqRowsAvg != 120_000 {
		t.Fatalf("missing scalars: %+v", rep.MissingIndexes[0])
	}
	if len(rep.UnusedIndexes) != 1 || rep.UnusedIndexes[0].Index != "idx_legacy_session" {
		t.Fatalf("unused: %+v", rep.UnusedIndexes)
	}
	if rep.UnusedIndexes[0].SizeBytes != 45_000_000 || rep.UnusedIndexes[0].IsUnique {
		t.Fatalf("unused scalars: %+v", rep.UnusedIndexes[0])
	}
}

func TestRunAdvisor_NoFindingsReturnsEmptyArrays(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{}}
	rep, err := RunAdvisor(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if rep.MissingIndexes == nil || rep.UnusedIndexes == nil {
		t.Fatal("expected non-nil empty slices for JSON shape")
	}
	if len(rep.MissingIndexes) != 0 || len(rep.UnusedIndexes) != 0 {
		t.Fatalf("expected empty, got %+v", rep)
	}
}

func TestRunAdvisor_ProbeFailureSurfacedAsError(t *testing.T) {
	pool := &fakePool{err: errors.New("permission denied")}
	if _, err := RunAdvisor(context.Background(), pool); err == nil ||
		!strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected wrapped permission error, got %v", err)
	}
}
