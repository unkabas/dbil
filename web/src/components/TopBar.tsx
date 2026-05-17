import { useEffect, useRef, useState } from 'react'
import { useLocation } from 'react-router-dom'
import type { Connection } from '../api/connections'
import Icon from './Icon'

const SCREEN_NAMES: Record<string, string> = {
  '/': 'Schema',
  '/data': 'Data',
  '/observ': 'Observability',
  '/query': 'Query',
  '/connections': 'Connections',
}

interface Props {
  connections: Connection[]
  activeConnID: number
  onSelectConnection(id: number): void
}

export default function TopBar({ connections, activeConnID, onSelectConnection }: Props) {
  const location = useLocation()
  const activeConn = connections.find((c) => c.id === activeConnID)
  const isProd = activeConn?.tag === 'production'

  // First segment of the path (best-guess) — handles /data/:schema/:name too.
  const screenKey =
    Object.keys(SCREEN_NAMES).find(
      (k) => k === location.pathname || (k !== '/' && location.pathname.startsWith(k)),
    ) ?? '/'
  const screenName = SCREEN_NAMES[screenKey]

  return (
    <div
      style={{
        height: 48,
        minHeight: 48,
        background: 'var(--bg-0)',
        borderBottom: '1px solid var(--line-1)',
        display: 'flex',
        alignItems: 'center',
        padding: '0 18px',
        gap: 14,
        position: 'relative',
      }}
    >
      {isProd && (
        <div
          aria-hidden
          style={{
            position: 'absolute',
            inset: 0,
            pointerEvents: 'none',
            boxShadow: 'inset 0 2px 0 var(--prod), inset 0 0 60px -20px var(--prod-soft)',
            zIndex: 0,
          }}
        />
      )}

      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          fontSize: 12.5,
          fontWeight: 500,
          color: 'var(--fg-2)',
        }}
      >
        <span style={{ color: 'var(--fg-4)' }}>Workspace</span>
        <Icon name="chevR" size={11} style={{ color: 'var(--fg-5)' }} />
        <span style={{ color: 'var(--fg-1)' }}>{screenName}</span>
      </div>

      <span style={{ flex: 1 }} />

      {connections.length > 0 && activeConn ? (
        <ConnectionSwitcher
          connections={connections}
          activeConn={activeConn}
          onChange={(id) => onSelectConnection(id)}
        />
      ) : (
        <span style={{ fontSize: 12, color: 'var(--fg-4)' }}>No connections registered</span>
      )}

      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 4,
          paddingLeft: 12,
          marginLeft: 6,
          borderLeft: '1px solid var(--line-1)',
        }}
      >
        <button className="btn-gh" title="What's new">
          <Icon name="sparkles" size={13} /> Updates
        </button>
        <button
          className="btn-gh"
          title="Notifications"
          style={{ width: 28, padding: 0, justifyContent: 'center' }}
        >
          <Icon name="bell" size={13} />
        </button>
      </div>
    </div>
  )
}

function ConnectionSwitcher({
  connections,
  activeConn,
  onChange,
}: {
  connections: Connection[]
  activeConn: Connection
  onChange(id: number): void
}) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    const h = (e: MouseEvent) => {
      if (ref.current && e.target instanceof Node && !ref.current.contains(e.target)) {
        setOpen(false)
      }
    }
    window.addEventListener('mousedown', h)
    return () => window.removeEventListener('mousedown', h)
  }, [])

  const dotColor =
    activeConn.tag === 'production' ? 'var(--prod)' :
    activeConn.tag === 'staging'    ? 'var(--warn)' :
    activeConn.tag === 'dev'        ? 'var(--ok)'   : 'var(--fg-3)'

  return (
    <div ref={ref} style={{ position: 'relative' }}>
      <button
        onClick={() => setOpen((o) => !o)}
        style={{
          height: 30,
          padding: '0 12px 0 10px',
          background: 'var(--bg-2)',
          border: '1px solid var(--line-2)',
          borderRadius: 8,
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          fontSize: 12.5,
          color: 'var(--fg-1)',
          minWidth: 280,
          cursor: 'pointer',
          fontFamily: 'inherit',
        }}
      >
        <span
          className={activeConn.tag === 'production' ? 'live-dot' : ''}
          style={{
            width: 7,
            height: 7,
            borderRadius: '50%',
            flexShrink: 0,
            background: dotColor,
            boxShadow: activeConn.tag === 'production' ? '0 0 8px var(--prod)' : 'none',
          }}
        />
        <span style={{ fontWeight: 500, whiteSpace: 'nowrap' }}>{activeConn.alias}</span>
        <span className="mono" style={{ color: 'var(--fg-4)', fontSize: 11, whiteSpace: 'nowrap' }}>
          {activeConn.host}:{activeConn.port}/{activeConn.tag}
        </span>
        <span style={{ flex: 1 }} />
        <Icon name="chev" size={12} style={{ color: 'var(--fg-3)' }} />
      </button>

      {open && (
        <div
          style={{
            position: 'absolute',
            top: 36,
            right: 0,
            width: 360,
            background: 'var(--bg-2)',
            border: '1px solid var(--line-2)',
            borderRadius: 10,
            boxShadow: 'var(--shadow-pop)',
            padding: 6,
            zIndex: 30,
          }}
        >
          <div
            style={{
              fontSize: 10,
              color: 'var(--fg-4)',
              letterSpacing: '0.08em',
              textTransform: 'uppercase',
              padding: '8px 10px 4px',
            }}
          >
            Connections
          </div>
          {connections.map((c) => {
            const cdot =
              c.tag === 'production' ? 'var(--prod)' :
              c.tag === 'staging'    ? 'var(--warn)' :
              c.tag === 'dev'        ? 'var(--ok)'   : 'var(--fg-3)'
            const selected = c.id === activeConn.id
            return (
              <button
                key={c.id}
                onClick={() => {
                  onChange(c.id)
                  setOpen(false)
                }}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 10,
                  width: '100%',
                  padding: '8px 10px',
                  background: selected ? 'var(--bg-3)' : 'transparent',
                  borderRadius: 6,
                  color: 'var(--fg-1)',
                  marginBottom: 2,
                  border: 0,
                  cursor: 'pointer',
                  fontFamily: 'inherit',
                  textAlign: 'left',
                }}
              >
                <span className={`tag ${c.tag}`}>
                  <span className="dot" style={{ background: cdot }} />
                  {c.tag}
                </span>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: 12.5, fontWeight: 500 }}>{c.alias}</div>
                  <div className="mono" style={{ fontSize: 10.5, color: 'var(--fg-4)' }}>
                    {c.host}:{c.port}
                  </div>
                </div>
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
