import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useVirtualizer } from '@tanstack/react-virtual'
import { useShellContext } from '../shell/context'
import {
  useSchema,
  useInfiniteTableRows,
  fetchDistinctValues,
  exportTable,
  type SchemaTable,
  type CellValue,
  type TableFilter,
  type DistinctValue,
} from '../api/schema'
import { ApiError } from '../api/client'
import Icon from '../components/Icon'

const PAGE_SIZE = 200
const ROW_HEIGHT = 30

export default function DataPage() {
  const { activeConnID, activeConn } = useShellContext()
  const { schema, name } = useParams<{ schema: string; name: string }>()
  const navigate = useNavigate()

  const schemaQuery = useSchema(activeConnID)
  const allTables = useMemo<SchemaTable[]>(() => {
    if (!schemaQuery.data) return []
    return schemaQuery.data.schemas.flatMap((s) => s.tables)
  }, [schemaQuery.data])

  const effective: SchemaTable | undefined = useMemo(() => {
    if (schema && name) {
      return allTables.find((t) => t.schema === schema && t.name === name)
    }
    return allTables[0]
  }, [allTables, schema, name])

  const [showPicker, setShowPicker] = useState(false)
  const [filters, setFilters] = useState<TableFilter[]>([])
  const [filterColumn, setFilterColumn] = useState<string | null>(null)
  const [showExport, setShowExport] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)
  const parentRef = useRef<HTMLDivElement | null>(null)

  const rowsQuery = useInfiniteTableRows(
    activeConnID,
    effective?.schema ?? null,
    effective?.name ?? null,
    PAGE_SIZE,
    filters,
  )

  // Reset scroll to top whenever the table or its filters change. Without this
  // the user lands wherever the previous table was — confusing on big tables.
  useEffect(() => {
    parentRef.current?.scrollTo({ top: 0 })
  }, [effective?.schema, effective?.name, filters])

  if (schemaQuery.isLoading && allTables.length === 0) {
    return (
      <div style={{ height: '100%', display: 'grid', placeItems: 'center', color: 'var(--fg-3)', fontSize: 13 }}>
        Loading schema…
      </div>
    )
  }
  if (schemaQuery.error) {
    return <FatalError msg={'schema: ' + errMsg(schemaQuery.error)} />
  }
  if (allTables.length === 0) {
    return (
      <div style={{ height: '100%', display: 'grid', placeItems: 'center', color: 'var(--fg-3)', fontSize: 13 }}>
        No tables in this connection.
      </div>
    )
  }
  if (!effective) {
    return (
      <div style={{ height: '100%', display: 'grid', placeItems: 'center', color: 'var(--fg-3)', fontSize: 13 }}>
        Table {schema}.{name} not found.
      </div>
    )
  }

  const pages = rowsQuery.data?.pages ?? []
  const rows: CellValue[][] = useMemo(() => pages.flatMap((p) => p.rows), [pages])
  const cols =
    pages[0]?.columns ?? effective.columns.map((c) => ({ name: c.name, type_name: c.type }))
  const latestEst = pages.length > 0 ? pages[pages.length - 1] : undefined
  const estimated = latestEst?.estimated_total ?? effective.rows
  // The total is exact when either the latest page told us so, or when we've
  // reached the tail (no more pages).
  const estimatedExact = (latestEst?.estimated_total_exact ?? false) || rowsQuery.hasNextPage === false
  const activeFilters = filters.filter((f) => f.values.length > 0)

  const applyColumnFilter = (column: string, values: CellValue[]) => {
    setFilters((prev) => {
      const rest = prev.filter((f) => f.column !== column)
      return values.length > 0 ? [...rest, { column, values }] : rest
    })
  }

  const handleExport = async (format: 'csv' | 'json' | 'xlsx', scope: 'filtered' | 'all') => {
    if (!activeConnID || !effective) return
    setExporting(true)
    setNotice(null)
    try {
      const res = await exportTable(activeConnID, effective.schema, effective.name, format, scope, filters)
      setShowExport(false)
      setNotice(
        res.truncated && res.limit > 0
          ? `Export capped at ${fmtCount(res.limit)} rows.`
          : `Export started: ${scope === 'all' ? 'full table' : 'filtered rows'} as ${format.toUpperCase()}.`,
      )
    } catch (err) {
      setNotice('Export failed: ' + errMsg(err))
    } finally {
      setExporting(false)
    }
  }

  const gridTemplate = `40px ${cols.map(() => 'minmax(140px, 1fr)').join(' ')}`

  return (
    <div
      style={{
        display: 'grid',
        gridTemplateRows: '44px 48px 1fr 28px',
        height: '100%',
        minHeight: 0,
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '0 18px',
          borderBottom: '1px solid var(--line-1)',
          background: 'var(--bg-0)',
        }}
      >
        <h1 style={{ fontSize: 14.5, fontWeight: 600, letterSpacing: '-0.02em', margin: 0 }}>Data</h1>
        <span style={{ fontSize: 12, color: 'var(--fg-4)' }}>·</span>
        <span className="mono" style={{ fontSize: 12, color: 'var(--fg-3)' }}>
          {effective.schema}.{effective.name}
        </span>
        <span style={{ flex: 1 }} />
        <button className="btn-gh" title="Refresh" onClick={() => rowsQuery.refetch()}>
          <Icon name="refresh" size={12} />
        </button>
      </div>

      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '0 18px',
          borderBottom: '1px solid var(--line-1)',
          background: 'var(--bg-1)',
        }}
      >
        <TablePicker
          tables={allTables}
          value={`${effective.schema}.${effective.name}`}
          onChange={(k) => {
            const [s, n] = k.split('.')
            setFilters([])
            navigate(`/data/${encodeURIComponent(s)}/${encodeURIComponent(n)}`)
          }}
          open={showPicker}
          setOpen={setShowPicker}
        />

        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            background: 'var(--bg-2)',
            border: '1px solid var(--line-2)',
            borderRadius: 7,
            height: 28,
            padding: '0 8px',
            flex: 1,
            minWidth: 0,
          }}
        >
          <Icon name="filter" size={12} style={{ color: 'var(--fg-4)' }} />
          <div
            style={{
              flex: 1,
              minWidth: 0,
              display: 'flex',
              alignItems: 'center',
              gap: 6,
              overflow: 'hidden',
            }}
          >
            {activeFilters.length === 0 ? (
              <span style={{ color: 'var(--fg-4)', fontSize: 12 }}>No column filters</span>
            ) : (
              activeFilters.map((f) => (
                <button
                  key={f.column}
                  onClick={() => applyColumnFilter(f.column, [])}
                  title="Clear filter"
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: 5,
                    height: 20,
                    maxWidth: 220,
                    padding: '0 6px',
                    borderRadius: 5,
                    border: '1px solid var(--line-2)',
                    background: 'var(--accent-mute)',
                    color: 'var(--fg-1)',
                    fontSize: 11,
                    cursor: 'pointer',
                    fontFamily: 'inherit',
                  }}
                >
                  <span className="mono" style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>
                    {f.column}: {f.values.length === 1 ? formatFilterValue(f.values[0]) : `${f.values.length} values`}
                  </span>
                  <Icon name="x" size={10} />
                </button>
              ))
            )}
          </div>
          {activeFilters.length > 0 && (
            <button
              onClick={() => setFilters([])}
              style={{
                color: 'var(--fg-4)',
                background: 'none',
                border: 0,
                cursor: 'pointer',
                display: 'grid',
                placeItems: 'center',
              }}
            >
              <Icon name="x" size={11} />
            </button>
          )}
        </div>

        <span style={{ flex: 1 }} />
        {notice && (
          <span style={{ fontSize: 11, color: notice.startsWith('Export failed') ? 'var(--danger)' : 'var(--fg-4)' }}>
            {notice}
          </span>
        )}
        <span className="mono tnum" style={{ fontSize: 11, color: 'var(--fg-4)' }}>
          {rowsQuery.isLoading
            ? 'loading…'
            : `${rows.length} rows · ${
                activeFilters.length > 0
                  ? fmtCount(estimated) + ' filtered'
                  : (estimatedExact ? '' : '~') + fmtCount(estimated) + ' total'
              }`}
        </span>
        <div style={{ position: 'relative' }}>
          <button
            className="btn-gh"
            title="Export"
            onClick={() => setShowExport((v) => !v)}
            disabled={exporting}
            style={{ gap: 6 }}
          >
            <Icon name="download" size={12} />
            <span style={{ fontSize: 11 }}>{exporting ? 'Exporting…' : 'Export'}</span>
          </button>
          {showExport && (
            <ExportMenu
              hasFilters={activeFilters.length > 0}
              onExport={handleExport}
              onClose={() => setShowExport(false)}
            />
          )}
        </div>
      </div>

      {rowsQuery.error ? (
        <div style={{ padding: 24, overflow: 'auto', background: 'var(--bg-0)' }}>
          <FatalError msg={'rows: ' + errMsg(rowsQuery.error)} />
        </div>
      ) : (
        <VirtualizedRows
          parentRef={parentRef}
          rows={rows}
          cols={cols}
          effective={effective}
          gridTemplate={gridTemplate}
          filters={filters}
          filterColumn={filterColumn}
          setFilterColumn={setFilterColumn}
          applyColumnFilter={applyColumnFilter}
          activeConnID={activeConnID}
          isLoading={rowsQuery.isLoading}
          hasNextPage={rowsQuery.hasNextPage}
          isFetchingNextPage={rowsQuery.isFetchingNextPage}
          fetchNextPage={() => rowsQuery.fetchNextPage()}
        />
      )}

      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          padding: '0 18px',
          borderTop: '1px solid var(--line-1)',
          background: 'var(--bg-1)',
          fontSize: 11,
          color: 'var(--fg-3)',
          gap: 14,
        }}
      >
        <span className="mono tnum" style={{ color: 'var(--fg-4)' }}>
          {rows.length} loaded
        </span>
        <span style={{ color: 'var(--fg-5)' }}>·</span>
        <span className="mono tnum" style={{ color: 'var(--fg-4)' }}>
          {activeFilters.length > 0
            ? fmtCount(estimated) + ' filtered'
            : (estimatedExact ? '' : '~') + fmtCount(estimated) + ' total'}
        </span>
        <span style={{ flex: 1 }} />
        <span style={{ color: 'var(--fg-4)' }}>
          {rowsQuery.isFetchingNextPage
            ? 'loading more…'
            : rowsQuery.hasNextPage
              ? 'scroll for more'
              : 'end of data'}
        </span>
        <span style={{ color: 'var(--fg-5)' }}>·</span>
        <span style={{ color: 'var(--fg-4)' }}>{activeConn?.alias ?? '—'}</span>
      </div>
    </div>
  )
}

