import { create } from 'zustand'
import { api } from '../lib/api'
import type {
  InsightsSummary,
  DailyMetric,
  AgentPerformance,
  PipelineHealth,
  FailureAnalysis,
} from '../lib/api'

type Period = '24h' | '7d' | '30d'

interface InsightState {
  period: Period
  summary: InsightsSummary | null
  timeseries: DailyMetric[]
  efficiency: AgentPerformance[]
  pipeline: PipelineHealth[]
  failures: FailureAnalysis[]
  isLoading: boolean
  error: string | null
  setPeriod: (period: Period) => void
  fetchAll: () => Promise<void>
  fetchSummary: () => Promise<void>
  fetchTimeseries: () => Promise<void>
  fetchEfficiency: () => Promise<void>
  fetchPipeline: () => Promise<void>
  fetchFailures: () => Promise<void>
}

export const useInsightStore = create<InsightState>((set, get) => ({
  period: '7d',
  summary: null,
  timeseries: [],
  efficiency: [],
  pipeline: [],
  failures: [],
  isLoading: false,
  error: null,

  setPeriod: (period) => {
    set({ period })
    get().fetchAll()
  },

  fetchAll: async () => {
    set({ isLoading: true, error: null })
    try {
      const { period } = get()
      const [summary, timeseries, efficiency, pipeline, failures] = await Promise.all([
        api.getInsightsSummary(period),
        api.getInsightsTimeseries(period),
        api.getInsightsEfficiency(),
        api.getInsightsPipeline(),
        api.getInsightsFailures(period),
      ])
      set({ summary, timeseries, efficiency, pipeline, failures, isLoading: false })
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch insights',
        isLoading: false,
      })
    }
  },

  fetchSummary: async () => {
    try {
      const summary = await api.getInsightsSummary(get().period)
      set({ summary })
    } catch (error) {
      console.error('Failed to fetch summary:', error)
    }
  },

  fetchTimeseries: async () => {
    try {
      const timeseries = await api.getInsightsTimeseries(get().period)
      set({ timeseries })
    } catch (error) {
      console.error('Failed to fetch timeseries:', error)
    }
  },

  fetchEfficiency: async () => {
    try {
      const efficiency = await api.getInsightsEfficiency()
      set({ efficiency })
    } catch (error) {
      console.error('Failed to fetch efficiency:', error)
    }
  },

  fetchPipeline: async () => {
    try {
      const pipeline = await api.getInsightsPipeline()
      set({ pipeline })
    } catch (error) {
      console.error('Failed to fetch pipeline:', error)
    }
  },

  fetchFailures: async () => {
    try {
      const failures = await api.getInsightsFailures(get().period)
      set({ failures })
    } catch (error) {
      console.error('Failed to fetch failures:', error)
    }
  },
}))
