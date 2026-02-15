import { create } from 'zustand'
import { Task, api } from '../lib/api'

type SortField = 'created_at' | 'updated_at'
type SortOrder = 'asc' | 'desc'

interface PRState {
  pendingPRs: Task[]
  loading: boolean
  error: string | null
  statusFilter: string
  sortBy: SortField
  sortOrder: SortOrder
  setStatusFilter: (filter: string) => void
  setSortBy: (field: SortField) => void
  toggleSortOrder: () => void
  fetchPendingPRs: () => Promise<void>
  approvePR: (taskId: string) => Promise<void>
  requestChanges: (taskId: string) => Promise<void>
  closePR: (taskId: string) => Promise<void>
}

const sortPRs = (tasks: Task[], sortBy: SortField, sortOrder: SortOrder): Task[] => {
  return [...tasks].sort((a, b) => {
    const aValue = sortBy === 'created_at' ? a.created_at : a.updated_at
    const bValue = sortBy === 'created_at' ? b.created_at : b.updated_at

    // Handle missing timestamps (sort to end)
    if (!aValue && !bValue) return 0
    if (!aValue) return 1
    if (!bValue) return -1

    const aTime = new Date(aValue).getTime()
    const bTime = new Date(bValue).getTime()

    return sortOrder === 'asc' ? aTime - bTime : bTime - aTime
  })
}

export const usePRStore = create<PRState>((set, get) => ({
  pendingPRs: [],
  loading: false,
  error: null,
  statusFilter: '',
  sortBy: 'updated_at',
  sortOrder: 'desc',

  setStatusFilter: (filter: string) => {
    set({ statusFilter: filter })
    get().fetchPendingPRs()
  },

  setSortBy: (field: SortField) => {
    const { sortBy, pendingPRs, sortOrder } = get()
    // If clicking the same field, toggle order; otherwise set new field with desc order
    if (field === sortBy) {
      const newOrder: SortOrder = sortOrder === 'desc' ? 'asc' : 'desc'
      set({ sortOrder: newOrder, pendingPRs: sortPRs(pendingPRs, field, newOrder) })
    } else {
      set({ sortBy: field, sortOrder: 'desc', pendingPRs: sortPRs(pendingPRs, field, 'desc') })
    }
  },

  toggleSortOrder: () => {
    const { sortOrder, sortBy, pendingPRs } = get()
    const newOrder: SortOrder = sortOrder === 'desc' ? 'asc' : 'desc'
    set({ sortOrder: newOrder, pendingPRs: sortPRs(pendingPRs, sortBy, newOrder) })
  },

  fetchPendingPRs: async () => {
    set({ loading: true, error: null })
    try {
      const { statusFilter, sortBy, sortOrder } = get()
      const tasks = await api.listPRs(statusFilter || undefined)
      const sortedTasks = sortPRs(tasks, sortBy, sortOrder)
      set({ pendingPRs: sortedTasks, loading: false })
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