function VirtualizedRows({
  parentRef,
  rows,
  cols,
  effective,
  gridTemplate,
  filters,
  filterColumn,
  setFilterColumn,
  applyColumnFilter,
  activeConnID,
  isLoading,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
}: {
  parentRef: React.MutableRefObject<HTMLDivElement | null>
  rows: CellValue[][]
  cols: Array<{ name: string; type_name: string }>
  effective: SchemaTable
  gridTemplate: string
  filters: TableFilter[]
  filterColumn: string | null
  setFilterColumn(c: string | null): void
  applyColumnFilter(column: string, values: CellValue[]): void
  activeConnID: number | null
  isLoading: boolean
  hasNextPage: boolean | undefined
  isFetchingNextPage: boolean
  fetchNextPage(): void
}) {
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
  })

  const items = virtualizer.getVirtualItems()

  // Infinite-scroll trigger: when the last virtual row is close to the loaded
  // tail, request the next page. Guard against repeated calls while a fetch
  // is in flight (TanStack already deduplicates, but we save the React work).
  useEffect(() => {
    const last = items[items.length - 1]
    if (!last) return
    if (!hasNextPage || isFetchingNextPage) return
    if (last.index >= rows.length - 5) fetchNextPage()
  }, [items, rows.length, hasNextPage, isFetchingNextPage, fetchNextPage])

  // Minimum width so wide tables get a horizontal scrollbar instead of
  // crushing every column to 0.
  const minWidth = 40 + cols.length * 140

  return (
    <div
      ref={parentRef}
      style={{
        overflow: 'auto',
        background: 'var(--bg-0)',
        position: 'relative',
        contain: 'strict',
      }}
    >
      <div style={{ minWidth, position: 'relative' }}>
        {/* Sticky header */}
        <div
          role="row"
          style={{
            display: 'grid',
            gridTemplateColumns: gridTemplate,
            position: 'sticky',
            top: 0,
            zIndex: 3,
            background: 'var(--bg-1)',
            borderBottom: '1px solid var(--line-1)',
          }}
        >
          <div style={{ ...thCellBase, textAlign: 'right' }}>#</div>
          {cols.map((c) => {
            const meta = effective.columns.find((ec) => ec.name === c.name)
            return (
              <div key={c.name} style={thCellBase}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, position: 'relative' }}>
                  <span style={{ color: 'var(--fg-1)' }}>{c.name}</span>
                  {meta?.pk && (
                    <span style={{ color: 'var(--c-amber)', fontSize: 9, letterSpacing: '0.05em' }}>PK</span>
                  )}
                  {meta?.fk && (
                    <span style={{ color: 'var(--c-cyan)', fontSize: 9, letterSpacing: '0.05em' }}>FK</span>
                  )}
                  <ColumnFilterButton
                    connID={activeConnID}
                    schema={effective.schema}
                    table={effective.name}
                    column={c.name}
                    filters={filters}
                    selected={filters.find((f) => f.column === c.name)?.values ?? []}
                    open={filterColumn === c.name}
                    setOpen={(open) => setFilterColumn(open ? c.name : null)}
                    onApply={(values) => applyColumnFilter(c.name, values)}
                  />
                </div>
                <div
                  style={{
                    fontSize: 10,
                    color: 'var(--c-violet)',
                    fontStyle: 'italic',
                    marginTop: 2,
                    textTransform: 'none',
                    letterSpacing: 0,
                  }}
                >
                  {meta?.type ?? c.type_name}
                </div>
              </div>
            )
          })}
        </div>

        {/* Empty / loading states */}
        {!isLoading && rows.length === 0 && (
          <div
            style={{
              padding: 28,
              textAlign: 'center',
              color: 'var(--fg-4)',
              fontSize: 12,
              fontFamily: 'var(--font-mono)',
            }}
          >
            No rows.
          </div>
        )}

        {/* Virtualized list */}
        <div
          style={{
            height: virtualizer.getTotalSize(),
            position: 'relative',
            fontFamily: 'var(--font-mono)',
            fontSize: 12,
          }}
        >
          {items.map((vi) => {
            const row = rows[vi.index]
            return (
              <div
                key={vi.key}
                role="row"
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  width: '100%',
                  height: vi.size,
                  transform: `translateY(${vi.start}px)`,
                  display: 'grid',
                  gridTemplateColumns: gridTemplate,
                  borderBottom: '1px solid var(--line-1)',
                  color: 'var(--fg-2)',
                }}
              >
                <div style={{ ...tdCellBase, textAlign: 'right', color: 'var(--fg-4)' }} className="tnum">
                  {vi.index + 1}
                </div>
                {row.map((cell, ci) => (
                  <div key={ci} style={tdCellBase} title={String(cell ?? '')}>
                    {renderCell(cell)}
                  </div>
                ))}
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

const thCellBase: React.CSSProperties = {
  color: 'var(--fg-3)',
  fontWeight: 500,
  textAlign: 'left',
  fontSize: 11,
  letterSpacing: '0.04em',
  textTransform: 'uppercase',
  padding: '8px 12px',
  whiteSpace: 'nowrap',
  fontFamily: 'var(--font-mono)',
}

const tdCellBase: React.CSSProperties = {
  padding: '0 12px',
  display: 'flex',
  alignItems: 'center',
  whiteSpace: 'nowrap',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  minWidth: 0,
}

function TablePicker({
  tables,
  value,
  onChange,
  open,
  setOpen,
}: {
  tables: SchemaTable[]
  value: string
  onChange(k: string): void
  open: boolean
  setOpen(b: boolean): void
}) {
  const [s, n] = value.split('.')
  return (
    <div style={{ position: 'relative' }}>
      <button
        onClick={() => setOpen(!open)}
        style={{
          height: 28,
          padding: '0 10px',
          background: 'var(--bg-2)',
          border: '1px solid var(--line-2)',
          borderRadius: 7,
          color: 'var(--fg-1)',
          fontSize: 12,
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          cursor: 'pointer',
          fontFamily: 'inherit',
        }}
      >
        <Icon name="table" size={12} style={{ color: 'var(--fg-3)' }} />
        <span className="mono">
          <span style={{ color: 'var(--fg-4)' }}>{s}.</span>
          <span style={{ color: 'var(--fg-1)', fontWeight: 500 }}>{n}</span>
        </span>
        <Icon name="chev" size={11} style={{ color: 'var(--fg-3)' }} />
      </button>
      {open && (
        <div
          onMouseLeave={() => setOpen(false)}
          style={{
            position: 'absolute',
            top: 32,
            left: 0,
            width: 320,
            background: 'var(--bg-2)',
            border: '1px solid var(--line-2)',
            borderRadius: 10,
            boxShadow: 'var(--shadow-pop)',
            padding: 6,
            zIndex: 30,
            maxHeight: 420,
            overflow: 'auto',
          }}
        >
          {tables.map((t) => {
            const k = `${t.schema}.${t.name}`
            const selected = k === value
            return (
              <button
                key={k}
                onClick={() => {
                  onChange(k)
                  setOpen(false)
                }}
                style={{
                  width: '100%',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                  padding: '7px 10px',
                  background: selected ? 'var(--accent-mute)' : 'transparent',
                  border: 0,
                  color: 'var(--fg-1)',
                  borderRadius: 6,
                  fontSize: 12,
                  marginBottom: 1,
                  cursor: 'pointer',
                  fontFamily: 'inherit',
                  textAlign: 'left',
                }}
              >
                <Icon name="table" size={11} style={{ color: selected ? 'var(--accent)' : 'var(--fg-4)' }} />
                <span className="mono" style={{ flex: 1 }}>
                  <span style={{ color: 'var(--fg-4)' }}>{t.schema}.</span>
                  <span>{t.name}</span>
                </span>
                <span className="mono tnum" style={{ fontSize: 10.5, color: 'var(--fg-4)' }}>
                  {fmtCount(t.rows)}
                </span>
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}

function ColumnFilterButton({
  connID,
  schema,
  table,
  column,
  filters,
  selected,
  open,
  setOpen,
  onApply,
}: {
  connID: number | null
  schema: string
  table: string
  column: string
  filters: TableFilter[]
  selected: CellValue[]
  open: boolean
  setOpen(open: boolean): void
  onApply(values: CellValue[]): void
}) {
  const [values, setValues] = useState<DistinctValue[]>([])
  const [picked, setPicked] = useState<CellValue[]>(selected)
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [truncated, setTruncated] = useState(false)

  useEffect(() => {
    setPicked(selected)
  }, [selected, open])

  useEffect(() => {
    if (!open || !connID) return
    let cancelled = false
    setLoading(true)
    setError(null)
    fetchDistinctValues(connID, schema, table, column, filters)
      .then((resp) => {
        if (cancelled) return
        setValues(resp.values)
        setTruncated(resp.truncated)
      })
      .catch((err) => {
        if (!cancelled) setError(errMsg(err))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [open, connID, schema, table, column, filters])

  const filteredValues = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return values
    return values.filter((v) => formatFilterValue(v.value).toLowerCase().includes(q))
  }, [values, search])

  const toggle = (value: CellValue) => {
    setPicked((prev) => {
      const exists = prev.some((v) => sameCellValue(v, value))
      return exists ? prev.filter((v) => !sameCellValue(v, value)) : [...prev, value]
    })
  }

  const active = selected.length > 0
  return (
    <span style={{ position: 'relative', display: 'inline-flex' }}>
      <button
        onClick={() => setOpen(!open)}
        title={`Filter ${column}`}
        style={{
          width: 18,
          height: 18,
          border: 0,
          borderRadius: 4,
          background: active ? 'var(--accent-mute)' : 'transparent',
          color: active ? 'var(--accent)' : 'var(--fg-4)',
          display: 'grid',
          placeItems: 'center',
          cursor: 'pointer',
        }}
      >
        <Icon name="filter" size={11} />
      </button>
      {open && (
        <div
          style={{
            position: 'absolute',
            top: 22,
            left: 0,
            width: 280,
            background: 'var(--bg-2)',
            border: '1px solid var(--line-2)',
            borderRadius: 8,
            boxShadow: 'var(--shadow-pop)',
            padding: 8,
            zIndex: 60,
            textTransform: 'none',
            letterSpacing: 0,
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8 }}>
            <span className="mono" style={{ color: 'var(--fg-1)', fontSize: 12, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {column}
            </span>
            <button onClick={() => setOpen(false)} style={plainIconBtn}>
              <Icon name="x" size={11} />
            </button>
          </div>
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search values"
            style={{
              width: '100%',
              height: 28,
              boxSizing: 'border-box',
              background: 'var(--bg-1)',
              border: '1px solid var(--line-2)',
              borderRadius: 6,
              color: 'var(--fg-1)',
              fontSize: 12,
              padding: '0 8px',
              outline: 0,
              marginBottom: 8,
            }}
          />
          <div style={{ maxHeight: 240, overflow: 'auto', display: 'grid', gap: 2 }}>
            {loading && <div style={filterMenuMsg}>Loading values…</div>}
            {error && <div style={{ ...filterMenuMsg, color: 'var(--danger)' }}>{error}</div>}
            {!loading && !error && filteredValues.length === 0 && <div style={filterMenuMsg}>No values.</div>}
            {!loading &&
              !error &&
              filteredValues.map((v, i) => {
                const checked = picked.some((p) => sameCellValue(p, v.value))
                return (
                  <label
                    key={`${formatFilterValue(v.value)}-${i}`}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 8,
                      minHeight: 26,
                      padding: '3px 4px',
                      borderRadius: 5,
                      color: 'var(--fg-2)',
                      fontSize: 12,
                      cursor: 'pointer',
                    }}
                  >
                    <input type="checkbox" checked={checked} onChange={() => toggle(v.value)} />
                    <span className="mono" style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                      {formatFilterValue(v.value)}
                    </span>
                    <span className="mono tnum" style={{ color: 'var(--fg-4)', fontSize: 10.5 }}>
                      {fmtCount(v.count)}
                    </span>
                  </label>
                )
              })}
          </div>
          {truncated && <div style={{ ...filterMenuMsg, paddingTop: 6 }}>Showing first {fmtCount(200)} values.</div>}
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 8 }}>
            <button className="btn-gh" onClick={() => setPicked([])} disabled={picked.length === 0}>
              Clear
            </button>
            <button
              className="btn-gh"
              onClick={() => {
                onApply(picked)
                setOpen(false)
              }}
            >
              Apply
            </button>
          </div>
        </div>
      )}
    </span>
  )
}

function ExportMenu({
  hasFilters,
  onExport,
  onClose,
}: {
  hasFilters: boolean
  onExport(format: 'csv' | 'json' | 'xlsx', scope: 'filtered' | 'all'): void
  onClose(): void
}) {
  const formats: Array<'csv' | 'json' | 'xlsx'> = ['csv', 'json', 'xlsx']
  return (
    <div
      onMouseLeave={onClose}
      style={{
        position: 'absolute',
        top: 32,
        right: 0,
        width: 220,
        background: 'var(--bg-2)',
        border: '1px solid var(--line-2)',
        borderRadius: 8,
        boxShadow: 'var(--shadow-pop)',
        padding: 6,
        zIndex: 50,
      }}
    >
      <div style={{ padding: '5px 7px', color: 'var(--fg-4)', fontSize: 10.5, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
        Current {hasFilters ? 'filter' : 'table'}
      </div>
      {formats.map((f) => (
        <ExportMenuButton key={'filtered-' + f} label={f.toUpperCase()} onClick={() => onExport(f, 'filtered')} />
      ))}
      <div style={{ height: 1, background: 'var(--line-1)', margin: '6px 4px' }} />
      <div style={{ padding: '5px 7px', color: 'var(--fg-4)', fontSize: 10.5, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
        Full table
      </div>
      {formats.map((f) => (
        <ExportMenuButton key={'all-' + f} label={f.toUpperCase()} onClick={() => onExport(f, 'all')} />
      ))}
    </div>
  )
}

function ExportMenuButton({ label, onClick }: { label: string; onClick(): void }) {
  return (
    <button
      onClick={onClick}
      style={{
        width: '100%',
        height: 28,
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        padding: '0 8px',
        background: 'transparent',
        border: 0,
        borderRadius: 5,
        color: 'var(--fg-1)',
        fontSize: 12,
        cursor: 'pointer',
        fontFamily: 'inherit',
        textAlign: 'left',
      }}
    >
      <Icon name="download" size={11} style={{ color: 'var(--fg-4)' }} />
      <span>{label}</span>
    </button>
  )
}

const plainIconBtn: React.CSSProperties = {
  width: 20,
  height: 20,
  border: 0,
  background: 'transparent',
  color: 'var(--fg-4)',
  display: 'grid',
  placeItems: 'center',
  cursor: 'pointer',
}

const filterMenuMsg: React.CSSProperties = {
  color: 'var(--fg-4)',
  fontSize: 12,
  padding: 8,
}

function sameCellValue(a: CellValue, b: CellValue) {
  return a === b || (a === null && b === null)
}

function formatFilterValue(v: CellValue) {
  if (v === null || v === undefined) return 'null'
  if (typeof v === 'boolean') return v ? 'true' : 'false'
  return String(v)
}

function renderCell(v: CellValue) {
  if (v === null || v === undefined)
    return <span style={{ color: 'var(--fg-5)', fontStyle: 'italic' }}>null</span>
  if (typeof v === 'boolean')
    return <span style={{ color: v ? 'var(--ok)' : 'var(--danger)' }}>{String(v)}</span>
  if (typeof v === 'number')
    return (
      <span style={{ color: 'var(--c-cyan)' }} className="tnum">
        {String(v)}
      </span>
    )
  return <span style={{ color: 'var(--fg-1)' }}>{String(v)}</span>
}

function FatalError({ msg }: { msg: string }) {
  return (
    <div
      className="mono"
      style={{
        margin: 16,
        padding: 12,
        borderRadius: 8,
        background: 'var(--danger-soft)',
        border: '1px solid rgba(255,107,122,0.3)',
        color: 'var(--danger)',
        fontSize: 12,
      }}
    >
      {msg}
    </div>
  )
}

function errMsg(err: unknown): string {
  if (err instanceof ApiError) return err.body.reason || err.body.error || `HTTP ${err.status}`
  if (err instanceof Error) return err.message
  return String(err)
}

function fmtCount(n: number): string {
  if (n < 0) return '?'
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1).replace(/\.0$/, '') + 'k'
  return String(n)
}
