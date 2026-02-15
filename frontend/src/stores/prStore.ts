import { create } from 'zustand'
import { Task, api } from '../lib/api'

type SortField = 'created_at' | 'updated_at'
type SortOrder = 'asc' | 'desc'

// Default statuses to show (actionable PRs)
const DEFAULT_VISIBLE_STATUSES = ['OPEN', 'CHANGES_REQUESTED']

interface PRState {
  pendingPRs: Task[]
  allPRs: Task[] // Store all fetched PRs
  loading: boolean
  error: string | null
  statusFilter: string // Empty string means "default view" (OPEN + CHANGES_REQUESTED)
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

const filterPRs = (tasks: Task[], statusFilter: string): Task[] => {
  // Empty filter means "default view" - show actionable PRs only
  if (statusFilter === '') {
    return tasks.filter(task => DEFAULT_VISIBLE_STATUSES.includes(task.pr_status || ''))
  }
  // 'ALL' shows everything
  if (statusFilter === 'ALL') {
    return tasks
  }
  // Specific filter selected
  return tasks.filter(task => task.pr_status === statusFilter)
}

export const usePRStore = create<PRState>((set, get) => ({
  pendingPRs: [],
  allPRs: [],
  loading: false,
  error: null,
  statusFilter: '', // Empty = default view (OPEN + CHANGES_REQUESTED)
  sortBy: 'updated_at',
  sortOrder: 'desc',

  setStatusFilter: (filter: string) => {
    const { allPRs, sortBy, sortOrder } = get()
    const filtered = filterPRs(allPRs, filter)
    const sorted = sortPRs(filtered, sortBy, sortOrder)
    set({ statusFilter: filter, pendingPRs: sorted })
  },

  setSortBy: (field: SortField) => {
    const { sortBy, allPRs, sortOrder, statusFilter } = get()
    // If clicking the same field, toggle order; otherwise set new field with desc order
    if (field === sortBy) {
      const newOrder: SortOrder = sortOrder === 'desc' ? 'asc' : 'desc'
      const filtered = filterPRs(allPRs, statusFilter)
      const sorted = sortPRs(filtered, field, newOrder)
      set({ sortOrder: newOrder, pendingPRs: sorted })
    } else {
      const filtered = filterPRs(allPRs, statusFilter)
      const sorted = sortPRs(filtered, field, 'desc')
      set({ sortBy: field, sortOrder: 'desc', pendingPRs: sorted })
    }
  },

  toggleSortOrder: () => {
    const { sortOrder, sortBy, allPRs, statusFilter } = get()
    const newOrder: SortOrder = sortOrder === 'desc' ? 'asc' : 'desc'
    const filtered = filterPRs(allPRs, statusFilter)
    const sorted = sortPRs(filtered, sortBy, newOrder)
    set({ sortOrder: newOrder, pendingPRs: sorted })
  },

  fetchPendingPRs: async () => {
    set({ loading: true, error: null })
    try {
      const { statusFilter, sortBy, sortOrder } = get()
      // Fetch all PRs (no status filter to API)
      const allTasks = await api.listPRs()
      // Apply client-side filtering based on current filter
      const filtered = filterPRs(allTasks, statusFilter)
      const sorted = sortPRs(filtered, sortBy, sortOrder)
      set({ allPRs: allTasks, pendingPRs: sorted, loading: false })
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
