import { useState } from 'react'
import Sparkline from './Sparkline'
import Icon from '../Icon'

interface Props {
  label: string
  value: string
  unit?: string
  delta?: { value: string; up: boolean } | null
  data: number[]
  accent?: string
  fresh?: boolean
  hint?: string
  tooltip?: string
  source?: string
  status?: 'ok' | 'empty' | 'unavailable'
  statusReason?: string
}

export default function MetricTile({
  label,
  value,
  unit,
  delta,
  data,
  accent = 'var(--accent)',
  fresh,
  hint,
  tooltip,
  source,
  status = 'ok',
  statusReason,
}: Props) {
  const [showTooltip, setShowTooltip] = useState(false)
  const unavailable = status === 'unavailable'
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
        opacity: unavailable ? 0.85 : 1,
        position: 'relative',
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
        {tooltip && (
          <span
            onMouseEnter={() => setShowTooltip(true)}
            onMouseLeave={() => setShowTooltip(false)}
            style={{
              display: 'inline-grid',
              placeItems: 'center',
              width: 14,
              height: 14,
              borderRadius: 7,
              color: 'var(--fg-4)',
              cursor: 'help',
              fontSize: 9,
              border: '1px solid var(--line-2)',
              fontFamily: 'var(--font-mono)',
              userSelect: 'none',
            }}
            aria-label="What is this?"
          >
            i
          </span>
        )}
        {fresh && !unavailable && (
          <span
            className="live-dot"
            style={{ width: 6, height: 6, borderRadius: 3, background: accent }}
          />
        )}
        {hint && (
          <span style={{ marginLeft: 'auto', fontSize: 10.5, color: 'var(--fg-4)' }}>{hint}</span>
        )}
      </div>

      {showTooltip && tooltip && (
        <div
          role="tooltip"
          style={{
            position: 'absolute',
            top: 36,
            left: 16,
            right: 16,
            zIndex: 20,
            background: 'var(--bg-2)',
            border: '1px solid var(--line-2)',
            borderRadius: 8,
            boxShadow: 'var(--shadow-pop)',
            padding: 10,
            fontSize: 11.5,
            color: 'var(--fg-2)',
            lineHeight: 1.4,
          }}
        >
          {tooltip}
          {source && (
            <div
              className="mono"
              style={{ marginTop: 6, color: 'var(--fg-4)', fontSize: 10.5 }}
            >
              Source: {source}
            </div>
          )}
        </div>
      )}

      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
        <span
          className="mono tnum"
          style={{
            fontSize: 28,
            fontWeight: 600,
            letterSpacing: '-0.02em',
            color: unavailable ? 'var(--fg-4)' : 'var(--fg-1)',
            lineHeight: 1,
          }}
        >
          {value}
        </span>
        {unit && !unavailable && <span style={{ fontSize: 12, color: 'var(--fg-3)' }}>{unit}</span>}
        {delta && !unavailable && (
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

      {unavailable && statusReason ? (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            fontSize: 11,
            color: 'var(--fg-4)',
            minHeight: 22,
          }}
        >
          <Icon name="warn" size={11} />
          <span>{statusReason}</span>
        </div>
      ) : (
        <Sparkline data={data} color={accent} />
      )}

      {source && !showTooltip && (
        <div
          className="mono"
          style={{
            fontSize: 10,
            color: 'var(--fg-5)',
            letterSpacing: '0.02em',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
          title={source}
        >
          {source}
        </div>
      )}
    </div>
  )
}
