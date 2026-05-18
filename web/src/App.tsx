import { useEffect, useState } from 'react'
import { BrowserRouter, Routes, Route, Navigate, Outlet } from 'react-router-dom'
import Sidebar from './components/Sidebar'
import TopBar from './components/TopBar'
import ProtectedRoute from './components/ProtectedRoute'
import LoginPage from './pages/LoginPage'
import SchemaPage from './pages/SchemaPage'
import QueryPage from './pages/QueryPage'
import DataPage from './pages/DataPage'
import ConnectionsPage from './pages/ConnectionsPage'
import ObservabilityPage from './pages/ObservabilityPage'
import DiscoverPage from './pages/DiscoverPage'
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
            <Route path="/observ" element={<ObservabilityPage />} />
            <Route path="/discover" element={<DiscoverPage />} />
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

  useEffect(() => {
    if (activeConnID === 0 && connections.length > 0) {
      setActiveConnID(connections[0].id)
    }
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
    <div className="app-bg" style={{ height: '100%', display: 'flex', overflow: 'hidden' }}>
      <Sidebar />
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        <TopBar
          connections={connections}
          activeConnID={activeConnID}
          onSelectConnection={setActiveConnID}
        />
        <div style={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
          <Outlet context={ctx} />
        </div>
      </div>
    </div>
  )
}
