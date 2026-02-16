import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useGoalStore } from '../stores/goalStore'
import { useTaskStore } from '../stores/taskStore'
import { useProjectStore } from '../stores/projectStore'
import { useWSStore } from '../stores/wsStore'
import { api, Pod, Insights } from '../lib/api'
import { countByStatus } from '../lib/utils'
import PageHeader from '../components/PageHeader'
import StatCard from '../components/StatCard'
import PodCard from '../components/PodCard'
import InsightsPanel from '../components/InsightsPanel'
import LoadingState from '../components/LoadingState'

export default function Dashboard() {
  const navigate = useNavigate()
  const { currentGoal, fetchCurrentGoal } = useGoalStore()
  const { tasks, fetchTasks, setFilters } = useTaskStore()
  const { projects, fetchProjects } = useProjectStore()
  const wsConnected = useWSStore((s) => s.connected)
  const wsReconnecting = useWSStore((s) => s.reconnecting)
  const [pods, setPods] = useState<Pod[]>([])
  const [insights, setInsights] = useState<Insights | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setFilters({})
    Promise.all([fetchCurrentGoal(), fetchTasks(), fetchProjects(), fetchPods(), fetchInsights()])
      .finally(() => setLoading(false))

    const interval = setInterval(fetchPods, 10000)
    return () => clearInterval(interval)
  }, [fetchCurrentGoal, fetchTasks, fetchProjects, setFilters])

  async function fetchPods() {
    try {
      setPods(await api.listPods())
    } catch (error) {
      console.error('Failed to fetch pods:', error)
    }
  }

  async function fetchInsights() {
    try {
      setInsights(await api.getInsights())
    } catch (error) {
      console.error('Failed to fetch insights:', error)
    }
  }

  const taskCounts = countByStatus(tasks)
  const pendingPRs = tasks.filter((t) => t.pr_url && t.pr_status === 'OPEN').length
  const activeProjectCount = projects.filter((p) => p.status === 'ACTIVE').length

  if (loading) {
    return (
      <div className="p-4 sm:p-6 lg:p-8">
        <LoadingState message="Loading dashboard..." />
      </div>
    )
  }

  return (
    <div className="p-4 sm:p-6 lg:p-8 space-y-6 lg:space-y-8">
      {/* Header with system status */}
      <PageHeader
        title="Dashboard"
        subtitle="System overview and status"
        action={
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-slate-800 border border-slate-700" role="status" aria-live="polite">
              <div
                className={`w-2.5 h-2.5 rounded-full ${
                  wsConnected
                    ? 'bg-green-500 animate-pulse'
                    : wsReconnecting
                    ? 'bg-amber-500 animate-pulse'
                    : 'bg-red-500'
                }`}
                aria-hidden="true"
              />
              <span className="text-sm text-slate-400">
                {wsConnected ? 'Connected' : wsReconnecting ? 'Reconnecting...' : 'Disconnected'}
              </span>
            </div>
          </div>
        }
      />

      {/* Current Goal */}
      <section>
        <button
          type="button"
          className="card p-6 w-full text-left hover:border-blue-500/50 transition-colors"
          onClick={() => navigate('/goals')}
        >
          <h2 className="text-sm font-medium text-slate-400 mb-3 uppercase tracking-wider">Active Goal</h2>
          {currentGoal ? (
            <div>
              <h3 className="text-lg font-semibold text-blue-400 mb-2">{currentGoal.title}</h3>
              <p className="text-slate-300 mb-3 line-clamp-2">{currentGoal.description}</p>
              {currentGoal.priorities.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {currentGoal.priorities.map((p, i) => (
                    <span key={i} className="badge-info text-xs">{p}</span>
                  ))}
                </div>
              )}
            </div>
          ) : (
            <p className="text-slate-500 italic">No active goal — click to set one</p>
          )}
        </button>
      </section>

      {/* Stats Grid */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 sm:gap-4">
        <StatCard label="Pending" value={taskCounts['PENDING'] || 0} color="text-slate-400" onClick={() => navigate('/tasks?status=PENDING')} />
        <StatCard label="Ready" value={taskCounts['READY'] || 0} color="text-blue-400" onClick={() => navigate('/tasks?status=READY')} />
        <StatCard label="Running" value={taskCounts['RUNNING'] || 0} color="text-amber-400" onClick={() => navigate('/tasks?status=RUNNING')} />
        <StatCard label="Completed" value={taskCounts['COMPLETED'] || 0} color="text-green-400" onClick={() => navigate('/tasks?status=COMPLETED')} />
        <StatCard label="Failed" value={taskCounts['FAILED'] || 0} color="text-red-400" onClick={() => navigate('/tasks?status=FAILED')} />
        {(taskCounts['DECOMPOSED'] || 0) > 0 && (
          <StatCard label="Decomposed" value={taskCounts['DECOMPOSED'] || 0} color="text-purple-400" onClick={() => navigate('/tasks?status=DECOMPOSED')} />
        )}
        <StatCard label="PRs Open" value={pendingPRs} color="text-purple-400" onClick={() => navigate('/prs')} />
      </div>

      {/* Pods Section */}
      <section className="card p-4 sm:p-6">
        <h2 className="text-xs sm:text-sm font-medium text-slate-400 uppercase tracking-wider mb-4">Pods</h2>
        {pods.length > 0 ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {[...pods].sort((a, b) => a.id.localeCompare(b.id)).map((pod) => (
              <PodCard key={pod.id} pod={pod} />
            ))}
          </div>
        ) : (
          <p className="text-slate-500 italic text-sm py-4 text-center">No pods active</p>
        )}
      </section>

      {/* Two-column layout */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 sm:gap-6">
        <section className="card p-4 sm:p-6 lg:col-span-2">
          <h2 className="text-xs sm:text-sm font-medium text-slate-400 uppercase tracking-wider mb-4">Insights</h2>
          <InsightsPanel insights={insights} onProjectClick={(id) => navigate(`/projects/${id}`)} />
        </section>

        <section className="space-y-4">
          <button
            type="button"
            className="card p-4 sm:p-5 w-full text-left hover:border-slate-600 transition-colors touch-manipulation"
            onClick={() => navigate('/projects')}
          >
            <h2 className="text-xs sm:text-sm font-medium text-slate-400 uppercase tracking-wider mb-3">Projects</h2>
            <div className="text-xl sm:text-2xl font-bold text-slate-100 mb-1">{activeProjectCount}</div>
            <p className="text-xs text-slate-500">active projects</p>
          </button>

          <div className="card p-4 sm:p-5">
            <h2 className="text-xs sm:text-sm font-medium text-slate-400 uppercase tracking-wider mb-3">System</h2>
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-sm text-slate-400">WebSocket</span>
                <span className={`text-sm ${wsConnected ? 'text-green-400' : 'text-red-400'}`}>
                  {wsConnected ? 'Connected' : 'Disconnected'}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-slate-400">Tasks in Queue</span>
                <span className="text-sm text-slate-200">{(taskCounts['READY'] || 0) + (taskCounts['PENDING'] || 0)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-slate-400">Active Workers</span>
                <span className="text-sm text-slate-200">{taskCounts['RUNNING'] || 0}</span>
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>
  )
}
