import { NavLink } from 'react-router-dom'
import Icon, { type IconName } from './Icon'
import { useAuth } from '../auth/AuthContext'
import { useDiscovered } from '../api/discover'

interface NavItem {
  to: string
  label: string
  icon: IconName
  hot: string
  disabled?: boolean
  adminOnly?: boolean
}

const ITEMS: NavItem[] = [
  { to: '/',            label: 'Schema',        icon: 'schema',   hot: 'S' },
  { to: '/data',        label: 'Data',          icon: 'data',     hot: 'D' },
  { to: '/observ',      label: 'Observability', icon: 'observ',   hot: 'O' },
  { to: '/discover',    label: 'Discover',      icon: 'sparkles', hot: 'I' },
  { to: '/query',       label: 'Query',         icon: 'query',    hot: 'Q' },
  { to: '/audit',       label: 'Audit',         icon: 'audit',    hot: 'A', disabled: true },
  { to: '/connections', label: 'Connections',   icon: 'conn',     hot: 'C' },
  { to: '/users',       label: 'Users',         icon: 'user',     hot: 'U', adminOnly: true },
]

export default function Sidebar() {
  const { user, logout } = useAuth()
  const initials = (user?.email ?? '?').slice(0, 2).toUpperCase()
  const { data: discover } = useDiscovered()
  const pendingCount = discover?.entries.filter((e) => e.status === 'pending').length ?? 0
  const items = ITEMS.filter((it) => !it.adminOnly || user?.role === 'admin')

  return (
    <aside
      className="no-sel"
      style={{
        width: 220,
        minWidth: 220,
        background: 'var(--bg-1)',
        borderRight: '1px solid var(--line-1)',
        display: 'flex',
        flexDirection: 'column',
        padding: '14px 12px',
      }}
    >
      {/* Logo */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '4px 6px 18px' }}>
        <div
          style={{
            width: 26,
            height: 26,
            background: 'linear-gradient(135deg, var(--accent) 0%, #4ED6FF 100%)',
            borderRadius: 7,
            display: 'grid',
            placeItems: 'center',
            boxShadow: '0 4px 12px -2px var(--accent-glow), inset 0 0 0 1px rgba(255,255,255,0.12)',
          }}
        >
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="white" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round">
            <ellipse cx="12" cy="6" rx="7" ry="2.5"/>
            <path d="M5 6v12c0 1.4 3.1 2.5 7 2.5s7-1.1 7-2.5V6"/>
            <path d="M5 12c0 1.4 3.1 2.5 7 2.5s7-1.1 7-2.5"/>
          </svg>
        </div>
        <div>
          <div style={{ fontSize: 15, fontWeight: 600, letterSpacing: '-0.02em', lineHeight: 1 }}>dbil</div>
          <div style={{ fontSize: 10.5, color: 'var(--fg-3)', marginTop: 3, lineHeight: 1 }}>postgres workspace</div>
        </div>
      </div>

      {/* Search placeholder (command palette comes later) */}
      <button
        type="button"
        style={{
          height: 30,
          marginBottom: 16,
          padding: '0 10px',
          background: 'var(--bg-2)',
          border: '1px solid var(--line-1)',
          borderRadius: 7,
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          color: 'var(--fg-3)',
          fontSize: 12,
          width: '100%',
          cursor: 'pointer',
          fontFamily: 'inherit',
        }}
      >
        <Icon name="search" size={13} />
        <span>Search or jump…</span>
        <span style={{ flex: 1 }} />
        <span className="kbd">⌘K</span>
      </button>

      <div
        style={{
          fontSize: 10,
          color: 'var(--fg-4)',
          letterSpacing: '0.08em',
          textTransform: 'uppercase',
          padding: '0 8px 6px',
          fontWeight: 500,
        }}
      >
        Workspace
      </div>

      <nav style={{ display: 'flex', flexDirection: 'column' }}>
        {items.map((it) => (
          <NavLink
            key={it.to}
            to={it.to}
            end={it.to === '/'}
            style={({ isActive }) => ({
              height: 30,
              padding: '0 10px',
              background: isActive ? 'var(--bg-3)' : 'transparent',
              color: isActive ? 'var(--fg-1)' : it.disabled ? 'var(--fg-4)' : 'var(--fg-2)',
              border: '1px solid ' + (isActive ? 'var(--line-2)' : 'transparent'),
              borderRadius: 7,
              display: 'flex',
              alignItems: 'center',
              gap: 10,
              fontSize: 12.5,
              fontWeight: 500,
              marginBottom: 2,
              position: 'relative',
              textDecoration: 'none',
              cursor: it.disabled ? 'default' : 'pointer',
              opacity: it.disabled ? 0.55 : 1,
              pointerEvents: it.disabled ? 'none' : undefined,
            })}
          >
            {({ isActive }) => (
              <>
                {isActive && (
                  <span
                    style={{
                      position: 'absolute',
                      left: -12,
                      top: 6,
                      bottom: 6,
                      width: 2,
                      background: 'var(--accent)',
                      borderRadius: 2,
                      boxShadow: '0 0 8px var(--accent-glow)',
                    }}
                  />
                )}
                <Icon name={it.icon} size={14} style={{ color: isActive ? 'var(--accent)' : 'currentColor' }} />
                <span style={{ flex: 1, textAlign: 'left' }}>{it.label}</span>
                {it.disabled ? (
                  <span style={{ fontSize: 9.5, color: 'var(--fg-4)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
                    soon
                  </span>
                ) : it.to === '/discover' && pendingCount > 0 ? (
                  <span
                    style={{
                      minWidth: 16,
                      height: 16,
                      padding: '0 5px',
                      borderRadius: 999,
                      background: 'var(--warn-soft)',
                      color: 'var(--warn)',
                      border: '1px solid rgba(245,165,36,0.4)',
                      fontSize: 10,
                      fontWeight: 600,
                      display: 'inline-flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      fontFamily: 'var(--font-mono)',
                    }}
                  >
                    {pendingCount}
                  </span>
                ) : (
                  <span className="kbd" style={{ opacity: 0.7 }}>
                    {it.hot}
                  </span>
                )}
              </>
            )}
          </NavLink>
        ))}
      </nav>

      <div style={{ flex: 1 }} />

      {/* Audit chain status card */}
      <div
        style={{
          marginTop: 12,
          padding: 10,
          background: 'var(--bg-2)',
          border: '1px solid var(--line-1)',
          borderRadius: 8,
          fontSize: 11,
          color: 'var(--fg-3)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 6 }}>
          <span
            className="live-dot"
            style={{ width: 6, height: 6, borderRadius: 3, background: 'var(--ok)', flexShrink: 0 }}
          />
          <span style={{ color: 'var(--fg-2)', fontWeight: 500 }}>Audit chain healthy</span>
        </div>
        <div className="mono tnum" style={{ fontSize: 10.5, color: 'var(--fg-4)' }}>
          tamper-evident hash chain
        </div>
      </div>

      {/* User block */}
      <div style={{ marginTop: 10, padding: '8px 10px', display: 'flex', alignItems: 'center', gap: 8 }}>
        <div
          style={{
            width: 26,
            height: 26,
            borderRadius: '50%',
            background: 'linear-gradient(135deg, #FF8B98 0%, #C99BFF 100%)',
            display: 'grid',
            placeItems: 'center',
            fontSize: 11,
            fontWeight: 600,
            color: 'white',
          }}
        >
          {initials}
        </div>
        <div style={{ lineHeight: 1.2, flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: 12, color: 'var(--fg-1)', fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {user?.email ?? 'guest'}
          </div>
          <div style={{ fontSize: 10.5, color: 'var(--fg-4)' }}>{user?.role ?? 'visitor'} · solo mode</div>
        </div>
        <button
          className="link-btn"
          title="Sign out"
          onClick={() => void logout()}
          style={{ padding: 4 }}
        >
          <Icon name="logout" size={13} />
        </button>
      </div>
    </aside>
  )
}
