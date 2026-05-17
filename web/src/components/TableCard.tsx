import type { MockTable, MockColumn } from '../mock/data'

interface Props {
  table: MockTable
  highlightedFkTo?: string | null
  onClick?: () => void
  isFocused?: boolean
}

export default function TableCard({ table, highlightedFkTo, onClick, isFocused }: Props) {
  const showRelHint = highlightedFkTo && highlightedFkTo === `${table.schema}.${table.name}`

  return (
    <div
      onClick={onClick}
      className={`relative bg-ink-800/70 backdrop-blur-sm border rounded-xl shadow-card overflow-hidden cursor-pointer transition-all duration-150 ${
        isFocused
          ? 'border-violet shadow-glow'
          : showRelHint
            ? 'border-accent-lilac/60'
            : 'border-ink-700 hover:border-ink-600'
      }`}
    >
      {/* Header strip */}
      <div className="bg-header-grad px-4 py-3 border-b border-ink-700 flex items-center gap-2">
        <TableGlyph />
        <div className="flex-1 min-w-0">
          <div className="flex items-baseline gap-2">
            <span className="text-ink-100 font-semibold font-mono text-[13.5px] truncate">
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
    <span className="w-7 h-7 rounded-lg bg-violet/15 ring-1 ring-violet/30 flex items-center justify-center">
      <svg viewBox="0 0 16 16" className="w-3.5 h-3.5 text-violet" fill="none" stroke="currentColor" strokeWidth="1.5">
        <rect x="2.5" y="3" width="11" height="10" rx="1.2" />
        <path d="M2.5 6.5h11M2.5 10h11M5.5 6.5V13" />
      </svg>
    </span>
  )
}

function RowCount({ n }: { n: number }) {
  return (
    <span
      className="font-mono text-[11px] text-ink-300 bg-ink-900/70 ring-1 ring-ink-700 rounded-md px-2 py-0.5"
      title={`${n.toLocaleString()} rows`}
    >
      {formatRowCount(n)}
    </span>
  )
}

function formatRowCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`
  return String(n)
}
