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
      <div className="p-5 sm:p-6 lg:p-8">
        <LoadingState message="Loading dashboard..." />
      </div>
    )
  }

  return (
    <div className="p-5 sm:p-6 lg:p-8 space-y-6 animate-fade-in">
      <PageHeader
        title="Dashboard"
        subtitle="System overview and status"
        action={
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-white/[0.04] border border-white/[0.06]" role="status" aria-live="polite">
            <div
              className={`w-2 h-2 rounded-full transition-colors ${
                wsConnected
                  ? 'bg-emerald-400 shadow-sm shadow-emerald-400/50'
                  : wsReconnecting
                  ? 'bg-amber-400 animate-pulse'
                  : 'bg-rose-400'
              }`}
              aria-hidden="true"
            />
            <span className="text-xs text-white/40">
              {wsConnected ? 'Live' : wsReconnecting ? 'Reconnecting' : 'Offline'}
            </span>
          </div>
        }
      />

      {/* Active Goal */}
      <button
        type="button"
        className="w-full text-left group"
        onClick={() => navigate('/goals')}
      >
        <div className="card p-5 relative overflow-hidden transition-all hover:border-accent-500/30">
          <div className="absolute inset-0 bg-gradient-to-r from-accent-600/5 to-violet-600/5 pointer-events-none" />
          <div className="relative">
            <div className="text-[11px] font-medium text-white/30 uppercase tracking-widest mb-3">Active Goal</div>
            {currentGoal ? (
              <div>
                <h3 className="text-base font-semibold text-white/90 mb-1.5 group-hover:text-accent-400 transition-colors">{currentGoal.title}</h3>
                <p className="text-sm text-white/50 mb-3 line-clamp-2">{currentGoal.description}</p>
                {currentGoal.priorities.length > 0 && (
                  <div className="flex flex-wrap gap-1.5">
                    {currentGoal.priorities.map((p, i) => (
                      <span key={i} className="badge-info">{p}</span>
                    ))}
                  </div>
                )}
              </div>
            ) : (
              <p className="text-sm text-white/20 italic">No active goal — click to set one</p>
            )}
          </div>
        </div>
      </button>

      {/* Stats Grid */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
        <StatCard label="Pending" value={taskCounts['PENDING'] || 0} color="text-white/40" onClick={() => navigate('/tasks?status=PENDING')} />
        <StatCard label="Ready" value={taskCounts['READY'] || 0} color="text-cyan-400" onClick={() => navigate('/tasks?status=READY')} />
        <StatCard label="Running" value={taskCounts['RUNNING'] || 0} color="text-amber-400" onClick={() => navigate('/tasks?status=RUNNING')} />
        <StatCard label="Completed" value={taskCounts['COMPLETED'] || 0} color="text-emerald-400" onClick={() => navigate('/tasks?status=COMPLETED')} />
        <StatCard label="Failed" value={taskCounts['FAILED'] || 0} color="text-rose-400" onClick={() => navigate('/tasks?status=FAILED')} />
        {(taskCounts['DECOMPOSED'] || 0) > 0 && (
          <StatCard label="Decomposed" value={taskCounts['DECOMPOSED'] || 0} color="text-violet-400" onClick={() => navigate('/tasks?status=DECOMPOSED')} />
        )}
        <StatCard label="PRs Open" value={pendingPRs} color="text-violet-400" onClick={() => navigate('/prs')} />
      </div>

      {/* Pods */}
      <section className="card p-5">
        <div className="text-[11px] font-medium text-white/30 uppercase tracking-widest mb-4">Pods</div>
        {pods.length > 0 ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {[...pods].sort((a, b) => a.id.localeCompare(b.id)).map((pod) => (
              <PodCard key={pod.id} pod={pod} />
            ))}
          </div>
        ) : (
          <p className="text-white/20 italic text-sm py-4 text-center">No pods active</p>
        )}
      </section>

      {/* Insights + System */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <section className="card p-5 lg:col-span-2">
          <div className="text-[11px] font-medium text-white/30 uppercase tracking-widest mb-4">Insights</div>
          <InsightsPanel insights={insights} onProjectClick={(id) => navigate(`/projects/${id}`)} />
        </section>

        <section className="space-y-4">
          <button
            type="button"
            className="card p-5 w-full text-left transition-all hover:border-white/[0.1]"
            onClick={() => navigate('/projects')}
          >
            <div className="text-[11px] font-medium text-white/30 uppercase tracking-widest mb-3">Projects</div>
            <div className="text-2xl font-bold text-white/90 mb-0.5">{activeProjectCount}</div>
            <p className="text-xs text-white/30">active projects</p>
          </button>

          <div className="card p-5">
            <div className="text-[11px] font-medium text-white/30 uppercase tracking-widest mb-3">System</div>
            <div className="space-y-2.5">
              <div className="flex items-center justify-between">
                <span className="text-xs text-white/40">WebSocket</span>
                <span className={`text-xs font-medium ${wsConnected ? 'text-emerald-400' : 'text-rose-400'}`}>
                  {wsConnected ? 'Connected' : 'Disconnected'}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-xs text-white/40">Tasks in Queue</span>
                <span className="text-xs text-white/70 font-medium">{(taskCounts['READY'] || 0) + (taskCounts['PENDING'] || 0)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-xs text-white/40">Active Workers</span>
                <span className="text-xs text-white/70 font-medium">{taskCounts['RUNNING'] || 0}</span>
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>
  )
}
