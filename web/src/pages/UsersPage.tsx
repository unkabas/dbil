import { useState } from 'react'
import {
  useUsers,
  useCreateUser,
  useUpdateUserRole,
  useDeleteUser,
  useResetPassword,
  type Role,
  type User,
} from '../api/users'
import { useAuth } from '../auth/AuthContext'
import { ApiError } from '../api/client'
import Icon from '../components/Icon'

const ROLES: Role[] = ['admin', 'member', 'viewer']

const roleTone: Record<Role, { bg: string; fg: string; ring: string }> = {
  admin: { bg: 'var(--danger-soft)', fg: 'var(--danger)', ring: 'rgba(255,107,122,0.3)' },
  member: { bg: 'var(--accent-mute)', fg: 'var(--accent-soft)', ring: 'var(--accent-glow)' },
  viewer: { bg: 'var(--bg-3)', fg: 'var(--fg-3)', ring: 'var(--line-2)' },
}

function RoleBadge({ role }: { role: Role }) {
  const t = roleTone[role]
  return (
    <span
      style={{
        fontSize: 10.5,
        fontWeight: 600,
        textTransform: 'uppercase',
        letterSpacing: '0.05em',
        padding: '2px 8px',
        borderRadius: 999,
        background: t.bg,
        color: t.fg,
        border: `1px solid ${t.ring}`,
      }}
    >
      {role}
    </span>
  )
}

export default function UsersPage() {
  const { user: me } = useAuth()
  const { data: users = [], isLoading } = useUsers()
  const updateRole = useUpdateUserRole()
  const del = useDeleteUser()
  const [creating, setCreating] = useState(false)
  const [generated, setGenerated] = useState<{ email: string; password: string } | null>(null)

  return (
    <div style={{ height: '100%', overflow: 'auto', padding: '20px 24px' }}>
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 18 }}>
        <div>
          <h1 style={{ fontSize: 18, fontWeight: 600, margin: 0 }}>Users</h1>
          <p style={{ fontSize: 12, color: 'var(--fg-3)', margin: '4px 0 0' }}>
            Create accounts and assign roles. New users get a one-time password and must rotate it
            at first login.
          </p>
        </div>
        <span style={{ flex: 1 }} />
        <button className="btn-pri" onClick={() => setCreating(true)}>
          <Icon name="plus" size={13} /> New user
        </button>
      </div>

      {generated && (
        <GeneratedPasswordBanner
          email={generated.email}
          password={generated.password}
          onDismiss={() => setGenerated(null)}
        />
      )}

      <div
        style={{
          border: '1px solid var(--line-1)',
          borderRadius: 10,
          overflow: 'hidden',
          background: 'var(--bg-1)',
        }}
      >
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: '1fr 140px 120px 160px',
            padding: '9px 14px',
            borderBottom: '1px solid var(--line-1)',
            fontSize: 10.5,
            color: 'var(--fg-4)',
            textTransform: 'uppercase',
            letterSpacing: '0.05em',
            fontWeight: 500,
          }}
        >
          <span>Email</span>
          <span>Role</span>
          <span>Status</span>
          <span style={{ textAlign: 'right' }}>Actions</span>
        </div>
        {isLoading && (
          <div style={{ padding: 16, fontSize: 12, color: 'var(--fg-3)' }}>Loading…</div>
        )}
        {users.map((u) => (
          <UserRow
            key={u.id}
            u={u}
            isSelf={u.id === me?.id}
            onRoleChange={(role) => updateRole.mutate({ id: u.id, role })}
            onDelete={() => {
              if (window.confirm(`Delete ${u.email}? This cannot be undone.`)) del.mutate(u.id)
            }}
            onResetShown={(password) => setGenerated({ email: u.email, password })}
          />
        ))}
        {!isLoading && users.length === 0 && (
          <div style={{ padding: 16, fontSize: 12, color: 'var(--fg-3)' }}>No users yet.</div>
        )}
      </div>

      {creating && (
        <CreateUserModal
          onClose={() => setCreating(false)}
          onCreated={(email, password) => {
            setGenerated({ email, password })
            setCreating(false)
          }}
        />
      )}
    </div>
  )
}

