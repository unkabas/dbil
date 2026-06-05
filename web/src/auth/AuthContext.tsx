import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import { ApiError, apiFetch, getStoredToken, setStoredToken } from '../api/client'

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
  logout: () => Promise<void>
  refresh: () => Promise<void>
}

const AuthCtx = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setTokenState] = useState<string | null>(() => getStoredToken())
  const [user, setUser] = useState<User | null>(null)
  const [ready, setReady] = useState(false)

  const setToken = (t: string | null) => {
    setStoredToken(t)
    setTokenState(t)
  }

  // On mount: if we have a stored token, validate it via /api/me.
  useEffect(() => {
    let cancelled = false
    if (!token) {
      setReady(true)
      setUser(null)
      return
    }
    apiFetch<User>('/api/me')
      .then((u) => {
        if (!cancelled) setUser(u)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        // Invalid / expired token: drop it.
        if (err instanceof ApiError && err.status === 401) {
          setToken(null)
        }
        setUser(null)
      })
      .finally(() => {
        if (!cancelled) setReady(true)
      })
    return () => {
      cancelled = true
    }
  }, [token])

  const login = async (email: string, password: string) => {
    const r = await apiFetch<{ token: string; expires_at: number }>(
      '/api/auth/login',
      { unauthed: true, body: { email, password } },
    )
    setToken(r.token)
    const me = await apiFetch<User>('/api/me')
    setUser(me)
  }

  // refresh re-reads the current user (e.g. after a forced password rotation
  // clears must_rotate).
  const refresh = async () => {
    const me = await apiFetch<User>('/api/me')
    setUser(me)
  }

  const logout = async () => {
    if (token) {
      try {
        await apiFetch<void>('/api/auth/logout', { method: 'POST' })
      } catch {
        // Best-effort: even if logout call fails, drop client-side state.
      }
    }
    setToken(null)
    setUser(null)
  }

  return (
    <AuthCtx.Provider value={{ token, user, ready, login, logout, refresh }}>{children}</AuthCtx.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthCtx)
  if (!ctx) throw new Error('useAuth must be used inside <AuthProvider>')
  return ctx
}
