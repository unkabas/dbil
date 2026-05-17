// Lightweight SVG sparkline: ribbon-style line with a subtle fill underneath.
// Falls back to a flat dashed baseline when the series has 0 or 1 points.

interface Props {
  data: number[]
  color?: string
  height?: number
  width?: number
}

export default function Sparkline({ data, color = 'var(--accent)', height = 36, width = 240 }: Props) {
  if (data.length < 2) {
    return (
      <svg width="100%" height={height} viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none">
        <line
          x1={0}
          x2={width}
          y1={height / 2}
          y2={height / 2}
          stroke="var(--line-2)"
          strokeWidth="1"
          strokeDasharray="3 4"
        />
      </svg>
    )
  }

  const min = Math.min(...data)
  const max = Math.max(...data)
  const range = max - min || 1
  const dx = width / (data.length - 1)

  const points = data.map((v, i) => {
    const x = i * dx
    const y = height - ((v - min) / range) * (height - 4) - 2
    return [x, y] as const
  })

  const line = points.map(([x, y], i) => `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`).join(' ')
  const fill = `${line} L ${width} ${height} L 0 ${height} Z`

  const id = `spark-${Math.random().toString(36).slice(2, 8)}`

  return (
    <svg width="100%" height={height} viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none">
      <defs>
        <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity="0.32" />
          <stop offset="100%" stopColor={color} stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={fill} fill={`url(#${id})`} />
      <path d={line} fill="none" stroke={color} strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx={points[points.length - 1][0]} cy={points[points.length - 1][1]} r="2.6" fill={color} />
    </svg>
  )
}
