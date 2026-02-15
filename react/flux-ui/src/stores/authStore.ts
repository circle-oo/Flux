import { create } from 'zustand'
import { api } from '../lib/api'
import { initializeWebSocket, cleanupWebSocket } from './wsStore'

interface AuthState {
  isAuthenticated: boolean
  isLoading: boolean
  error: string | null
  login: (password: string) => Promise<void>
  logout: () => Promise<void>
  clearError: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  isAuthenticated: false,
  isLoading: false,
  error: null,

  login: async (password: string) => {
    set({ isLoading: true, error: null })
    try {
      await api.login(password)
      set({ isAuthenticated: true, isLoading: false })
      // Connect WebSocket after successful login
      initializeWebSocket()
    } catch (error) {
      set({
        isAuthenticated: false,
        isLoading: false,
        error: error instanceof Error ? error.message : 'Login failed',
      })
      throw error
    }
  },

  logout: async () => {
    set({ isLoading: true })
    try {
      // Disconnect WebSocket before logout
      cleanupWebSocket()
      await api.logout()
      set({ isAuthenticated: false, isLoading: false, error: null })
    } catch (error) {
      console.error('Logout failed:', error)
      // Force logout on client side even if server call fails
      cleanupWebSocket()
      set({ isAuthenticated: false, isLoading: false, error: null })
    }
  },

  clearError: () => set({ error: null }),
}))
