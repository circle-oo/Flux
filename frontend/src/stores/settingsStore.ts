import { create } from 'zustand'
import { getCookie, setCookie } from '../lib/cookies'

export type Theme = 'blue' | 'green'
export type Mode = 'light' | 'dark'
export type DashboardLayout = 'bento' | 'classic'

interface Settings {
  theme: Theme
  mode: Mode
  mesh: boolean
  sidebarCollapsed: boolean
  dashboardLayout: DashboardLayout
  podRefreshInterval: number
}

interface SettingsState extends Settings {
  setTheme: (theme: Theme) => void
  setMode: (mode: Mode) => void
  setMesh: (mesh: boolean) => void
  setSidebarCollapsed: (collapsed: boolean) => void
  setDashboardLayout: (layout: DashboardLayout) => void
  setPodRefreshInterval: (seconds: number) => void
  updateSetting: <K extends keyof Settings>(key: K, value: Settings[K]) => void
}

const COOKIE_NAME = 'flux-settings'

const defaults: Settings = {
  theme: 'blue',
  mode: 'light',
  mesh: true,
  sidebarCollapsed: false,
  dashboardLayout: 'bento',
  podRefreshInterval: 10,
}

function loadSettings(): Settings {
  const raw = getCookie(COOKIE_NAME)
  if (!raw) return { ...defaults }
  try {
    const parsed = JSON.parse(raw)
    const mode = parsed.mode === 'dark' ? 'dark' : 'light'
    const mesh = parsed.mesh !== false
    return {
      theme: parsed.theme === 'green' ? 'green' : 'blue',
      mode,
      mesh,
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

function applyAppearance(theme: Theme, mode: Mode, mesh: boolean): void {
  document.documentElement.setAttribute('data-svc', 'flux')
  document.documentElement.setAttribute('data-theme', theme)
  document.documentElement.setAttribute('data-mode', mode)
  document.documentElement.setAttribute('data-mesh', mesh ? 'on' : 'off')
}

// Load and apply on module init
const initial = loadSettings()
applyAppearance(initial.theme, initial.mode, initial.mesh)

export const useSettingsStore = create<SettingsState>((set, get) => ({
  ...initial,

  setTheme: (theme) => {
    applyAppearance(theme, get().mode, get().mesh)
    set({ theme })
    saveSettings({ ...get(), theme })
  },

  setMode: (mode) => {
    applyAppearance(get().theme, mode, get().mesh)
    set({ mode })
    saveSettings({ ...get(), mode })
  },

  setMesh: (mesh) => {
    applyAppearance(get().theme, get().mode, mesh)
    set({ mesh })
    saveSettings({ ...get(), mesh })
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
    if (key === 'theme') applyAppearance(value as Theme, get().mode, get().mesh)
    if (key === 'mode') applyAppearance(get().theme, value as Mode, get().mesh)
    if (key === 'mesh') applyAppearance(get().theme, get().mode, value as boolean)
    set({ [key]: value } as Partial<SettingsState>)
    saveSettings({ ...get(), [key]: value })
  },
}))
