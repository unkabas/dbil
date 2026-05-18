import { useAdvisor, type MissingIndexHint, type UnusedIndexHint } from '../../api/observ'
import { ApiError } from '../../api/client'
import Icon from '../Icon'

interface Props {
  connID: number | null
}

export default function AdvisorCard({ connID }: Props) {
  const { data, isLoading, error } = useAdvisor(connID)
  const missing = data?.missing_indexes ?? []
  const unused = data?.unused_indexes ?? []
  const total = missing.length + unused.length

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
        <Icon name="sparkles" size={14} style={{ color: 'var(--c-violet)' }} />
        <h2
          style={{
            margin: 0,
            fontSize: 13,
            fontWeight: 600,
            color: 'var(--fg-1)',
            letterSpacing: '-0.01em',
          }}
        >
          Index advisor
        </h2>
        <span className="tnum" style={{ fontSize: 11, color: 'var(--fg-4)' }}>
          {isLoading
            ? 'scanning…'
            : total === 0
              ? 'no hints'
              : `${missing.length} missing · ${unused.length} unused`}
        </span>
      </div>

      {error ? (
        <Banner>
          {error instanceof ApiError
            ? error.body.reason || error.body.error || `HTTP ${error.status}`
            : String(error)}
        </Banner>
      ) : isLoading && total === 0 ? (
        <Empty>Scanning pg_stat_user_tables…</Empty>
      ) : total === 0 ? (
        <Empty>No suggestions right now — your statistics look healthy.</Empty>
      ) : (
        <div style={{ padding: 12, display: 'flex', flexDirection: 'column', gap: 12 }}>
          {missing.length > 0 && (
            <Section title="Likely missing indexes" hint="seq_scan-heavy on a non-trivial table">
              {missing.map((m) => (
                <MissingRow key={`${m.schema}.${m.table}`} m={m} />
              ))}
            </Section>
          )}
          {unused.length > 0 && (
            <Section title="Unused indexes" hint="idx_scan = 0 since last stats reset">
              {unused.map((u) => (
                <UnusedRow key={`${u.schema}.${u.table}.${u.index}`} u={u} />
              ))}
            </Section>
          )}
        </div>
      )}
    </div>
  )
}

function Section({
  title,
  hint,
  children,
}: {
  title: string
  hint: string
  children: React.ReactNode
}) {
  return (
    <div
      style={{
        background: 'var(--bg-2)',
        border: '1px solid var(--line-1)',
        borderRadius: 8,
      }}
    >
      <div
        style={{
          padding: '8px 12px',
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          borderBottom: '1px solid var(--line-1)',
        }}
      >
        <span
          style={{
            fontSize: 10.5,
            color: 'var(--fg-3)',
            letterSpacing: '0.08em',
            textTransform: 'uppercase',
            fontWeight: 500,
          }}
        >
          {title}
        </span>
        <span style={{ fontSize: 11, color: 'var(--fg-4)' }}>{hint}</span>
      </div>
      <div>{children}</div>
    </div>
  )
}

function MissingRow({ m }: { m: MissingIndexHint }) {
  return (
    <div
      style={{
        padding: '8px 12px',
        borderTop: '1px solid var(--line-1)',
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        flexWrap: 'wrap',
      }}
    >
      <Icon name="warn" size={12} style={{ color: 'var(--c-amber)' }} />
      <span className="mono" style={{ fontSize: 12, color: 'var(--fg-1)' }}>
        <span style={{ color: 'var(--fg-4)' }}>{m.schema}.</span>
        <span style={{ fontWeight: 500 }}>{m.table}</span>
      </span>
      <span className="mono tnum" style={{ fontSize: 11, color: 'var(--fg-3)' }}>
        seq={fmtCount(m.seq_scans)} · idx={fmtCount(m.idx_scans)} · ~{fmtCount(m.seq_rows_avg)} rows/scan
      </span>
      <span style={{ flex: 1 }} />
      <span className="mono tnum" style={{ fontSize: 11, color: 'var(--fg-4)' }}>
        {fmtBytes(m.size_bytes)}
      </span>
    </div>
  )
}

function UnusedRow({ u }: { u: UnusedIndexHint }) {
  return (
    <div
      style={{
        padding: '8px 12px',
        borderTop: '1px solid var(--line-1)',
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        flexWrap: 'wrap',
      }}
    >
      <Icon name="trash" size={12} style={{ color: 'var(--fg-3)' }} />
      <span className="mono" style={{ fontSize: 12, color: 'var(--fg-1)' }}>
        <span style={{ color: 'var(--fg-4)' }}>{u.schema}.</span>
        <span>{u.table}</span>
        <span style={{ color: 'var(--fg-4)' }}> · </span>
        <span style={{ fontWeight: 500 }}>{u.index}</span>
      </span>
      {u.is_unique && (
        <span
          style={{
            fontSize: 10,
            color: 'var(--c-amber)',
            background: 'var(--warn-soft)',
            border: '1px solid rgba(245,165,36,0.3)',
            padding: '1px 6px',
            borderRadius: 999,
          }}
        >
          UNIQUE
        </span>
      )}
      <span style={{ flex: 1 }} />
      <span className="mono tnum" style={{ fontSize: 11, color: 'var(--fg-4)' }}>
        {fmtBytes(u.size_bytes)}
      </span>
    </div>
  )
}

function Banner({ children }: { children: React.ReactNode }) {
  return (
    <div
      className="mono"
      style={{
        margin: 12,
        padding: 12,
        borderRadius: 8,
        background: 'var(--danger-soft)',
        border: '1px solid rgba(255,107,122,0.3)',
        color: 'var(--danger)',
        fontSize: 12,
        whiteSpace: 'pre-wrap',
      }}
    >
      {children}
    </div>
  )
}

function Empty({ children }: { children: React.ReactNode }) {
  return (
    <div
      style={{
        padding: 24,
        textAlign: 'center',
        color: 'var(--fg-3)',
        fontSize: 12.5,
      }}
    >
      {children}
    </div>
  )
}

function fmtCount(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1).replace(/\.0$/, '') + 'k'
  return String(n)
}

function fmtBytes(b: number): string {
  if (b >= 1 << 30) return (b / (1 << 30)).toFixed(1) + ' GB'
  if (b >= 1 << 20) return (b / (1 << 20)).toFixed(1) + ' MB'
  if (b >= 1 << 10) return (b / (1 << 10)).toFixed(1) + ' KB'
  return b + ' B'
}
