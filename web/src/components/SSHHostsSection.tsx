import { useState } from 'react'
import {
  useSSHHosts,
  useCreateSSHHost,
  useDeleteSSHHost,
  useTestSSHHost,
  type SSHHost,
  type SSHAuthMethod,
  type CreateSSHHostInput,
} from '../api/sshHosts'
import { ApiError } from '../api/client'
import Icon from './Icon'

// SSHHostsSection manages reusable SSH bastions that connections tunnel
// through. Writers only (the API is RequireRole admin/member).
export default function SSHHostsSection() {
  const { data: hosts = [], isLoading } = useSSHHosts()
  const [showCreate, setShowCreate] = useState(false)

  return (
    <section style={{ marginTop: 32 }}>
      <header
        style={{
          display: 'flex',
          alignItems: 'flex-end',
          justifyContent: 'space-between',
          marginBottom: 14,
        }}
      >
        <div>
          <h2 style={{ fontSize: 14.5, fontWeight: 600, color: 'var(--fg-1)', margin: 0 }}>
            SSH tunnels
          </h2>
          <p style={{ fontSize: 12, color: 'var(--fg-3)', margin: '4px 0 0' }}>
            Reach databases whose ports are closed by tunnelling through a bastion.
          </p>
        </div>
        <button className="btn-gh" onClick={() => setShowCreate(true)}>
          <Icon name="plus" size={12} /> Add SSH host
        </button>
      </header>

      {isLoading && <div style={{ fontSize: 12, color: 'var(--fg-3)' }}>Loading…</div>}

      {!isLoading && hosts.length === 0 && (
        <div
          style={{
            border: '1px dashed var(--line-2)',
            borderRadius: 10,
            padding: 20,
            textAlign: 'center',
            color: 'var(--fg-3)',
            fontSize: 12,
            background: 'var(--bg-1)',
          }}
        >
          No SSH hosts yet. Add one, then pick it as the tunnel when creating a connection.
        </div>
      )}

      {hosts.length > 0 && (
        <div
          style={{
            background: 'var(--bg-1)',
            border: '1px solid var(--line-1)',
            borderRadius: 10,
            overflow: 'hidden',
          }}
        >
          {hosts.map((h, i) => (
            <SSHHostRow key={h.id} host={h} showDivider={i > 0} />
          ))}
        </div>
      )}

      {showCreate && <CreateSSHHostModal onClose={() => setShowCreate(false)} />}
    </section>
  )
}

