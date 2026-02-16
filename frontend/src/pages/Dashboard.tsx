import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useGoalStore } from '../stores/goalStore'
import { useTaskStore } from '../stores/taskStore'
import { useProjectStore } from '../stores/projectStore'
import { useWSStore } from '../stores/wsStore'
import { useSettingsStore } from '../stores/settingsStore'
import { api, Pod, Insights, InsightsSummary, TaskStats, Goal } from '../lib/api'
import { countByStatus } from '../lib/utils'
import PageHeader from '../components/PageHeader'
import StatCard from '../components/StatCard'
import LoadingState from '../components/LoadingState'

type HealthLevel = 'healthy' | 'warning' | 'critical'

function deriveHealth(wsConnected: boolean, failedToday: number, completedToday: number): HealthLevel {
  if (!wsConnected) return 'critical'
  const total = failedToday + completedToday
  if (total > 0 && failedToday / total > 0.3) return 'warning'
  return 'healthy'
}

const healthStyles: Record<HealthLevel, { bg: string; border: string; dot: string }> = {
  healthy: {
    bg: 'from-primary-600/5 to-primary-400/5',
    border: 'border-l-primary-500',
    dot: 'bg-emerald-400 shadow-sm shadow-emerald-500/30',
  },
  warning: {
    bg: 'from-amber-500/10 to-amber-400/5',
    border: 'border-l-amber-500',
    dot: 'bg-amber-400 shadow-sm shadow-amber-500/30',
  },
  critical: {
    bg: 'from-rose-500/10 to-rose-400/5',
    border: 'border-l-rose-500',
    dot: 'bg-rose-400 shadow-sm shadow-rose-500/30',
  },
}

