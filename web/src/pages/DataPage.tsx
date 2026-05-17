import { useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { findTable, mockRowsFor, tablesFor } from '../mock/data'
import StatusPill from '../components/StatusPill'
import Icon from '../components/Icon'

interface Props {
  activeConnID: number
}

export default function DataPage({ activeConnID }: Props) {
  const { schema, name } = useParams<{ schema: string; name: string }>()
  const navigate = useNavigate()
  const tables = tablesFor(activeConnID)

  // No table in URL → land on the first table for this connection.
  const effective =
    schema && name
      ? findTable(activeConnID, schema, name)
      : tables[0]

  const [filter, setFilter] = useState('')
  const [sort, setSort] = useState<{ col: number; dir: 'asc' | 'desc' } | null>(null)
  const [page, setPage] = useState(0)
  const pageSize = 25

  if (!effective) {
    return (
      <div className="h-full flex items-center justify-center text-ink-300 text-[13px]">
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

  const totalPages = Math.max(1, Math.ceil(visibleRows.length / pageSize))
  const pageRows = visibleRows.slice(page * pageSize, page * pageSize + pageSize)

  return (
    <div className="h-full overflow-auto bg-app-grad">
      <div className="max-w-[1500px] mx-auto px-6 py-6">
        {/* Breadcrumb */}
        <div className="flex items-center gap-2 text-[12.5px] mb-3">
          <button
            onClick={() => navigate('/')}
            className="text-ink-400 hover:text-ink-100 transition-colors"
          >
            Schema
          </button>
          <Icon name="chevron-right" className="w-3 h-3 text-ink-500" />
          <span className="text-ink-400 font-mono">{effective.schema}</span>
          <Icon name="chevron-right" className="w-3 h-3 text-ink-500" />
          <span className="text-ink-50 font-mono font-medium">{effective.name}</span>
        </div>

        {/* Header */}
        <header className="flex items-end justify-between mb-5">
          <div>
            <h1 className="text-[24px] font-semibold text-ink-50 tracking-tight font-mono">
              {effective.schema}.{effective.name}
            </h1>
            <div className="flex items-center gap-2 mt-1.5 text-ink-300 text-[12.5px]">
              <span>{effective.rows.toLocaleString()} rows total</span>
              <Dot />
              <span>{effective.columns.length} columns</span>
              <Dot />
              <StatusPill tone="success" size="xs">in sync</StatusPill>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <SearchInput value={filter} onChange={setFilter} />
            <IconButton title="Refresh" name="refresh" />
            <IconButton title="Export" name="plus" />
          </div>
        </header>

        {/* Table */}
        <div className="bg-ink-900/50 border border-ink-700 rounded-xl shadow-card overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full font-mono text-[12.5px] border-collapse">
              <thead className="bg-ink-800/60 backdrop-blur-sm sticky top-0 z-10">
                <tr>
                  <th className="w-12 px-3 py-2.5 text-right text-ink-400 font-normal border-b border-ink-700">#</th>
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
                        className="px-3 py-2.5 text-left border-b border-ink-700 cursor-pointer hover:bg-ink-700/40 select-none"
                      >
                        <div className="flex items-center gap-1.5">
                          <span className="text-ink-50 font-medium">{c.name}</span>
                          {c.pk && (
                            <span className="text-[9px] text-accent-gold tracking-wider">PK</span>
                          )}
                          {c.fk && (
                            <span className="text-[9px] text-accent-lilac tracking-wider">FK</span>
                          )}
                          <span className="ml-auto text-ink-400 opacity-60 group-hover:opacity-100">
                            <SortChevron state={isSorted ? sort!.dir : null} />
                          </span>
                        </div>
                        <div className="text-[10px] text-accent-lilac font-normal italic mt-0.5">
                          {c.type}
                        </div>
                      </th>
                    )
                  })}
                </tr>
              </thead>
              <tbody>
                {pageRows.map((row, i) => {
                  const absoluteIndex = page * pageSize + i
                  return (
                    <tr key={i} className="hover:bg-violet/[0.04] transition-colors">
                      <td className="px-3 py-1.5 text-right text-ink-400 border-b border-ink-800">
                        {absoluteIndex + 1}
                      </td>
                      {row.map((cell, ci) => (
                        <td
                          key={ci}
                          className="px-3 py-1.5 border-b border-ink-800 truncate max-w-[260px]"
                          title={String(cell ?? '')}
                        >
                          {renderCell(effective.columns[ci].type, cell)}
                        </td>
                      ))}
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>

          <div className="flex items-center justify-between px-4 py-2.5 border-t border-ink-700 text-[11.5px] text-ink-300">
            <div className="flex items-center gap-3">
              <span>Rows per page</span>
              <select className="appearance-none bg-ink-800 border border-ink-700 hover:border-ink-600 rounded-md px-2 py-0.5 text-ink-100" value={pageSize} disabled>
                <option>{pageSize}</option>
              </select>
            </div>
            <div className="flex items-center gap-3">
              <span>
                {visibleRows.length === 0
                  ? '0 of 0'
                  : `${page * pageSize + 1} – ${Math.min((page + 1) * pageSize, visibleRows.length)} of ${visibleRows.length}`}
              </span>
              <button
                onClick={() => setPage(0)}
                disabled={page === 0}
                className="p-1.5 rounded hover:bg-ink-800 disabled:opacity-30 disabled:hover:bg-transparent"
                title="First"
              >
                «
              </button>
              <button
                onClick={() => setPage((p) => Math.max(0, p - 1))}
                disabled={page === 0}
                className="p-1.5 rounded hover:bg-ink-800 disabled:opacity-30 disabled:hover:bg-transparent"
                title="Previous"
              >
                ‹
              </button>
              <button
                onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
                disabled={page >= totalPages - 1}
                className="p-1.5 rounded hover:bg-ink-800 disabled:opacity-30 disabled:hover:bg-transparent"
                title="Next"
              >
                ›
              </button>
              <button
                onClick={() => setPage(totalPages - 1)}
                disabled={page >= totalPages - 1}
                className="p-1.5 rounded hover:bg-ink-800 disabled:opacity-30 disabled:hover:bg-transparent"
                title="Last"
              >
                »
              </button>
            </div>
          </div>
        </div>

        {/* Other tables in this connection (quick jump) */}
        <div className="mt-6">
          <div className="text-ink-400 text-[11px] uppercase tracking-wider mb-2">
            Other tables in this connection
          </div>
          <div className="flex flex-wrap gap-2">
            {tables
              .filter((t) => !(t.schema === effective.schema && t.name === effective.name))
              .map((t) => (
                <button
                  key={`${t.schema}.${t.name}`}
                  onClick={() => navigate(`/data/${t.schema}/${t.name}`)}
                  className="px-3 py-1.5 rounded-lg bg-ink-800 border border-ink-700 hover:border-violet/40 text-[12px] font-mono text-ink-200 hover:text-ink-50 transition-colors"
                >
                  {t.schema}.{t.name}
                  <span className="ml-2 text-ink-400 text-[10px]">
                    {formatRowCount(t.rows)} rows
                  </span>
                </button>
              ))}
          </div>
        </div>
      </div>
    </div>
  )
}

function renderCell(type: string, v: unknown) {
  if (v === null || v === undefined) return <span className="text-ink-500 italic">null</span>
  if (typeof v === 'boolean')
    return (
      <span className={v ? 'text-accent-lime' : 'text-accent-coral'}>{String(v)}</span>
    )
  if (typeof v === 'number')
    return <span className="text-accent-sky tabular-nums">{String(v)}</span>
  const s = String(v)
  // status-like columns get a pill
  if (type.toLowerCase() === 'order_status' || ['placed','paid','shipped','delivered','returned'].includes(s)) {
    const tone = s === 'returned' ? 'danger' : s === 'shipped' || s === 'delivered' ? 'success' : 'info'
    return <StatusPill tone={tone} size="xs">{s}</StatusPill>
  }
  return <span className="text-ink-100">{s}</span>
}

function SortChevron({ state }: { state: 'asc' | 'desc' | null }) {
  return (
    <svg viewBox="0 0 10 14" width="8" height="11" fill="none" stroke="currentColor" strokeWidth="1.6">
      <path
        d="M5 1l3 3.5H2L5 1z"
        className={state === 'asc' ? 'text-violet' : 'text-ink-500'}
        stroke="currentColor"
        fill={state === 'asc' ? 'currentColor' : 'none'}
      />
      <path
        d="M5 13l3-3.5H2L5 13z"
        className={state === 'desc' ? 'text-violet' : 'text-ink-500'}
        stroke="currentColor"
        fill={state === 'desc' ? 'currentColor' : 'none'}
      />
    </svg>
  )
}

function SearchInput({ value, onChange }: { value: string; onChange(v: string): void }) {
  return (
    <div className="relative">
      <svg viewBox="0 0 16 16" className="w-4 h-4 absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-400" fill="none" stroke="currentColor" strokeWidth="1.6">
        <circle cx="7" cy="7" r="4.5" />
        <path d="M11 11l3 3" strokeLinecap="round" />
      </svg>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="Search rows…"
        className="w-72 h-9 pl-8 pr-3 rounded-lg bg-ink-800 border border-ink-700 focus:border-violet focus:outline-none text-[12.5px] text-ink-100 placeholder:text-ink-400"
      />
    </div>
  )
}

function IconButton({ title, name }: { title: string; name: 'refresh' | 'plus' }) {
  return (
    <button
      title={title}
      className="w-9 h-9 flex items-center justify-center rounded-lg bg-ink-800 border border-ink-700 hover:border-violet/40 text-ink-300 hover:text-ink-50"
    >
      <Icon name={name} className="w-3.5 h-3.5" />
    </button>
  )
}

function Dot() {
  return <span className="w-1 h-1 rounded-full bg-ink-600" />
}

function formatRowCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`
  return String(n)
}
