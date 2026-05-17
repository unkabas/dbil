import { useEffect, useState } from 'react'
import { BrowserRouter, Routes, Route, Navigate, Outlet } from 'react-router-dom'
import TopNav from './components/TopNav'
import ProtectedRoute from './components/ProtectedRoute'
import LoginPage from './pages/LoginPage'
import SchemaPage from './pages/SchemaPage'
import QueryPage from './pages/QueryPage'
import ConnectionsPage from './pages/ConnectionsPage'
import DataPage from './pages/DataPage'
import { useConnections } from './api/connections'
import type { ShellContext } from './shell/context'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route element={<ProtectedRoute />}>
          <Route element={<AppShell />}>
            <Route path="/" element={<SchemaPage />} />
            <Route path="/query" element={<QueryPage />} />
            <Route path="/data" element={<DataPage />} />
            <Route path="/data/:schema/:name" element={<DataPage />} />
            <Route path="/connections" element={<ConnectionsPage />} />
          </Route>
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}

function AppShell() {
  const { data: connections = [] } = useConnections()
  const [activeConnID, setActiveConnID] = useState<number>(0)

  // Seed active connection from the first server-returned entry once the
  // list arrives. Don't auto-clobber if the user has already picked one.
  useEffect(() => {
    if (activeConnID === 0 && connections.length > 0) {
      setActiveConnID(connections[0].id)
    }
    // If the active connection got deleted, fall back to the first remaining.
    if (activeConnID !== 0 && connections.length > 0 && !connections.some((c) => c.id === activeConnID)) {
      setActiveConnID(connections[0].id)
    }
    if (connections.length === 0 && activeConnID !== 0) {
      setActiveConnID(0)
    }
  }, [connections, activeConnID])

  const activeConn = connections.find((c) => c.id === activeConnID)
  const ctx: ShellContext = { activeConnID, activeConn, connections, setActiveConnID }

  return (
    <div className="h-full flex flex-col">
      <TopNav
        connections={connections}
        activeConnID={activeConnID}
        onSelectConnection={setActiveConnID}
      />
      <main className="flex-1 min-h-0">
        <Outlet context={ctx} />
      </main>
    </div>
  )
}
