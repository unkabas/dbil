import { useOutletContext } from 'react-router-dom'
import type { Connection } from '../api/connections'

export interface ShellContext {
  activeConnID: number
  activeConn: Connection | undefined
  connections: Connection[]
  setActiveConnID: (id: number) => void
}

// useShellContext exposes the active-connection state pages read on render.
// Provided by the <AppShell> route element via React Router's Outlet context.
export function useShellContext() {
  return useOutletContext<ShellContext>()
}
