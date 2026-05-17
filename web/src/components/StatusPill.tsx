// TriumphNet-style outline pill — soft bg + colored ring + colored text.
// Use for binary or short-text status indicators.

type Tone = 'success' | 'warning' | 'danger' | 'info' | 'neutral'

const tones: Record<Tone, { ring: string; text: string; bg: string }> = {
  success: { ring: 'ring-accent-lime/50',  text: 'text-accent-lime',  bg: 'bg-accent-lime/10' },
  warning: { ring: 'ring-accent-gold/50',  text: 'text-accent-gold',  bg: 'bg-accent-gold/10' },
  danger:  { ring: 'ring-accent-coral/50', text: 'text-accent-coral', bg: 'bg-accent-coral/10' },
  info:    { ring: 'ring-violet/40',       text: 'text-violet',       bg: 'bg-violet/10' },
  neutral: { ring: 'ring-ink-600',         text: 'text-ink-200',      bg: 'bg-ink-800/60' },
}

export default function StatusPill({
  tone,
  children,
  size = 'sm',
}: {
  tone: Tone
  children: React.ReactNode
  size?: 'xs' | 'sm'
}) {
  const t = tones[tone]
  const pad = size === 'xs' ? 'px-2 py-[1px] text-[10.5px]' : 'px-2.5 py-0.5 text-[11.5px]'
  return (
    <span
      className={`inline-flex items-center rounded-full font-medium ring-1 ${pad} ${t.ring} ${t.text} ${t.bg}`}
    >
      {children}
    </span>
  )
}
