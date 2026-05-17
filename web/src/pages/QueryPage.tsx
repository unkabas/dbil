import { useState } from 'react'
import CodeMirror from '@uiw/react-codemirror'
import { sql, PostgreSQL } from '@codemirror/lang-sql'
import { darcula } from '@uiw/codemirror-theme-darcula'
import { EditorView } from '@codemirror/view'
import { useShellContext } from '../shell/context'
import { useExecuteQuery, type QueryResult } from '../api/connections'
import { ApiError } from '../api/client'
import { sampleSQL } from '../mock/data'
import Icon from '../components/Icon'

export default function QueryPage() {
  const { activeConn } = useShellContext()
  const [text, setText] = useState(sampleSQL)
  const [result, setResult] = useState<QueryResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [confirm, setConfirm] = useState(false)
  const [passphrase, setPassphrase] = useState('')
  const [needsPassphrase, setNeedsPassphrase] = useState(false)

  const execute = useExecuteQuery()

  if (!activeConn) {
    return (
      <div
        style={{
          height: '100%',
          display: 'grid',
          placeItems: 'center',
          color: 'var(--fg-3)',
          fontSize: 13,
        }}
      >
        Add a connection first to run queries.
      </div>
    )
  }

  const needsConfirm = activeConn.tag === 'production' || activeConn.tag === 'staging'
  const passphraseRequired = activeConn.requires_passphrase || needsPassphrase

  const run = async () => {
    if (execute.isPending) return
    setError(null)
    try {
      const r = await execute.mutateAsync({
        id: activeConn.id,
        sql: text,
        confirm,
        passphrase: passphrase || undefined,
      })
      setResult(r)
      setNeedsPassphrase(false)
    } catch (err) {
      setResult(null)
      if (err instanceof ApiError) {
        if (err.status === 428) {
          const msg = (err.body.error || '').toLowerCase()
          if (msg.includes('passphrase')) {
            setNeedsPassphrase(true)
            setError('Passphrase required for this connection')
          } else {
            setError(err.body.reason || 'Confirmation required (tick the checkbox)')
          }
        } else if (err.status === 401) setError('Invalid passphrase')
        else if (err.status === 403) setError(`Blocked by policy: ${err.body.reason || 'see the server reason'}`)
        else if (err.status === 504) setError('Statement timeout')
        else setError(err.body.error || `Query failed (${err.status})`)
      } else {
        setError(err instanceof Error ? err.message : 'Query failed')
      }
    }
  }

  return (
    <div
      style={{ display: 'grid', gridTemplateRows: '44px 1fr', height: '100%', minHeight: 0 }}
      onKeyDown={(e) => {
        if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
          e.preventDefault()
          void run()
        }
      }}
    >
      {/* Page title strip */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '0 18px',
          borderBottom: '1px solid var(--line-1)',
          background: 'var(--bg-0)',
        }}
      >
        <h1 style={{ fontSize: 14.5, fontWeight: 600, letterSpacing: '-0.02em', margin: 0 }}>Query</h1>
        <span style={{ fontSize: 12, color: 'var(--fg-4)' }}>·</span>
        <span className="mono" style={{ fontSize: 12, color: 'var(--fg-3)' }}>
          {activeConn.alias}
        </span>
        <span style={{ flex: 1 }} />
        {passphraseRequired && (
          <input
            type="password"
            value={passphrase}
            onChange={(e) => setPassphrase(e.target.value)}
            placeholder="connection passphrase"
            style={{
              width: 220,
              height: 28,
              padding: '0 10px',
              borderRadius: 7,
              background: 'var(--bg-2)',
              border: '1px solid var(--line-2)',
              color: 'var(--fg-1)',
              fontSize: 12,
              outline: 0,
              fontFamily: 'inherit',
            }}
          />
        )}
        {needsConfirm && (
          <label
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
              fontSize: 11.5,
              color: 'var(--fg-3)',
              cursor: 'pointer',
            }}
          >
            <input
              type="checkbox"
              checked={confirm}
              onChange={(e) => setConfirm(e.target.checked)}
              style={{ accentColor: 'var(--accent)' }}
            />
            <span>
              I understand this hits{' '}
              <span style={{ color: 'var(--fg-1)', fontWeight: 500 }}>{activeConn.tag}</span>
            </span>
          </label>
        )}
        <button className="btn-pri" onClick={() => void run()} disabled={execute.isPending}>
          {execute.isPending ? (
            <>
              <Spinner />
              <span>Running…</span>
            </>
          ) : (
            <>
              <Icon name="play" size={11} />
              <span>Execute</span>
              <span style={{ fontSize: 10, color: 'rgba(255,255,255,0.7)' }}>⌘⏎</span>
            </>
          )}
        </button>
      </div>

      {/* Editor + result */}
      <div
        style={{
          display: 'grid',
          gridTemplateRows: 'minmax(180px, 38%) 1fr',
          minHeight: 0,
        }}
      >
        <div style={{ borderBottom: '1px solid var(--line-1)', overflow: 'hidden', background: 'var(--bg-0)' }}>
          <CodeMirror
            value={text}
            onChange={setText}
            theme={darcula}
            height="100%"
            extensions={[
              sql({ dialect: PostgreSQL, upperCaseKeywords: true }),
              EditorView.lineWrapping,
            ]}
            basicSetup={{
              lineNumbers: true,
              foldGutter: true,
              highlightActiveLine: true,
              highlightActiveLineGutter: true,
            }}
          />
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', minHeight: 0, background: 'var(--bg-1)' }}>
          <div
            style={{
              height: 32,
              padding: '0 14px',
              display: 'flex',
              alignItems: 'center',
              gap: 12,
              borderBottom: '1px solid var(--line-1)',
              fontSize: 11.5,
            }}
          >
            <span style={{ color: 'var(--fg-2)', fontWeight: 500 }}>Result</span>
            {result && (
              <>
                <Dot />
                <span className="mono" style={{ color: 'var(--fg-3)' }}>{result.command_tag}</span>
                <Dot />
                <span className="mono tnum" style={{ color: 'var(--fg-3)' }}>{result.duration_ms} ms</span>
                {result.rows.length > 0 && (
                  <>
                    <Dot />
                    <span className="mono tnum" style={{ color: 'var(--fg-3)' }}>{result.rows.length} rows</span>
                  </>
                )}
                {result.truncated && (
                  <>
                    <Dot />
                    <span style={{ color: 'var(--warn)' }}>truncated</span>
                  </>
                )}
              </>
            )}
          </div>
          <div style={{ flex: 1, overflow: 'auto', background: 'var(--bg-0)' }}>
            {execute.isPending && <CenterMessage>Executing…</CenterMessage>}
            {!execute.isPending && error && <ErrorBanner text={error} />}
            {!execute.isPending && !error && result && <ResultGrid result={result} />}
            {!execute.isPending && !error && !result && (
              <CenterMessage>Press Execute (⌘⏎) to run the query.</CenterMessage>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function ResultGrid({ result }: { result: QueryResult }) {
  if (result.rows.length === 0) {
    return (
      <div
        className="mono"
        style={{ padding: 18, fontSize: 12, color: 'var(--fg-3)' }}
      >
        {result.command_tag} — {result.rows_affected} {result.rows_affected === 1 ? 'row' : 'rows'} affected
      </div>
    )
  }
  return (
    <table className="tbl">
      <thead>
        <tr>
          <th style={{ width: 48, textAlign: 'right' }}>#</th>
          {result.columns.map((c, i) => (
            <th key={i}>
              <div style={{ color: 'var(--fg-2)' }}>{c.name}</div>
              <div style={{ fontSize: 10, color: 'var(--c-violet)', fontStyle: 'italic', textTransform: 'none', letterSpacing: 0 }}>
                {c.type_name}
              </div>
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {result.rows.map((row, i) => (
          <tr key={i}>
            <td className="tnum" style={{ textAlign: 'right', color: 'var(--fg-4)' }}>{i + 1}</td>
            {row.map((cell, ci) => (
              <td key={ci} title={String(cell ?? '')}>
                {renderCell(cell)}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function renderCell(v: unknown) {
  if (v === null || v === undefined)
    return <span style={{ color: 'var(--fg-5)', fontStyle: 'italic' }}>null</span>
  if (typeof v === 'boolean')
    return <span style={{ color: v ? 'var(--ok)' : 'var(--danger)' }}>{String(v)}</span>
  if (typeof v === 'number')
    return <span style={{ color: 'var(--c-cyan)' }} className="tnum">{String(v)}</span>
  return <span style={{ color: 'var(--fg-1)' }}>{String(v)}</span>
}

function ErrorBanner({ text }: { text: string }) {
  return (
    <div
      className="mono"
      style={{
        margin: 16,
        padding: 12,
        borderRadius: 8,
        background: 'var(--danger-soft)',
        border: '1px solid rgba(255,107,122,0.3)',
        color: 'var(--danger)',
        fontSize: 12,
        whiteSpace: 'pre-wrap',
      }}
    >
      {text}
    </div>
  )
}

function CenterMessage({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ height: '100%', display: 'grid', placeItems: 'center', color: 'var(--fg-4)', fontSize: 12 }}>
      {children}
    </div>
  )
}

function Spinner() {
  return (
    <svg className="animate-spin" width="12" height="12" viewBox="0 0 16 16" fill="none">
      <circle cx="8" cy="8" r="6" stroke="currentColor" strokeOpacity="0.3" strokeWidth="2" />
      <path d="M14 8a6 6 0 0 1-6 6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  )
}

function Dot() {
  return <span style={{ width: 3, height: 3, borderRadius: 2, background: 'var(--line-3)' }} />
}
