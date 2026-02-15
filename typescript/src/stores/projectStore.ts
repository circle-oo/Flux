import { create } from 'zustand'
import { api, Project } from '../lib/api'

interface ProjectState {
  projects: Project[]
  isLoading: boolean
  error: string | null
  fetchProjects: () => Promise<void>
  getProject: (id: string) => Promise<Project>
  createProject: (project: {
    name: string
    type: Project['type']
    description: string
    tech_stack: string[]
    inspiration?: string
  }) => Promise<Project>
  approveProject: (id: string) => Promise<void>
  rejectProject: (id: string) => Promise<void>
}

export const useProjectStore = create<ProjectState>((set, _get) => ({
  projects: [],
  isLoading: false,
  error: null,

  fetchProjects: async () => {
    set({ isLoading: true, error: null })
    try {
      const projects = await api.listProjects()
      set({ projects, isLoading: false })
    } catch (error) {
      set({
        error:
          error instanceof Error ? error.message : 'Failed to fetch projects',
        isLoading: false,
      })
    }
  },

  getProject: async (id) => {
    try {
      return await api.getProject(id)
    } catch (error) {
      throw error
    }
  },

  createProject: async (project) => {
    set({ isLoading: true, error: null })
    try {
      const newProject = await api.createProject(project)
      set((state) => ({
        projects: [...state.projects, newProject],
        isLoading: false,
      }))
      return newProject
    } catch (error) {
      set({
        error:
          error instanceof Error ? error.message : 'Failed to create project',
        isLoading: false,
      })
      throw error
    }
  },

  approveProject: async (id) => {
    set({ isLoading: true, error: null })
    try {
      const approved = await api.approveProject(id)
      set((state) => ({
        projects: state.projects.map((p) => (p.id === id ? approved : p)),
        isLoading: false,
      }))
    } catch (error) {
      set({
        error:
          error instanceof Error ? error.message : 'Failed to approve project',
        isLoading: false,
      })
      throw error
    }
  },

  rejectProject: async (id) => {
    set({ isLoading: true, error: null })
    try {
      const rejected = await api.rejectProject(id)
      set((state) => ({
        projects: state.projects.map((p) => (p.id === id ? rejected : p)),
        isLoading: false,
      }))
    } catch (error) {
      set({
        error:
          error instanceof Error ? error.message : 'Failed to reject project',
        isLoading: false,
      })
      throw error
    }
  },
}))
