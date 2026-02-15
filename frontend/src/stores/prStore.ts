import { create } from 'zustand'
import { Task, api } from '../lib/api'

interface PRState {
  pendingPRs: Task[]
  loading: boolean
  error: string | null
  statusFilter: string
  setStatusFilter: (filter: string) => void
  fetchPendingPRs: () => Promise<void>
  approvePR: (taskId: string) => Promise<void>
  requestChanges: (taskId: string) => Promise<void>
  closePR: (taskId: string) => Promise<void>
}

export const usePRStore = create<PRState>((set, get) => ({
  pendingPRs: [],
  loading: false,
  error: null,
  statusFilter: '',

  setStatusFilter: (filter: string) => {
    set({ statusFilter: filter })
    get().fetchPendingPRs()
  },

  fetchPendingPRs: async () => {
    set({ loading: true, error: null })
    try {
      const { statusFilter } = get()
      const tasks = await api.listPRs(statusFilter || undefined)
      set({ pendingPRs: tasks, loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch PRs',
        loading: false,
      })
    }
  },

  approvePR: async (taskId: string) => {
    set({ loading: true, error: null })
    try {
      await api.approvePR(taskId)
      await get().fetchPendingPRs()
      set({ loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to approve PR',
        loading: false,
      })
      throw error
    }
  },

  requestChanges: async (taskId: string) => {
    set({ loading: true, error: null })
    try {
      await api.requestPRChanges(taskId)
      await get().fetchPendingPRs()
      set({ loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to request changes',
        loading: false,
      })
      throw error
    }
  },

  closePR: async (taskId: string) => {
    set({ loading: true, error: null })
    try {
      await api.closePR(taskId)
      await get().fetchPendingPRs()
      set({ loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to close PR',
        loading: false,
      })
      throw error
    }
  },
}))
