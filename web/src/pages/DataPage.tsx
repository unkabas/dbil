import { useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { findTable, mockRowsFor, tablesFor, type MockTable } from '../mock/data'
import { useShellContext } from '../shell/context'
import Icon from '../components/Icon'

const PAGE_SIZE = 25

export default function DataPage() {
  const { activeConnID, activeConn } = useShellContext()
  const { schema, name } = useParams<{ schema: string; name: string }>()
  const navigate = useNavigate()
  const tables = tablesFor(activeConnID)

  const effective: MockTable | undefined =
    schema && name ? findTable(activeConnID, schema, name) : tables[0]

  const [filter, setFilter] = useState('')
  const [sort, setSort] = useState<{ col: number; dir: 'asc' | 'desc' } | null>(null)
  const [page, setPage] = useState(0)
  const [showPicker, setShowPicker] = useState(false)

  if (!effective) {
    return (
      <div style={{ height: '100%', display: 'grid', placeItems: 'center', color: 'var(--fg-3)', fontSize: 13 }}>
        No tables in this connection.
      </div>
    )
  }

  const allRows = useMemo(() => mockRowsFor(effective, 220), [effective])

  const visibleRows = useMemo(() => {
    let rows = allRows
    if (filter.trim()) {
      const f = filter.toLowerCase()
      rows = rows.filter((r) => r.some((v) => String(v ?? '').toLowerCase().includes(f)))
    }
    if (sort) {
      const dir = sort.dir === 'asc' ? 1 : -1
      rows = [...rows].sort((a, b) => {
        const av = a[sort.col]
        const bv = b[sort.col]
        if (av === bv) return 0
        if (av === null || av === undefined) return 1
        if (bv === null || bv === undefined) return -1
        return av > bv ? dir : -dir
      })
    }
    return rows
  }, [allRows, filter, sort])

  const totalPages = Math.max(1, Math.ceil(visibleRows.length / PAGE_SIZE))
  const pageRows = visibleRows.slice(page * PAGE_SIZE, page * PAGE_SIZE + PAGE_SIZE)

  return (
    <div
      style={{
        display: 'grid',
        gridTemplateRows: '44px 48px 1fr 36px',
        height: '100%',
        minHeight: 0,
      }}
    >
      {/* Top title */}
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
        <button className="btn-gh" title="Refresh">
          <Icon name="refresh" size={12} />
        </button>
        <button className="btn-gh">
          <Icon name="download" size={12} /> Export
        </button>
        <button className="btn-gh">
          <Icon name="plus" size={12} /> Insert row
        </button>
      </div>

      {/* Sub bar: table picker + filter */}
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
          tables={tables}
          value={`${effective.schema}.${effective.name}`}
          onChange={(k) => {
            const [s, n] = k.split('.')
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
            onChange={(e) => {
              setFilter(e.target.value)
              setPage(0)
            }}
            placeholder="Quick filter (any column contains…)"
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
            <button onClick={() => setFilter('')} style={{ color: 'var(--fg-4)', background: 'none', border: 0, cursor: 'pointer' }}>
              <Icon name="x" size={11} />
            </button>
          )}
        </div>

        <span style={{ flex: 1 }} />
        <span className="mono tnum" style={{ fontSize: 11, color: 'var(--fg-4)' }}>
          {visibleRows.length === allRows.length
            ? `${allRows.length} rows`
            : `${visibleRows.length} of ${allRows.length} rows`}
        </span>
      </div>

      {/* Table */}
      <div style={{ overflow: 'auto', background: 'var(--bg-0)' }}>
        <table className="tbl" style={{ width: '100%', borderCollapse: 'collapse', fontFamily: 'var(--font-mono)', fontSize: 12 }}>
          <thead>
            <tr>
              <th style={thBase}>#</th>
              {effective.columns.map((c, ci) => {
                const isSorted = sort?.col === ci
                return (
                  <th
                    key={c.name}
                    onClick={() =>
                      setSort((prev) =>
                        prev?.col === ci
                          ? prev.dir === 'asc'
                            ? { col: ci, dir: 'desc' }
                            : null
                          : { col: ci, dir: 'asc' },
                      )
                    }
                    style={{ ...thBase, cursor: 'pointer', textAlign: 'left' }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                      <span style={{ color: 'var(--fg-1)' }}>{c.name}</span>
                      {c.pk && (
                        <span style={{ color: 'var(--c-amber)', fontSize: 9, letterSpacing: '0.05em' }}>
                          PK
                        </span>
                      )}
                      {c.fk && (
                        <span style={{ color: 'var(--c-cyan)', fontSize: 9, letterSpacing: '0.05em' }}>
                          FK
                        </span>
                      )}
                      <span style={{ marginLeft: 'auto' }}>
                        <SortChevron state={isSorted ? sort!.dir : null} />
                      </span>
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
                      {c.type}
                    </div>
                  </th>
                )
              })}
            </tr>
          </thead>
          <tbody>
            {pageRows.map((row, i) => {
              const idx = page * PAGE_SIZE + i
              return (
                <tr key={i} style={trBase}>
                  <td style={{ ...tdBase, textAlign: 'right', color: 'var(--fg-4)' }} className="tnum">
                    {idx + 1}
                  </td>
                  {row.map((cell, ci) => (
                    <td key={ci} style={tdBase} title={String(cell ?? '')}>
                      {renderCell(effective.columns[ci].type, cell)}
                    </td>
                  ))}
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>

      {/* Pagination strip */}
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
          Showing{' '}
          <span className="tnum mono" style={{ color: 'var(--fg-1)' }}>
            {visibleRows.length === 0 ? 0 : page * PAGE_SIZE + 1}–{Math.min((page + 1) * PAGE_SIZE, visibleRows.length)}
          </span>{' '}
          of{' '}
          <span className="tnum mono" style={{ color: 'var(--fg-1)' }}>{visibleRows.length}</span>
        </span>
        <span style={{ flex: 1 }} />
        <PageBtn disabled={page === 0} onClick={() => setPage(0)} title="First">
          «
        </PageBtn>
        <PageBtn disabled={page === 0} onClick={() => setPage((p) => Math.max(0, p - 1))}>
          ‹
        </PageBtn>
        <span className="tnum mono" style={{ color: 'var(--fg-2)' }}>
          page {page + 1}/{totalPages}
        </span>
        <PageBtn
          disabled={page >= totalPages - 1}
          onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
        >
          ›
        </PageBtn>
        <PageBtn disabled={page >= totalPages - 1} onClick={() => setPage(totalPages - 1)} title="Last">
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

const trBase: React.CSSProperties = {
  // hover handled inline; CSS rule would need a sibling class
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
      onMouseEnter={(e) => {
        if (!disabled) e.currentTarget.style.background = 'var(--bg-3)'
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.background = 'transparent'
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
  tables: MockTable[]
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
            width: 280,
            background: 'var(--bg-2)',
            border: '1px solid var(--line-2)',
            borderRadius: 10,
            boxShadow: 'var(--shadow-pop)',
            padding: 6,
            zIndex: 30,
            maxHeight: 360,
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

function renderCell(type: string, v: unknown) {
  if (v === null || v === undefined) return <span style={{ color: 'var(--fg-5)', fontStyle: 'italic' }}>null</span>
  if (typeof v === 'boolean') return <span style={{ color: v ? 'var(--ok)' : 'var(--danger)' }}>{String(v)}</span>
  if (typeof v === 'number') return <span style={{ color: 'var(--c-cyan)' }} className="tnum">{String(v)}</span>
  const s = String(v)
  if (type === 'user_tier' || ['free', 'pro', 'team'].includes(s)) {
    const tone =
      s === 'pro' ? 'var(--c-amber)' :
      s === 'team' ? 'var(--c-violet)' : 'var(--fg-3)'
    return (
      <span
        style={{
          display: 'inline-block',
          padding: '0 6px',
          borderRadius: 4,
          fontSize: 11,
          color: tone,
          background: 'var(--bg-3)',
          fontFamily: 'var(--font-mono)',
        }}
      >
        {s}
      </span>
    )
  }
  if (type === 'order_status' || ['placed', 'paid', 'shipped', 'delivered', 'returned'].includes(s)) {
    const tone =
      s === 'returned' ? 'var(--danger)' :
      s === 'shipped' || s === 'delivered' ? 'var(--ok)' :
      'var(--c-cyan)'
    return <span style={{ color: tone }}>{s}</span>
  }
  return <span style={{ color: 'var(--fg-1)' }}>{s}</span>
}

function SortChevron({ state }: { state: 'asc' | 'desc' | null }) {
  return (
    <svg viewBox="0 0 10 14" width="8" height="11" fill="none" stroke="currentColor" strokeWidth="1.6">
      <path
        d="M5 1l3 3.5H2L5 1z"
        stroke="currentColor"
        fill={state === 'asc' ? 'currentColor' : 'none'}
        style={{ color: state === 'asc' ? 'var(--accent)' : 'var(--fg-5)' }}
      />
      <path
        d="M5 13l3-3.5H2L5 13z"
        stroke="currentColor"
        fill={state === 'desc' ? 'currentColor' : 'none'}
        style={{ color: state === 'desc' ? 'var(--accent)' : 'var(--fg-5)' }}
      />
    </svg>
  )
}

function fmtCount(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1).replace(/\.0$/, '') + 'k'
  return String(n)
}
