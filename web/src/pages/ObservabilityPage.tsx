import { useShellContext } from '../shell/context'
import { useOverview, useSlowQueries, useLocks } from '../api/observ'
import { ApiError } from '../api/client'
import MetricTile from '../components/observ/MetricTile'
import SlowQueriesTable from '../components/observ/SlowQueriesTable'
import LockChainCard from '../components/observ/LockChainCard'
import Icon from '../components/Icon'

export default function ObservabilityPage() {
  const { activeConn } = useShellContext()

  if (!activeConn) {
    return (
      <div style={{ height: '100%', display: 'grid', placeItems: 'center', color: 'var(--fg-3)', fontSize: 13 }}>
        Add a connection first.
      </div>
    )
  }

  return <ObservabilityBody />
}

function ObservabilityBody() {
  const { activeConn } = useShellContext()
  // Safe — caller verified activeConn is set.
  const conn = activeConn!

  const overview = useOverview(conn.id, conn.tag)
  const slow = useSlowQueries(conn.id, conn.tag)
  const locks = useLocks(conn.id)

  const samples = overview.data?.samples ?? []
  const latest = samples.length > 0 ? samples[samples.length - 1] : undefined
  const prev = samples.length > 1 ? samples[samples.length - 2] : undefined
  const fresh = !!latest && Date.now() - latest.ts_ms < 30_000

  return (
    <div className="app-bg" style={{ height: '100%', overflow: 'auto' }}>
      <div style={{ maxWidth: 1480, margin: '0 auto', padding: '18px 24px 40px' }}>
        <header
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 12,
            marginBottom: 18,
            flexWrap: 'wrap',
          }}
        >
          <h1 style={{ fontSize: 16, fontWeight: 600, letterSpacing: '-0.02em', color: 'var(--fg-1)', margin: 0 }}>
            Observability
          </h1>
          <span style={{ fontSize: 12, color: 'var(--fg-4)' }}>·</span>
          <span style={{ fontSize: 12, color: 'var(--fg-3)' }} className="mono">{conn.alias}</span>
          <span className="tnum" style={{ fontSize: 11.5, color: 'var(--fg-4)' }}>
            last 5 min · 5–60 s tick (by tag)
          </span>
          <span
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
              padding: '3px 10px 3px 8px',
              background: fresh ? 'var(--ok-soft)' : 'var(--warn-soft)',
              border: '1px solid ' + (fresh ? 'rgba(52,211,153,0.3)' : 'rgba(245,165,36,0.3)'),
              borderRadius: 999,
              color: fresh ? 'var(--ok)' : 'var(--warn)',
              fontSize: 11,
              fontWeight: 500,
            }}
          >
            <span
              className="live-dot"
              style={{ width: 6, height: 6, borderRadius: 3, background: fresh ? 'var(--ok)' : 'var(--warn)' }}
            />
            {fresh ? 'Live' : 'Stale'}
          </span>
          <span style={{ flex: 1 }} />
          <button className="btn-gh" onClick={() => overview.refetch()}>
            <Icon name="refresh" size={12} /> Refresh
          </button>
        </header>

        {overview.error && <ErrorBanner err={overview.error} />}

        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(4, 1fr)',
            gap: 14,
            marginBottom: 18,
          }}
        >
          <MetricTile
            label="TPS"
            value={latest ? fmtNum(latest.tps, 1) : '—'}
            unit="xacts/s"
            delta={deltaPct(latest?.tps, prev?.tps)}
            data={samples.map((s) => s.tps)}
            accent="var(--c-violet)"
            fresh={fresh}
          />
          <MetricTile
            label="Cache hit"
            value={latest ? (latest.cache_hit * 100).toFixed(2) : '—'}
            unit="%"
            delta={deltaPct(latest?.cache_hit, prev?.cache_hit)}
            data={samples.map((s) => s.cache_hit * 100)}
            accent="var(--c-mint)"
            fresh={fresh}
          />
          <MetricTile
            label="Sessions"
            value={latest ? String(latest.active_conns + latest.idle_conns) : '—'}
            unit={latest ? `${latest.active_conns} active` : ''}
            delta={null}
            data={samples.map((s) => s.active_conns + s.idle_conns)}
            accent="var(--c-cyan)"
            fresh={fresh}
          />
          <MetricTile
            label="Replication lag"
            value={
              latest && latest.rep_lag_ms !== undefined ? fmtNum(latest.rep_lag_ms, 0) : '—'
            }
            unit={latest && latest.rep_lag_ms !== undefined ? 'ms' : ''}
            hint={latest && latest.rep_lag_ms === undefined ? 'no replica' : undefined}
            delta={null}
            data={samples.map((s) => s.rep_lag_ms ?? 0)}
            accent="var(--c-amber)"
            fresh={fresh}
          />
        </div>

        <div style={{ marginBottom: 18 }}>
          <SlowQueriesTable
            rows={slow.data?.rows ?? []}
            takenAtMs={slow.data?.taken_at_ms ?? 0}
            loading={slow.isLoading}
          />
        </div>

        <LockChainCard
          chains={locks.data?.chains ?? []}
          loading={locks.isLoading}
          connID={conn.id}
          tag={conn.tag}
          error={
            locks.error
              ? locks.error instanceof ApiError
                ? locks.error.body.reason || locks.error.body.error || `HTTP ${locks.error.status}`
                : String(locks.error)
              : null
          }
        />
      </div>
    </div>
  )
}

function fmtNum(n: number, digits = 1): string {
  if (!Number.isFinite(n)) return '—'
  if (n >= 1000) return (n / 1000).toFixed(digits) + 'k'
  return n.toFixed(digits)
}

function deltaPct(curr?: number, prev?: number): { value: string; up: boolean } | null {
  if (curr === undefined || prev === undefined || prev === 0) return null
  const pct = ((curr - prev) / Math.abs(prev)) * 100
  if (Math.abs(pct) < 0.5) return null
  return { value: `${Math.abs(pct).toFixed(1)}%`, up: pct > 0 }
}

function ErrorBanner({ err }: { err: unknown }) {
  const msg = err instanceof ApiError ? err.body.error || `HTTP ${err.status}` : String(err)
  return (
    <div
      className="mono"
      style={{
        marginBottom: 16,
        padding: 12,
        borderRadius: 8,
        background: 'var(--danger-soft)',
        border: '1px solid rgba(255,107,122,0.3)',
        color: 'var(--danger)',
        fontSize: 12,
      }}
    >
      observability fetch failed: {msg}
    </div>
  )
}
