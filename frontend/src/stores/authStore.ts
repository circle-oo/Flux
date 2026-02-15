import { create } from 'zustand'
import { api } from '../lib/api'
import { initializeWebSocket, cleanupWebSocket } from './wsStore'

interface AuthState {
  isAuthenticated: boolean
  authEnabled: boolean | null // null = not yet checked, true/false = config value
  isLoading: boolean
  error: string | null
  checkAuthConfig: () => Promise<void>
  login: (password: string) => Promise<void>
  logout: () => Promise<void>
  clearError: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  isAuthenticated: false,
  authEnabled: null,
  isLoading: false,
  error: null,

  checkAuthConfig: async () => {
    try {
      const health = await api.getHealth()
      set({ authEnabled: health.auth_enabled })
      // If auth is disabled, automatically mark as authenticated
      if (!health.auth_enabled) {
        set({ isAuthenticated: true })
        // Connect WebSocket when auth is disabled
        initializeWebSocket()
      }
    } catch (error) {
      console.error('Failed to check auth config:', error)
      // Default to auth enabled for security if config check fails
      set({ authEnabled: true })
    }
  },

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
