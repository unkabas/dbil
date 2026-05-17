import { useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useShellContext } from '../shell/context'
import {
  useSchema,
  useTableRows,
  type SchemaTable,
  type CellValue,
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

  const [filter, setFilter] = useState('')
  const [page, setPage] = useState(0)
  const [showPicker, setShowPicker] = useState(false)

  const rowsQuery = useTableRows(
    activeConnID,
    effective?.schema ?? null,
    effective?.name ?? null,
    page,
    PAGE_SIZE,
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

  const visibleRows = useMemo(() => {
    if (!filter.trim()) return rows
    const f = filter.toLowerCase()
    return rows.filter((r) => r.some((v) => String(v ?? '').toLowerCase().includes(f)))
  }, [rows, filter])

  const totalPages =
    estimated > 0 ? Math.max(1, Math.ceil(estimated / PAGE_SIZE)) : Math.max(1, page + (rows.length === PAGE_SIZE ? 2 : 1))

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
            maxWidth: 360,
          }}
        >
          <Icon name="filter" size={12} style={{ color: 'var(--fg-4)' }} />
          <input
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter visible page (any column contains…)"
            style={{
              flex: 1,
              height: 26,
              background: 'transparent',
              border: 0,
              outline: 0,
              color: 'var(--fg-1)',
              fontSize: 12,
              fontFamily: 'inherit',
            }}
          />
          {filter && (
            <button
              onClick={() => setFilter('')}
              style={{ color: 'var(--fg-4)', background: 'none', border: 0, cursor: 'pointer' }}
            >
              <Icon name="x" size={11} />
            </button>
          )}
        </div>

        <span style={{ flex: 1 }} />
        <span className="mono tnum" style={{ fontSize: 11, color: 'var(--fg-4)' }}>
          {rowsQuery.isLoading
            ? 'loading…'
            : filter
              ? `${visibleRows.length} of ${rows.length} on page · ~${fmtCount(estimated)} total`
              : `${rows.length} rows · ~${fmtCount(estimated)} total`}
        </span>
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
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <span style={{ color: 'var(--fg-1)' }}>{c.name}</span>
                        {meta?.pk && (
                          <span style={{ color: 'var(--c-amber)', fontSize: 9, letterSpacing: '0.05em' }}>PK</span>
                        )}
                        {meta?.fk && (
                          <span style={{ color: 'var(--c-cyan)', fontSize: 9, letterSpacing: '0.05em' }}>FK</span>
                        )}
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
              {visibleRows.map((row, i) => {
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
              {!rowsQuery.isLoading && visibleRows.length === 0 && (
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
