import type { Tag } from '../mock/data'

const styles: Record<Tag, { bg: string; text: string; ring: string; label: string }> = {
  local:      { bg: 'bg-tag-local-soft',      text: 'text-tag-local',      ring: 'ring-tag-local/30',      label: 'LOCAL' },
  dev:        { bg: 'bg-tag-dev-soft',        text: 'text-tag-dev',        ring: 'ring-tag-dev/30',        label: 'DEV' },
  staging:    { bg: 'bg-tag-staging-soft',    text: 'text-tag-staging',    ring: 'ring-tag-staging/30',    label: 'STAGING' },
  production: { bg: 'bg-tag-production-soft', text: 'text-tag-production', ring: 'ring-tag-production/30', label: 'PROD' },
}

export default function TagBadge({ tag, size = 'sm' }: { tag: Tag; size?: 'xs' | 'sm' }) {
  const s = styles[tag]
  const pad = size === 'xs' ? 'px-1.5 py-px text-[10px]' : 'px-2 py-0.5 text-[11px]'
  return (
    <span
      className={`inline-flex items-center rounded-md font-semibold tracking-wider ring-1 ${pad} ${s.bg} ${s.text} ${s.ring}`}
    >
      {s.label}
    </span>
  )
}
