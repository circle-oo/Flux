import { create } from 'zustand'

export type Theme = 'blue' | 'green'

interface ThemeState {
  theme: Theme
  setTheme: (theme: Theme) => void
}

const STORAGE_KEY = 'flux-theme'

function getInitialTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY) as Theme | null
  if (stored === 'blue' || stored === 'green') return stored
  return 'blue'
}

function applyTheme(theme: Theme) {
  document.documentElement.setAttribute('data-theme', theme)
  localStorage.setItem(STORAGE_KEY, theme)
}

// Apply on load
applyTheme(getInitialTheme())

export const useThemeStore = create<ThemeState>((set) => ({
  theme: getInitialTheme(),
  setTheme: (theme) => {
    applyTheme(theme)
    set({ theme })
  },
}))
