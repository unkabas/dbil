import { createContext, useContext, useState, ReactNode } from 'react'

export interface User {
  id: number
  email: string
  role: string
  must_rotate: boolean
}

interface AuthState {
  token: string | null
  user: User | null
  ready: boolean
  login: (email: string, password: string) => Promise<void>
  logout: () => void
}

const AuthCtx = createContext<AuthState | null>(null)

// Mock mode: the frontend ships a fake admin@local session so the IDE can be
// browsed without a backend. Real login wiring lives in a later phase of
// Plan 5.
const MOCK_USER: User = {
  id: 1,
  email: 'admin@local',
  role: 'admin',
  must_rotate: false,
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(MOCK_USER)
  const [token] = useState<string | null>('mock-session')

  const login = async (email: string, _password: string) => {
    setUser({ ...MOCK_USER, email })
  }

  const logout = () => {
    setUser(null)
  }

  return (
    <AuthCtx.Provider value={{ token, user, ready: true, login, logout }}>
      {children}
    </AuthCtx.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthCtx)
  if (!ctx) throw new Error('useAuth must be used inside <AuthProvider>')
  return ctx
}
