import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useShellContext } from '../shell/context'
import { useSchema, type SchemaTable, type SchemaColumn, type SchemaIndex } from '../api/schema'
import { ApiError } from '../api/client'
import Icon from '../components/Icon'

// Three-pane Schema screen.
//   Left  — filter + schema-grouped table list.
//   Center — auto-laid-out ER cards with SVG FK connectors.
//   Right — focused-table detail pane: stats, columns, FK in/out, DDL preview.
// Schema data comes from /api/connections/{id}/schema; positions are
// computed locally (3-column grid).

const TABLE_W = 280
const ROW_H = 26
const HEADER_H = 46
const COL_GAP = 60
const ROW_GAP = 80
const CANVAS_PAD = 60
const COLS = 3

interface PositionedTable extends SchemaTable {
  pos: { x: number; y: number }
}

export default function SchemaPage() {
  const { activeConnID, activeConn } = useShellContext()
  const { data, isLoading, error } = useSchema(activeConnID)

  const flatTables = useMemo<SchemaTable[]>(() => {
    if (!data) return []
    const flat: SchemaTable[] = []
    for (const s of data.schemas) flat.push(...s.tables)
    return flat
  }, [data])

  const [zoom, setZoom] = useState(1)
  const zoomRef = useRef(zoom)
  useEffect(() => {
    zoomRef.current = zoom
  }, [zoom])

  const [positions, setPositions, resetPositions] = useTablePositions(activeConnID, flatTables)

  const positioned = useMemo<PositionedTable[]>(() => {
    return flatTables.map((t, i) => {
      const k = `${t.schema}.${t.name}`
      const stored = positions[k]
      return {
        ...t,
        pos: stored ?? defaultGridPos(i, t.columns.length),
      }
    })
  }, [flatTables, positions])

  const movePosition = useCallback(
    (key: string, dx: number, dy: number) => {
      setPositions((prev) => {
        const idx = flatTables.findIndex((t) => `${t.schema}.${t.name}` === key)
        const base = prev[key] ?? defaultGridPos(idx, flatTables[idx]?.columns.length ?? 0)
        return {
          ...prev,
          [key]: { x: Math.max(0, base.x + dx), y: Math.max(0, base.y + dy) },
        }
      })
    },
    [flatTables, setPositions],
  )

  const snapPosition = useCallback(
    (key: string) => {
      setPositions((prev) => {
        const p = prev[key]
        if (!p) return prev
        return { ...prev, [key]: { x: Math.round(p.x / 10) * 10, y: Math.round(p.y / 10) * 10 } }
      })
    },
    [setPositions],
  )

  const [focused, setFocused] = useState<string | null>(null)
  const [hovered, setHovered] = useState<string | null>(null)
  const [filter, setFilter] = useState('')

  const focusedKey = focused ?? (positioned[0] ? keyOf(positioned[0]) : null)

  const filtered = useMemo(() => {
    if (!filter.trim()) return positioned
    const f = filter.toLowerCase()
    return positioned.filter(
      (t) =>
        t.name.toLowerCase().includes(f) ||
        t.schema.toLowerCase().includes(f) ||
        t.columns.some((c) => c.name.toLowerCase().includes(f)),
    )
  }, [positioned, filter])

  const focusedTable = useMemo(
    () => positioned.find((t) => keyOf(t) === focusedKey),
    [focusedKey, positioned],
  )

  const edges = useMemo(() => {
    const out: Array<{ fromKey: string; fromCol: string; toKey: string; toCol: string }> = []
    for (const t of positioned) {
      for (const c of t.columns) {
        if (!c.fk) continue
        const target = positioned.find((x) => x.name === c.fk!.table)
        if (!target) continue
        out.push({
          fromKey: keyOf(t),
          fromCol: c.name,
          toKey: keyOf(target),
          toCol: c.fk.column,
        })
      }
    }
    return out
  }, [positioned])

  const highlightSet = useMemo(() => {
    const target = hovered ?? focusedKey
    if (!target) return null
    const set = new Set([target])
    for (const e of edges) {
      if (e.fromKey === target) set.add(e.toKey)
      if (e.toKey === target) set.add(e.fromKey)
    }
    return set
  }, [hovered, focusedKey, edges])

  const totalRows = positioned.reduce((a, t) => a + Math.max(0, displayRows(t)), 0)
  const anyEstimated = positioned.some((t) => t.rows_estimated && t.rows >= 0)
  const totalCols = positioned.reduce((a, t) => a + t.columns.length, 0)
  const schemas = uniqueSchemas(filtered)

  const canvasH = useMemo(() => {
    let maxY = 0
    for (const t of positioned) {
      const cardH = HEADER_H + Math.max(1, t.columns.length) * ROW_H
      maxY = Math.max(maxY, t.pos.y + cardH)
    }
    return Math.max(640, maxY + CANVAS_PAD)
  }, [positioned])

  const canvasW = useMemo(() => {
    let maxX = CANVAS_PAD * 2 + COLS * TABLE_W + (COLS - 1) * COL_GAP
    for (const t of positioned) maxX = Math.max(maxX, t.pos.x + TABLE_W + CANVAS_PAD)
    return maxX
  }, [positioned])

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '260px 1fr 320px', height: '100%', minHeight: 0 }}>
      {/* ── Left ──────────────────────────────────────────────────── */}
      <div
        style={{
          borderRight: '1px solid var(--line-1)',
          background: 'var(--bg-1)',
          display: 'flex',
          flexDirection: 'column',
          minHeight: 0,
        }}
      >
        <div style={{ padding: '14px 14px 10px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
            <span
              style={{
                fontSize: 11,
                fontWeight: 500,
                color: 'var(--fg-3)',
                letterSpacing: '0.06em',
                textTransform: 'uppercase',
              }}
            >
              Tables
            </span>
            <span className="mono tnum" style={{ fontSize: 10.5, color: 'var(--fg-4)' }}>
              {positioned.length}
            </span>
          </div>
          <div style={{ position: 'relative' }}>
            <Icon
              name="search"
              size={12}
              style={{ position: 'absolute', left: 9, top: 8, color: 'var(--fg-4)' }}
            />
            <input
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Filter tables, columns…"
              style={{
                width: '100%',
                height: 28,
                background: 'var(--bg-2)',
                border: '1px solid var(--line-1)',
                borderRadius: 7,
                padding: '0 8px 0 27px',
                color: 'var(--fg-1)',
                fontSize: 12,
                outline: 0,
              }}
            />
          </div>
        </div>

        <div style={{ flex: 1, overflow: 'auto', padding: '0 8px 12px' }}>
          {isLoading && positioned.length === 0 && (
            <div style={{ padding: 14, color: 'var(--fg-3)', fontSize: 12 }}>Loading schema…</div>
          )}
          {error && <SidebarError err={error} />}
          {schemas.map((sch) => {
            const list = filtered.filter((t) => t.schema === sch)
            if (list.length === 0) return null
            return (
              <div key={sch} style={{ marginBottom: 12 }}>
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 8,
                    padding: '6px 6px 6px 8px',
                    fontSize: 11.5,
                    color: 'var(--fg-3)',
                    fontFamily: 'var(--font-mono)',
                  }}
                >
                  <Icon name="branch" size={11} />
                  <span style={{ fontWeight: 500 }}>{sch}</span>
                  <span className="tnum" style={{ color: 'var(--fg-5)', fontSize: 10.5 }}>
                    {list.length}
                  </span>
                </div>
                {list.map((t) => {
                  const key = keyOf(t)
                  const isFocused = focusedKey === key
                  return (
                    <button
                      key={key}
                      onClick={() => setFocused(key)}
                      onMouseEnter={() => setHovered(key)}
                      onMouseLeave={() => setHovered(null)}
                      style={{
                        width: '100%',
                        display: 'flex',
                        alignItems: 'center',
                        gap: 8,
                        padding: '5px 8px',
                        borderRadius: 6,
                        background: isFocused ? 'var(--accent-mute)' : 'transparent',
                        color: isFocused ? 'var(--fg-1)' : 'var(--fg-2)',
                        marginBottom: 1,
                        cursor: 'pointer',
                        border: 0,
                        fontFamily: 'inherit',
                      }}
                    >
                      <Icon name="table" size={11} style={{ color: isFocused ? 'var(--accent)' : 'var(--fg-4)' }} />
                      <span
                        className="mono"
                        style={{
                          fontSize: 12,
                          fontWeight: isFocused ? 500 : 400,
                          flex: 1,
                          textAlign: 'left',
                        }}
                      >
                        {t.name}
                      </span>
                      <span className="mono tnum" style={{ fontSize: 10.5, color: 'var(--fg-4)' }} title={rowsTooltip(t)}>
                        {rowsLabel(t)}
                      </span>
                    </button>
                  )
                })}
              </div>
            )
          })}
        </div>

        <div
          style={{
            borderTop: '1px solid var(--line-1)',
            padding: '10px 14px',
            display: 'flex',
            gap: 8,
            fontSize: 11,
            color: 'var(--fg-3)',
          }}
        >
          <button className="link-btn">
            <Icon name="refresh" size={12} style={{ marginRight: 4, verticalAlign: -1 }} />
            Refresh
          </button>
          <span style={{ flex: 1 }} />
          <span className="mono tnum" style={{ color: 'var(--fg-4)' }}>
            pg_catalog
          </span>
        </div>
      </div>

      {/* ── Center: ER canvas ─────────────────────────────────────── */}
      <div className="app-bg" style={{ position: 'relative', overflow: 'auto', background: 'var(--bg-0)' }}>
        <div
          aria-hidden
          style={{
            position: 'absolute',
            inset: 0,
            backgroundImage: 'radial-gradient(rgba(255,255,255,0.025) 1px, transparent 1px)',
            backgroundSize: '22px 22px',
            backgroundPosition: 'center',
            pointerEvents: 'none',
          }}
        />

        <div
          style={{
            position: 'sticky',
            top: 0,
            zIndex: 5,
            height: 44,
            background: 'linear-gradient(to bottom, var(--bg-0) 60%, transparent)',
            display: 'flex',
            alignItems: 'center',
            gap: 10,
            padding: '0 18px',
          }}
        >
          <h1 style={{ fontSize: 14.5, fontWeight: 600, letterSpacing: '-0.02em', color: 'var(--fg-1)', margin: 0 }}>
            Schema
          </h1>
          <span style={{ fontSize: 12, color: 'var(--fg-4)' }}>·</span>
          <span className="tnum" style={{ fontSize: 12, color: 'var(--fg-3)' }}>
            {activeConn?.alias ?? '—'} · {positioned.length} tables · {totalCols} columns ·{' '}
            {anyEstimated ? '~' : ''}
            {fmtCount(totalRows)} rows
          </span>
          <span style={{ flex: 1 }} />
          <button
            onClick={resetPositions}
            disabled={Object.keys(positions).length === 0}
            title="Reset table positions to the default grid"
            style={{
              height: 28,
              padding: '0 10px',
              background: 'var(--bg-2)',
              border: '1px solid var(--line-2)',
              borderRadius: 7,
              color: 'var(--fg-2)',
              fontSize: 11,
              cursor: Object.keys(positions).length === 0 ? 'default' : 'pointer',
              opacity: Object.keys(positions).length === 0 ? 0.4 : 1,
              fontFamily: 'inherit',
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
            }}
          >
            <Icon name="refresh" size={11} />
            Reset layout
          </button>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              background: 'var(--bg-2)',
              border: '1px solid var(--line-2)',
              borderRadius: 7,
              height: 28,
            }}
          >
            <button
              onClick={() => setZoom((z) => Math.max(0.5, z - 0.1))}
              style={{ width: 28, height: 28, color: 'var(--fg-3)', border: 0, background: 'none', cursor: 'pointer' }}
            >
              –
            </button>
            <span
              className="mono tnum"
              style={{
                fontSize: 11,
                color: 'var(--fg-2)',
                width: 40,
                textAlign: 'center',
                borderLeft: '1px solid var(--line-2)',
                borderRight: '1px solid var(--line-2)',
                height: '100%',
                lineHeight: '28px',
              }}
            >
              {Math.round(zoom * 100)}%
            </span>
            <button
              onClick={() => setZoom((z) => Math.min(1.6, z + 0.1))}
              style={{ width: 28, height: 28, color: 'var(--fg-3)', border: 0, background: 'none', cursor: 'pointer' }}
            >
              +
            </button>
          </div>
        </div>

        {error && <CanvasError err={error} />}
        {!error && positioned.length === 0 && !isLoading && (
          <div style={{ padding: 40, textAlign: 'center', color: 'var(--fg-3)', fontSize: 13 }}>
            No user-visible tables in this database yet.
          </div>
        )}

        <div
          style={{
            position: 'relative',
            width: canvasW,
            height: canvasH,
            transform: `scale(${zoom})`,
            transformOrigin: '0 0',
            margin: '0 24px 24px',
          }}
        >
          <ErEdges tables={positioned} edges={edges} highlightSet={highlightSet} canvasW={canvasW} canvasH={canvasH} />

          {positioned.map((t) => (
            <TableNode
              key={keyOf(t)}
              t={t}
              focused={focusedKey === keyOf(t)}
              dimmed={!!highlightSet && !highlightSet.has(keyOf(t))}
              highlighted={!!highlightSet?.has(keyOf(t))}
              onClick={() => setFocused(keyOf(t))}
              onMouseEnter={() => setHovered(keyOf(t))}
              onMouseLeave={() => setHovered(null)}
              onDrag={(dx, dy) => movePosition(keyOf(t), dx, dy)}
              onDragEnd={() => snapPosition(keyOf(t))}
              zoomRef={zoomRef}
            />
          ))}
        </div>
      </div>

      {/* ── Right: detail pane ────────────────────────────────────── */}
      <div
        style={{
          borderLeft: '1px solid var(--line-1)',
          background: 'var(--bg-1)',
          overflow: 'auto',
        }}
      >
        {focusedTable && <TableDetail t={focusedTable} edges={edges} />}
      </div>
    </div>
  )
}

