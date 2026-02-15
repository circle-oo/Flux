import { create } from 'zustand'
import { api, Goal } from '../lib/api'

interface GoalState {
  goals: Goal[]
  currentGoal: Goal | null
  isLoading: boolean
  error: string | null
  fetchGoals: () => Promise<void>
  fetchCurrentGoal: () => Promise<void>
  createGoal: (goal: {
    title: string
    description: string
    priorities: string[]
    metrics: string[]
  }) => Promise<Goal>
  activateGoal: (id: string) => Promise<void>
  updateGoal: (id: string, updates: Partial<Goal>) => Promise<void>
}

export const useGoalStore = create<GoalState>((set, get) => ({
  goals: [],
  currentGoal: null,
  isLoading: false,
  error: null,

  fetchGoals: async () => {
    set({ isLoading: true, error: null })
    try {
      const goals = await api.listGoals()
      set({ goals, isLoading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch goals',
        isLoading: false,
      })
    }
  },

  fetchCurrentGoal: async () => {
    try {
      const currentGoal = await api.getCurrentGoal()
      set({ currentGoal })
    } catch (error) {
      console.error('Failed to fetch current goal:', error)
      set({ currentGoal: null })
    }
  },

  createGoal: async (goal) => {
    set({ isLoading: true, error: null })
    try {
      const newGoal = await api.createGoal(goal)
      set((state) => ({
        goals: [...state.goals, newGoal],
        isLoading: false,
      }))
      return newGoal
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to create goal',
        isLoading: false,
      })
      throw error
    }
  },

  activateGoal: async (id) => {
    set({ isLoading: true, error: null })
    try {
      const activated = await api.activateGoal(id)
      await get().fetchGoals()
      set({ currentGoal: activated, isLoading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to activate goal',
        isLoading: false,
      })
      throw error
    }
  },

  updateGoal: async (id, updates) => {
    set({ isLoading: true, error: null })
    try {
      const updated = await api.updateGoal(id, updates)
      set((state) => ({
        goals: state.goals.map((g) => (g.id === id ? updated : g)),
        currentGoal: state.currentGoal?.id === id ? updated : state.currentGoal,
        isLoading: false,
      }))
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to update goal',
        isLoading: false,
      })
      throw error
    }
  },
}))
