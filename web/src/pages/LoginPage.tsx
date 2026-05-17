import { FormEvent, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { ApiError } from '../api/client'

interface LocationState {
  from?: string
}

export default function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const { login } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (submitting) return
    setSubmitting(true)
    setError(null)
    try {
      await login(email, password)
      const from = (location.state as LocationState | null)?.from ?? '/'
      navigate(from, { replace: true })
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 401) setError('Invalid email or password')
        else setError(err.body.error || `Login failed (${err.status})`)
      } else {
        setError(err instanceof Error ? err.message : 'Login failed')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="app-bg" style={{ height: '100%', display: 'grid', placeItems: 'center', padding: 24 }}>
      <div style={{ width: '100%', maxWidth: 400 }}>
        {/* Brand */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24, justifyContent: 'center' }}>
          <div
            style={{
              width: 34,
              height: 34,
              background: 'linear-gradient(135deg, var(--accent) 0%, #4ED6FF 100%)',
              borderRadius: 9,
              display: 'grid',
              placeItems: 'center',
              boxShadow: '0 6px 16px -2px var(--accent-glow), inset 0 0 0 1px rgba(255,255,255,0.12)',
            }}
          >
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="white" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round">
              <ellipse cx="12" cy="6" rx="7" ry="2.5" />
              <path d="M5 6v12c0 1.4 3.1 2.5 7 2.5s7-1.1 7-2.5V6" />
              <path d="M5 12c0 1.4 3.1 2.5 7 2.5s7-1.1 7-2.5" />
            </svg>
          </div>
          <div>
            <div style={{ fontSize: 18, fontWeight: 600, color: 'var(--fg-1)', letterSpacing: '-0.02em' }}>
              dbil
            </div>
            <div style={{ fontSize: 11, color: 'var(--fg-3)' }}>postgres workspace</div>
          </div>
        </div>

        <form
          onSubmit={onSubmit}
          style={{
            background: 'var(--bg-1)',
            border: '1px solid var(--line-1)',
            borderRadius: 14,
            boxShadow: 'var(--shadow-pop)',
            padding: 26,
          }}
        >
          <h1
            style={{
              fontSize: 16,
              fontWeight: 600,
              letterSpacing: '-0.02em',
              color: 'var(--fg-1)',
              margin: 0,
              marginBottom: 18,
            }}
          >
            Sign in
          </h1>

          <Field label="Email">
            <input
              type="email"
              autoComplete="username"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              style={inputStyle}
            />
          </Field>

          <Field label="Password">
            <input
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              style={inputStyle}
            />
          </Field>

          {error && (
            <div
              style={{
                marginBottom: 14,
                padding: 10,
                borderRadius: 7,
                background: 'var(--danger-soft)',
                border: '1px solid rgba(255,107,122,0.32)',
                color: 'var(--danger)',
                fontSize: 12,
              }}
            >
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={submitting}
            className="btn-pri"
            style={{ width: '100%', height: 34, justifyContent: 'center' }}
          >
            {submitting ? <Spinner /> : 'Sign in'}
          </button>

          <p
            style={{
              marginTop: 18,
              marginBottom: 0,
              fontSize: 11.5,
              color: 'var(--fg-4)',
              lineHeight: 1.55,
            }}
          >
            First run? The admin password is in{' '}
            <code className="mono" style={{ color: 'var(--fg-2)' }}>initial-credentials.txt</code>{' '}
            inside the data directory (default{' '}
            <code className="mono" style={{ color: 'var(--fg-2)' }}>/data</code> in the container).
          </p>
        </form>
      </div>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label style={{ display: 'block', marginBottom: 14 }}>
      <span
        style={{
          fontSize: 11,
          color: 'var(--fg-3)',
          fontWeight: 500,
          marginBottom: 6,
          display: 'block',
          letterSpacing: '0.02em',
        }}
      >
        {label}
      </span>
      {children}
    </label>
  )
}

const inputStyle: React.CSSProperties = {
  width: '100%',
  height: 36,
  padding: '0 12px',
  borderRadius: 7,
  background: 'var(--bg-2)',
  border: '1px solid var(--line-2)',
  color: 'var(--fg-1)',
  fontSize: 13,
  outline: 0,
  fontFamily: 'inherit',
}

function Spinner() {
  return (
    <svg className="animate-spin" width="14" height="14" viewBox="0 0 16 16" fill="none">
      <circle cx="8" cy="8" r="6" stroke="currentColor" strokeOpacity="0.3" strokeWidth="2" />
      <path d="M14 8a6 6 0 0 1-6 6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  )
}
