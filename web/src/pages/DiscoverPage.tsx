import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  useDiscovered,
  useApproveDiscovered,
  useRejectDiscovered,
  type DiscoverEntry,
} from '../api/discover'
import { ApiError } from '../api/client'
import TagBadge from '../components/TagBadge'
import Icon from '../components/Icon'
import { useShellContext } from '../shell/context'

export default function DiscoverPage() {
  const { data, isLoading, error } = useDiscovered()
  const entries = data?.entries ?? []
  const pending = entries.filter((e) => e.status === 'pending')
  const others = entries.filter((e) => e.status !== 'pending')

  return (
    <div className="app-bg" style={{ height: '100%', overflow: 'auto' }}>
      <div style={{ maxWidth: 1100, margin: '0 auto', padding: '20px 24px 40px' }}>
        <header
          style={{
            display: 'flex',
            alignItems: 'flex-end',
            justifyContent: 'space-between',
            marginBottom: 18,
            flexWrap: 'wrap',
            gap: 12,
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
              Discover
            </h1>
            <p style={{ fontSize: 12, color: 'var(--fg-3)', margin: '4px 0 0' }} className="tnum">
              {pending.length > 0
                ? `${pending.length} pending · ${entries.length} total`
                : entries.length > 0
                  ? `${entries.length} discovered`
                  : 'Waiting for candidates'}
            </p>
          </div>
          <span
            style={{
              fontSize: 11,
              color: 'var(--fg-4)',
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
            }}
          >
            <span
              className="live-dot"
              style={{ width: 6, height: 6, borderRadius: 3, background: 'var(--accent)' }}
            />
            scans every 30 s
          </span>
        </header>

        {isLoading && entries.length === 0 && (
          <CenterMessage>Loading discovery feed…</CenterMessage>
        )}

        {error && (
          <ErrorBanner>
            {error instanceof ApiError ? error.body.error || `HTTP ${error.status}` : String(error)}
          </ErrorBanner>
        )}

        {!isLoading && entries.length === 0 && <EmptyState />}

        {pending.length > 0 && (
          <SectionTitle title="Pending approval" count={pending.length} tone="pending" />
        )}
        {pending.length > 0 && <EntryGroup entries={pending} />}

        {others.length > 0 && (
          <SectionTitle title="History" count={others.length} tone="history" />
        )}
        {others.length > 0 && <EntryGroup entries={others} />}
      </div>
    </div>
  )
}

function SectionTitle({
  title,
  count,
  tone,
}: {
  title: string
  count: number
  tone: 'pending' | 'history'
}) {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        margin: '16px 0 8px',
      }}
    >
      <span
        style={{
          fontSize: 10.5,
          color: tone === 'pending' ? 'var(--c-amber)' : 'var(--fg-4)',
          letterSpacing: '0.08em',
          textTransform: 'uppercase',
          fontWeight: 500,
        }}
      >
        {title}
      </span>
      <span className="mono tnum" style={{ fontSize: 10.5, color: 'var(--fg-5)' }}>
        {count}
      </span>
    </div>
  )
}

function EntryGroup({ entries }: { entries: DiscoverEntry[] }) {
  return (
    <div
      style={{
        background: 'var(--bg-1)',
        border: '1px solid var(--line-1)',
        borderRadius: 10,
        boxShadow: 'var(--shadow-1)',
        overflow: 'hidden',
      }}
    >
      {entries.map((e, i) => (
        <EntryRow key={e.id} entry={e} showDivider={i > 0} />
      ))}
    </div>
  )
}

