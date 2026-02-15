import { create } from 'zustand'
import { Task } from '../lib/api'

type PRStatusFilter = 'ALL' | 'OPEN' | 'APPROVED' | 'CHANGES_REQUESTED' | 'MERGED' | 'CLOSED'

interface PRState {
  prs: Task[]
  pendingPRs: Task[]
  loading: boolean
  error: string | null
  statusFilter: PRStatusFilter
  setStatusFilter: (filter: PRStatusFilter) => void
  fetchPRs: () => Promise<void>
  fetchPendingPRs: () => Promise<void>
  approvePR: (taskId: string) => Promise<void>
  requestChanges: (taskId: string) => Promise<void>
  closePR: (taskId: string) => Promise<void>
}

export const usePRStore = create<PRState>((set, get) => ({
  prs: [],
  pendingPRs: [],
  loading: false,
  error: null,
  statusFilter: 'ALL',

  setStatusFilter: (filter: PRStatusFilter) => {
    set({ statusFilter: filter })
    get().fetchPRs()
  },

  fetchPRs: async () => {
    set({ loading: true, error: null })
    try {
      const filter = get().statusFilter
      const query = filter !== 'ALL' ? `?status=${filter}` : ''
      const response = await fetch(`/api/prs${query}`, {
        credentials: 'same-origin',
        headers: {
          'Content-Type': 'application/json',
        },
      })

      if (!response.ok) {
        const error = await response.text()
        throw new Error(error || `HTTP ${response.status}`)
      }

      const data = await response.json()
      set({ prs: data.tasks || [], loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch PRs',
        loading: false,
      })
    }
  },

  fetchPendingPRs: async () => {
    set({ loading: true, error: null })
    try {
      const response = await fetch('/api/prs/pending', {
        credentials: 'same-origin',
        headers: {
          'Content-Type': 'application/json',
        },
      })

      if (!response.ok) {
        const error = await response.text()
        throw new Error(error || `HTTP ${response.status}`)
      }

      const data = await response.json()
      set({ pendingPRs: data.tasks || [], loading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch pending PRs',
        loading: false,
      })
    }
  },

  approvePR: async (taskId: string) => {
    set({ loading: true, error: null })
    try {
      const response = await fetch(`/api/prs/${taskId}/approve`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
          'Content-Type': 'application/json',
        },
      })

      if (!response.ok) {
        const error = await response.text()
        throw new Error(error || `HTTP ${response.status}`)
      }

      await get().fetchPRs()
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
      const response = await fetch(`/api/prs/${taskId}/request-changes`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
          'Content-Type': 'application/json',
        },
      })

      if (!response.ok) {
        const error = await response.text()
        throw new Error(error || `HTTP ${response.status}`)
      }

      await get().fetchPRs()
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
      const response = await fetch(`/api/prs/${taskId}/close`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
          'Content-Type': 'application/json',
        },
      })

      if (!response.ok) {
        const error = await response.text()
        throw new Error(error || `HTTP ${response.status}`)
      }

      await get().fetchPRs()
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
