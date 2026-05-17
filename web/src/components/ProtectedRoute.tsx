import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'

export default function ProtectedRoute() {
  const { token, user, ready } = useAuth()
  const location = useLocation()

  if (!ready) {
    return (
      <div className="h-full flex items-center justify-center text-ink-300 text-[13px]">
        <div className="flex items-center gap-3">
          <Spinner />
          <span>Restoring session…</span>
        </div>
      </div>
    )
  }
  if (!token || !user) {
    return <Navigate to="/login" state={{ from: location.pathname }} replace />
  }
  return <Outlet />
}

function Spinner() {
  return (
    <svg className="w-4 h-4 animate-spin text-violet" viewBox="0 0 16 16" fill="none">
      <circle cx="8" cy="8" r="6" stroke="currentColor" strokeOpacity="0.3" strokeWidth="2" />
      <path d="M14 8a6 6 0 0 1-6 6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  )
}