function keyOf(t: SchemaTable): string {
  return `${t.schema}.${t.name}`
}

function uniqueSchemas(tables: SchemaTable[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const t of tables) {
    if (!seen.has(t.schema)) {
      seen.add(t.schema)
      out.push(t.schema)
    }
  }
  return out
}

function fmtCount(n: number): string {
  if (n < 0) return '?'
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1).replace(/\.0$/, '') + 'k'
  return String(n)
}

// displayRows picks the best number to render for a SchemaTable. Backend
// already populates `rows` with the exact count when it has one; this helper
// exists so callers can keep using the field directly while we also tolerate
// servers that only fill `rows_exact`.
function displayRows(t: SchemaTable): number {
  if (typeof t.rows_exact === 'number') return t.rows_exact
  return t.rows
}

// rowsLabel formats a row count with a tilde prefix when the backend marked
// it as an estimate (pg_class.reltuples). For never-analyzed tables (rows<0)
// it returns "—".
function rowsLabel(t: SchemaTable): string {
  const n = displayRows(t)
  if (n < 0) return '—'
  return (t.rows_estimated ? '~' : '') + fmtCount(n)
}

function rowsTooltip(t: SchemaTable): string {
  const n = displayRows(t)
  if (n < 0) return 'Never analyzed — run ANALYZE to populate statistics.'
  if (!t.rows_estimated) return `Exact count (${n.toLocaleString('en-US')})`
  const fresh = t.last_analyze ? ` Last analyzed ${t.last_analyze}.` : ''
  return `Estimate from pg_class.reltuples.${fresh} Run ANALYZE for a fresh estimate.`
}

