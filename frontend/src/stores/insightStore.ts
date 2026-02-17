import { create } from 'zustand'
import { api } from '../lib/api'
import type {
  InsightsSummary,
  DailyMetric,
  AgentPerformance,
  PipelineHealth,
  FailureAnalysis,
  UsageTimePoint,
  ProjectActivity,
  BillingInfo,
} from '../lib/api'

type Period = '24h' | '7d' | '30d'

interface InsightState {
  period: Period
  summary: InsightsSummary | null
  timeseries: DailyMetric[]
  efficiency: AgentPerformance[]
  pipeline: PipelineHealth[]
  failures: FailureAnalysis[]
  realtimeUsage: UsageTimePoint[]
  projectActivities: ProjectActivity[]
  billing: BillingInfo | null
  isLoading: boolean
  error: string | null
  _refreshTimer: ReturnType<typeof setInterval> | null
  setPeriod: (period: Period) => void
  fetchAll: () => Promise<void>
  fetchBilling: () => Promise<void>
  fetchSummary: () => Promise<void>
  fetchTimeseries: () => Promise<void>
  fetchEfficiency: () => Promise<void>
  fetchPipeline: () => Promise<void>
  fetchFailures: () => Promise<void>
  fetchRealtimeUsage: () => Promise<void>
  startAutoRefresh: () => void
  stopAutoRefresh: () => void
}

export const useInsightStore = create<InsightState>((set, get) => ({
  period: '7d',
  summary: null,
  timeseries: [],
  efficiency: [],
  pipeline: [],
  failures: [],
  realtimeUsage: [],
  projectActivities: [],
  billing: null,
  isLoading: false,
  error: null,
  _refreshTimer: null,

  setPeriod: (period) => {
    set({ period })
    get().fetchAll()
  },

  fetchBilling: async () => {
    try {
      const billing = await api.getBillingInfo()
      set({ billing })
    } catch (error) {
      console.error('Failed to fetch billing info:', error)
    }
  },

  fetchAll: async () => {
    set({ isLoading: true, error: null })
    const { period } = get()

    const emptySummary: InsightsSummary = {
      total_tasks: 0, completed_tasks: 0, failed_tasks: 0, success_rate: 0,
      total_tokens: 0, total_cost: 0, triage_tokens: 0, triage_cost: 0,
      execution_tokens: 0, execution_cost: 0, avg_latency_min: 0, active_projects: 0,
    }

    // Each call has its own .catch() so a single endpoint failure
    // doesn't block the entire page from loading.
    const [summary, timeseries, efficiency, pipeline, failures, realtimeUsage, insights, billing] = await Promise.all([
      api.getInsightsSummary(period).catch(() => emptySummary),
      api.getInsightsTimeseries(period).catch(() => []),
      api.getInsightsEfficiency().catch(() => []),
      api.getInsightsPipeline().catch(() => []),
      api.getInsightsFailures(period).catch(() => []),
      api.getInsightsUsageRealtime(60).catch(() => []),
      api.getInsights().catch(() => ({ total_tokens: 0, total_cost: 0, project_activities: [] as ProjectActivity[] })),
      api.getBillingInfo().catch(() => null),
    ])
    set({
      summary, timeseries, efficiency, pipeline, failures, realtimeUsage,
      projectActivities: insights.project_activities || [],
      billing,
      isLoading: false,
    })
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

  fetchRealtimeUsage: async () => {
    try {
      const realtimeUsage = await api.getInsightsUsageRealtime(60)
      set({ realtimeUsage })
    } catch (error) {
      console.error('Failed to fetch realtime usage:', error)
    }
  },

  // Auto-refresh realtime data every 30s while the insights page is mounted
  startAutoRefresh: () => {
    const { _refreshTimer } = get()
    if (_refreshTimer) return
    const timer = setInterval(() => {
      get().fetchRealtimeUsage()
    }, 30_000)
    set({ _refreshTimer: timer })
  },

  stopAutoRefresh: () => {
    const { _refreshTimer } = get()
    if (_refreshTimer) {
      clearInterval(_refreshTimer)
      set({ _refreshTimer: null })
    }
  },
}))