function EntryRow({ entry, showDivider }: { entry: DiscoverEntry; showDivider: boolean }) {
  const approve = useApproveDiscovered()
  const reject = useRejectDiscovered()
  const navigate = useNavigate()
  const { setActiveConnID } = useShellContext()
  const [passphrase, setPassphrase] = useState('')
  const [needPass, setNeedPass] = useState(entry.tag === 'production')
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  const isPending = entry.status === 'pending' || entry.status === 'unreachable'
  const canAct = isPending

  const onApprove = async () => {
    setErrorMsg(null)
    try {
      const r = await approve.mutateAsync({ id: entry.id, passphrase })
      setActiveConnID(r.connection_id)
      navigate('/')
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 428) {
          setNeedPass(true)
          setErrorMsg('Passphrase required for production connections')
        } else {
          setErrorMsg(err.body.reason || err.body.error || `HTTP ${err.status}`)
        }
      } else {
        setErrorMsg(err instanceof Error ? err.message : 'Approval failed')
      }
    }
  }

  const onReject = async () => {
    setErrorMsg(null)
    try {
      await reject.mutateAsync(entry.id)
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : 'Reject failed')
    }
  }

  return (
    <div
      style={{
        padding: '14px 18px',
        borderTop: showDivider ? '1px solid var(--line-1)' : 'none',
        display: 'grid',
        gap: 10,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
        <StatusBadge status={entry.status} />
        <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--fg-1)', letterSpacing: '-0.01em' }}>
          {entry.alias}
        </span>
        <TagBadge tag={entry.tag} />
        <SourcePill source={entry.source} />
        <span style={{ flex: 1 }} />
        <span className="mono tnum" style={{ fontSize: 11, color: 'var(--fg-4)' }}>
          {fmtRelative(entry.last_seen_ms)}
        </span>
      </div>

      <div className="mono" style={{ fontSize: 11.5, color: 'var(--fg-4)' }}>
        {entry.host}:{entry.port}
        <span style={{ color: 'var(--fg-5)', margin: '0 6px' }}>·</span>
        user=<span style={{ color: 'var(--c-cyan)' }}>{entry.username}</span>
        <span style={{ color: 'var(--fg-5)', margin: '0 6px' }}>·</span>
        db=<span style={{ color: 'var(--c-mint)' }}>{entry.database}</span>
        {entry.has_password ? null : (
          <>
            <span style={{ color: 'var(--fg-5)', margin: '0 6px' }}>·</span>
            <span style={{ color: 'var(--warn)' }}>passwordless</span>
          </>
        )}
      </div>

      {needPass && canAct && (
        <input
          type="password"
          placeholder="Connection passphrase (required for production)"
          value={passphrase}
          onChange={(e) => setPassphrase(e.target.value)}
          autoFocus
          style={{
            height: 30,
            background: 'var(--bg-2)',
            border: '1px solid var(--line-2)',
            borderRadius: 7,
            padding: '0 10px',
            color: 'var(--fg-1)',
            fontSize: 12,
            fontFamily: 'var(--font-mono)',
            outline: 0,
          }}
        />
      )}

      {errorMsg && (
        <div
          className="mono"
          style={{
            fontSize: 11.5,
            color: 'var(--danger)',
            background: 'var(--danger-soft)',
            border: '1px solid rgba(255,107,122,0.3)',
            padding: '6px 10px',
            borderRadius: 6,
          }}
        >
          {errorMsg}
        </div>
      )}

      {canAct && (
        <div style={{ display: 'flex', gap: 6 }}>
          <button
            className="btn-pri"
            onClick={onApprove}
            disabled={approve.isPending || (entry.tag === 'production' && !passphrase)}
          >
            <Icon name="check" size={12} />
            {approve.isPending ? 'Approving…' : 'Approve'}
          </button>
          <button className="btn-gh" onClick={onReject} disabled={reject.isPending}>
            <Icon name="x" size={12} />
            {reject.isPending ? 'Rejecting…' : 'Reject'}
          </button>
          <span style={{ flex: 1 }} />
        </div>
      )}
    </div>
  )
}

function StatusBadge({ status }: { status: DiscoverEntry['status'] }) {
  const map = {
    pending: { color: 'var(--c-amber)', bg: 'var(--warn-soft)', border: 'rgba(245,165,36,0.3)', label: 'pending' },
    approved: { color: 'var(--ok)', bg: 'var(--ok-soft)', border: 'rgba(52,211,153,0.3)', label: 'approved' },
    rejected: { color: 'var(--fg-4)', bg: 'var(--bg-3)', border: 'var(--line-2)', label: 'rejected' },
    unreachable: { color: 'var(--danger)', bg: 'var(--danger-soft)', border: 'rgba(255,107,122,0.3)', label: 'unreachable' },
  } as const
  const s = map[status]
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 6,
        padding: '2px 8px',
        background: s.bg,
        border: `1px solid ${s.border}`,
        borderRadius: 999,
        color: s.color,
        fontSize: 10.5,
        fontWeight: 500,
        textTransform: 'uppercase',
        letterSpacing: '0.05em',
      }}
    >
      {s.label}
    </span>
  )
}