function SSHHostRow({ host, showDivider }: { host: SSHHost; showDivider: boolean }) {
  const test = useTestSSHHost()
  const del = useDeleteSSHHost()
  const [passphrase, setPassphrase] = useState('')
  const [needPass, setNeedPass] = useState(false)
  const [status, setStatus] = useState<null | { ok: boolean; msg: string }>(null)

  const runTest = async () => {
    setStatus(null)
    try {
      const r = await test.mutateAsync({ id: host.id, passphrase: passphrase || undefined })
      setStatus({ ok: true, msg: r.host_key_fingerprint })
      setNeedPass(false)
    } catch (err) {
      if (err instanceof ApiError && err.status === 428) {
        setNeedPass(true)
        setStatus({ ok: false, msg: 'Passphrase required' })
      } else if (err instanceof ApiError) {
        setStatus({ ok: false, msg: err.body.error || `HTTP ${err.status}` })
      } else {
        setStatus({ ok: false, msg: 'Test failed' })
      }
    }
  }

  const handleDelete = async () => {
    if (!confirm(`Delete SSH host "${host.alias}"?`)) return
    try {
      await del.mutateAsync(host.id)
    } catch (err) {
      alert(err instanceof ApiError ? err.body.error || 'Delete failed' : 'Delete failed')
    }
  }

  return (
    <div
      style={{
        padding: '12px 18px',
        borderTop: showDivider ? '1px solid var(--line-1)' : 'none',
        display: 'grid',
        gap: 10,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
        <div style={{ minWidth: 0, flex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
            <span style={{ fontSize: 13.5, fontWeight: 600, color: 'var(--fg-1)' }}>{host.alias}</span>
            <span style={{ fontSize: 10.5, color: 'var(--fg-3)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
              {host.auth_method}
            </span>
            {host.requires_passphrase && (
              <span className="tag staging" style={{ background: 'var(--accent-mute)', color: 'var(--accent-soft)' }}>
                <Icon name="lock" size={9} /> passphrase
              </span>
            )}
          </div>
          <div className="mono" style={{ fontSize: 11.5, color: 'var(--fg-4)', marginTop: 4 }}>
            {host.username}@{host.host}:{host.port}
            {host.host_key_fingerprint ? ` · ${host.host_key_fingerprint}` : ' · key not yet pinned'}
          </div>
        </div>
        <div style={{ display: 'flex', gap: 6 }}>
          <button className="btn-gh" onClick={runTest} disabled={test.isPending}>
            <Icon name="refresh" size={12} /> {test.isPending ? 'Testing…' : 'Test'}
          </button>
          <button
            className="btn-gh"
            onClick={handleDelete}
            disabled={del.isPending}
            style={{ color: 'var(--danger)' }}
            title="Delete"
          >
            <Icon name="trash" size={12} />
          </button>
        </div>
      </div>

      {needPass && (
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <input
            type="password"
            value={passphrase}
            onChange={(e) => setPassphrase(e.target.value)}
            placeholder="SSH host passphrase"
            style={inputSm}
          />
          <button className="btn-pri" onClick={runTest} disabled={!passphrase} style={{ height: 26 }}>
            Retry
          </button>
        </div>
      )}

      {status && (
        <div style={{ fontSize: 11.5, fontFamily: 'var(--font-mono)' }}>
          {status.ok ? (
            <span style={{ color: 'var(--ok)' }}>● reachable · {status.msg}</span>
          ) : (
            <span style={{ color: 'var(--danger)' }}>● {status.msg}</span>
          )}
        </div>
      )}
    </div>
  )
}

function CreateSSHHostModal({ onClose }: { onClose(): void }) {
  const create = useCreateSSHHost()
  const [form, setForm] = useState<CreateSSHHostInput>({
    alias: '',
    host: '',
    port: 22,
    username: '',
    auth_method: 'key',
    secret: '',
  })
  const [error, setError] = useState('')
  const set = <K extends keyof CreateSSHHostInput>(k: K, v: CreateSSHHostInput[K]) =>
    setForm((p) => ({ ...p, [k]: v }))

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    try {
      await create.mutateAsync(form)
      onClose()
    } catch (err) {
      setError(err instanceof ApiError ? err.body.error || 'Create failed' : 'Create failed')
    }
  }

  return (
    <div role="dialog" aria-modal="true" className="scrim" onClick={onClose} style={{ display: 'grid', placeItems: 'center', padding: 24 }}>
      <form onClick={(e) => e.stopPropagation()} onSubmit={submit} style={modalForm}>
        <div style={{ padding: '14px 18px', borderBottom: '1px solid var(--line-1)' }}>
          <h2 style={{ fontSize: 14, fontWeight: 600, margin: 0 }}>New SSH host</h2>
        </div>
        <div style={{ padding: 18, display: 'grid', gap: 12 }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 120px', gap: 12 }}>
            <L label="Alias">
              <input style={inputMd} value={form.alias} onChange={(e) => set('alias', e.target.value)} required placeholder="prod-bastion" />
            </L>
            <L label="Auth">
              <select style={inputMd} value={form.auth_method} onChange={(e) => set('auth_method', e.target.value as SSHAuthMethod)}>
                <option value="key">private key</option>
                <option value="password">password</option>
              </select>
            </L>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 100px', gap: 12 }}>
            <L label="Host">
              <input style={inputMd} value={form.host} onChange={(e) => set('host', e.target.value)} required placeholder="bastion.example.com" />
            </L>
            <L label="Port">
              <input style={inputMd} value={String(form.port)} onChange={(e) => set('port', Number(e.target.value) || 0)} inputMode="numeric" />
            </L>
          </div>
          <L label="SSH username">
            <input style={inputMd} value={form.username} onChange={(e) => set('username', e.target.value)} required placeholder="deploy" />
          </L>
          {form.auth_method === 'key' ? (
            <>
              <L label="Private key (PEM / OpenSSH)">
                <textarea
                  value={form.secret}
                  onChange={(e) => set('secret', e.target.value)}
                  required
                  rows={5}
                  placeholder={'-----BEGIN OPENSSH PRIVATE KEY-----\n…'}
                  style={{ ...inputMd, height: 'auto', padding: 8, fontFamily: 'var(--font-mono)', fontSize: 11.5, resize: 'vertical' }}
                />
              </L>
              <L label="Key passphrase (if the key is encrypted)">
                <input style={inputMd} type="password" value={form.key_passphrase ?? ''} onChange={(e) => set('key_passphrase', e.target.value || undefined)} />
              </L>
            </>
          ) : (
            <L label="SSH password">
              <input style={inputMd} type="password" value={form.secret} onChange={(e) => set('secret', e.target.value)} required />
            </L>
          )}
          <L label="At-rest passphrase (optional — required to open the tunnel later)">
            <input style={inputMd} type="password" value={form.passphrase ?? ''} onChange={(e) => set('passphrase', e.target.value || undefined)} placeholder="wrap the secret with a passphrase" />
          </L>
          {error && (
            <div style={{ padding: 10, borderRadius: 7, background: 'var(--danger-soft)', border: '1px solid rgba(255,107,122,0.3)', color: 'var(--danger)', fontSize: 12 }}>
              {error}
            </div>
          )}
        </div>
        <div style={{ padding: '12px 18px', borderTop: '1px solid var(--line-1)', display: 'flex', justifyContent: 'flex-end', gap: 8, background: 'var(--bg-1)' }}>
          <button type="button" className="btn-gh" onClick={onClose}>Cancel</button>
          <button type="submit" className="btn-pri" disabled={create.isPending}>
            {create.isPending ? 'Saving…' : 'Create SSH host'}
          </button>
        </div>
      </form>
    </div>
  )
}

function L({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label style={{ display: 'block' }}>
      <span style={{ fontSize: 11, color: 'var(--fg-3)', fontWeight: 500, marginBottom: 5, display: 'block' }}>{label}</span>
      {children}
    </label>
  )
}

const inputMd: React.CSSProperties = {
  width: '100%',
  height: 32,
  padding: '0 10px',
  borderRadius: 7,
  background: 'var(--bg-1)',
  border: '1px solid var(--line-2)',
  color: 'var(--fg-1)',
  fontSize: 12.5,
  outline: 0,
  fontFamily: 'inherit',
}

const inputSm: React.CSSProperties = {
  flex: 1,
  height: 26,
  padding: '0 8px',
  borderRadius: 6,
  background: 'var(--bg-1)',
  border: '1px solid var(--line-2)',
  color: 'var(--fg-1)',
  fontSize: 12,
  outline: 0,
  fontFamily: 'inherit',
}

const modalForm: React.CSSProperties = {
  width: '100%',
  maxWidth: 520,
  background: 'var(--bg-2)',
  border: '1px solid var(--line-2)',
  borderRadius: 14,
  boxShadow: 'var(--shadow-pop)',
  overflow: 'hidden',
}
