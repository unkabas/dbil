import { useState } from 'react'
import { useTerminateBackend, type LockChain, type LockSession } from '../../api/observ'
import { ApiError } from '../../api/client'
import type { Tag } from '../../api/connections'
import Icon from '../Icon'

interface Props {
  chains: LockChain[]
  loading?: boolean
  error?: string | null
  connID: number | null
  tag: Tag | null
}

export default function LockChainCard({ chains, loading, error, connID, tag }: Props) {
  return (
    <div
      style={{
        background: 'var(--bg-1)',
        border: '1px solid var(--line-1)',
        borderRadius: 'var(--radius)',
        overflow: 'hidden',
        boxShadow: 'var(--shadow-1)',
      }}
    >
      <div
        style={{
          padding: '12px 16px',
          borderBottom: '1px solid var(--line-1)',
          display: 'flex',
          alignItems: 'center',
          gap: 10,
        }}
      >
        <h2 style={{ margin: 0, fontSize: 13, fontWeight: 600, color: 'var(--fg-1)', letterSpacing: '-0.01em' }}>
          Lock chains
        </h2>
        <span className="tnum" style={{ fontSize: 11, color: 'var(--fg-4)' }}>
          {chains.length === 0 ? 'no contention' : `${chains.length} chain${chains.length === 1 ? '' : 's'}`}
        </span>
        <span
          className="mono"
          style={{ fontSize: 10.5, color: 'var(--fg-5)', letterSpacing: '0.02em' }}
          title="Live query: pg_stat_activity joined to pg_blocking_pids()"
        >
          · pg_stat_activity + pg_blocking_pids
        </span>
        <span style={{ flex: 1 }} />
        {!error && chains.length === 0 && (
          <span
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
              padding: '3px 9px',
              background: 'var(--ok-soft)',
              border: '1px solid rgba(52,211,153,0.3)',
              borderRadius: 999,
              color: 'var(--ok)',
              fontSize: 11,
              fontWeight: 500,
            }}
          >
            <span className="live-dot" style={{ width: 5, height: 5, borderRadius: 3, background: 'var(--ok)' }} />
            healthy
          </span>
        )}
      </div>

      {error ? (
        <div
          className="mono"
          style={{
            margin: 12,
            padding: 12,
            borderRadius: 8,
            background: 'var(--danger-soft)',
            border: '1px solid rgba(255,107,122,0.3)',
            color: 'var(--danger)',
            fontSize: 12,
            whiteSpace: 'pre-wrap',
          }}
        >
          {error}
        </div>
      ) : loading && chains.length === 0 ? (
        <div style={{ padding: 24, textAlign: 'center', color: 'var(--fg-3)', fontSize: 12.5 }}>
          Loading…
        </div>
      ) : chains.length === 0 ? (
        <div style={{ padding: 24, textAlign: 'center', color: 'var(--fg-3)', fontSize: 12.5 }}>
          No waiting chains right now.
        </div>
      ) : (
        <div style={{ padding: 12, display: 'flex', flexDirection: 'column', gap: 12 }}>
          {chains.map((c, i) => (
            <ChainRow key={i} chain={c} connID={connID} tag={tag} />
          ))}
        </div>
      )}
    </div>
  )
}

function ChainRow({ chain, connID, tag }: { chain: LockChain; connID: number | null; tag: Tag | null }) {
  const terminate = useTerminateBackend(connID)
  const [killError, setKillError] = useState<string | null>(null)
  const [pendingPID, setPendingPID] = useState<number | null>(null)

  const needsConfirm = tag === 'staging' || tag === 'production'

  const onKill = async (pid: number) => {
    if (connID === null) return
    const protectedPrompt =
      needsConfirm &&
      !window.confirm(
        `Send pg_terminate_backend(${pid}) on a ${tag} connection?\n\nThe session will be killed immediately.`,
      )
    if (protectedPrompt) return
    setKillError(null)
    setPendingPID(pid)
    try {
      await terminate.mutateAsync({ pid, confirm: needsConfirm })
    } catch (err) {
      if (err instanceof ApiError) {
        setKillError(err.body.reason || err.body.error || `HTTP ${err.status}`)
      } else if (err instanceof Error) {
        setKillError(err.message)
      } else {
        setKillError('Kill failed')
      }
    } finally {
      setPendingPID(null)
    }
  }

  return (
    <div
      style={{
        background: 'var(--bg-2)',
        border: '1px solid var(--line-1)',
        borderRadius: 8,
        padding: 12,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <Icon name="lock" size={14} style={{ color: 'var(--c-amber)' }} />
        <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--fg-1)' }}>Holder</span>
        <SessionLine s={chain.holder} />
        <span style={{ flex: 1 }} />
        <button
          className="btn-gh"
          style={{ color: 'var(--danger)' }}
          title={`pg_terminate_backend(${chain.holder.pid})`}
          disabled={connID === null || pendingPID === chain.holder.pid}
          onClick={() => onKill(chain.holder.pid)}
        >
          <Icon name="kill" size={12} />{' '}
          {pendingPID === chain.holder.pid ? 'Killing…' : 'Kill'}
        </button>
      </div>
      {killError && (
        <div
          className="mono"
          style={{
            marginTop: 8,
            padding: '6px 10px',
            borderRadius: 6,
            background: 'var(--danger-soft)',
            border: '1px solid rgba(255,107,122,0.3)',
            color: 'var(--danger)',
            fontSize: 11.5,
          }}
        >
          {killError}
        </div>
      )}
      <div style={{ marginTop: 10, marginLeft: 24, display: 'flex', flexDirection: 'column', gap: 6 }}>
        {chain.blocked.length === 0 && (
          <span style={{ fontSize: 11, color: 'var(--fg-4)' }}>(nothing waiting)</span>
        )}
        {chain.blocked.map((s) => (
          <div key={s.pid} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <span style={{ fontSize: 11, color: 'var(--fg-4)' }}>blocked</span>
            <SessionLine s={s} />
            <span className="mono tnum" style={{ marginLeft: 'auto', fontSize: 11, color: 'var(--warn)' }}>
              {fmtAge(s.age_ms)}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

function SessionLine({ s }: { s: LockSession }) {
  return (
    <span
      className="mono"
      style={{
        fontSize: 11.5,
        color: 'var(--fg-2)',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        flex: 1,
        minWidth: 0,
      }}
      title={s.query}
    >
      <span style={{ color: 'var(--fg-3)' }}>pid={s.pid}</span>
      {' '}
      <span style={{ color: 'var(--c-cyan)' }}>{s.user || 'anon'}</span>
      {' '}
      <span style={{ color: 'var(--fg-2)' }}>{s.query || '(no query)'}</span>
    </span>
  )
}

function fmtAge(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60_000).toFixed(1)}m`
}
