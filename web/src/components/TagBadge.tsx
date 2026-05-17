import type { Tag } from '../api/connections'

// `size` is accepted but currently ignored — the design's tag pill has a
// single 20px height. The prop stays in the signature so older callsites
// from the previous iteration keep compiling.
export default function TagBadge({ tag }: { tag: Tag; size?: 'xs' | 'sm' }) {
  return (
    <span className={`tag ${tag}`}>
      <span className="dot" />
      {tag}
    </span>
  )
}
