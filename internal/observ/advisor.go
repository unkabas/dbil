package observ

import (
	"context"
	"fmt"

	"github.com/unkabas/dbil/internal/postgres"
)

// AdvisorReport is the live result of running the index-advisor heuristics
// against a target Postgres. It is computed on demand from pg_stat_*
// catalog views; no historical storage in v1.1.
type AdvisorReport struct {
	MissingIndexes []MissingIndexHint `json:"missing_indexes"`
	UnusedIndexes  []UnusedIndexHint  `json:"unused_indexes"`
}

// MissingIndexHint is a table that has accumulated many sequential scans
// relative to its index scans — a likely target for a new btree index.
type MissingIndexHint struct {
	Schema     string `json:"schema"`
	Table      string `json:"table"`
	SeqScans   int64  `json:"seq_scans"`
	IdxScans   int64  `json:"idx_scans"`
	LiveRows   int64  `json:"live_rows"`
	SizeBytes  int64  `json:"size_bytes"`
	SeqRowsAvg int64  `json:"seq_rows_avg"` // rows returned per seq_scan on average
}

// UnusedIndexHint is a non-primary index that has never been used by the
// planner (idx_scan = 0) — a candidate for DROP INDEX.
type UnusedIndexHint struct {
	Schema    string `json:"schema"`
	Table     string `json:"table"`
	Index     string `json:"index"`
	SizeBytes int64  `json:"size_bytes"`
	IsUnique  bool   `json:"is_unique"`
}

// RunAdvisor executes both heuristic queries and assembles the report.
// The thresholds aim to surface only signal (skip toy tables and one-off
// scans) and may be tuned in later versions.
func RunAdvisor(ctx context.Context, pool postgres.Pool) (*AdvisorReport, error) {
	miss, err := pool.Execute(ctx, missingIndexSQL)
	if err != nil {
		return nil, fmt.Errorf("missing-index probe: %w", err)
	}
	unused, err := pool.Execute(ctx, unusedIndexSQL)
	if err != nil {
		return nil, fmt.Errorf("unused-index probe: %w", err)
	}

	rep := &AdvisorReport{
		MissingIndexes: []MissingIndexHint{},
		UnusedIndexes:  []UnusedIndexHint{},
	}

	for _, r := range miss.Rows {
		if len(r) < 7 {
			continue
		}
		rep.MissingIndexes = append(rep.MissingIndexes, MissingIndexHint{
			Schema:     asString(r[0]),
			Table:      asString(r[1]),
			SeqScans:   asInt64(r[2]),
			IdxScans:   asInt64(r[3]),
			LiveRows:   asInt64(r[4]),
			SizeBytes:  asInt64(r[5]),
			SeqRowsAvg: asInt64(r[6]),
		})
	}
	for _, r := range unused.Rows {
		if len(r) < 5 {
			continue
		}
		rep.UnusedIndexes = append(rep.UnusedIndexes, UnusedIndexHint{
			Schema:    asString(r[0]),
			Table:     asString(r[1]),
			Index:     asString(r[2]),
			SizeBytes: asInt64(r[3]),
			IsUnique:  asBool(r[4]),
		})
	}
	return rep, nil
}

// missingIndexSQL ranks tables where sequential scans dominate index scans
// AND the relation is non-trivial in size. Thresholds:
//   - skip relations smaller than 8 MB (toy tables don't need indexes).
//   - require seq_scan > 100 (one-off scans are noise).
//   - require seq_scan > idx_scan * 4 (clearly seq-heavy).
//   - require avg-rows-per-seq-scan > 1000 (a scan that returns one row
//     is probably already a unique-by-PK lookup).
const missingIndexSQL = `
SELECT
    schemaname                                                    AS schema,
    relname                                                       AS table,
    seq_scan::bigint                                              AS seq_scan,
    COALESCE(idx_scan, 0)::bigint                                 AS idx_scan,
    COALESCE(n_live_tup, 0)::bigint                               AS live_rows,
    COALESCE(pg_total_relation_size(relid), 0)::bigint            AS size_bytes,
    CASE WHEN seq_scan = 0 THEN 0
         ELSE (seq_tup_read / seq_scan)::bigint
    END                                                           AS seq_rows_avg
FROM pg_stat_user_tables
WHERE pg_total_relation_size(relid) > 8 * 1024 * 1024
  AND seq_scan > 100
  AND seq_scan > COALESCE(idx_scan, 0) * 4
  AND (CASE WHEN seq_scan = 0 THEN 0 ELSE seq_tup_read / seq_scan END) > 1000
ORDER BY seq_scan DESC
LIMIT 50
`

// unusedIndexSQL lists indexes with idx_scan = 0 (never used by the
// planner since the last stats reset) excluding unique / primary
// constraints. Index size > 1 MB filters out cheap noise.
const unusedIndexSQL = `
SELECT
    schemaname                                                    AS schema,
    relname                                                       AS table,
    indexrelname                                                  AS index,
    pg_relation_size(indexrelid)::bigint                          AS size_bytes,
    COALESCE(pi.indisunique, false)                               AS is_unique
FROM pg_stat_user_indexes psi
JOIN pg_index pi ON pi.indexrelid = psi.indexrelid
WHERE idx_scan = 0
  AND NOT pi.indisunique
  AND NOT pi.indisprimary
  AND pg_relation_size(indexrelid) > 1024 * 1024
ORDER BY pg_relation_size(indexrelid) DESC
LIMIT 50
`
