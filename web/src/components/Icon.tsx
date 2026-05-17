// Tiny inline SVG icon set so we don't pull a 50 KB icon font.

type IconName =
  | 'chevron-right'
  | 'chevron-down'
  | 'database'
  | 'schema'
  | 'table'
  | 'column-pk'
  | 'column-fk'
  | 'column'
  | 'play'
  | 'stop'
  | 'refresh'
  | 'plus'
  | 'x'
  | 'trash'
  | 'pencil'
  | 'history'
  | 'logout'

const paths: Record<IconName, JSX.Element> = {
  'chevron-right': (
    <path d="M6 4l4 4-4 4" strokeLinecap="round" strokeLinejoin="round" />
  ),
  'chevron-down': (
    <path d="M4 6l4 4 4-4" strokeLinecap="round" strokeLinejoin="round" />
  ),
  database: (
    <>
      <ellipse cx="8" cy="3.5" rx="5" ry="1.7" />
      <path d="M3 3.5v4c0 .94 2.24 1.7 5 1.7s5-.76 5-1.7v-4" />
      <path d="M3 8.5v4c0 .94 2.24 1.7 5 1.7s5-.76 5-1.7v-4" />
    </>
  ),
  schema: (
    <>
      <rect x="2.5" y="3" width="11" height="2.4" rx="0.5" />
      <rect x="2.5" y="6.8" width="11" height="2.4" rx="0.5" />
      <rect x="2.5" y="10.6" width="11" height="2.4" rx="0.5" />
    </>
  ),
  table: (
    <>
      <rect x="2.5" y="3" width="11" height="10" rx="1" />
      <path d="M2.5 6.5H13.5M2.5 10H13.5M5.5 6.5V13" />
    </>
  ),
  'column-pk': (
    <>
      <circle cx="8" cy="6" r="2.2" />
      <path d="M8 8.2v4.3M6 10.4h4" strokeLinecap="round" />
    </>
  ),
  'column-fk': (
    <>
      <path d="M5 7.5h6M5 7.5l1.5-1.5M5 7.5l1.5 1.5M11 8.5l-1.5 1.5M11 8.5l-1.5-1.5" strokeLinecap="round" strokeLinejoin="round" />
    </>
  ),
  column: (
    <>
      <circle cx="8" cy="8" r="1.5" />
    </>
  ),
  play: (
    <path d="M4 3l9 5-9 5z" />
  ),
  stop: (
    <rect x="3.5" y="3.5" width="9" height="9" rx="1" />
  ),
  refresh: (
    <path d="M3.5 7.5a4.5 4.5 0 1 1 1.25 3.13M3.5 11v-3.5h3.5" strokeLinecap="round" strokeLinejoin="round" />
  ),
  plus: (
    <path d="M8 3v10M3 8h10" strokeLinecap="round" />
  ),
  x: (
    <path d="M4 4l8 8M12 4l-8 8" strokeLinecap="round" />
  ),
  trash: (
    <>
      <path d="M3.5 5h9M6.5 5V3.5h3V5M5 5l.5 8h5L11 5" />
    </>
  ),
  pencil: (
    <path d="M11.5 2.5l2 2L5 13H3v-2L11.5 2.5z" />
  ),
  history: (
    <>
      <circle cx="8" cy="8" r="5.5" />
      <path d="M8 4.5V8l2.5 2" strokeLinecap="round" />
    </>
  ),
  logout: (
    <>
      <path d="M10 3.5H4v9h6" />
      <path d="M8 8h6M11 5l3 3-3 3" strokeLinecap="round" strokeLinejoin="round" />
    </>
  ),
}

export default function Icon({
  name,
  className = '',
  fill = 'none',
}: {
  name: IconName
  className?: string
  fill?: 'none' | 'currentColor'
}) {
  return (
    <svg
      viewBox="0 0 16 16"
      width="16"
      height="16"
      fill={fill}
      stroke="currentColor"
      strokeWidth="1.4"
      className={className}
      aria-hidden
    >
      {paths[name]}
    </svg>
  )
}

export type { IconName }