function fmtBytes(b: number): string {
  if (b >= 1 << 30) return (b / (1 << 30)).toFixed(1) + ' GB'
  if (b >= 1 << 20) return (b / (1 << 20)).toFixed(1) + ' MB'
  if (b >= 1 << 10) return (b / (1 << 10)).toFixed(1) + ' KB'
  return b + ' B'
}

function errMsg(err: unknown): string {
  if (err instanceof ApiError) return err.body.reason || err.body.error || `HTTP ${err.status}`
  if (err instanceof Error) return err.message
  return String(err)
}

function SidebarError({ err }: { err: unknown }) {
  return (
    <div
      className="mono"
      style={{
        margin: 10,
        padding: 10,
        borderRadius: 8,
        background: 'var(--danger-soft)',
        border: '1px solid rgba(255,107,122,0.3)',
        color: 'var(--danger)',
        fontSize: 11.5,
        whiteSpace: 'pre-wrap',
      }}
    >
      {errMsg(err)}
    </div>
  )
}

function CanvasError({ err }: { err: unknown }) {
  return (
    <div
      className="mono"
      style={{
        margin: '12px 24px',
        padding: 14,
        borderRadius: 8,
        background: 'var(--danger-soft)',
        border: '1px solid rgba(255,107,122,0.3)',
        color: 'var(--danger)',
        fontSize: 12,
      }}
    >
      schema fetch failed: {errMsg(err)}
    </div>
  )
}

