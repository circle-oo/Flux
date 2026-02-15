import { create } from 'zustand'
import { api, Task } from '../lib/api'

interface TaskState {
  tasks: Task[]
  isLoading: boolean
  error: string | null
  filters: {
    status?: string
    project_id?: string
  }
  setFilters: (filters: { status?: string; project_id?: string }) => void
  fetchTasks: () => Promise<void>
  getTask: (id: string) => Promise<Task>
  fetchSubtasks: (parentId: string) => Promise<Task[]>
  createTask: (task: {
    title: string
    description: string
    type: Task['type']
    priority: number
    project_id: string
    goal_id?: string
    depends_on?: string[]
    tags?: string[]
  }) => Promise<Task>
  updateTask: (id: string, updates: Partial<Task>) => Promise<void>
  deleteTask: (id: string) => Promise<void>
  cancelTask: (id: string) => Promise<void>
  retryTask: (id: string) => Promise<void>
  archiveTask: (id: string) => Promise<void>
}

export const useTaskStore = create<TaskState>((set, get) => ({
  tasks: [],
  isLoading: false,
  error: null,
  filters: {},

  setFilters: (filters) => {
    set({ filters })
    get().fetchTasks()
  },

  fetchTasks: async () => {
    set({ isLoading: true, error: null })
    try {
      const tasks = await api.listTasks(get().filters)
      set({ tasks, isLoading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch tasks',
        isLoading: false,
      })
    }
  },

  getTask: async (id) => {
    try {
      return await api.getTask(id)
    } catch (error) {
      throw error
    }
  },

  fetchSubtasks: async (parentId) => {
    try {
      return await api.listSubtasks(parentId)
    } catch (error) {
      console.error('Failed to fetch subtasks:', error)
      return []
    }
  },

  createTask: async (task) => {
    set({ isLoading: true, error: null })
    try {
      const newTask = await api.createTask(task)
      set((state) => ({
        tasks: [newTask, ...state.tasks],
        isLoading: false,
      }))
      return newTask
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to create task',
        isLoading: false,
      })
      throw error
    }
  },

  updateTask: async (id, updates) => {
    set({ isLoading: true, error: null })
    try {
      const updated = await api.updateTask(id, updates)
      set((state) => ({
        tasks: state.tasks.map((t) => (t.id === id ? updated : t)),
        isLoading: false,
      }))
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to update task',
        isLoading: false,
      })
      throw error
    }
  },

  deleteTask: async (id) => {
    set({ isLoading: true, error: null })
    try {
      await api.deleteTask(id)
      set((state) => ({
        tasks: state.tasks.filter((t) => t.id !== id),
        isLoading: false,
      }))
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to delete task',
        isLoading: false,
      })
      throw error
    }
  },

  cancelTask: async (id) => {
    set({ isLoading: true, error: null })
    try {
      await api.cancelTask(id)
      await get().fetchTasks()
      set({ isLoading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to cancel task',
        isLoading: false,
      })
      throw error
    }
  },

  retryTask: async (id) => {
    set({ isLoading: true, error: null })
    try {
      await api.retryTask(id)
      await get().fetchTasks()
      set({ isLoading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to retry task',
        isLoading: false,
      })
      throw error
    }
  },

  archiveTask: async (id) => {
    set({ isLoading: true, error: null })
    try {
      await api.archiveTask(id)
      await get().fetchTasks()
      set({ isLoading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to archive task',
        isLoading: false,
      })
      throw error
    }
  },
}))
