# DBil v0.1 — Plan 7 (Schema introspector + real Data view)

## Context

Plans 5-6 ship the chrome and observability with mock-driven schema/data.
Plan 7 reads real pg_catalog so the Schema ER view and the Data row
browser stop lying.

## Scope (v0.7.0)

| In | Out (v0.7.1+) |
|---|---|
| `GET /api/connections/{id}/schema` — tables + columns + PKs + FKs + sizes + row estimates from pg_catalog | UI for editing tag policies |
| `GET /api/connections/{id}/table/{schema}/{name}/rows?page=…&page_size=…` — paginated rows via `SELECT * FROM … LIMIT … OFFSET …` | Inline row edit |
| Frontend `useSchema` + `useTableRows` TanStack hooks | Schema diffs across environments |
| SchemaPage + DataPage read real data; auto-layout positions cards in a grid | ER drag-to-move + persisted positions |
| `web/src/mock/data.ts` deleted (no longer referenced) | |

## Tech additions

None — pg_catalog only.

## File structure added

```
internal/pg/                        # new
  introspect.go                     # ListSchema(pool) -> SchemaDoc
  introspect_test.go                # uses fakePool with canned pg_catalog rows
  rows.go                           # FetchRows(pool, schema, name, page, pageSize)
internal/server/handlers/
  schema.go                         # GET /api/connections/{id}/schema
  rows.go                           # GET .../table/{schema}/{name}/rows
  handlers.go                       # MODIFIED — adds two routes
web/src/api/
  schema.ts                         # types + useSchema + useTableRows hooks
web/src/pages/
  SchemaPage.tsx                    # MODIFIED — useSchema; auto grid layout
  DataPage.tsx                      # MODIFIED — useSchema + useTableRows
web/src/mock/data.ts                # REMOVED
```

## Data model on the wire

```ts
// /api/connections/{id}/schema
interface SchemaDoc {
  schemas: Array<{
    name: string
    tables: Array<{
      schema: string
      name: string
      rows: number              // pg_class.reltuples; live count is a slow path
      size_bytes: number        // pg_total_relation_size
      columns: Array<{
        name: string
        type: string            // formatted via format_type(atttypid, atttypmod)
        nullable: boolean
        pk: boolean
        unique: boolean
        fk?: { table: string; column: string }
      }>
    }>
  }>
}

// /api/connections/{id}/table/{schema}/{name}/rows?page=0&page_size=50
interface RowsResponse {
  columns: Array<{ name: string; type_name: string }>
  rows: Array<Array<string | number | boolean | null>>
  estimated_total: number       // pg_class.reltuples; -1 if unknown
}
```

## Tasks

### Phase A — pg_catalog introspection

**Task 1 — `internal/pg/introspect.go`** with a single SQL that joins
`pg_namespace`, `pg_class`, `pg_attribute`, `pg_constraint`, returning all
non-system schemas. Tested against a `fakePool` returning canned rows.

**Task 2 — `internal/pg/rows.go`** with `FetchRows(pool, schema, name,
page, pageSize)`. SQL: `SELECT * FROM <schema>.<name> LIMIT <n> OFFSET
<m>`. Estimates from `pg_class.reltuples` for `<schema>.<name>`. Quotes
identifiers via `pgx/v5`'s `pgconn.SanitizeSQL` equivalent (manual
quoting; we already validate input via the URL pattern).

### Phase B — Endpoints

**Task 3 — `handlers/schema.go`** + `handlers/rows.go` registered under
RequireAuth. Open the pool via `Manager.OpenByID(ctx, id,
X-Connection-Passphrase)` — same shape as the observ/locks handler.

**Task 4 — Mount + lint-auth** updated.

### Phase C — Frontend

**Task 5 — `api/schema.ts`** types + `useSchema(connID)` + `useTableRows
(connID, schema, name, page)`.

**Task 6 — `SchemaPage`** drops mock; uses `useSchema`. Auto-layout: sort
schemas → tables, place in 3-column grid (`Math.floor(i / 3) * 420 + 60`,
`Math.floor(i % 3) ... wait, swap`). Empty/loading states.

**Task 7 — `DataPage`** drops mock; table picker reads `useSchema`, rows
read `useTableRows`. Pagination triggers a fresh fetch.

**Task 8 — Delete `web/src/mock/data.ts`**. Audit imports across the
codebase (`grep -r mock/data` should be empty).

### Phase D — Test + tag

**Task 9** — `go test ./...` + `make lint-auth` + frontend build green.
Tag `v0.7.0-schema`.

## Risks acknowledged

- **`SELECT * FROM …` for row browse** is fine for normal tables; for tables
  with `bytea`/large text columns it can punch over the 10-row-cap that the
  query executor enforces. Rows endpoint sets its own LIMIT/OFFSET and a
  separate `result_max_rows` cap (200 by default) so the page-grid stays
  responsive.
- **Identifier quoting** uses a stricter regex than pg's full grammar
  (`^[a-zA-Z_][a-zA-Z_0-9]*$`); names with mixed case or unicode return
  400. v0.7.1 may relax this via proper `pgx` quoting helpers.
- **Row-count estimates from `reltuples`** can be stale after a big
  insert/delete without ANALYZE. The UI labels the count as estimated.
