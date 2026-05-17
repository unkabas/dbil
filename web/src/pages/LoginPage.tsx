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
    <div className="h-full flex items-center justify-center bg-app-grad p-6">
      <div className="w-full max-w-sm">
        <div className="flex flex-col items-center mb-7">
          <div className="w-12 h-12 rounded-2xl bg-gradient-to-br from-violet to-accent-lilac flex items-center justify-center shadow-glow mb-3">
            <svg viewBox="0 0 16 16" className="w-6 h-6 text-white" fill="currentColor">
              <path d="M8 1c3 0 5.5 1.1 5.5 2.5v9C13.5 13.9 11 15 8 15s-5.5-1.1-5.5-2.5v-9C2.5 2.1 5 1 8 1zm0 1.4c-2.2 0-4 .7-4 1.6S5.8 5.6 8 5.6s4-.7 4-1.6S10.2 2.4 8 2.4z" />
            </svg>
          </div>
          <div className="text-ink-50 font-semibold text-[20px] tracking-tight">DBil</div>
          <div className="text-ink-300 text-[12.5px] mt-0.5">PostgreSQL workspace</div>
        </div>

        <form
          onSubmit={onSubmit}
          className="bg-ink-800/60 backdrop-blur-sm border border-ink-700 rounded-2xl shadow-card p-6"
        >
          <h1 className="text-ink-50 text-[16px] font-semibold tracking-tight mb-5">
            Sign in
          </h1>

          <Field label="Email">
            <input
              type="email"
              autoComplete="username"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              className="w-full h-10 px-3 rounded-lg bg-ink-900 border border-ink-700 focus:border-violet focus:outline-none text-[13px] text-ink-50"
            />
          </Field>

          <Field label="Password">
            <input
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              className="w-full h-10 px-3 rounded-lg bg-ink-900 border border-ink-700 focus:border-violet focus:outline-none text-[13px] text-ink-50"
            />
          </Field>

          {error && (
            <div className="mb-4 p-2.5 rounded-md bg-accent-coral/10 border border-accent-coral/40 text-accent-coral text-[12px]">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={submitting}
            className="w-full h-10 rounded-lg bg-violet text-white font-medium text-[13px] flex items-center justify-center gap-2 hover:bg-violet-deep transition-colors shadow-glow disabled:opacity-50 disabled:shadow-none"
          >
            {submitting ? <Spinner /> : 'Sign in'}
          </button>

          <p className="text-ink-400 text-[11.5px] mt-5 leading-relaxed">
            First run? The admin password is in{' '}
            <code className="font-mono text-ink-200">initial-credentials.txt</code> inside
            the data directory (default <code className="font-mono text-ink-200">/data</code>{' '}
            in the container).
          </p>
        </form>
      </div>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block mb-4">
      <span className="text-ink-300 text-[11.5px] font-medium mb-1.5 block">{label}</span>
      {children}
    </label>
  )
}

function Spinner() {
  return (
    <svg className="w-4 h-4 animate-spin" viewBox="0 0 16 16" fill="none">
      <circle cx="8" cy="8" r="6" stroke="currentColor" strokeOpacity="0.3" strokeWidth="2" />
      <path d="M14 8a6 6 0 0 1-6 6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  )
}