// ── ER edges (SVG path between table cards) ──────────────────────
function ErEdges({
  tables,
  edges,
  highlightSet,
  canvasW,
  canvasH,
}: {
  tables: PositionedTable[]
  edges: Array<{ fromKey: string; fromCol: string; toKey: string; toCol: string }>
  highlightSet: Set<string> | null
  canvasW: number
  canvasH: number
}) {
  return (
    <svg width={canvasW} height={canvasH} style={{ position: 'absolute', inset: 0, pointerEvents: 'none' }}>
      <defs>
        <marker id="arrow-soft" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">
          <path d="M0,0 L10,5 L0,10 z" fill="var(--fg-4)" />
        </marker>
        <marker id="arrow-hot" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto">
          <path d="M0,0 L10,5 L0,10 z" fill="var(--accent)" />
        </marker>
      </defs>
      {edges.map((e, i) => {
        const from = tables.find((t) => keyOf(t) === e.fromKey)
        const to = tables.find((t) => keyOf(t) === e.toKey)
        if (!from || !to) return null
        const hot = !!highlightSet && highlightSet.has(e.fromKey) && highlightSet.has(e.toKey)
        const dim = !!highlightSet && !hot

        const fromColIdx = from.columns.findIndex((c) => c.name === e.fromCol)
        const fromY = from.pos.y + HEADER_H + Math.max(0, fromColIdx) * ROW_H + ROW_H / 2
        const toColIdx = to.columns.findIndex((c) => c.name === e.toCol)
        const toY = to.pos.y + HEADER_H + Math.max(0, toColIdx) * ROW_H + ROW_H / 2

        const fromXAdj = to.pos.x > from.pos.x ? from.pos.x + TABLE_W : from.pos.x
        const toX = to.pos.x > from.pos.x ? to.pos.x : to.pos.x + TABLE_W
        const dx = Math.abs(toX - fromXAdj)
        const c1x = fromXAdj + Math.max(40, dx * 0.4) * (toX > fromXAdj ? 1 : -1)
        const c2x = toX - Math.max(40, dx * 0.4) * (toX > fromXAdj ? 1 : -1)

        return (
          <g key={i} style={{ opacity: dim ? 0.18 : 1, transition: 'opacity 180ms' }}>
            <path
              d={`M ${fromXAdj} ${fromY} C ${c1x} ${fromY}, ${c2x} ${toY}, ${toX} ${toY}`}
              stroke={hot ? 'var(--accent)' : 'var(--line-3)'}
              strokeWidth={hot ? 1.6 : 1.2}
              fill="none"
              markerEnd={hot ? 'url(#arrow-hot)' : 'url(#arrow-soft)'}
            />
            {hot && (
              <circle cx={fromXAdj} cy={fromY} r={3} fill="var(--accent)">
                <animate attributeName="r" values="3;5;3" dur="2s" repeatCount="indefinite" />
                <animate attributeName="opacity" values="1;0.5;1" dur="2s" repeatCount="indefinite" />
              </circle>
            )}
          </g>
        )
      })}
    </svg>
  )
}

