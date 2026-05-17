// Placeholder until Plan 6 ships real pg_stat_* collectors. The design's
// observability layout (metric tiles + slow queries + lock chains + index
// advisor) is implemented there.

import Icon from '../components/Icon'

export default function ObservabilityPage() {
  return (
    <div className="app-bg" style={{ height: '100%', overflow: 'auto' }}>
      <div style={{ maxWidth: 760, margin: '0 auto', padding: '60px 24px' }}>
        <div
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 8,
            padding: '4px 10px',
            background: 'var(--accent-mute)',
            color: 'var(--accent-soft)',
            borderRadius: 999,
            fontSize: 11,
            fontWeight: 500,
            letterSpacing: '0.06em',
            textTransform: 'uppercase',
            marginBottom: 14,
          }}
        >
          <Icon name="sparkles" size={11} />
          Coming in Plan 6
        </div>
        <h1 style={{ fontSize: 28, fontWeight: 600, letterSpacing: '-0.02em', margin: 0, color: 'var(--fg-1)' }}>
          Observability
        </h1>
        <p style={{ marginTop: 12, fontSize: 14, lineHeight: 1.6, color: 'var(--fg-2)' }}>
          The wow feature. Real-time TPS, cache hit ratio, replication lag, lock
          waiting chains with a one-click kill on the blocker, slow query rankings
          out of <code className="mono" style={{ color: 'var(--fg-1)' }}>pg_stat_statements</code> with regression
          flags, and an index advisor that surfaces missing / unused / duplicate
          indexes alongside a ready-to-copy <code className="mono" style={{ color: 'var(--fg-1)' }}>CREATE INDEX
          CONCURRENTLY</code> statement.
        </p>
        <p style={{ marginTop: 14, fontSize: 13, color: 'var(--fg-3)' }}>
          Backend collectors land in Plan 6. The chrome is already wired — once
          the API ships, this page lights up automatically.
        </p>
      </div>
    </div>
  )
}
