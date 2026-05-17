import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  useConnections,
  useDeleteConnection,
  useTestConnection,
  type Connection,
} from '../api/connections'
import { ApiError } from '../api/client'
import TagBadge from '../components/TagBadge'
import Icon from '../components/Icon'
import ConnectionFormDialog from '../components/ConnectionFormDialog'
import { useShellContext } from '../shell/context'

export default function ConnectionsPage() {
  const navigate = useNavigate()
  const { data: connections = [], isLoading, error } = useConnections()
  const [showCreate, setShowCreate] = useState(false)
  const { setActiveConnID } = useShellContext()

  return (
    <div className="app-bg" style={{ height: '100%', overflow: 'auto' }}>
      <div style={{ maxWidth: 1100, margin: '0 auto', padding: '20px 24px 40px' }}>
        <header
          style={{
            display: 'flex',
            alignItems: 'flex-end',
            justifyContent: 'space-between',
            marginBottom: 20,
          }}
        >
          <div>
            <h1
              style={{
                fontSize: 16,
                fontWeight: 600,
                letterSpacing: '-0.02em',
                color: 'var(--fg-1)',
                margin: 0,
              }}
            >
              Connections
            </h1>
            <p style={{ fontSize: 12, color: 'var(--fg-3)', margin: '4px 0 0' }} className="tnum">
              {connections.length === 0
                ? 'No PostgreSQL databases registered yet'
                : `${connections.length} registered PostgreSQL database${connections.length === 1 ? '' : 's'}`}
            </p>
          </div>
          <button className="btn-pri" onClick={() => setShowCreate(true)}>
            <Icon name="plus" size={12} />
            Add connection
          </button>
        </header>

        {isLoading && <CenterMessage>Loading…</CenterMessage>}

        {error && !isLoading && (
          <ErrorBanner>
            {error instanceof ApiError ? error.body.error || `HTTP ${error.status}` : String(error)}
          </ErrorBanner>
        )}

        {!isLoading && !error && connections.length === 0 && (
          <EmptyState onAdd={() => setShowCreate(true)} />
        )}

        {!isLoading && connections.length > 0 && (
          <div
            style={{
              background: 'var(--bg-1)',
              border: '1px solid var(--line-1)',
              borderRadius: 10,
              boxShadow: 'var(--shadow-1)',
              overflow: 'hidden',
            }}
          >
            {connections.map((c, i) => (
              <ConnectionRow
                key={c.id}
                conn={c}
                showDivider={i > 0}
                onOpen={() => {
                  setActiveConnID(c.id)
                  navigate('/')
                }}
              />
            ))}
          </div>
        )}
      </div>

      <ConnectionFormDialog
        open={showCreate}
        onClose={() => setShowCreate(false)}
        onCreated={(id) => setActiveConnID(id)}
      />
    </div>
  )
}

