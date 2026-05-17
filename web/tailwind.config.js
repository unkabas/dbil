/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        // Modern dark palette — deep navy base with electric violet primary.
        // Inspired by Linear / Supabase / Vercel; intentionally NOT gray.
        ink: {
          50:  '#E6EAF2',
          100: '#C5CCDB',
          200: '#9CA8C0',
          300: '#7585A1',
          400: '#5A6A87',
          500: '#3F4D67',
          600: '#2A3548',
          700: '#1C2335',
          800: '#141A28',
          900: '#0C111C',
          950: '#070A12',
        },
        violet: {
          DEFAULT: '#7C9CFF',
          bright:  '#A8C0FF',
          deep:    '#5470E0',
          glow:    '#7C9CFF',
        },
        accent: {
          gold:    '#FFB454', // PK
          lilac:   '#C792EA', // FK
          mint:    '#7FDBCA', // string / text
          salmon:  '#FFA07A', // timestamp / interval
          lime:    '#A3DA77', // boolean / success
          coral:   '#F07178', // danger / errors
          sky:     '#82AAFF', // numeric / int
        },
        tag: {
          local:      { DEFAULT: '#82AAFF', soft: '#1B2B4D' },
          dev:        { DEFAULT: '#A3DA77', soft: '#243B22' },
          staging:    { DEFAULT: '#FFB454', soft: '#3F2E15' },
          production: { DEFAULT: '#F07178', soft: '#3B1B1F' },
        },
      },
      fontFamily: {
        sans: ['Inter', 'ui-sans-serif', 'system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace'],
      },
      fontSize: {
        xxs: ['11px', '15px'],
      },
      backgroundImage: {
        'card-grad': 'linear-gradient(180deg, rgba(124,156,255,0.06) 0%, rgba(124,156,255,0) 60%)',
        'header-grad': 'linear-gradient(135deg, rgba(124,156,255,0.18) 0%, rgba(199,146,234,0.12) 100%)',
        'app-grad': 'radial-gradient(ellipse at top, rgba(124,156,255,0.07), transparent 50%), radial-gradient(ellipse at bottom right, rgba(199,146,234,0.05), transparent 60%)',
      },
      boxShadow: {
        glow: '0 0 0 1px rgba(124,156,255,0.35), 0 8px 24px -8px rgba(124,156,255,0.25)',
        card: '0 1px 0 0 rgba(255,255,255,0.04) inset, 0 8px 24px -12px rgba(0,0,0,0.6)',
      },
    },
  },
  plugins: [],
}
