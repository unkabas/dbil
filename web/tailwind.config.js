/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      // Map Tailwind colour tokens to the design's CSS variables so we can use
      // bg-bg-1 / text-fg-2 / border-line-1 in JSX while keeping a single
      // source of truth in styles.css (:root).
      colors: {
        bg: {
          0: 'var(--bg-0)',
          1: 'var(--bg-1)',
          2: 'var(--bg-2)',
          3: 'var(--bg-3)',
          4: 'var(--bg-4)',
        },
        line: {
          1: 'var(--line-1)',
          2: 'var(--line-2)',
          3: 'var(--line-3)',
        },
        fg: {
          1: 'var(--fg-1)',
          2: 'var(--fg-2)',
          3: 'var(--fg-3)',
          4: 'var(--fg-4)',
          5: 'var(--fg-5)',
        },
        accent: {
          DEFAULT: 'var(--accent)',
          deep: 'var(--accent-deep)',
          soft: 'var(--accent-soft)',
        },
        ok: 'var(--ok)',
        warn: 'var(--warn)',
        danger: 'var(--danger)',
        prod: 'var(--prod)',
        c: {
          cyan: 'var(--c-cyan)',
          mint: 'var(--c-mint)',
          amber: 'var(--c-amber)',
          rose: 'var(--c-rose)',
          violet: 'var(--c-violet)',
        },
        // Compatibility aliases for pages that still use the previous palette.
        // They resolve to the new tokens — kept so older callsites compile
        // until each page is migrated to the new design.
        ink: {
          50: 'var(--fg-1)',
          100: 'var(--fg-1)',
          200: 'var(--fg-2)',
          300: 'var(--fg-3)',
          400: 'var(--fg-4)',
          500: 'var(--fg-5)',
          600: 'var(--bg-4)',
          700: 'var(--bg-3)',
          800: 'var(--bg-2)',
          900: 'var(--bg-1)',
          950: 'var(--bg-0)',
        },
        violet: {
          DEFAULT: 'var(--accent)',
          bright: 'var(--accent-soft)',
          deep: 'var(--accent-deep)',
          glow: 'var(--accent-glow)',
        },
        tag: {
          local:      { DEFAULT: 'var(--fg-3)', soft: 'rgba(124,133,151,0.10)' },
          dev:        { DEFAULT: 'var(--ok)',   soft: 'rgba(52,211,153,0.10)' },
          staging:    { DEFAULT: 'var(--warn)', soft: 'rgba(245,165,36,0.10)' },
          production: { DEFAULT: 'var(--prod)', soft: 'rgba(255,85,119,0.12)' },
        },
      },
      backgroundImage: {
        'app-grad':
          "radial-gradient(1200px 600px at 80% -10%, rgba(139,124,255,0.05), transparent 60%), radial-gradient(800px 500px at -10% 110%, rgba(78,214,255,0.04), transparent 60%)",
        'header-grad':
          'linear-gradient(135deg, rgba(139,124,255,0.18) 0%, rgba(78,214,255,0.10) 100%)',
        'card-grad':
          'linear-gradient(180deg, rgba(139,124,255,0.06) 0%, rgba(139,124,255,0) 60%)',
      },
      fontFamily: {
        sans: ['Geist', 'Inter', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        mono: ['Geist Mono', 'JetBrains Mono', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace'],
      },
      fontSize: {
        xxs: ['11px', '15px'],
      },
      boxShadow: {
        card: '0 1px 0 rgba(255,255,255,0.02) inset, 0 1px 2px rgba(0,0,0,0.4)',
        pop: '0 12px 32px -8px rgba(0,0,0,0.6), 0 1px 0 rgba(255,255,255,0.03) inset',
        glow: '0 0 0 1px var(--accent-mute), 0 0 24px -4px var(--accent-glow)',
      },
    },
  },
  plugins: [],
}
