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
import StatusPill from '../components/StatusPill'
import Icon from '../components/Icon'
import ConnectionFormDialog from '../components/ConnectionFormDialog'
import { useShellContext } from '../shell/context'

export default function ConnectionsPage() {
  const navigate = useNavigate()
  const { data: connections = [], isLoading, error } = useConnections()
  const [showCreate, setShowCreate] = useState(false)
  const { setActiveConnID } = useShellContext()

  return (
    <div className="h-full overflow-auto bg-app-grad">
      <div className="max-w-[1100px] mx-auto px-6 py-6">
        <header className="flex items-end justify-between mb-5">
          <div>
            <h1 className="text-[22px] font-semibold text-ink-50 tracking-tight">Connections</h1>
            <p className="text-ink-300 text-[13px] mt-0.5">
              {connections.length === 0
                ? 'No PostgreSQL databases registered yet'
                : `${connections.length} registered PostgreSQL database${connections.length === 1 ? '' : 's'}`}
            </p>
          </div>
          <button
            onClick={() => setShowCreate(true)}
            className="h-9 px-4 rounded-md bg-violet text-white font-medium text-[13px] flex items-center gap-2 hover:bg-violet-deep transition-colors shadow-glow"
          >
            <Icon name="plus" className="w-3.5 h-3.5" />
            <span>Add connection</span>
          </button>
        </header>

        {isLoading && <CenterMessage>Loading…</CenterMessage>}

        {error && !isLoading && (
          <div className="p-3 rounded-lg bg-accent-coral/10 border border-accent-coral/40 text-accent-coral text-[12.5px] font-mono">
            {error instanceof ApiError ? error.body.error || `HTTP ${error.status}` : String(error)}
          </div>
        )}

        {!isLoading && !error && connections.length === 0 && (
          <EmptyState onAdd={() => setShowCreate(true)} />
        )}

        {!isLoading && connections.length > 0 && (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {connections.map((c) => (
              <ConnectionCard
                key={c.id}
                conn={c}
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

function ConnectionCard({ conn, onOpen }: { conn: Connection; onOpen(): void }) {
  const del = useDeleteConnection()
  const test = useTestConnection()
  const [passphrase, setPassphrase] = useState('')
  const [showPassphrase, setShowPassphrase] = useState(false)
  const [testStatus, setTestStatus] = useState<
    null | { kind: 'ok'; version: string } | { kind: 'err'; msg: string }
  >(null)

  const runTest = async () => {
    setTestStatus(null)
    try {
      const r = await test.mutateAsync({
        id: conn.id,
        passphrase: passphrase || undefined,
      })
      setTestStatus({ kind: 'ok', version: r.version })
      setShowPassphrase(false)
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 428) {
          setShowPassphrase(true)
          setTestStatus({ kind: 'err', msg: 'Passphrase required' })
        } else {
          setTestStatus({
            kind: 'err',
            msg: err.body.reason || err.body.error || `HTTP ${err.status}`,
          })
        }
      } else {
        setTestStatus({ kind: 'err', msg: err instanceof Error ? err.message : 'Test failed' })
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
    <div className="bg-ink-800/70 backdrop-blur-sm border border-ink-700 rounded-xl shadow-card overflow-hidden hover:border-ink-600 transition-colors">
      <div className="bg-header-grad px-5 py-4 border-b border-ink-700 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-ink-50 text-[15px] truncate">{conn.alias}</span>
            <TagBadge tag={conn.tag} size="xs" />
            {conn.requires_passphrase && <StatusPill tone="warning" size="xs">passphrase</StatusPill>}
          </div>
          <div className="font-mono text-ink-300 text-[12px] mt-1 truncate">
            {conn.host}:{conn.port}
          </div>
        </div>
        <div className="flex items-center gap-1">
          <IconBtn title="Test" name="refresh" onClick={runTest} loading={test.isPending} />
          <IconBtn title="Delete" name="trash" onClick={handleDelete} loading={del.isPending} />
        </div>
      </div>

      <dl className="px-5 py-3 grid grid-cols-3 gap-2 text-[12.5px]">
        <Field label="TLS" value={conn.tls_mode} />
        <Field label="Tag" value={conn.tag} />
        <Field label="ID" value={String(conn.id)} />
      </dl>

      {showPassphrase && (
        <div className="px-5 pb-3">
          <label className="block">
            <span className="text-ink-300 text-[11.5px] font-medium mb-1 block">
              Passphrase
            </span>
            <div className="flex gap-2">
              <input
                type="password"
                value={passphrase}
                onChange={(e) => setPassphrase(e.target.value)}
                placeholder="connection passphrase"
                className="flex-1 h-8 px-3 rounded-md bg-ink-900 border border-ink-700 focus:border-violet focus:outline-none text-[12.5px] text-ink-50"
              />
              <button
                onClick={runTest}
                disabled={!passphrase || test.isPending}
                className="h-8 px-3 rounded-md bg-violet text-white text-[12px] font-medium hover:bg-violet-deep disabled:opacity-50"
              >
                Retry
              </button>
            </div>
          </label>
        </div>
      )}

      {testStatus && (
        <div className="px-5 pb-3">
          {testStatus.kind === 'ok' ? (
            <div className="text-[11.5px] flex items-center gap-2">
              <StatusPill tone="success" size="xs">Reachable</StatusPill>
              <span className="text-ink-300 font-mono truncate">{testStatus.version}</span>
            </div>
          ) : (
            <div className="text-[11.5px] flex items-center gap-2">
              <StatusPill tone="danger" size="xs">Failed</StatusPill>
              <span className="text-accent-coral font-mono truncate">{testStatus.msg}</span>
            </div>
          )}
        </div>
      )}

      <div className="px-5 py-3 border-t border-ink-700 flex justify-end">
        <button
          onClick={onOpen}
          className="text-[12.5px] font-medium text-violet hover:text-violet-bright"
        >
          Browse schema →
        </button>
      </div>
    </div>
  )
}

function EmptyState({ onAdd }: { onAdd(): void }) {
  return (
    <div className="border-2 border-dashed border-ink-700 rounded-2xl p-10 text-center">
      <div className="w-12 h-12 rounded-2xl bg-violet/10 border border-violet/30 flex items-center justify-center mx-auto mb-4">
        <Icon name="database" className="w-5 h-5 text-violet" />
      </div>
      <h3 className="text-ink-50 font-semibold text-[15px] mb-1">No connections yet</h3>
      <p className="text-ink-300 text-[12.5px] mb-5">
        Register a PostgreSQL database to browse its schema and run queries.
      </p>
      <button
        onClick={onAdd}
        className="h-9 px-4 rounded-md bg-violet text-white font-medium text-[13px] hover:bg-violet-deep transition-colors shadow-glow inline-flex items-center gap-2"
      >
        <Icon name="plus" className="w-3.5 h-3.5" />
        <span>Add your first connection</span>
      </button>
    </div>
  )
}

function CenterMessage({ children }: { children: React.ReactNode }) {
  return (
    <div className="h-40 flex items-center justify-center text-ink-300 text-[13px]">
      {children}
    </div>
  )
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-ink-400 text-[10.5px] uppercase tracking-wider">{label}</dt>
      <dd className="text-ink-100 font-mono">{value}</dd>
    </div>
  )
}

function IconBtn({
  title,
  name,
  onClick,
  loading,
}: {
  title: string
  name: 'refresh' | 'trash'
  onClick(): void
  loading?: boolean
}) {
  return (
    <button
      title={title}
      onClick={onClick}
      disabled={loading}
      className="p-1.5 rounded-md hover:bg-ink-700 text-ink-300 hover:text-ink-50 disabled:opacity-50"
    >
      <Icon name={name} className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
    </button>
  )
}