function UserRow({
  u,
  isSelf,
  onRoleChange,
  onDelete,
  onResetShown,
}: {
  u: User
  isSelf: boolean
  onRoleChange: (role: Role) => void
  onDelete: () => void
  onResetShown: (password: string) => void
}) {
  const reset = useResetPassword()
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: '1fr 140px 120px 160px',
        alignItems: 'center',
        padding: '10px 14px',
        borderBottom: '1px solid var(--line-1)',
        fontSize: 12.5,
      }}
    >
      <span style={{ color: 'var(--fg-1)' }}>
        {u.email}
        {isSelf && <span style={{ color: 'var(--fg-4)', fontSize: 11 }}> (you)</span>}
      </span>
      <span>
        {isSelf ? (
          <RoleBadge role={u.role} />
        ) : (
          <select
            value={u.role}
            onChange={(e) => onRoleChange(e.target.value as Role)}
            style={{
              height: 26,
              background: 'var(--bg-2)',
              border: '1px solid var(--line-2)',
              borderRadius: 6,
              color: 'var(--fg-1)',
              fontSize: 12,
              padding: '0 6px',
            }}
          >
            {ROLES.map((r) => (
              <option key={r} value={r}>
                {r}
              </option>
            ))}
          </select>
        )}
      </span>
      <span>
        {u.must_rotate ? (
          <span style={{ fontSize: 11, color: 'var(--warn)' }}>must rotate</span>
        ) : (
          <span style={{ fontSize: 11, color: 'var(--fg-4)' }}>active</span>
        )}
      </span>
      <span style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
        <button
          className="btn-gh"
          style={{ height: 26, fontSize: 11.5 }}
          disabled={reset.isPending}
          onClick={async () => {
            const r = await reset.mutateAsync(u.id)
            onResetShown(r.password)
          }}
        >
          <Icon name="key" size={12} /> Reset
        </button>
        {!isSelf && (
          <button
            className="btn-gh"
            style={{ height: 26, fontSize: 11.5, color: 'var(--danger)' }}
            onClick={onDelete}
          >
            <Icon name="trash" size={12} />
          </button>
        )}
      </span>
    </div>
  )
}

function GeneratedPasswordBanner({
  email,
  password,
  onDismiss,
}: {
  email: string
  password: string
  onDismiss: () => void
}) {
  const [copied, setCopied] = useState(false)
  return (
    <div
      style={{
        marginBottom: 16,
        padding: 14,
        borderRadius: 10,
        background: 'var(--warn-soft)',
        border: '1px solid rgba(245,165,36,0.35)',
      }}
    >
      <div style={{ fontSize: 12.5, color: 'var(--fg-1)', fontWeight: 500, marginBottom: 6 }}>
        One-time password for {email}
      </div>
      <div style={{ fontSize: 11.5, color: 'var(--fg-3)', marginBottom: 10 }}>
        Copy it now — it is shown only once. The user must change it at first login.
      </div>
      <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
        <code
          className="mono"
          style={{
            flex: 1,
            padding: '8px 10px',
            background: 'var(--bg-0)',
            border: '1px solid var(--line-2)',
            borderRadius: 7,
            fontSize: 13,
            color: 'var(--fg-1)',
            userSelect: 'all',
          }}
        >
          {password}
        </code>
        <button
          className="btn-gh"
          onClick={() => {
            void navigator.clipboard.writeText(password)
            setCopied(true)
          }}
        >
          <Icon name="copy" size={13} /> {copied ? 'Copied' : 'Copy'}
        </button>
        <button className="btn-gh" onClick={onDismiss}>
          Dismiss
        </button>
      </div>
    </div>
  )
}

function CreateUserModal({
  onClose,
  onCreated,
}: {
  onClose: () => void
  onCreated: (email: string, password: string) => void
}) {
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<Role>('member')
  const [error, setError] = useState('')
  const create = useCreateUser()

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    try {
      const r = await create.mutateAsync({ email: email.trim(), role })
      onCreated(r.email, r.password)
    } catch (err) {
      if (err instanceof ApiError) setError(err.body.error ?? 'Could not create user.')
      else setError('Could not create user.')
    }
  }

  return (
    <div role="dialog" aria-modal="true" className="scrim" onClick={onClose}>
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
          <h2 style={{ fontSize: 14, fontWeight: 600, margin: 0 }}>New user</h2>
        </div>
        <div style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div>
            <label
              style={{
                fontSize: 11,
                color: 'var(--fg-3)',
                fontWeight: 500,
                textTransform: 'uppercase',
                marginBottom: 5,
                display: 'block',
              }}
            >
              Email
            </label>
            <input
              autoFocus
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="person@team"
              style={{
                height: 32,
                width: '100%',
                padding: '0 10px',
                background: 'var(--bg-1)',
                border: '1px solid var(--line-2)',
                borderRadius: 7,
                color: 'var(--fg-1)',
                fontSize: 12.5,
              }}
            />
          </div>
          <div>
            <label
              style={{
                fontSize: 11,
                color: 'var(--fg-3)',
                fontWeight: 500,
                textTransform: 'uppercase',
                marginBottom: 5,
                display: 'block',
              }}
            >
              Role
            </label>
            <select
              value={role}
              onChange={(e) => setRole(e.target.value as Role)}
              style={{
                height: 32,
                width: '100%',
                background: 'var(--bg-1)',
                border: '1px solid var(--line-2)',
                borderRadius: 7,
                color: 'var(--fg-1)',
                fontSize: 12.5,
                padding: '0 8px',
              }}
            >
              {ROLES.map((r) => (
                <option key={r} value={r}>
                  {r} {r === 'viewer' ? '— read only' : r === 'member' ? '— can edit data' : '— full access'}
                </option>
              ))}
            </select>
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
          <button type="button" className="btn-gh" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn-pri" disabled={create.isPending}>
            {create.isPending ? 'Creating…' : 'Create user'}
          </button>
        </div>
      </form>
    </div>
  )
}
