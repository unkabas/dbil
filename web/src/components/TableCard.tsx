import { useNavigate } from 'react-router-dom'
import type { MockTable, MockColumn } from '../mock/data'
import StatusPill from './StatusPill'

interface Props {
  table: MockTable
  highlightedFkTo?: string | null
  onClick?: () => void
  isFocused?: boolean
}

export default function TableCard({ table, highlightedFkTo, onClick, isFocused }: Props) {
  const navigate = useNavigate()
  const fqdn = `${table.schema}.${table.name}`
  const showRelHint = !!highlightedFkTo && highlightedFkTo === fqdn
  const pkCount = table.columns.filter((c) => c.pk).length
  const fkCount = table.columns.filter((c) => c.fk).length

  return (
    <div
      onClick={onClick}
      className={`group relative bg-ink-800/60 backdrop-blur-sm border rounded-2xl shadow-card overflow-hidden cursor-pointer transition-all duration-150 ${
        isFocused
          ? 'border-violet shadow-glow'
          : showRelHint
            ? 'border-accent-lilac/60'
            : 'border-ink-700 hover:border-violet/40 hover:shadow-glow'
      }`}
    >
      {/* Header strip */}
      <div className="bg-card-grad px-4 py-3 border-b border-ink-700 flex items-center gap-3">
        <TableGlyph />
        <div className="flex-1 min-w-0">
          <div className="flex items-baseline gap-2">
            <span className="text-ink-50 font-semibold font-mono text-[14px] truncate">
              {table.name}
            </span>
            <span className="text-ink-400 text-[11px] font-mono">{table.schema}</span>
          </div>
          {table.description && (
            <div className="text-ink-300 text-[11px] mt-0.5 truncate">{table.description}</div>
          )}
        </div>
        <RowCount n={table.rows} />
      </div>

      {/* Columns */}
      <ul className="py-1 font-mono text-[12.5px]">
        {table.columns.map((c) => (
          <ColumnRow key={c.name} col={c} />
        ))}
      </ul>

      {/* Footer */}
      <div className="px-4 py-2 border-t border-ink-700 flex items-center gap-2 text-[11px]">
        {pkCount > 0 && (
          <StatusPill tone="warning" size="xs">{pkCount === 1 ? 'PK' : `${pkCount} PK`}</StatusPill>
        )}
        {fkCount > 0 && (
          <StatusPill tone="info" size="xs">{fkCount} FK</StatusPill>
        )}
        <StatusPill tone="success" size="xs">in sync</StatusPill>
        <button
          onClick={(e) => {
            e.stopPropagation()
            navigate(`/data/${table.schema}/${table.name}`)
          }}
          className="ml-auto text-[11px] font-medium text-violet hover:text-violet-bright opacity-0 group-hover:opacity-100 transition-opacity"
        >
          Browse data →
        </button>
      </div>
    </div>
  )
}

function ColumnRow({ col }: { col: MockColumn }) {
  return (
    <li className="flex items-center gap-2 px-4 py-1.5 hover:bg-ink-700/40 group">
      <span className="w-3.5 flex-shrink-0 flex items-center justify-center">
        {col.pk ? (
          <KeyDot color="text-accent-gold" filled title="Primary key" />
        ) : col.fk ? (
          <KeyDot color="text-accent-lilac" title={`References ${col.fk.table}.${col.fk.column}`} />
        ) : col.unique ? (
          <KeyDot color="text-accent-sky" title="Unique" />
        ) : (
          <span className="w-1.5 h-1.5 rounded-full bg-ink-700" />
        )}
      </span>
      <span className={`flex-1 truncate ${col.pk ? 'text-ink-50 font-semibold' : 'text-ink-100'}`}>
        {col.name}
      </span>
      {col.fk && (
        <span className="text-[10.5px] text-accent-lilac/80 truncate" title={`${col.fk.table}.${col.fk.column}`}>
          → {col.fk.table}.{col.fk.column}
        </span>
      )}
      {col.nullable && (
        <span className="text-[10px] text-ink-400 font-sans italic">null</span>
      )}
      <TypeBadge type={col.type} />
    </li>
  )
}

function TypeBadge({ type }: { type: string }) {
  const cls = typeColor(type)
  return (
    <span className={`px-1.5 py-px rounded text-[10.5px] font-medium ${cls}`}>{type}</span>
  )
}

function typeColor(type: string): string {
  if (/int|numeric|float|decimal|bigint/i.test(type)) return 'text-accent-sky bg-accent-sky/10'
  if (/text|varchar|char|name|inet|uuid/i.test(type)) return 'text-accent-mint bg-accent-mint/10'
  if (/timestamp|date|time|interval/i.test(type)) return 'text-accent-salmon bg-accent-salmon/10'
  if (/bool/i.test(type)) return 'text-accent-lime bg-accent-lime/10'
  if (/json|bytea|array|status/i.test(type)) return 'text-accent-lilac bg-accent-lilac/10'
  return 'text-ink-200 bg-ink-700'
}

function KeyDot({ color, filled, title }: { color: string; filled?: boolean; title?: string }) {
  return (
    <span title={title}>
      <svg viewBox="0 0 12 12" className={`w-3 h-3 ${color}`}>
        <circle cx="6" cy="6" r="3.4" fill={filled ? 'currentColor' : 'none'} stroke="currentColor" strokeWidth="1.6" />
      </svg>
    </span>
  )
}

function TableGlyph() {
  return (
    <span className="w-9 h-9 rounded-xl bg-violet/15 ring-1 ring-violet/30 flex items-center justify-center">
      <svg viewBox="0 0 16 16" className="w-4 h-4 text-violet" fill="none" stroke="currentColor" strokeWidth="1.5">
        <rect x="2.5" y="3" width="11" height="10" rx="1.2" />
        <path d="M2.5 6.5h11M2.5 10h11M5.5 6.5V13" />
      </svg>
    </span>
  )
}

function RowCount({ n }: { n: number }) {
  return (
    <div className="text-right">
      <div className="font-mono text-[18px] font-semibold text-ink-50 leading-none">
        {formatRowCount(n)}
      </div>
      <div className="text-[10px] text-ink-400 uppercase tracking-wider mt-0.5">rows</div>
    </div>
  )
}

function formatRowCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`
  return String(n)
}
