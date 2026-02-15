import { create } from 'zustand'
import { api } from '../lib/api'

export interface LogEntry {
  time: string
  level: string
  msg: string
  attrs: Record<string, unknown>
}

interface LogFilter {
  level: string
  search: string
}

interface LogState {
  logs: LogEntry[]
  filter: LogFilter
  autoScroll: boolean
  paused: boolean
  fetchRecentLogs: () => Promise<void>
  addLog: (entry: LogEntry) => void
  clearLogs: () => void
  setFilter: (filter: Partial<LogFilter>) => void
  setAutoScroll: (enabled: boolean) => void
  setPaused: (paused: boolean) => void
}

const MAX_CLIENT_LOGS = 2000

export const useLogStore = create<LogState>((set, get) => ({
  logs: [],
  filter: { level: '', search: '' },
  autoScroll: true,
  paused: false,

  fetchRecentLogs: async () => {
    try {
      const data = await api.getRecentLogs()
      set({ logs: data })
    } catch (error) {
      console.error('Failed to fetch recent logs:', error)
    }
  },

  addLog: (entry: LogEntry) => {
    if (get().paused) return
    set((state) => {
      const logs = [...state.logs, entry]
      if (logs.length > MAX_CLIENT_LOGS) {
        return { logs: logs.slice(logs.length - MAX_CLIENT_LOGS) }
      }
      return { logs }
    })
  },

  clearLogs: () => set({ logs: [] }),

  setFilter: (filter) =>
    set((state) => ({ filter: { ...state.filter, ...filter } })),

  setAutoScroll: (autoScroll) => set({ autoScroll }),

  setPaused: (paused) => set({ paused }),
}))
