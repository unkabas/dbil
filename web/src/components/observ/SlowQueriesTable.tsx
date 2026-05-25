import { useState } from 'react'
import type { SlowRow } from '../../api/observ'
import Icon from '../Icon'

interface Props {
  rows: SlowRow[]
  takenAtMs: number
  loading?: boolean
  hasPgStat?: boolean
  installHint?: string
}

type SortKey = 'total_ms' | 'mean_ms' | 'p95_ms' | 'p99_ms' | 'calls'

export default function SlowQueriesTable({ rows, takenAtMs, loading, hasPgStat = true, installHint }: Props) {
  const [sort, setSort] = useState<SortKey>('total_ms')
  const sorted = [...rows].sort((a, b) => b[sort] - a[sort])

  return (
    <div
      style={{
        background: 'var(--bg-1)',
        border: '1px solid var(--line-1)',
        borderRadius: 'var(--radius)',
        overflow: 'hidden',
        boxShadow: 'var(--shadow-1)',
      }}
    >
      <div
        style={{
          padding: '12px 16px',
          borderBottom: '1px solid var(--line-1)',
          display: 'flex',
          alignItems: 'center',
          gap: 10,
        }}
      >
        <h2
          style={{
            margin: 0,
            fontSize: 13,
            fontWeight: 600,
            color: 'var(--fg-1)',
            letterSpacing: '-0.01em',
          }}
        >
          Slow queries
        </h2>
        <span style={{ fontSize: 11, color: 'var(--fg-4)' }} className="tnum">
          {rows.length} query{rows.length === 1 ? '' : 's'}
          {takenAtMs > 0 && (
            <>
              {' · taken '}
              {formatAgo(takenAtMs)}
            </>
          )}
        </span>
        <span style={{ flex: 1 }} />
        <button className="btn-gh" title="Copy as SQL">
          <Icon name="copy" size={12} />
        </button>
        <button className="btn-gh">
          <Icon name="download" size={12} /> Export
        </button>
      </div>

      <div
        style={{
          padding: '6px 16px',
          color: 'var(--fg-5)',
          fontSize: 10.5,
          letterSpacing: '0.04em',
          textTransform: 'uppercase',
          fontFamily: 'var(--font-mono)',
          borderBottom: '1px solid var(--line-1)',
        }}
      >
        Source: pg_stat_statements
      </div>

      {!hasPgStat ? (
        <PgStatStatementsMissingBanner hint={installHint} />
      ) : loading && rows.length === 0 ? (
        <Empty>Loading…</Empty>
      ) : rows.length === 0 ? (
        <Empty>No queries recorded yet. Run some workload and check back in ~10 s.</Empty>
      ) : (
        <div style={{ overflowX: 'auto' }}>
          <table className="tbl" style={{ width: '100%' }}>
            <thead>
              <tr>
                <SortHeader k="total_ms" sort={sort} onSort={setSort}>Total ms</SortHeader>
                <SortHeader k="mean_ms" sort={sort} onSort={setSort}>Mean ms</SortHeader>
                <SortHeader k="p95_ms" sort={sort} onSort={setSort}>p95</SortHeader>
                <SortHeader k="p99_ms" sort={sort} onSort={setSort}>p99</SortHeader>
                <SortHeader k="calls" sort={sort} onSort={setSort}>Calls</SortHeader>
                <th style={{ textAlign: 'left' }}>Preview</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((r) => (
                <tr key={r.query_hash}>
                  <td className="tnum" style={{ color: 'var(--fg-1)' }}>{fmtMs(r.total_ms)}</td>
                  <td className="tnum">{fmtMs(r.mean_ms)}</td>
                  <td className="tnum">{fmtMs(r.p95_ms)}</td>
                  <td className="tnum">{fmtMs(r.p99_ms)}</td>
                  <td className="tnum">{fmtN(r.calls)}</td>
                  <td
                    className="mono"
                    style={{ color: 'var(--fg-1)', maxWidth: 520, overflow: 'hidden', textOverflow: 'ellipsis' }}
                    title={r.preview}
                  >
                    {r.preview}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function PgStatStatementsMissingBanner({ hint }: { hint?: string }) {
  return (
    <div style={{ padding: 16, color: 'var(--fg-3)', fontSize: 12.5 }}>
      <div style={{ fontWeight: 500, color: 'var(--warn)', marginBottom: 8 }}>
        <Icon name="warn" size={12} style={{ verticalAlign: -1, marginRight: 6 }} />
        pg_stat_statements is not installed on this database.
      </div>
      <p style={{ margin: 0, marginBottom: 10 }}>
        {hint ??
          'Add it to shared_preload_libraries and create the extension, then slow queries will start populating on the next tick.'}
      </p>
      <pre
        className="mono sql"
        style={{
          margin: 0,
          padding: 10,
          background: 'var(--bg-0)',
          border: '1px solid var(--line-1)',
          borderRadius: 6,
          fontSize: 11,
          overflowX: 'auto',
          color: 'var(--fg-2)',
        }}
      >
{`ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
-- Restart postgres, then in psql:
CREATE EXTENSION pg_stat_statements;`}
      </pre>
    </div>
  )
}

function SortHeader({
  k,
  sort,
  onSort,
  children,
}: {
  k: SortKey
  sort: SortKey
  onSort(k: SortKey): void
  children: React.ReactNode
}) {
  const active = sort === k
  return (
    <th
      onClick={() => onSort(k)}
      style={{ cursor: 'pointer', textAlign: 'left', color: active ? 'var(--fg-1)' : undefined }}
    >
      {children}
      {active && <span style={{ marginLeft: 4, color: 'var(--accent)' }}>↓</span>}
    </th>
  )
}

function Empty({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ padding: 24, textAlign: 'center', color: 'var(--fg-3)', fontSize: 12.5 }}>
      {children}
    </div>
  )
}

function fmtMs(v: number): string {
  if (v < 1) return v.toFixed(2)
  if (v < 1000) return v.toFixed(0)
  return (v / 1000).toFixed(1) + 'k'
}

function fmtN(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'k'
  return String(n)
}

function formatAgo(ts: number): string {
  const diff = Math.max(0, Date.now() - ts)
  if (diff < 1000) return 'just now'
  if (diff < 60_000) return `${Math.round(diff / 1000)}s ago`
  return `${Math.round(diff / 60_000)}m ago`
}