function SourcePill({ source }: { source: DiscoverEntry['source'] }) {
  return (
    <span
      style={{
        fontSize: 10.5,
        color: 'var(--fg-3)',
        fontFamily: 'var(--font-mono)',
        background: 'var(--bg-3)',
        padding: '2px 7px',
        borderRadius: 4,
        border: '1px solid var(--line-1)',
      }}
    >
      {source}
    </span>
  )
}

function fmtRelative(ms: number): string {
  if (!ms) return '—'
  const diff = Date.now() - ms
  if (diff < 0) return 'just now'
  if (diff < 60_000) return `${Math.floor(diff / 1000)}s ago`
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`
  return `${Math.floor(diff / 86_400_000)}d ago`
}

function CenterMessage({ children }: { children: React.ReactNode }) {
  return (
    <div
      style={{
        background: 'var(--bg-1)',
        border: '1px solid var(--line-1)',
        borderRadius: 10,
        padding: 28,
        textAlign: 'center',
        color: 'var(--fg-3)',
        fontSize: 13,
      }}
    >
      {children}
    </div>
  )
}

function ErrorBanner({ children }: { children: React.ReactNode }) {
  return (
    <div
      className="mono"
      style={{
        margin: '12px 0',
        padding: 12,
        borderRadius: 8,
        background: 'var(--danger-soft)',
        border: '1px solid rgba(255,107,122,0.3)',
        color: 'var(--danger)',
        fontSize: 12,
      }}
    >
      {children}
    </div>
  )
}

function EmptyState() {
  return (
    <div
      style={{
        background: 'var(--bg-1)',
        border: '1px dashed var(--line-2)',
        borderRadius: 12,
        padding: 28,
        textAlign: 'center',
      }}
    >
      <div
        style={{
          width: 44,
          height: 44,
          borderRadius: 12,
          margin: '0 auto 14px',
          display: 'grid',
          placeItems: 'center',
          background: 'var(--accent-mute)',
          color: 'var(--accent)',
        }}
      >
        <Icon name="sparkles" size={18} />
      </div>
      <h3
        style={{
          margin: '0 0 6px',
          fontSize: 14,
          color: 'var(--fg-1)',
          fontWeight: 600,
          letterSpacing: '-0.01em',
        }}
      >
        No discovered Postgres services yet
      </h3>
      <p style={{ margin: '0 auto 14px', fontSize: 12, color: 'var(--fg-3)', maxWidth: 520 }}>
        DBil watches your Docker socket or a JSON config for compose services it can
        auto-import. Set <span className="mono" style={{ color: 'var(--fg-1)' }}>DBIL_DISCOVER</span>
        {' '}on the dbil container, label your Postgres service with{' '}
        <span className="mono" style={{ color: 'var(--fg-1)' }}>dbil.enable=true</span>, and
        restart.
      </p>
      <pre
        className="mono"
        style={{
          margin: '0 auto',
          maxWidth: 620,
          padding: 12,
          background: 'var(--bg-0)',
          border: '1px solid var(--line-1)',
          borderRadius: 8,
          fontSize: 11,
          color: 'var(--fg-2)',
          textAlign: 'left',
          lineHeight: 1.6,
          overflowX: 'auto',
        }}
      >
        <span style={{ color: 'var(--fg-4)' }}># dbil container env</span>
        {'\n'}DBIL_DISCOVER=docker{'\n'}DBIL_NETWORK=appnet{'\n\n'}
        <span style={{ color: 'var(--fg-4)' }}># postgres service labels</span>
        {'\n'}labels:{'\n'}{'  '}dbil.enable: <span style={{ color: 'var(--c-mint)' }}>"true"</span>{'\n'}
        {'  '}dbil.alias: <span style={{ color: 'var(--c-mint)' }}>"app"</span>{'\n'}
        {'  '}dbil.tag: <span style={{ color: 'var(--c-mint)' }}>"dev"</span>{'\n'}
        {'  '}dbil.creds.username_env: <span style={{ color: 'var(--c-mint)' }}>"POSTGRES_USER"</span>{'\n'}
        {'  '}dbil.creds.password_env: <span style={{ color: 'var(--c-mint)' }}>"POSTGRES_PASSWORD"</span>{'\n'}
        {'  '}dbil.creds.database_env: <span style={{ color: 'var(--c-mint)' }}>"POSTGRES_DB"</span>
      </pre>
    </div>
  )
}