function ConnectionRow({
  conn,
  showDivider,
  onOpen,
}: {
  conn: Connection
  showDivider: boolean
  onOpen(): void
}) {
  const del = useDeleteConnection()
  const test = useTestConnection()
  const [passphrase, setPassphrase] = useState('')
  const [needPassphrase, setNeedPassphrase] = useState(false)
  const [status, setStatus] = useState<
    | null
    | { kind: 'ok'; version: string }
    | { kind: 'err'; msg: string }
  >(null)

  const runTest = async () => {
    setStatus(null)
    try {
      const r = await test.mutateAsync({
        id: conn.id,
        passphrase: passphrase || undefined,
      })
      setStatus({ kind: 'ok', version: r.version })
      setNeedPassphrase(false)
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 428) {
          setNeedPassphrase(true)
          setStatus({ kind: 'err', msg: 'Passphrase required' })
        } else {
          setStatus({
            kind: 'err',
            msg: err.body.reason || err.body.error || `HTTP ${err.status}`,
          })
        }
      } else {
        setStatus({ kind: 'err', msg: err instanceof Error ? err.message : 'Test failed' })
      }
    }
  }

  const handleDelete = async () => {
    if (!confirm(`Delete connection "${conn.alias}"?`)) return
    try {
      await del.mutateAsync(conn.id)
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Delete failed')
    }
  }

  return (
    <div
      style={{
        padding: '14px 18px',
        borderTop: showDivider ? '1px solid var(--line-1)' : 'none',
        display: 'grid',
        gap: 14,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
        <div style={{ minWidth: 0, flex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
            <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--fg-1)', letterSpacing: '-0.01em' }}>
              {conn.alias}
            </span>
            <TagBadge tag={conn.tag} />
            {conn.requires_passphrase && (
              <span
                className="tag staging"
                style={{ background: 'var(--accent-mute)', color: 'var(--accent-soft)' }}
              >
                <Icon name="lock" size={9} /> passphrase
              </span>
            )}
          </div>
          <div className="mono" style={{ fontSize: 11.5, color: 'var(--fg-4)', marginTop: 4 }}>
            {conn.host}:{conn.port} · tls={conn.tls_mode}
          </div>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <button className="btn-gh" onClick={runTest} disabled={test.isPending}>
            <Icon name="refresh" size={12} style={{ opacity: test.isPending ? 0.4 : 1 }} />
            {test.isPending ? 'Testing…' : 'Test'}
          </button>
          <button className="btn-gh" onClick={onOpen}>
            <Icon name="schema" size={12} />
            Browse
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

      {needPassphrase && (
        <div
          style={{
            display: 'flex',
            gap: 8,
            alignItems: 'center',
            padding: '10px 12px',
            background: 'var(--bg-2)',
            border: '1px solid var(--line-2)',
            borderRadius: 8,
          }}
        >
          <Icon name="lock" size={13} style={{ color: 'var(--accent-soft)' }} />
          <input
            type="password"
            value={passphrase}
            onChange={(e) => setPassphrase(e.target.value)}
            placeholder="Connection passphrase"
            style={{
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
            }}
          />
          <button
            className="btn-pri"
            onClick={runTest}
            disabled={!passphrase || test.isPending}
            style={{ height: 26 }}
          >
            Retry
          </button>
        </div>
      )}

      {status && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 8,
            fontSize: 11.5,
            fontFamily: 'var(--font-mono)',
          }}
        >
          {status.kind === 'ok' ? (
            <>
              <span style={{ color: 'var(--ok)' }}>● reachable</span>
              <span style={{ color: 'var(--fg-3)' }}>{status.version}</span>
            </>
          ) : (
            <>
              <span style={{ color: 'var(--danger)' }}>● failed</span>
              <span style={{ color: 'var(--danger)' }}>{status.msg}</span>
            </>
          )}
        </div>
      )}
    </div>
  )
}

function EmptyState({ onAdd }: { onAdd(): void }) {
  return (
    <div
      style={{
        border: '1px dashed var(--line-2)',
        borderRadius: 14,
        padding: 40,
        textAlign: 'center',
        background: 'var(--bg-1)',
      }}
    >
      <div
        style={{
          width: 48,
          height: 48,
          margin: '0 auto 16px',
          borderRadius: 12,
          background: 'var(--accent-mute)',
          border: '1px solid var(--accent-soft)',
          display: 'grid',
          placeItems: 'center',
          color: 'var(--accent)',
        }}
      >
        <Icon name="schema" size={20} />
      </div>
      <h3 style={{ fontSize: 14, fontWeight: 600, color: 'var(--fg-1)', margin: 0 }}>
        No connections yet
      </h3>
      <p style={{ fontSize: 12, color: 'var(--fg-3)', marginTop: 6, marginBottom: 20 }}>
        Register a PostgreSQL database to browse its schema and run queries.
      </p>
      <button className="btn-pri" onClick={onAdd}>
        <Icon name="plus" size={12} />
        Add your first connection
      </button>
    </div>
  )
}

function ErrorBanner({ children }: { children: React.ReactNode }) {
  return (
    <div
      style={{
        padding: '10px 12px',
        borderRadius: 8,
        background: 'var(--danger-soft)',
        border: '1px solid rgba(255,107,122,0.3)',
        color: 'var(--danger)',
        fontSize: 12,
        fontFamily: 'var(--font-mono)',
      }}
    >
      {children}
    </div>
  )
}

function CenterMessage({ children }: { children: React.ReactNode }) {
  return (
    <div
      style={{
        height: 160,
        display: 'grid',
        placeItems: 'center',
        color: 'var(--fg-3)',
        fontSize: 13,
      }}
    >
      {children}
    </div>
  )
}
