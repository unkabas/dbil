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

  const linkClass = ({ isActive }: { isActive: boolean }) =>
    `relative px-3 h-9 flex items-center text-[13px] font-medium transition-colors ${
      isActive ? 'text-ink-50' : 'text-ink-300 hover:text-ink-100'
    }`

  return (
    <header className="h-12 border-b border-ink-700 bg-ink-900/80 backdrop-blur-sm flex items-center px-4 gap-4 select-none">
      <div className="flex items-center gap-2 pr-3 border-r border-ink-700 mr-1">
        <Logo />
      </div>

      <nav className="flex items-center gap-1 h-12 -my-3">
        <NavItem to="/" label="Schema" />
        <NavItem to="/query" label="Query" />
        <NavItem to="/connections" label="Connections" />
      </nav>

      <div className="ml-auto flex items-center gap-3">
        <ConnectionSelect value={active.id} onChange={onSelectConnection} />
        <TagBadge tag={active.tag} />
        <div className="flex items-center gap-1 pl-2 border-l border-ink-700">
          <span className="text-ink-300 text-xxs">{user?.email}</span>
          <button
            onClick={logout}
            className="ml-1 p-1.5 rounded-md hover:bg-ink-700 text-ink-300 hover:text-ink-50"
            title="Sign out"
          >
            <Icon name="logout" className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </header>
  )

  function NavItem({ to, label }: { to: string; label: string }) {
    return (
      <NavLink to={to} end className={linkClass}>
        {({ isActive }) => (
          <>
            <span>{label}</span>
            {isActive && (
              <span className="absolute left-3 right-3 -bottom-px h-0.5 bg-violet rounded-full" />
            )}
          </>
        )}
      </NavLink>
    )
  }
}

function Logo() {
  return (
    <div className="flex items-center gap-2">
      <div className="w-6 h-6 rounded-md bg-gradient-to-br from-violet to-accent-lilac flex items-center justify-center shadow-glow">
        <svg viewBox="0 0 16 16" className="w-3.5 h-3.5 text-white" fill="currentColor">
          <path d="M8 1c3 0 5.5 1.1 5.5 2.5v9C13.5 13.9 11 15 8 15s-5.5-1.1-5.5-2.5v-9C2.5 2.1 5 1 8 1zm0 1.4c-2.2 0-4 .7-4 1.6S5.8 5.6 8 5.6s4-.7 4-1.6S10.2 2.4 8 2.4z" />
        </svg>
      </div>
      <span className="font-semibold tracking-tight text-ink-50 text-[15px]">DBil</span>
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
        className="appearance-none bg-ink-800 border border-ink-700 hover:border-ink-600 focus:border-violet focus:outline-none rounded-md h-8 pl-3 pr-8 text-[12.5px] text-ink-50 font-medium"
      >
        {mockConnections.map((c) => (
          <option key={c.id} value={c.id}>
            {c.alias}
          </option>
        ))}
      </select>
      <Icon name="chevron-down" className="w-3.5 h-3.5 absolute right-2 top-1/2 -translate-y-1/2 text-ink-400 pointer-events-none" />
    </div>
  )
}
