import { create } from 'zustand'
import { api } from '../lib/api'

interface KnowledgeStats {
  note_count: number
  folder_count: number
  total_size: number
  mode: string
  healthy: boolean
}

interface KnowledgeHealth {
  mode: string
  healthy: boolean
}

interface KnowledgeState {
  notes: string[]
  currentNote: { path: string; content: string } | null
  searchResults: string[]
  searchQuery: string
  stats: KnowledgeStats | null
  folders: string[]
  health: KnowledgeHealth | null
  recentNotes: { path: string; mod_time: string }[]
  orphans: string[]
  isLoading: boolean
  error: string | null

  fetchNotes: (folder?: string) => Promise<void>
  fetchNote: (path: string) => Promise<void>
  searchNotes: (query: string) => Promise<void>
  fetchStats: () => Promise<void>
  fetchHealth: () => Promise<void>
  fetchFolders: () => Promise<void>
  fetchRecentNotes: () => Promise<void>
  fetchOrphans: () => Promise<void>
  createNote: (path: string, content: string) => Promise<void>
  deleteNote: (path: string) => Promise<void>
  appendDaily: (content: string) => Promise<void>
  clearCurrentNote: () => void
}

export const useKnowledgeStore = create<KnowledgeState>((set) => ({
  notes: [],
  currentNote: null,
  searchResults: [],
  searchQuery: '',
  stats: null,
  folders: [],
  health: null,
  recentNotes: [],
  orphans: [],
  isLoading: false,
  error: null,

  fetchNotes: async (folder?: string) => {
    set({ isLoading: true, error: null })
    try {
      const notes = await api.listNotes(folder)
      set({ notes, isLoading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch notes',
        isLoading: false,
      })
    }
  },

  fetchNote: async (path: string) => {
    set({ isLoading: true, error: null })
    try {
      const note = await api.getNote(path)
      set({ currentNote: note, isLoading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch note',
        isLoading: false,
      })
    }
  },

  searchNotes: async (query: string) => {
    set({ isLoading: true, error: null, searchQuery: query })
    try {
      const data = await api.searchKnowledge(query)
      set({ searchResults: data.results, isLoading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to search notes',
        isLoading: false,
      })
    }
  },

  fetchStats: async () => {
    try {
      const stats = await api.getKnowledgeStats()
      set({ stats })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch stats',
      })
    }
  },

  fetchHealth: async () => {
    try {
      const health = await api.getKnowledgeHealth()
      set({ health })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch health',
      })
    }
  },

  fetchFolders: async () => {
    try {
      const folders = await api.listFolders()
      set({ folders })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch folders',
      })
    }
  },

  fetchRecentNotes: async () => {
    try {
      const recentNotes = await api.getRecentNotes()
      set({ recentNotes })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch recent notes',
      })
    }
  },

  fetchOrphans: async () => {
    try {
      const orphans = await api.getOrphans()
      set({ orphans })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch orphans',
      })
    }
  },

  createNote: async (path: string, content: string) => {
    set({ isLoading: true, error: null })
    try {
      await api.createNote(path, content)
      set((state) => ({
        notes: [...state.notes, path],
        isLoading: false,
      }))
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to create note',
        isLoading: false,
      })
      throw error
    }
  },

  deleteNote: async (path: string) => {
    set({ isLoading: true, error: null })
    try {
      await api.deleteNote(path)
      set((state) => ({
        notes: state.notes.filter((n) => n !== path),
        currentNote: state.currentNote?.path === path ? null : state.currentNote,
        isLoading: false,
      }))
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to delete note',
        isLoading: false,
      })
      throw error
    }
  },

  appendDaily: async (content: string) => {
    set({ isLoading: true, error: null })
    try {
      await api.appendDaily(content)
      set({ isLoading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to append to daily',
        isLoading: false,
      })
      throw error
    }
  },

  clearCurrentNote: () => set({ currentNote: null }),
}))
