import { useState } from 'react'
import CodeMirror from '@uiw/react-codemirror'
import { sql, PostgreSQL } from '@codemirror/lang-sql'
import { darcula } from '@uiw/codemirror-theme-darcula'
import { EditorView } from '@codemirror/view'
import { mockConnections, mockResultFor, sampleSQL, type MockResult } from '../mock/data'
import Icon from '../components/Icon'

interface Props {
  activeConnID: number
}

export default function QueryPage({ activeConnID }: Props) {
  const conn = mockConnections.find((c) => c.id === activeConnID) ?? mockConnections[0]
  const [text, setText] = useState(sampleSQL)
  const [result, setResult] = useState<MockResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [running, setRunning] = useState(false)
  const [confirm, setConfirm] = useState(false)

  const needsConfirm = conn.tag === 'production' || conn.tag === 'staging'

  const run = () => {
    if (running) return
    if (needsConfirm && !confirm) {
      setResult(null)
      setError(
        conn.tag === 'production'
          ? 'production: confirmation required (tick the box) — DDL is blocked outright by policy'
          : 'staging: confirmation required (tick the box)',
      )
      return
    }
    setRunning(true)
    setError(null)
    setTimeout(() => {
      setResult(mockResultFor(text))
      setRunning(false)
    }, 250 + Math.random() * 400)
  }

  return (
    <div
      className="h-full flex flex-col bg-app-grad"
      onKeyDown={(e) => {
        if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
          e.preventDefault()
          run()
        }
      }}
    >
      <div className="px-6 pt-6 pb-4">
        <div className="flex items-center justify-between mb-3">
          <h1 className="text-[22px] font-semibold text-ink-50 tracking-tight">Query</h1>
          <div className="flex items-center gap-3">
            {needsConfirm && (
              <label className="flex items-center gap-2 text-ink-300 text-[12px] cursor-pointer">
                <input
                  type="checkbox"
                  checked={confirm}
                  onChange={(e) => setConfirm(e.target.checked)}
                  className="accent-violet"
                />
                <span>
                  I understand this hits <span className="text-ink-100 font-medium">{conn.tag}</span>
                </span>
              </label>
            )}
            <button
              onClick={run}
              disabled={running}
              className="h-9 px-4 rounded-md bg-violet text-white font-medium text-[13px] flex items-center gap-2 hover:bg-violet-deep transition-colors shadow-glow disabled:opacity-50 disabled:shadow-none"
            >
              {running ? (
                <>
                  <Spinner />
                  <span>Running…</span>
                </>
              ) : (
                <>
                  <Icon name="play" className="w-3.5 h-3.5" fill="currentColor" />
                  <span>Execute</span>
                  <span className="text-[11px] text-white/70 font-normal ml-1">⌘⏎</span>
                </>
              )}
            </button>
          </div>
        </div>

        <div className="h-72 rounded-xl border border-ink-700 overflow-hidden shadow-card">
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
      </div>

      <div className="flex-1 min-h-0 px-6 pb-6">
        <div className="h-full rounded-xl border border-ink-700 overflow-hidden bg-ink-900/50 shadow-card flex flex-col">
          <div className="h-9 px-3 flex items-center gap-3 border-b border-ink-700 text-[12px]">
            <span className="text-ink-100 font-medium">Result</span>
            {result && (
              <>
                <Dot />
                <span className="text-ink-300">{result.commandTag}</span>
                <Dot />
                <span className="text-ink-300">{result.durationMs} ms</span>
                {result.rows.length > 0 && (
                  <>
                    <Dot />
                    <span className="text-ink-300">{result.rows.length} rows</span>
                  </>
                )}
              </>
            )}
          </div>
          <div className="flex-1 overflow-auto">
            {running && <CenterMessage>Executing…</CenterMessage>}
            {!running && error && <ErrorBanner text={error} />}
            {!running && !error && result && <ResultGrid result={result} />}
            {!running && !error && !result && (
              <CenterMessage>Press Execute (⌘⏎) to run the query.</CenterMessage>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function ResultGrid({ result }: { result: MockResult }) {
  if (result.rows.length === 0) {
    return (
      <div className="p-6 text-ink-300 text-[13px] font-mono">
        {result.commandTag} — {result.rowsAffected} {result.rowsAffected === 1 ? 'row' : 'rows'} affected
      </div>
    )
  }
  return (
    <table className="font-mono text-[12.5px] border-collapse w-full">
      <thead className="sticky top-0 z-10 bg-ink-900">
        <tr>
          <th className="w-12 px-3 py-2 text-right text-ink-400 border-b border-ink-700 font-normal">
            #
          </th>
          {result.columns.map((c, i) => (
            <th
              key={i}
              className="px-3 py-2 text-left border-b border-ink-700 font-medium text-ink-100"
            >
              <div>{c.name}</div>
              <div className="text-[10.5px] text-accent-lilac font-normal italic">
                {c.typeName}
              </div>
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {result.rows.map((row, i) => (
          <tr key={i} className="hover:bg-violet/5 transition-colors">
            <td className="px-3 py-1.5 text-ink-400 text-right border-b border-ink-800">
              {i + 1}
            </td>
            {row.map((cell, ci) => (
              <td
                key={ci}
                className="px-3 py-1.5 border-b border-ink-800 truncate max-w-[280px]"
                title={String(cell ?? '')}
              >
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
  if (v === null || v === undefined) return <span className="text-ink-400 italic">null</span>
  if (typeof v === 'boolean') return <span className="text-accent-lime">{String(v)}</span>
  if (typeof v === 'number') return <span className="text-accent-sky">{String(v)}</span>
  return <span className="text-ink-100">{String(v)}</span>
}

function ErrorBanner({ text }: { text: string }) {
  return (
    <div className="m-4 p-3 rounded-lg bg-accent-coral/10 border border-accent-coral/40 text-accent-coral text-[12.5px] font-mono">
      {text}
    </div>
  )
}

function CenterMessage({ children }: { children: React.ReactNode }) {
  return (
    <div className="h-full flex items-center justify-center text-ink-400 text-[12.5px]">
      {children}
    </div>
  )
}

function Spinner() {
  return (
    <svg className="w-3.5 h-3.5 animate-spin" viewBox="0 0 16 16" fill="none">
      <circle cx="8" cy="8" r="6" stroke="currentColor" strokeOpacity="0.3" strokeWidth="2" />
      <path d="M14 8a6 6 0 0 1-6 6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  )
}

function Dot() {
  return <span className="w-1 h-1 rounded-full bg-ink-600" />
}
