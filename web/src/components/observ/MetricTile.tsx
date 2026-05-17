import Sparkline from './Sparkline'

interface Props {
  label: string
  value: string
  unit?: string
  delta?: { value: string; up: boolean } | null
  data: number[]
  accent?: string
  fresh?: boolean
  hint?: string
}

export default function MetricTile({ label, value, unit, delta, data, accent = 'var(--accent)', fresh, hint }: Props) {
  return (
    <div
      style={{
        background: 'var(--bg-1)',
        border: '1px solid var(--line-1)',
        borderRadius: 'var(--radius)',
        padding: 16,
        boxShadow: 'var(--shadow-1)',
        display: 'flex',
        flexDirection: 'column',
        gap: 10,
        minWidth: 0,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <span
          style={{
            fontSize: 11,
            color: 'var(--fg-3)',
            letterSpacing: '0.06em',
            textTransform: 'uppercase',
            fontWeight: 500,
            whiteSpace: 'nowrap',
          }}
        >
          {label}
        </span>
        {fresh && (
          <span
            className="live-dot"
            style={{ width: 6, height: 6, borderRadius: 3, background: accent }}
          />
        )}
        {hint && (
          <span style={{ marginLeft: 'auto', fontSize: 10.5, color: 'var(--fg-4)' }}>{hint}</span>
        )}
      </div>

      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
        <span
          className="mono tnum"
          style={{
            fontSize: 28,
            fontWeight: 600,
            letterSpacing: '-0.02em',
            color: 'var(--fg-1)',
            lineHeight: 1,
          }}
        >
          {value}
        </span>
        {unit && <span style={{ fontSize: 12, color: 'var(--fg-3)' }}>{unit}</span>}
        {delta && (
          <span
            className="mono tnum"
            style={{
              marginLeft: 'auto',
              fontSize: 11,
              color: delta.up ? 'var(--ok)' : 'var(--danger)',
            }}
          >
            {delta.up ? '↑' : '↓'} {delta.value}
          </span>
        )}
      </div>

      <Sparkline data={data} color={accent} />
    </div>
  )
}
