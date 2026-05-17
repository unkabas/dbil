// Inline SVG icon set for dbil. Stroke-based, 1.5px, currentColor.

type IconName =
  | 'schema' | 'data' | 'observ' | 'query' | 'audit' | 'conn'
  | 'search' | 'cmd' | 'chev' | 'chevR'
  | 'plus' | 'minus' | 'filter' | 'refresh' | 'play' | 'bolt' | 'warn' | 'check' | 'x'
  | 'key' | 'link' | 'copy' | 'edit' | 'trash'
  | 'user' | 'settings' | 'history' | 'download' | 'kill' | 'lock'
  | 'sparkles' | 'layers' | 'branch' | 'table' | 'bell' | 'logout'

const paths: Record<IconName, JSX.Element> = {
  schema: (<><ellipse cx="12" cy="6" rx="8" ry="3"/><path d="M4 6v6c0 1.7 3.6 3 8 3s8-1.3 8-3V6"/><path d="M4 12v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6"/></>),
  data:   (<><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 10h18M3 15h18M10 4v16"/></>),
  observ: (<><path d="M3 17l5-7 4 4 5-9 4 7"/><circle cx="8" cy="10" r="1.2" fill="currentColor"/><circle cx="12" cy="14" r="1.2" fill="currentColor"/><circle cx="17" cy="5" r="1.2" fill="currentColor"/></>),
  query:  (<><path d="M8 5l-5 7 5 7"/><path d="M16 5l5 7-5 7"/><path d="M14 4l-4 16"/></>),
  audit:  (<><path d="M12 3l8 4v5c0 4.4-3.4 8.4-8 9-4.6-.6-8-4.6-8-9V7l8-4z"/><path d="M9 12l2 2 4-4"/></>),
  conn:   (<><circle cx="6" cy="6" r="2.5"/><circle cx="18" cy="18" r="2.5"/><path d="M7.8 7.8L16 16"/><circle cx="18" cy="6" r="2.5"/><path d="M16 7.8L8 16"/></>),
  search: (<><circle cx="11" cy="11" r="7"/><path d="M16.5 16.5L21 21"/></>),
  cmd:    (<path d="M9 6a3 3 0 1 1-3 3h12a3 3 0 1 1-3 3V6"/>),
  chev:   (<polyline points="6 9 12 15 18 9"/>),
  chevR:  (<polyline points="9 6 15 12 9 18"/>),
  plus:   (<><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></>),
  minus:  (<line x1="5" y1="12" x2="19" y2="12"/>),
  filter: (<path d="M3 4h18l-7 9v7l-4-2v-5L3 4z"/>),
  refresh:(<><polyline points="1 4 1 10 7 10"/><path d="M3.5 15a9 9 0 0 0 16.5-4M20.5 9A9 9 0 0 0 4 13"/></>),
  play:   (<polygon points="6 4 20 12 6 20 6 4" fill="currentColor" stroke="none"/>),
  bolt:   (<polygon points="13 2 4 14 11 14 9 22 20 9 13 9 13 2" fill="currentColor" stroke="none"/>),
  warn:   (<><path d="M12 3l10 18H2L12 3z"/><line x1="12" y1="10" x2="12" y2="14"/><circle cx="12" cy="17" r="0.8" fill="currentColor"/></>),
  check:  (<polyline points="4 12 10 18 20 6"/>),
  x:      (<><line x1="6" y1="6" x2="18" y2="18"/><line x1="6" y1="18" x2="18" y2="6"/></>),
  key:    (<><circle cx="7" cy="14" r="4"/><path d="M10 11l9-9 3 3-3 3 2 2-3 3-2-2-3 3"/></>),
  link:   (<><path d="M10 14a4 4 0 0 0 5.5 0l3-3a4 4 0 0 0-5.5-5.5l-1.5 1.5"/><path d="M14 10a4 4 0 0 0-5.5 0l-3 3a4 4 0 0 0 5.5 5.5l1.5-1.5"/></>),
  copy:   (<><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></>),
  edit:   (<><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 1 1 3 3L7 19l-4 1 1-4L16.5 3.5z"/></>),
  trash:  (<><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/><path d="M10 11v6M14 11v6"/></>),
  user:   (<><circle cx="12" cy="8" r="4"/><path d="M4 21c0-4.4 3.6-8 8-8s8 3.6 8 8"/></>),
  settings:(<><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/></>),
  history:(<><path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><polyline points="3 3 3 8 8 8"/><polyline points="12 7 12 12 16 14"/></>),
  download:(<><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></>),
  kill:   (<><circle cx="12" cy="12" r="9"/><line x1="6" y1="6" x2="18" y2="18"/></>),
  lock:   (<><rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 1 1 8 0v4"/></>),
  sparkles:(<><path d="M12 3l1.8 4.2L18 9l-4.2 1.8L12 15l-1.8-4.2L6 9l4.2-1.8L12 3z"/><path d="M19 14l.8 1.8L21.6 17l-1.8.8L19 19.6l-.8-1.8L16.4 17l1.8-.8L19 14z"/></>),
  layers: (<><polygon points="12 2 22 8 12 14 2 8 12 2"/><polyline points="2 16 12 22 22 16"/><polyline points="2 12 12 18 22 12"/></>),
  branch: (<><circle cx="6" cy="6" r="2"/><circle cx="6" cy="18" r="2"/><circle cx="18" cy="18" r="2"/><path d="M6 8v8M6 12c6 0 6-6 12-6"/></>),
  table:  (<><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 9h18M3 15h18M9 3v18M15 3v18"/></>),
  bell:   (<><path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/></>),
  logout: (<><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></>),
}

// Aliases keep older callsites compiling while the design is being rolled
// out across all pages.
const aliases: Record<string, IconName> = {
  'chevron-down': 'chev',
  'chevron-right': 'chevR',
  'database': 'schema',
  'pencil': 'edit',
  'column': 'table',
  'column-pk': 'key',
  'column-fk': 'link',
}

interface IconProps {
  name: IconName | string
  size?: number
  className?: string
  style?: React.CSSProperties
  fill?: string
}

export default function Icon({ name, size = 16, className = '', style }: IconProps) {
  const resolved = (aliases[name] ?? name) as IconName
  const p = paths[resolved]
  if (!p) return null
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      style={style}
      aria-hidden
    >
      {p}
    </svg>
  )
}

export type { IconName }
