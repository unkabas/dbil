import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useShellContext } from '../shell/context'
import {
  useSchema,
  useTableRows,
  fetchDistinctValues,
  exportTable,
  type SchemaTable,
  type CellValue,
  type TableFilter,
  type DistinctValue,
} from '../api/schema'
import { ApiError } from '../api/client'
import Icon from '../components/Icon'

const PAGE_SIZE = 50

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

  const [page, setPage] = useState(0)
  const [showPicker, setShowPicker] = useState(false)
  const [filters, setFilters] = useState<TableFilter[]>([])
  const [filterColumn, setFilterColumn] = useState<string | null>(null)
  const [showExport, setShowExport] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)

  const rowsQuery = useTableRows(
    activeConnID,
    effective?.schema ?? null,
    effective?.name ?? null,
    page,
    PAGE_SIZE,
    filters,
  )

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

  const rows: CellValue[][] = rowsQuery.data?.rows ?? []
  const cols = rowsQuery.data?.columns ?? effective.columns.map((c) => ({ name: c.name, type_name: c.type }))
  const estimated = rowsQuery.data?.estimated_total ?? effective.rows
  const activeFilters = filters.filter((f) => f.values.length > 0)

  const totalPages =
    estimated > 0 ? Math.max(1, Math.ceil(estimated / PAGE_SIZE)) : Math.max(1, page + (rows.length === PAGE_SIZE ? 2 : 1))

  const applyColumnFilter = (column: string, values: CellValue[]) => {
    setPage(0)
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

  return (
    <div
      style={{
        display: 'grid',
        gridTemplateRows: '44px 48px 1fr 36px',
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
            setPage(0)
            setFilters([])
            navigate(`/data/${s}/${n}`)
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
              onClick={() => {
                setPage(0)
                setFilters([])
              }}
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
            : `${rows.length} rows · ${activeFilters.length > 0 ? fmtCount(estimated) + ' filtered' : '~' + fmtCount(estimated) + ' total'}`}
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

      <div style={{ overflow: 'auto', background: 'var(--bg-0)' }}>
        {rowsQuery.error ? (
          <div style={{ padding: 24 }}>
            <FatalError msg={'rows: ' + errMsg(rowsQuery.error)} />
          </div>
        ) : (
          <table
            className="tbl"
            style={{
              width: '100%',
              borderCollapse: 'collapse',
              fontFamily: 'var(--font-mono)',
              fontSize: 12,
            }}
          >
            <thead>
              <tr>
                <th style={thBase}>#</th>
                {cols.map((c) => {
                  const meta = effective.columns.find((ec) => ec.name === c.name)
                  return (
                    <th key={c.name} style={{ ...thBase, textAlign: 'left' }}>
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
                    </th>
                  )
                })}
              </tr>
            </thead>
            <tbody>
              {rows.map((row, i) => {
                const idx = page * PAGE_SIZE + i
                return (
                  <tr key={i}>
                    <td style={{ ...tdBase, textAlign: 'right', color: 'var(--fg-4)' }} className="tnum">
                      {idx + 1}
                    </td>
                    {row.map((cell, ci) => (
                      <td key={ci} style={tdBase} title={String(cell ?? '')}>
                        {renderCell(cell)}
                      </td>
                    ))}
                  </tr>
                )
              })}
              {!rowsQuery.isLoading && rows.length === 0 && (
                <tr>
                  <td
                    colSpan={cols.length + 1}
                    style={{ ...tdBase, textAlign: 'center', color: 'var(--fg-4)', padding: 28 }}
                  >
                    No rows on this page.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        )}
      </div>

      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          padding: '0 18px',
          borderTop: '1px solid var(--line-1)',
          background: 'var(--bg-1)',
          fontSize: 11.5,
          color: 'var(--fg-3)',
          gap: 14,
        }}
      >
        <span>
          Page{' '}
          <span className="tnum mono" style={{ color: 'var(--fg-1)' }}>
            {page + 1}
          </span>
          {estimated > 0 && (
            <>
              {' '}
              of{' '}
              <span className="tnum mono" style={{ color: 'var(--fg-1)' }}>
                {totalPages}
              </span>
            </>
          )}
        </span>
        <span style={{ flex: 1 }} />
        <PageBtn disabled={page === 0} onClick={() => setPage(0)} title="First">
          «
        </PageBtn>
        <PageBtn disabled={page === 0} onClick={() => setPage((p) => Math.max(0, p - 1))}>
          ‹
        </PageBtn>
        <PageBtn
          disabled={rows.length < PAGE_SIZE}
          onClick={() => setPage((p) => p + 1)}
        >
          ›
        </PageBtn>
        <PageBtn
          disabled={rows.length < PAGE_SIZE || estimated <= 0}
          onClick={() => setPage(totalPages - 1)}
          title="Last"
        >
          »
        </PageBtn>
        <span style={{ marginLeft: 12, color: 'var(--fg-4)' }}>{activeConn?.alias ?? '—'}</span>
      </div>
    </div>
  )
}

const thBase: React.CSSProperties = {
  position: 'sticky',
  top: 0,
  zIndex: 2,
  background: 'var(--bg-1)',
  color: 'var(--fg-3)',
  fontWeight: 500,
  textAlign: 'left',
  fontSize: 11,
  letterSpacing: '0.04em',
  textTransform: 'uppercase',
  padding: '8px 12px',
  borderBottom: '1px solid var(--line-1)',
  whiteSpace: 'nowrap',
}

const tdBase: React.CSSProperties = {
  padding: '0 12px',
  height: 'var(--row-h, 30px)',
  borderBottom: '1px solid var(--line-1)',
  color: 'var(--fg-2)',
  whiteSpace: 'nowrap',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  maxWidth: 260,
}

function PageBtn({
  children,
  onClick,
  disabled,
  title,
}: {
  children: React.ReactNode
  onClick(): void
  disabled?: boolean
  title?: string
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      title={title}
      style={{
        width: 22,
        height: 22,
        borderRadius: 5,
        background: 'transparent',
        border: 0,
        color: 'var(--fg-3)',
        cursor: disabled ? 'default' : 'pointer',
        opacity: disabled ? 0.35 : 1,
        fontFamily: 'inherit',
        fontSize: 13,
      }}
    >
      {children}
    </button>
  )
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