export default function Dashboard() {
  const navigate = useNavigate()
  const { currentGoal, fetchCurrentGoal } = useGoalStore()
  const { tasks, fetchTasks, setFilters } = useTaskStore()
  const { projects, fetchProjects } = useProjectStore()
  const wsConnected = useWSStore((s) => s.connected)
  const wsReconnecting = useWSStore((s) => s.reconnecting)
  const dashboardLayout = useSettingsStore((s) => s.dashboardLayout)
  const podRefreshInterval = useSettingsStore((s) => s.podRefreshInterval)
  const [pods, setPods] = useState<Pod[]>([])
  const [insights, setInsights] = useState<Insights | null>(null)
  const [insightSummary, setInsightSummary] = useState<InsightsSummary | null>(null)
  const [taskStats, setTaskStats] = useState<TaskStats | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setFilters({})
    Promise.all([fetchCurrentGoal(), fetchTasks(), fetchProjects(), fetchPods(), fetchInsights(), fetchTaskStats()])
      .finally(() => setLoading(false))

    const interval = setInterval(fetchPods, podRefreshInterval * 1000)
    return () => clearInterval(interval)
  }, [fetchCurrentGoal, fetchTasks, fetchProjects, setFilters, podRefreshInterval])

  async function fetchPods() {
    try {
      setPods(await api.listPods())
    } catch (error) {
      console.error('Failed to fetch pods:', error)
    }
  }

  async function fetchInsights() {
    try {
      const [ins, sum] = await Promise.all([
        api.getInsights(),
        api.getInsightsSummary('24h').catch(() => null),
      ])
      setInsights(ins)
      if (sum) setInsightSummary(sum)
    } catch (error) {
      console.error('Failed to fetch insights:', error)
    }
  }

  async function fetchTaskStats() {
    try {
      setTaskStats(await api.getTaskStats())
    } catch (error) {
      console.error('Failed to fetch task stats:', error)
    }
  }

  const taskCounts = countByStatus(tasks)
  const pendingPRs = tasks.filter((t) => t.pr_url && t.pr_status === 'OPEN').length
  const activeProjectCount = projects.filter((p) => p.status === 'ACTIVE').length

  // Deltas
  const completedDelta = taskStats ? taskStats.today.completed - taskStats.yesterday.completed : undefined
  const failedDelta = taskStats ? taskStats.today.failed - taskStats.yesterday.failed : undefined

  // Health
  const health = deriveHealth(
    wsConnected,
    taskStats?.today.failed ?? 0,
    taskStats?.today.completed ?? 0,
  )

  if (loading) {
    return (
      <div className="p-5 sm:p-6 lg:p-8">
        <LoadingState message="Loading dashboard..." />
      </div>
    )
  }

  // Classic layout — original flat grid
  if (dashboardLayout === 'classic') {
    return (
      <div className="p-5 sm:p-6 lg:p-8 space-y-6 animate-fade-in">
        <PageHeader
          title="Dashboard"
          subtitle="System overview and status"
          action={<ConnectionBadge wsConnected={wsConnected} wsReconnecting={wsReconnecting} />}
        />

        {/* Active Goal */}
        <GoalHero goal={currentGoal} health={health} onClick={() => navigate('/goals')} />

        {/* Stats Grid */}
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
          <StatCard label="Pending" value={taskCounts['PENDING'] || 0} color="text-content-muted" onClick={() => navigate('/tasks?status=PENDING')} />
          <StatCard label="Ready" value={taskCounts['READY'] || 0} color="text-cyan-600" onClick={() => navigate('/tasks?status=READY')} />
          <StatCard label="Running" value={taskCounts['RUNNING'] || 0} color="text-amber-600" onClick={() => navigate('/tasks?status=RUNNING')} />
          <StatCard label="Completed" value={taskCounts['COMPLETED'] || 0} color="text-emerald-600" delta={completedDelta} deltaLabel="vs yesterday" onClick={() => navigate('/tasks?status=COMPLETED')} />
          <StatCard label="Failed" value={taskCounts['FAILED'] || 0} color="text-rose-600" delta={failedDelta} deltaLabel="vs yesterday" onClick={() => navigate('/tasks?status=FAILED')} />
          <StatCard label="PRs Open" value={pendingPRs} color="text-violet-600" onClick={() => navigate('/prs')} />
        </div>

        {/* Token Usage + System */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
          <TokenUsageCard insights={insights} insightSummary={insightSummary} navigate={navigate} />
          <PodsQuickStat pods={pods} navigate={navigate} />
          <SystemSidebar
            wsConnected={wsConnected}
            taskCounts={taskCounts}
            activeProjectCount={activeProjectCount}
            navigate={navigate}
          />
        </div>
      </div>
    )
  }

  // Bento layout — asymmetric grid
  return (
    <div className="p-5 sm:p-6 lg:p-8 space-y-5 animate-fade-in">
      <PageHeader
        title="Dashboard"
        subtitle="System overview and status"
        action={<ConnectionBadge wsConnected={wsConnected} wsReconnecting={wsReconnecting} />}
      />

      {/* Bento Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-4 gap-4">
        {/* Hero: Active Goal — spans 3 cols */}
        <div className="lg:col-span-3">
          <GoalHero goal={currentGoal} health={health} onClick={() => navigate('/goals')} />
        </div>

        {/* System Health — side column */}
        <div className="card p-5">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3">System</div>
          <div className="space-y-2.5">
            <div className="flex items-center justify-between">
              <span className="text-xs text-content-muted">Connection</span>
              <span className={`text-xs font-medium ${wsConnected ? 'text-emerald-600' : 'text-rose-600'}`}>
                {wsConnected ? 'Live' : 'Offline'}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-xs text-content-muted">Health</span>
              <span className={`text-xs font-medium capitalize ${
                health === 'healthy' ? 'text-emerald-600' : health === 'warning' ? 'text-amber-600' : 'text-rose-600'
              }`}>{health}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-xs text-content-muted">Active Pods</span>
              <span className="text-xs text-content-secondary font-medium">{pods.filter(p => p.status === 'busy').length}/{pods.length}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-xs text-content-muted">Queue</span>
              <span className="text-xs text-content-secondary font-medium">{(taskCounts['READY'] || 0) + (taskCounts['PENDING'] || 0)}</span>
            </div>
          </div>
        </div>

        {/* Stat Cards — 3 in a row + side project card */}
        <StatCard label="Completed" value={taskCounts['COMPLETED'] || 0} color="text-emerald-600" delta={completedDelta} deltaLabel="vs yesterday" onClick={() => navigate('/tasks?status=COMPLETED')} />
        <StatCard label="Failed" value={taskCounts['FAILED'] || 0} color="text-rose-600" delta={failedDelta} deltaLabel="vs yesterday" onClick={() => navigate('/tasks?status=FAILED')} />
        <StatCard label="Running" value={taskCounts['RUNNING'] || 0} color="text-amber-600" onClick={() => navigate('/tasks?status=RUNNING')} />

        {/* Projects + PRs side card */}
        <button
          type="button"
          className="card p-4 w-full text-left transition-all hover:border-line-hover"
          onClick={() => navigate('/projects')}
        >
          <div className="flex items-center justify-between mb-2">
            <div className="text-[11px] font-medium text-content-faint uppercase tracking-wider">Projects</div>
            <div className="text-lg font-bold text-content">{activeProjectCount}</div>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-[11px] text-content-faint uppercase tracking-wider">PRs Open</span>
            <span className="text-sm font-semibold text-violet-600">{pendingPRs}</span>
          </div>
        </button>

        {/* Token Usage — 2 cols */}
        <div className="lg:col-span-2">
          <TokenUsageCard insights={insights} insightSummary={insightSummary} navigate={navigate} />
        </div>

        {/* Pods quick stat */}
        <PodsQuickStat pods={pods} navigate={navigate} />

        {/* Activity card — side */}
        <div className="card p-5">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3">Activity</div>
          <div className="space-y-2.5">
            <div className="flex items-center justify-between">
              <span className="text-xs text-content-muted">Workers</span>
              <span className="text-xs text-content-secondary font-medium">{taskCounts['RUNNING'] || 0}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-xs text-content-muted">Pending</span>
              <span className="text-xs text-content-secondary font-medium">{taskCounts['PENDING'] || 0}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-xs text-content-muted">Ready</span>
              <span className="text-xs text-content-secondary font-medium">{taskCounts['READY'] || 0}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

// ── Subcomponents ──

function ConnectionBadge({ wsConnected, wsReconnecting }: { wsConnected: boolean; wsReconnecting: boolean }) {
  return (
    <div className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-surface-hover border border-line" role="status" aria-live="polite">
      <div
        className={`w-2 h-2 rounded-full transition-colors ${
          wsConnected
            ? 'bg-emerald-400 shadow-sm shadow-emerald-500/30'
            : wsReconnecting
            ? 'bg-amber-400 animate-pulse'
            : 'bg-rose-400'
        }`}
        aria-hidden="true"
      />
      <span className="text-xs text-content-muted">
        {wsConnected ? 'Live' : wsReconnecting ? 'Reconnecting' : 'Offline'}
      </span>
    </div>
  )
}

function GoalHero({ goal, health, onClick }: { goal: Goal | null; health: HealthLevel; onClick: () => void }) {
  const hs = healthStyles[health]
  return (
    <button type="button" className="w-full text-left group" onClick={onClick}>
      <div className={`card p-5 relative overflow-hidden transition-all hover:border-primary-500/30 border-l-4 ${hs.border}`}>
        <div className={`absolute inset-0 bg-gradient-to-r ${hs.bg} pointer-events-none`} />
        <div className="relative">
          <div className="flex items-center gap-2 mb-3">
            <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest">Active Goal</div>
            <div className={`w-1.5 h-1.5 rounded-full ${hs.dot}`} />
          </div>
          {goal ? (
            <div>
              <h3 className="text-base font-semibold text-content mb-1.5 group-hover:text-primary-400 transition-colors">{goal.title}</h3>
              <p className="text-sm text-content-muted mb-3 line-clamp-2">{goal.description}</p>
              {goal.priorities.length > 0 && (
                <div className="flex flex-wrap gap-1.5">
                  {goal.priorities.map((p, i) => (
                    <span key={i} className="badge-info">{p}</span>
                  ))}
                </div>
              )}
            </div>
          ) : (
            <p className="text-sm text-content-faint italic">No active goal — click to set one</p>
          )}
        </div>
      </div>
    </button>
  )
}

function TokenUsageCard({ insights, insightSummary, navigate }: {
  insights: Insights | null
  insightSummary: InsightsSummary | null
  navigate: (path: string) => void
}) {
  return (
    <button
      type="button"
      className="card p-5 w-full text-left transition-all hover:border-line-hover"
      onClick={() => navigate('/insights')}
    >
      <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3">Token Usage</div>
      <div className="grid grid-cols-2 gap-4">
        <div>
          <div className="text-[10px] text-content-faint uppercase tracking-wider mb-0.5">Total Tokens</div>
          <div className="text-lg font-bold text-cyan-500 tabular-nums">
            {insights ? insights.total_tokens.toLocaleString() : '--'}
          </div>
        </div>
        <div>
          <div className="text-[10px] text-content-faint uppercase tracking-wider mb-0.5">Total Cost</div>
          <div className="text-lg font-bold text-emerald-500 tabular-nums">
            {insights ? `$${insights.total_cost.toFixed(2)}` : '--'}
          </div>
        </div>
      </div>
      {insightSummary && (
        <div className="mt-3 pt-3 border-t border-line-subtle flex items-center justify-between">
          <span className="text-[10px] text-content-faint uppercase tracking-wider">Success Rate</span>
          <span className={`text-sm font-semibold ${
            insightSummary.success_rate >= 80 ? 'text-emerald-500' :
            insightSummary.success_rate >= 50 ? 'text-amber-500' : 'text-rose-500'
          }`}>{insightSummary.success_rate.toFixed(1)}%</span>
        </div>
      )}
    </button>
  )
}

function PodsQuickStat({ pods, navigate }: { pods: Pod[]; navigate: (path: string) => void }) {
  const busy = pods.filter((p) => p.status === 'busy').length
  return (
    <button
      type="button"
      className="card p-5 w-full text-left transition-all hover:border-line-hover"
      onClick={() => navigate('/pods')}
    >
      <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3">Pods</div>
      <div className="flex items-end gap-2">
        <span className="text-2xl font-bold text-content tabular-nums">{busy}/{pods.length}</span>
        <span className="text-xs text-content-faint mb-1">active</span>
      </div>
      <div className="mt-2 flex items-center gap-1.5">
        <div className={`w-2 h-2 rounded-full ${busy > 0 ? 'bg-amber-400 animate-pulse' : 'bg-emerald-400'}`} />
        <span className="text-xs text-content-muted">{busy > 0 ? 'Working' : 'Idle'}</span>
      </div>
    </button>
  )
}

function SystemSidebar({ wsConnected, taskCounts, activeProjectCount, navigate }: {
  wsConnected: boolean
  taskCounts: Record<string, number>
  activeProjectCount: number
  navigate: (path: string) => void
}) {
  return (
    <section className="space-y-4">
      <button
        type="button"
        className="card p-5 w-full text-left transition-all hover:border-line-hover"
        onClick={() => navigate('/projects')}
      >
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3">Projects</div>
        <div className="text-2xl font-bold text-content mb-0.5">{activeProjectCount}</div>
        <p className="text-xs text-content-faint">active projects</p>
      </button>

      <div className="card p-5">
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3">System</div>
        <div className="space-y-2.5">
          <div className="flex items-center justify-between">
            <span className="text-xs text-content-muted">WebSocket</span>
            <span className={`text-xs font-medium ${wsConnected ? 'text-emerald-600' : 'text-rose-600'}`}>
              {wsConnected ? 'Connected' : 'Disconnected'}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-xs text-content-muted">Tasks in Queue</span>
            <span className="text-xs text-content-secondary font-medium">{(taskCounts['READY'] || 0) + (taskCounts['PENDING'] || 0)}</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-xs text-content-muted">Active Workers</span>
            <span className="text-xs text-content-secondary font-medium">{taskCounts['RUNNING'] || 0}</span>
          </div>
        </div>
      </div>
    </section>
  )
}
