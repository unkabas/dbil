import { useState } from 'react'
import { BrowserRouter, Routes, Route, Navigate, Outlet } from 'react-router-dom'
import TopNav from './components/TopNav'
import ProtectedRoute from './components/ProtectedRoute'
import LoginPage from './pages/LoginPage'
import SchemaPage from './pages/SchemaPage'
import QueryPage from './pages/QueryPage'
import ConnectionsPage from './pages/ConnectionsPage'
import DataPage from './pages/DataPage'
import { mockConnections } from './mock/data'

export default function App() {
  // Active connection id lives in App so every page can read it from the
  // selector in TopNav. Once ConnectionsPage starts returning real data
  // (Phase D), the initial value is reseeded from the API response.
  const [activeConnID, setActiveConnID] = useState<number>(mockConnections[0].id)

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route element={<ProtectedRoute />}>
          <Route
            element={
              <AppShell
                activeConnID={activeConnID}
                onSelectConnection={setActiveConnID}
              />
            }
          >
            <Route path="/" element={<SchemaPage activeConnID={activeConnID} />} />
            <Route path="/query" element={<QueryPage activeConnID={activeConnID} />} />
            <Route path="/data" element={<DataPage activeConnID={activeConnID} />} />
            <Route
              path="/data/:schema/:name"
              element={<DataPage activeConnID={activeConnID} />}
            />
            <Route path="/connections" element={<ConnectionsPage />} />
          </Route>
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}

function AppShell({
  activeConnID,
  onSelectConnection,
}: {
  activeConnID: number
  onSelectConnection: (id: number) => void
}) {
  return (
    <div className="h-full flex flex-col">
      <TopNav activeConnID={activeConnID} onSelectConnection={onSelectConnection} />
      <main className="flex-1 min-h-0">
        <Outlet />
      </main>
    </div>
  )
}
