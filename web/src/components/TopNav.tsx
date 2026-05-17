import { NavLink } from 'react-router-dom'
import { mockConnections } from '../mock/data'
import TagBadge from './TagBadge'
import Icon from './Icon'
import { useAuth } from '../auth/AuthContext'

interface Props {
  activeConnID: number
  onSelectConnection(id: number): void
}

export default function TopNav({ activeConnID, onSelectConnection }: Props) {
  const { user, logout } = useAuth()
  const active = mockConnections.find((c) => c.id === activeConnID) ?? mockConnections[0]

  return (
    <header className="bg-ink-900/85 backdrop-blur-sm border-b border-ink-700 select-none">
      <div className="h-14 flex items-center px-5 gap-6">
        <Logo />
        <div className="flex-1" />
        <ConnectionSelect value={active.id} onChange={onSelectConnection} />
        <TagBadge tag={active.tag} />
        <div className="flex items-center gap-1 pl-3 ml-1 border-l border-ink-700">
          <span className="text-ink-300 text-[12px]">{user?.email}</span>
          <button
            onClick={logout}
            className="ml-1 p-1.5 rounded-md hover:bg-ink-700 text-ink-300 hover:text-ink-50"
            title="Sign out"
          >
            <Icon name="logout" className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <nav className="px-5 -mt-px flex items-stretch gap-1">
        <TabLink to="/" label="Schema" />
        <TabLink to="/query" label="Query" />
        <TabLink to="/data" label="Data" />
        <TabLink to="/connections" label="Connections" />
      </nav>
    </header>
  )
}

function TabLink({ to, label }: { to: string; label: string }) {
  return (
    <NavLink
      to={to}
      end={to === '/'}
      className={({ isActive }) =>
        `relative px-4 h-10 flex items-center text-[14px] font-medium transition-colors ${
          isActive ? 'text-ink-50' : 'text-ink-300 hover:text-ink-100'
        }`
      }
    >
      {({ isActive }) => (
        <>
          <span>{label}</span>
          {isActive && (
            <span className="absolute left-3 right-3 -bottom-px h-[3px] rounded-t-full bg-gradient-to-r from-violet to-accent-lilac shadow-[0_0_12px_rgba(124,156,255,0.6)]" />
          )}
        </>
      )}
    </NavLink>
  )
}

function Logo() {
  return (
    <div className="flex items-center gap-2.5">
      <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-violet to-accent-lilac flex items-center justify-center shadow-glow">
        <svg viewBox="0 0 16 16" className="w-4 h-4 text-white" fill="currentColor">
          <path d="M8 1c3 0 5.5 1.1 5.5 2.5v9C13.5 13.9 11 15 8 15s-5.5-1.1-5.5-2.5v-9C2.5 2.1 5 1 8 1zm0 1.4c-2.2 0-4 .7-4 1.6S5.8 5.6 8 5.6s4-.7 4-1.6S10.2 2.4 8 2.4z" />
        </svg>
      </div>
      <div>
        <div className="font-semibold tracking-tight text-ink-50 text-[16px] leading-none">DBil</div>
        <div className="text-ink-400 text-[10px] leading-none mt-0.5">PostgreSQL workspace</div>
      </div>
    </div>
  )
}

function ConnectionSelect({
  value,
  onChange,
}: {
  value: number
  onChange(id: number): void
}) {
  return (
    <div className="relative">
      <select
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="appearance-none bg-ink-800 border border-ink-700 hover:border-ink-600 focus:border-violet focus:outline-none rounded-lg h-9 pl-3 pr-9 text-[13px] text-ink-50 font-medium min-w-[220px]"
      >
        {mockConnections.map((c) => (
          <option key={c.id} value={c.id}>
            {c.alias}
          </option>
        ))}
      </select>
      <Icon name="chevron-down" className="w-3.5 h-3.5 absolute right-3 top-1/2 -translate-y-1/2 text-ink-400 pointer-events-none" />
    </div>
  )
}
