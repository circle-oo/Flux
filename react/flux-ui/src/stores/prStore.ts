import { create } from 'zustand'
import { Task } from '../lib/api'

interface PRState {
  pendingPRs: Task[]
  loading: boolean
  error: string | null
  fetchPendingPRs: () => Promise<void>
  approvePR: (taskId: string) => Promise<void>
  requestChanges: (taskId: string) => Promise<void>
}

export const usePRStore = create<PRState>((set, get) => ({
  pendingPRs: [],
  loading: false,
  error: null,

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

      // Refresh pending PRs after approval
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

      // Refresh pending PRs after requesting changes
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
}))
