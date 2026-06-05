import { useState } from 'react'
import { useChangePassword } from '../api/users'
import { ApiError } from '../api/client'

interface Props {
  // forced: rendered as a non-dismissable gate after an admin reset / first login.
  forced?: boolean
  onClose?: () => void
  onSuccess: () => void
}

const labelStyle: React.CSSProperties = {
  fontSize: 11,
  color: 'var(--fg-3)',
  fontWeight: 500,
  textTransform: 'uppercase',
  letterSpacing: '0.02em',
  marginBottom: 5,
  display: 'block',
}

const inputStyle: React.CSSProperties = {
  height: 32,
  width: '100%',
  padding: '0 10px',
  background: 'var(--bg-1)',
  border: '1px solid var(--line-2)',
  borderRadius: 7,
  color: 'var(--fg-1)',
  fontSize: 12.5,
  fontFamily: 'inherit',
}

export default function ChangePasswordModal({ forced, onClose, onSuccess }: Props) {
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const change = useChangePassword()

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (next.length < 12) {
      setError('New password must be at least 12 characters.')
      return
    }
    if (next !== confirm) {
      setError('New password and confirmation do not match.')
      return
    }
    try {
      await change.mutateAsync({ current, new: next })
      onSuccess()
    } catch (err) {
      if (err instanceof ApiError) setError(err.body.error ?? 'Could not change password.')
      else setError('Could not change password.')
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      className="scrim"
      onClick={forced ? undefined : onClose}
    >
      <form
        onClick={(e) => e.stopPropagation()}
        onSubmit={submit}
        style={{
          width: 380,
          background: 'var(--bg-2)',
          border: '1px solid var(--line-2)',
          borderRadius: 14,
          boxShadow: 'var(--shadow-pop)',
          overflow: 'hidden',
        }}
      >
        <div style={{ padding: '14px 18px', borderBottom: '1px solid var(--line-1)' }}>
          <h2 style={{ fontSize: 14, fontWeight: 600, margin: 0 }}>
            {forced ? 'Set a new password' : 'Change password'}
          </h2>
          {forced && (
            <p style={{ fontSize: 11.5, color: 'var(--fg-3)', margin: '6px 0 0' }}>
              Your account uses a temporary password. Choose a new one to continue.
            </p>
          )}
        </div>
        <div style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div>
            <label style={labelStyle}>Current password</label>
            <input
              style={inputStyle}
              type="password"
              autoFocus
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
              placeholder="current or temporary password"
            />
          </div>
          <div>
            <label style={labelStyle}>New password</label>
            <input
              style={inputStyle}
              type="password"
              value={next}
              onChange={(e) => setNext(e.target.value)}
              placeholder="at least 12 characters"
            />
          </div>
          <div>
            <label style={labelStyle}>Confirm new password</label>
            <input
              style={inputStyle}
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
            />
          </div>
          {error && (
            <div
              style={{
                padding: 10,
                borderRadius: 7,
                background: 'var(--danger-soft)',
                border: '1px solid rgba(255,107,122,0.3)',
                color: 'var(--danger)',
                fontSize: 12,
              }}
            >
              {error}
            </div>
          )}
        </div>
        <div
          style={{
            padding: '12px 18px',
            borderTop: '1px solid var(--line-1)',
            display: 'flex',
            justifyContent: 'flex-end',
            gap: 8,
          }}
        >
          {!forced && (
            <button type="button" className="btn-gh" onClick={onClose}>
              Cancel
            </button>
          )}
          <button type="submit" className="btn-pri" disabled={change.isPending}>
            {change.isPending ? 'Saving…' : 'Save password'}
          </button>
        </div>
      </form>
    </div>
  )
}
