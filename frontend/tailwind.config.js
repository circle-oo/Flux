/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        page: 'var(--c-page)',
        surface: {
          DEFAULT: 'var(--c-surface)',
          hover: 'var(--c-surface-hover)',
          active: 'var(--c-surface-active)',
          alt: 'var(--c-surface-alt)',
          deep: 'var(--c-surface-deep)',
        },
        sidebar: 'var(--c-sidebar)',
        content: {
          DEFAULT: 'var(--c-text)',
          secondary: 'var(--c-text-secondary)',
          muted: 'var(--c-text-muted)',
          faint: 'var(--c-text-faint)',
          inverted: '#FFFFFF',
        },
        line: {
          DEFAULT: 'var(--c-border)',
          hover: 'var(--c-border-hover)',
          subtle: 'var(--c-border-subtle)',
        },
        primary: {
          50: 'rgb(var(--c-p50) / <alpha-value>)',
          100: 'rgb(var(--c-p100) / <alpha-value>)',
          200: 'rgb(var(--c-p200) / <alpha-value>)',
          300: 'rgb(var(--c-p300) / <alpha-value>)',
          400: 'rgb(var(--c-p400) / <alpha-value>)',
          500: 'rgb(var(--c-p500) / <alpha-value>)',
          600: 'rgb(var(--c-p600) / <alpha-value>)',
          700: 'rgb(var(--c-p700) / <alpha-value>)',
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'sans-serif'],
        mono: ['JetBrains Mono', 'Fira Code', 'monospace'],
      },
      animation: {
        'fade-in': 'fade-in 0.3s ease-out',
        'slide-up': 'slide-up 0.3s ease-out',
        'slide-in-right': 'slide-in-right 0.2s ease-out',
        'glow-pulse': 'glow-pulse 2s ease-in-out infinite',
        'shimmer': 'shimmer 2s linear infinite',
      },
      keyframes: {
        'fade-in': {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        'slide-up': {
          '0%': { opacity: '0', transform: 'translateY(8px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        'slide-in-right': {
          '0%': { opacity: '0', transform: 'translateX(12px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' },
        },
        'glow-pulse': {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0.5' },
        },
        'shimmer': {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' },
        },
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
      },
    },
  },
  plugins: [],
}
