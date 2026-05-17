import { useMemo, useState } from 'react'
import { tablesFor, type MockTable } from '../mock/data'
import TableCard from '../components/TableCard'
import { useShellContext } from '../shell/context'

export default function SchemaPage() {
  const { activeConnID, activeConn } = useShellContext()
  // Plan 7 replaces this with pg_catalog-driven introspection. Until then,
  // every connection sees the sample schema so the page stays meaningful.
  const tables = tablesFor(activeConnID)
  const [focused, setFocused] = useState<string | null>(null)
  const [filter, setFilter] = useState('')

  const filtered = useMemo(() => {
    if (!filter.trim()) return tables
    const f = filter.toLowerCase()
    return tables.filter(
      (t) =>
        t.name.toLowerCase().includes(f) ||
        t.schema.toLowerCase().includes(f) ||
        t.columns.some((c) => c.name.toLowerCase().includes(f)),
    )
  }, [tables, filter])

  const grouped = useMemo(() => groupBySchema(filtered), [filtered])

  // When a table is focused, surface its FK targets so we can highlight them.
  const focusedFkTargets = useMemo(() => {
    if (!focused) return new Set<string>()
    const focusedTable = tables.find((t) => `${t.schema}.${t.name}` === focused)
    if (!focusedTable) return new Set<string>()
    return new Set(
      focusedTable.columns.filter((c) => c.fk).map((c) => `${focusedTable.schema}.${c.fk!.table}`),
    )
  }, [focused, tables])

  // Stats for the header strip.
  const totalRows = tables.reduce((sum, t) => sum + t.rows, 0)
  const totalCols = tables.reduce((sum, t) => sum + t.columns.length, 0)

  return (
    <div className="h-full overflow-auto bg-app-grad">
      <div className="max-w-[1400px] mx-auto px-6 py-6">
        <header className="flex items-end justify-between mb-5">
          <div>
            <h1 className="text-[22px] font-semibold text-ink-50 tracking-tight">Schema</h1>
            <p className="text-ink-300 text-[13px] mt-0.5">
              {activeConn && <span className="font-mono">{activeConn.alias}</span>}
              {activeConn && <span className="text-ink-500"> · </span>}
              {tables.length} tables &nbsp;·&nbsp; {totalCols} columns &nbsp;·&nbsp;{' '}
              {totalRows.toLocaleString()} rows total
            </p>
          </div>
          <input
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter tables or columns…"
            className="w-72 h-9 px-3 rounded-md bg-ink-800 border border-ink-700 focus:border-violet focus:outline-none text-[13px] text-ink-100 placeholder:text-ink-400"
          />
        </header>

        {Object.entries(grouped).map(([schema, schTables]) => (
          <section key={schema} className="mb-8">
            <div className="flex items-center gap-2 mb-3">
              <h2 className="text-ink-100 font-semibold font-mono text-[14px]">{schema}</h2>
              <span className="text-ink-400 text-[11.5px]">{schTables.length} tables</span>
              <span className="flex-1 h-px bg-ink-700 ml-2" />
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              {schTables.map((t) => {
                const key = `${t.schema}.${t.name}`
                return (
                  <TableCard
                    key={key}
                    table={t}
                    isFocused={focused === key}
                    highlightedFkTo={focused && focusedFkTargets.has(key) ? key : null}
                    onClick={() => setFocused((cur) => (cur === key ? null : key))}
                  />
                )
              })}
            </div>
          </section>
        ))}

        {filtered.length === 0 && (
          <div className="text-ink-300 text-center py-12 text-[13px]">
            No tables match &quot;{filter}&quot;
          </div>
        )}
      </div>
    </div>
  )
}

function groupBySchema(tables: MockTable[]): Record<string, MockTable[]> {
  const out: Record<string, MockTable[]> = {}
  for (const t of tables) {
    ;(out[t.schema] ??= []).push(t)
  }
  return out
}
