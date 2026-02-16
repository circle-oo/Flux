import { create } from 'zustand'
import { getCookie, setCookie } from '../lib/cookies'

export type Theme = 'blue' | 'green'
export type DashboardLayout = 'bento' | 'classic'

interface Settings {
  theme: Theme
  sidebarCollapsed: boolean
  dashboardLayout: DashboardLayout
  podRefreshInterval: number
}

interface SettingsState extends Settings {
  setTheme: (theme: Theme) => void
  setSidebarCollapsed: (collapsed: boolean) => void
  setDashboardLayout: (layout: DashboardLayout) => void
  setPodRefreshInterval: (seconds: number) => void
  updateSetting: <K extends keyof Settings>(key: K, value: Settings[K]) => void
}

const COOKIE_NAME = 'flux-settings'

const defaults: Settings = {
  theme: 'blue',
  sidebarCollapsed: false,
  dashboardLayout: 'bento',
  podRefreshInterval: 10,
}

function loadSettings(): Settings {
  const raw = getCookie(COOKIE_NAME)
  if (!raw) return { ...defaults }
  try {
    const parsed = JSON.parse(raw)
    return {
      theme: parsed.theme === 'green' ? 'green' : 'blue',
      sidebarCollapsed: parsed.sidebarCollapsed === true,
      dashboardLayout: parsed.dashboardLayout === 'classic' ? 'classic' : 'bento',
      podRefreshInterval: typeof parsed.podRefreshInterval === 'number' && parsed.podRefreshInterval >= 5
        ? parsed.podRefreshInterval
        : defaults.podRefreshInterval,
    }
  } catch {
    return { ...defaults }
  }
}

function saveSettings(settings: Settings): void {
  setCookie(COOKIE_NAME, JSON.stringify(settings))
}

function applyTheme(theme: Theme): void {
  document.documentElement.setAttribute('data-theme', theme)
}

// Load and apply on module init
const initial = loadSettings()
applyTheme(initial.theme)

export const useSettingsStore = create<SettingsState>((set, get) => ({
  ...initial,

  setTheme: (theme) => {
    applyTheme(theme)
    set({ theme })
    saveSettings({ ...get(), theme })
  },

  setSidebarCollapsed: (sidebarCollapsed) => {
    set({ sidebarCollapsed })
    saveSettings({ ...get(), sidebarCollapsed })
  },

  setDashboardLayout: (dashboardLayout) => {
    set({ dashboardLayout })
    saveSettings({ ...get(), dashboardLayout })
  },

  setPodRefreshInterval: (podRefreshInterval) => {
    set({ podRefreshInterval })
    saveSettings({ ...get(), podRefreshInterval })
  },

  updateSetting: (key, value) => {
    if (key === 'theme') applyTheme(value as Theme)
    set({ [key]: value } as Partial<SettingsState>)
    saveSettings({ ...get(), [key]: value })
  },
}))
