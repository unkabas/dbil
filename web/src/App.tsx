import { useState } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import TopNav from './components/TopNav'
import SchemaPage from './pages/SchemaPage'
import QueryPage from './pages/QueryPage'
import ConnectionsPage from './pages/ConnectionsPage'
import { mockConnections } from './mock/data'

export default function App() {
  const [activeConnID, setActiveConnID] = useState<number>(mockConnections[0].id)

  return (
    <BrowserRouter>
      <div className="h-full flex flex-col">
        <TopNav activeConnID={activeConnID} onSelectConnection={setActiveConnID} />
        <main className="flex-1 min-h-0">
          <Routes>
            <Route path="/" element={<SchemaPage activeConnID={activeConnID} />} />
            <Route path="/query" element={<QueryPage activeConnID={activeConnID} />} />
            <Route path="/connections" element={<ConnectionsPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  )
}
