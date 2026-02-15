import { create } from 'zustand'
import { api, DeployStatusResponse } from '../lib/api'

interface DeployState {
  status: DeployStatusResponse | null
  isLoading: boolean
  isDeploying: boolean
  isChecking: boolean
  error: string | null
  fetchStatus: () => Promise<void>
  triggerDeploy: () => Promise<void>
  checkRemote: () => Promise<void>
}

export const useDeployStore = create<DeployState>((set) => ({
  status: null,
  isLoading: false,
  isDeploying: false,
  isChecking: false,
  error: null,

  fetchStatus: async () => {
    set({ isLoading: true, error: null })
    try {
      const status = await api.getDeployStatus()
      set({ status, isLoading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch deploy status',
        isLoading: false,
      })
    }
  },

  triggerDeploy: async () => {
    set({ isDeploying: true, error: null })
    try {
      await api.triggerDeploy()
      set({ isDeploying: true }) // stays deploying until page reloads
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to trigger deploy',
        isDeploying: false,
      })
    }
  },

  checkRemote: async () => {
    set({ isChecking: true, error: null })
    try {
      const result = await api.checkRemote()
      set({ status: { version: result.updater.local_commit || 'unknown', updater: result.updater }, isChecking: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to check remote',
        isChecking: false,
      })
    }
  },
}))