function TableNode({
  t,
  focused,
  dimmed,
  highlighted,
  onClick,
  onMouseEnter,
  onMouseLeave,
  onDrag,
  onDragEnd,
  zoomRef,
}: {
  t: PositionedTable
  focused: boolean
  dimmed: boolean
  highlighted: boolean
  onClick(): void
  onMouseEnter(): void
  onMouseLeave(): void
  onDrag(dx: number, dy: number): void
  onDragEnd(): void
  zoomRef: React.MutableRefObject<number>
}) {
  const dragging = useRef<{ x: number; y: number; moved: boolean } | null>(null)
  const rafPending = useRef(false)
  const pendingDelta = useRef({ dx: 0, dy: 0 })

  const onPointerDown = useCallback(
    (e: React.MouseEvent) => {
      // Only drag with primary button on the header — let column-row clicks
      // through so users can still interact with the inner content.
      if (e.button !== 0) return
      dragging.current = { x: e.clientX, y: e.clientY, moved: false }
      const flush = () => {
        rafPending.current = false
        if (pendingDelta.current.dx === 0 && pendingDelta.current.dy === 0) return
        onDrag(pendingDelta.current.dx, pendingDelta.current.dy)
        pendingDelta.current = { dx: 0, dy: 0 }
      }
      const onMove = (ev: MouseEvent) => {
        const cur = dragging.current
        if (!cur) return
        const z = zoomRef.current || 1
        const dx = (ev.clientX - cur.x) / z
        const dy = (ev.clientY - cur.y) / z
        if (!cur.moved && Math.abs(dx) + Math.abs(dy) < 3) return
        cur.moved = true
        cur.x = ev.clientX
        cur.y = ev.clientY
        pendingDelta.current.dx += dx
        pendingDelta.current.dy += dy
        if (!rafPending.current) {
          rafPending.current = true
          requestAnimationFrame(flush)
        }
      }
      const onUp = () => {
        const cur = dragging.current
        dragging.current = null
        window.removeEventListener('mousemove', onMove)
        window.removeEventListener('mouseup', onUp)
        if (cur?.moved) onDragEnd()
      }
      window.addEventListener('mousemove', onMove)
      window.addEventListener('mouseup', onUp)
    },
    [onDrag, onDragEnd, zoomRef],
  )

  const handleClick = useCallback(() => {
    // Suppress click if we just finished a drag (mouseup already fired).
    if (dragging.current === null && !rafPending.current) onClick()
    else if (!dragging.current) onClick()
  }, [onClick])

  return (
    <div
      onClick={handleClick}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      style={{
        position: 'absolute',
        left: t.pos.x,
        top: t.pos.y,
        width: TABLE_W,
        background: 'var(--bg-1)',
        border: '1px solid ' + (focused ? 'var(--accent)' : highlighted ? 'var(--line-3)' : 'var(--line-1)'),
        borderRadius: 10,
        boxShadow: focused
          ? '0 0 0 1px var(--accent-mute), 0 18px 36px -8px rgba(0,0,0,0.55), 0 0 32px -8px var(--accent-glow)'
          : highlighted
            ? '0 8px 18px -6px rgba(0,0,0,0.5)'
            : 'var(--shadow-1)',
        opacity: dimmed ? 0.4 : 1,
        transition: 'opacity 180ms, border-color 180ms, box-shadow 180ms',
        overflow: 'hidden',
        userSelect: 'none',
      }}
    >
      <div
        onMouseDown={onPointerDown}
        style={{
          height: HEADER_H,
          padding: '10px 12px',
          borderBottom: '1px solid var(--line-1)',
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          background: focused ? 'linear-gradient(to bottom, var(--accent-mute), transparent)' : 'transparent',
          cursor: 'grab',
        }}
      >
        <Icon name="table" size={12} style={{ color: focused ? 'var(--accent)' : 'var(--fg-3)' }} />
        <span
          className="mono"
          style={{ fontSize: 12.5, fontWeight: 600, letterSpacing: '-0.01em', color: 'var(--fg-1)' }}
        >
          {t.name}
        </span>
        {t.schema !== 'public' && (
          <span
            style={{
              fontSize: 9.5,
              color: 'var(--fg-3)',
              fontFamily: 'var(--font-mono)',
              background: 'var(--bg-3)',
              padding: '1px 5px',
              borderRadius: 3,
            }}
          >
            {t.schema}
          </span>
        )}
        <span style={{ flex: 1 }} />
        <span className="mono tnum" style={{ fontSize: 10.5, color: 'var(--fg-4)' }} title={rowsTooltip(t)}>
          {rowsLabel(t)}
        </span>
      </div>
      <div>
        {t.columns.map((c, i) => (
          <div
            key={c.name}
            style={{
              display: 'flex',
              alignItems: 'center',
              height: ROW_H,
              padding: '0 12px',
              borderTop: i === 0 ? 'none' : '1px solid var(--line-1)',
              gap: 8,
            }}
          >
            <span style={{ width: 12, display: 'inline-flex', justifyContent: 'center' }}>
              {c.pk ? (
                <Icon name="key" size={11} style={{ color: 'var(--c-amber)' }} />
              ) : c.fk ? (
                <Icon name="link" size={11} style={{ color: 'var(--c-cyan)' }} />
              ) : c.unique ? (
                <span style={{ fontSize: 9, color: 'var(--fg-4)', fontFamily: 'var(--font-mono)' }}>U</span>
              ) : (
                <span style={{ width: 4, height: 4, borderRadius: 2, background: 'var(--fg-5)' }} />
              )}
            </span>
            <span
              className="mono"
              style={{
                fontSize: 11.5,
                color: 'var(--fg-1)',
                fontWeight: c.pk ? 500 : 400,
                flex: 1,
              }}
            >
              {c.name}
            </span>
            <span className="mono" style={{ fontSize: 11, color: 'var(--fg-3)' }}>
              {c.type}
              {c.nullable ? '?' : ''}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

function TableDetail({
  t,
  edges,
}: {
  t: SchemaTable
  edges: Array<{ fromKey: string; fromCol: string; toKey: string; toCol: string }>
}) {
  const me = keyOf(t)
  const incoming = edges.filter((e) => e.toKey === me)
  const outgoing = edges.filter((e) => e.fromKey === me)

  return (
    <div>
      <div style={{ padding: '16px 18px 14px', borderBottom: '1px solid var(--line-1)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
          <Icon name="table" size={14} style={{ color: 'var(--accent)' }} />
          <span
            style={{
              fontSize: 9.5,
              color: 'var(--fg-4)',
              letterSpacing: '0.08em',
              textTransform: 'uppercase',
              fontFamily: 'var(--font-mono)',
            }}
          >
            {t.schema}
          </span>
        </div>
        <h2
          className="mono"
          style={{
            fontSize: 18,
            fontWeight: 600,
            letterSpacing: '-0.02em',
            margin: 0,
            color: 'var(--fg-1)',
          }}
        >
          {t.name}
        </h2>
        <div style={{ display: 'flex', gap: 18, marginTop: 12 }}>
          <Stat
            label="Rows"
            value={rowsStatValue(t)}
            tooltip={rowsTooltip(t)}
          />
          <Stat label="Size" value={fmtBytes(t.size_bytes)} />
          <Stat label="Columns" value={String(t.columns.length)} />
          {t.indexes && t.indexes.length > 0 && (
            <Stat label="Indexes" value={String(t.indexes.length)} />
          )}
        </div>
        <div style={{ display: 'flex', gap: 6, marginTop: 14 }}>
          <a
            className="btn-pri"
            style={{ flex: 1, textAlign: 'center', textDecoration: 'none' }}
            href={`/data/${encodeURIComponent(t.schema)}/${encodeURIComponent(t.name)}`}
          >
            <Icon name="data" size={12} /> Browse data
          </a>
          <button className="btn-gh" title="Copy CREATE">
            <Icon name="copy" size={12} />
          </button>
        </div>
      </div>

      <Section title="Columns" count={t.columns.length}>
        {t.columns.map((c) => (
          <ColumnRow key={c.name} c={c} />
        ))}
      </Section>

      <Section title="Foreign keys" count={outgoing.length + incoming.length}>
        {outgoing.length === 0 && incoming.length === 0 && (
          <div style={{ padding: '0 18px 8px', fontSize: 11.5, color: 'var(--fg-4)' }}>None.</div>
        )}
        {outgoing.map((e, i) => {
          const fkMeta = t.columns.find((col) => col.name === e.fromCol)?.fk
          return (
            <FkRow
              key={'o' + i}
              dir="out"
              leftCol={e.fromCol}
              rightCol={e.toKey.split('.')[1] + '.' + e.toCol}
              onDelete={fkMeta?.on_delete}
              onUpdate={fkMeta?.on_update}
            />
          )
        })}
        {incoming.map((e, i) => (
          <FkRow key={'i' + i} dir="in" leftCol={e.fromKey.split('.')[1] + '.' + e.fromCol} rightCol={e.toCol} />
        ))}
      </Section>

      {t.indexes && t.indexes.length > 0 && (
        <Section title="Indexes" count={t.indexes.length}>
          {t.indexes.map((ix) => (
            <IndexRow key={ix.name} ix={ix} />
          ))}
        </Section>
      )}

      <Section title="DDL" preview>
        <pre
          className="mono sql"
          style={{
            margin: '0 18px 16px',
            padding: 12,
            background: 'var(--bg-0)',
            border: '1px solid var(--line-1)',
            borderRadius: 8,
            fontSize: 11,
            overflowX: 'auto',
            lineHeight: 1.6,
          }}
          dangerouslySetInnerHTML={{ __html: buildDDL(t) }}
        />
      </Section>
    </div>
  )
}

function ColumnRow({ c }: { c: SchemaColumn }) {
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: '14px 1fr auto',
        alignItems: 'center',
        gap: 8,
        padding: '6px 18px',
        fontFamily: 'var(--font-mono)',
        fontSize: 11.5,
      }}
      title={c.comment || undefined}
    >
      <span style={{ display: 'inline-flex', justifyContent: 'center' }}>
        {c.pk ? (
          <Icon name="key" size={11} style={{ color: 'var(--c-amber)' }} />
        ) : c.fk ? (
          <Icon name="link" size={11} style={{ color: 'var(--c-cyan)' }} />
        ) : (
          <span style={{ width: 3, height: 3, borderRadius: 2, background: 'var(--fg-5)' }} />
        )}
      </span>
      <span style={{ color: 'var(--fg-1)', display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        <span>{c.name}</span>
        {(c.default || c.comment) && (
          <span
            style={{
              fontSize: 10.5,
              color: 'var(--fg-4)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {c.default && <span style={{ color: 'var(--c-violet)' }}>= {c.default}</span>}
            {c.default && c.comment && <span style={{ color: 'var(--fg-5)' }}> · </span>}
            {c.comment && <span style={{ fontStyle: 'italic' }}>{c.comment}</span>}
          </span>
        )}
      </span>
      <span style={{ color: 'var(--fg-3)' }}>
        {c.type}
        {c.nullable ? '?' : ''}
      </span>
    </div>
  )
}

function IndexRow({ ix }: { ix: SchemaIndex }) {
  return (
    <div
      style={{
        padding: '6px 18px',
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        fontFamily: 'var(--font-mono)',
        fontSize: 11.5,
      }}
    >
      <Icon
        name={ix.primary ? 'key' : 'link'}
        size={11}
        style={{ color: ix.primary ? 'var(--c-amber)' : ix.unique ? 'var(--c-cyan)' : 'var(--fg-4)' }}
      />
      <span style={{ color: 'var(--fg-1)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis' }}>
        {ix.name}
        <span style={{ color: 'var(--fg-4)', marginLeft: 6 }}>({ix.columns.join(', ')})</span>
      </span>
      <span style={{ color: 'var(--fg-4)', fontSize: 10.5 }}>
        {ix.method}
        {ix.unique && !ix.primary ? ' · UNIQUE' : ''}
      </span>
      <span className="tnum" style={{ color: 'var(--fg-5)', fontSize: 10.5 }}>
        {fmtBytes(ix.size_bytes)}
      </span>
    </div>
  )
}

function Section({
  title,
  count,
  children,
  preview,
}: {
  title: string
  count?: number
  children: React.ReactNode
  preview?: boolean
}) {
  return (
    <div style={{ borderBottom: '1px solid var(--line-1)', padding: '12px 0 14px' }}>
      <div style={{ padding: '0 18px 8px', display: 'flex', alignItems: 'center', gap: 8 }}>
        <span
          style={{
            fontSize: 10.5,
            color: 'var(--fg-3)',
            letterSpacing: '0.08em',
            textTransform: 'uppercase',
            fontWeight: 500,
          }}
        >
          {title}
        </span>
        {count !== undefined && (
          <span className="mono tnum" style={{ fontSize: 10.5, color: 'var(--fg-5)' }}>
            {count}
          </span>
        )}
      </div>
      {preview ? children : <div>{children}</div>}
    </div>
  )
}

function Stat({ label, value, tooltip }: { label: string; value: string; tooltip?: string }) {
  return (
    <div title={tooltip}>
      <div
        style={{
          fontSize: 10,
          color: 'var(--fg-4)',
          letterSpacing: '0.06em',
          textTransform: 'uppercase',
          marginBottom: 2,
        }}
      >
        {label}
      </div>
      <div className="mono tnum" style={{ fontSize: 14, color: 'var(--fg-1)', fontWeight: 500 }}>
        {value}
      </div>
    </div>
  )
}

function rowsStatValue(t: SchemaTable): string {
  const n = displayRows(t)
  if (n < 0) return '—'
  const formatted = n.toLocaleString('en-US')
  return t.rows_estimated ? '~' + formatted : formatted
}

function FkRow({
  dir,
  leftCol,
  rightCol,
  onDelete,
  onUpdate,
}: {
  dir: 'in' | 'out'
  leftCol: string
  rightCol: string
  onDelete?: string
  onUpdate?: string
}) {
  const actions = [
    onDelete && onDelete !== 'NO ACTION' ? `ON DELETE ${onDelete}` : null,
    onUpdate && onUpdate !== 'NO ACTION' ? `ON UPDATE ${onUpdate}` : null,
  ].filter(Boolean)
  return (
    <div
      style={{
        padding: '6px 18px',
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        fontSize: 11.5,
        fontFamily: 'var(--font-mono)',
        flexWrap: 'wrap',
      }}
    >
      {dir === 'out' ? (
        <>
          <span style={{ color: 'var(--c-cyan)' }}>→</span>
          <span style={{ color: 'var(--fg-1)' }}>{leftCol}</span>
          <span style={{ color: 'var(--fg-4)' }}>references</span>
          <span style={{ color: 'var(--fg-1)' }}>{rightCol}</span>
        </>
      ) : (
        <>
          <span style={{ color: 'var(--accent)' }}>←</span>
          <span style={{ color: 'var(--fg-2)' }}>{leftCol}</span>
          <span style={{ color: 'var(--fg-4)' }}>→ {rightCol}</span>
        </>
      )}
      {actions.length > 0 && (
        <span style={{ color: 'var(--c-amber)', fontSize: 10.5 }}>{actions.join(' · ')}</span>
      )}
    </div>
  )
}

function buildDDL(t: SchemaTable): string {
  const cols = t.columns
    .map((c: SchemaColumn) => {
      const parts = [
        `  <span class="id">${pad(c.name, 16)}</span>`,
        `<span class="kw">${c.type}</span>`,
        c.default ? `<span class="kw">DEFAULT</span> <span class="lit">${escapeHTML(c.default)}</span>` : '',
        c.pk ? '<span class="kw">PRIMARY KEY</span>' : '',
        c.nullable ? '' : '<span class="kw">NOT NULL</span>',
        c.unique && !c.pk ? '<span class="kw">UNIQUE</span>' : '',
      ].filter(Boolean)
      return parts.join(' ')
    })
    .join(',\n')
  let ddl =
    `<span class="com">-- ${t.schema}.${t.name}</span>\n` +
    `<span class="kw">CREATE TABLE</span> <span class="id">${t.schema}.${t.name}</span> (\n` +
    cols +
    `\n);`
  // FK constraints as ALTER TABLE so they stay readable in the column list.
  for (const c of t.columns) {
    if (!c.fk) continue
    const acts = []
    if (c.fk.on_delete && c.fk.on_delete !== 'NO ACTION') acts.push(`ON DELETE ${c.fk.on_delete}`)
    if (c.fk.on_update && c.fk.on_update !== 'NO ACTION') acts.push(`ON UPDATE ${c.fk.on_update}`)
    ddl +=
      `\n<span class="kw">ALTER TABLE</span> <span class="id">${t.schema}.${t.name}</span>\n` +
      `  <span class="kw">ADD FOREIGN KEY</span> (<span class="id">${c.name}</span>) ` +
      `<span class="kw">REFERENCES</span> <span class="id">${c.fk.table}</span>(<span class="id">${c.fk.column}</span>)` +
      (acts.length > 0 ? ' <span class="kw">' + acts.join(' ') + '</span>' : '') +
      ';'
  }
  // Indexes as CREATE INDEX; skip those Postgres builds automatically for PK
  // or UNIQUE constraints (those are already covered above).
  if (t.indexes) {
    for (const ix of t.indexes) {
      if (ix.primary) continue
      // Skip indexes that exactly match a single UNIQUE column constraint.
      if (
        ix.unique &&
        ix.columns.length === 1 &&
        t.columns.find((c) => c.name === ix.columns[0])?.unique
      )
        continue
      ddl +=
        `\n<span class="kw">CREATE ${ix.unique ? 'UNIQUE ' : ''}INDEX</span> ` +
        `<span class="id">${ix.name}</span> <span class="kw">ON</span> ` +
        `<span class="id">${t.schema}.${t.name}</span> ` +
        `<span class="kw">USING</span> ${ix.method} ` +
        `(${ix.columns.map((c) => `<span class="id">${escapeHTML(c)}</span>`).join(', ')});`
    }
  }
  // Column comments.
  for (const c of t.columns) {
    if (!c.comment) continue
    ddl +=
      `\n<span class="kw">COMMENT ON COLUMN</span> <span class="id">${t.schema}.${t.name}.${c.name}</span> ` +
      `<span class="kw">IS</span> <span class="lit">'${escapeHTML(c.comment.replace(/'/g, "''"))}'</span>;`
  }
  return ddl
}

function escapeHTML(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function defaultGridPos(idx: number, columnCount: number): { x: number; y: number } {
  const col = idx % COLS
  const row = Math.floor(idx / COLS)
  const cardH = HEADER_H + Math.max(1, columnCount) * ROW_H
  return {
    x: CANVAS_PAD + col * (TABLE_W + COL_GAP),
    y: CANVAS_PAD + row * (cardH > 200 ? cardH + ROW_GAP / 2 : 220 + ROW_GAP),
  }
}

// useTablePositions stores per-table x/y in localStorage namespaced by
// connection id. Stale keys (tables that no longer exist in the schema) are
// pruned on every persist so the storage doesn't grow unbounded.
function useTablePositions(
  connID: number | null,
  tables: SchemaTable[],
): readonly [
  Record<string, { x: number; y: number }>,
  React.Dispatch<React.SetStateAction<Record<string, { x: number; y: number }>>>,
  () => void,
] {
  const storageKey = connID ? `dbil:schema:positions:${connID}` : null
  const [positions, setPositions] = useState<Record<string, { x: number; y: number }>>({})

  // Load whenever the connection changes.
  useEffect(() => {
    if (!storageKey) {
      setPositions({})
      return
    }
    try {
      const raw = localStorage.getItem(storageKey)
      setPositions(raw ? (JSON.parse(raw) as Record<string, { x: number; y: number }>) : {})
    } catch {
      setPositions({})
    }
  }, [storageKey])

  // Debounced persist, with prune of stale keys.
  useEffect(() => {
    if (!storageKey) return
    const handle = setTimeout(() => {
      const known = new Set(tables.map((t) => `${t.schema}.${t.name}`))
      const entries = Object.entries(positions).filter(([k]) => known.has(k))
      if (entries.length === 0) {
        localStorage.removeItem(storageKey)
        return
      }
      localStorage.setItem(storageKey, JSON.stringify(Object.fromEntries(entries)))
    }, 500)
    return () => clearTimeout(handle)
  }, [positions, tables, storageKey])

  const reset = useCallback(() => {
    if (storageKey) localStorage.removeItem(storageKey)
    setPositions({})
  }, [storageKey])

  return [positions, setPositions, reset] as const
}

function pad(s: string, n: number): string {
  return s.length >= n ? s : s + ' '.repeat(n - s.length)
}
