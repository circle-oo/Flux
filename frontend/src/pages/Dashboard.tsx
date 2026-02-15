import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useGoalStore } from '../stores/goalStore'
import { useTaskStore } from '../stores/taskStore'
import { useProjectStore } from '../stores/projectStore'
import { useWSStore } from '../stores/wsStore'
import { api, Pod, Insights } from '../lib/api'

function formatUptime(ms: number): string {
  const totalMinutes = Math.floor(ms / 1000 / 60)
  if (totalMinutes < 60) return `${totalMinutes}m`
  const hours = Math.floor(totalMinutes / 60)
  const minutes = totalMinutes % 60
  if (hours < 24) return `${hours}h ${minutes}m`
  const days = Math.floor(hours / 24)
  const remainingHours = hours % 24
  return `${days}d ${remainingHours}h`
}

function StatCard({
  label,
  value,
  color,
  onClick,
}: {
  label: string
  value: number
  color: string
  onClick?: () => void
}) {
  return (
    <div
      className={`card p-4 sm:p-5 ${onClick ? 'cursor-pointer hover:bg-slate-700/50 transition-colors touch-manipulation' : ''}`}
      onClick={onClick}
    >
      <div className="text-xs sm:text-sm font-medium text-slate-400 mb-1">{label}</div>
      <div className={`text-2xl sm:text-3xl font-bold ${color}`}>{value}</div>
    </div>
  )
}

export default function Dashboard() {
  const navigate = useNavigate()
  const { currentGoal, fetchCurrentGoal } = useGoalStore()
  const { tasks, fetchTasks, setFilters } = useTaskStore()
  const { projects, fetchProjects } = useProjectStore()
  const wsConnected = useWSStore((s) => s.connected)
  const wsReconnecting = useWSStore((s) => s.reconnecting)
  const [pods, setPods] = useState<Pod[]>([])
  const [insights, setInsights] = useState<Insights | null>(null)

  useEffect(() => {
    // Reset filters to ensure dashboard shows all tasks
    setFilters({})
    fetchCurrentGoal()
    fetchTasks()
    fetchProjects()
    fetchPods()
    fetchInsights()

    // Poll pod status every 10 seconds
    const interval = setInterval(fetchPods, 10000)
    return () => clearInterval(interval)
  }, [fetchCurrentGoal, fetchTasks, fetchProjects, setFilters])

  async function fetchPods() {
    try {
      const data = await api.listPods()
      setPods(data)
    } catch (error) {
      console.error('Failed to fetch pods:', error)
    }
  }

  async function fetchInsights() {
    try {
      const data = await api.getInsights()
      setInsights(data)
    } catch (error) {
      console.error('Failed to fetch insights:', error)
    }
  }

  const tasksByStatus = {
    PENDING: tasks.filter((t) => t.status === 'PENDING').length,
    READY: tasks.filter((t) => t.status === 'READY').length,
    RUNNING: tasks.filter((t) => t.status === 'RUNNING').length,
    DECOMPOSED: tasks.filter((t) => t.status === 'DECOMPOSED').length,
    COMPLETED: tasks.filter((t) => t.status === 'COMPLETED').length,
    FAILED: tasks.filter((t) => t.status === 'FAILED').length,
  }

  const pendingPRs = tasks.filter(
    (t) => t.pr_url && t.pr_status === 'OPEN'
  ).length
  const activeProjects = projects.filter((p) => p.status === 'ACTIVE').length

  return (
    <div className="p-4 sm:p-6 lg:p-8 space-y-6 lg:space-y-8">
      {/* Header with system status */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl sm:text-3xl font-bold text-slate-100 mb-1 sm:mb-2">Dashboard</h1>
          <p className="text-sm sm:text-base text-slate-400">System overview and status</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-slate-800 border border-slate-700">
            <div
              className={`w-2.5 h-2.5 rounded-full ${
                wsConnected
                  ? 'bg-green-500 animate-pulse'
                  : wsReconnecting
                  ? 'bg-amber-500 animate-pulse'
                  : 'bg-red-500'
              }`}
            />
            <span className="text-sm text-slate-400">
              {wsConnected ? 'Connected' : wsReconnecting ? 'Reconnecting...' : 'Disconnected'}
            </span>
          </div>
        </div>
      </div>

      {/* Current Goal */}
      <section
        className="card p-6 cursor-pointer hover:border-blue-500/50 transition-colors"
        onClick={() => navigate('/goals')}
      >
        <h2 className="text-sm font-medium text-slate-400 mb-3 uppercase tracking-wider">
          Active Goal
        </h2>
        {currentGoal ? (
          <div>
            <h3 className="text-lg font-semibold text-blue-400 mb-2">
              {currentGoal.title}
            </h3>
            <p className="text-slate-300 mb-3 line-clamp-2">{currentGoal.description}</p>
            {currentGoal.priorities.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {currentGoal.priorities.map((p, i) => (
                  <span key={i} className="badge-info text-xs">
                    {p}
                  </span>
                ))}
              </div>
            )}
          </div>
        ) : (
          <p className="text-slate-500 italic">No active goal — click to set one</p>
        )}
      </section>

      {/* Stats Grid */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 sm:gap-4">
        <StatCard
          label="Pending"
          value={tasksByStatus.PENDING}
          color="text-slate-400"
          onClick={() => navigate('/tasks?status=PENDING')}
        />
        <StatCard
          label="Ready"
          value={tasksByStatus.READY}
          color="text-blue-400"
          onClick={() => navigate('/tasks?status=READY')}
        />
        <StatCard
          label="Running"
          value={tasksByStatus.RUNNING}
          color="text-amber-400"
          onClick={() => navigate('/tasks?status=RUNNING')}
        />
        <StatCard
          label="Completed"
          value={tasksByStatus.COMPLETED}
          color="text-green-400"
          onClick={() => navigate('/tasks?status=COMPLETED')}
        />
        <StatCard
          label="Failed"
          value={tasksByStatus.FAILED}
          color="text-red-400"
          onClick={() => navigate('/tasks?status=FAILED')}
        />
        {tasksByStatus.DECOMPOSED > 0 && (
          <StatCard
            label="Decomposed"
            value={tasksByStatus.DECOMPOSED}
            color="text-purple-400"
            onClick={() => navigate('/tasks?status=DECOMPOSED')}
          />
        )}
        <StatCard
          label="PRs Open"
          value={pendingPRs}
          color="text-purple-400"
          onClick={() => navigate('/prs')}
        />
      </div>

      {/* Pods Section */}
      <section className="card p-4 sm:p-6">
        <h2 className="text-xs sm:text-sm font-medium text-slate-400 uppercase tracking-wider mb-4">
          Pods
        </h2>
        {pods.length > 0 ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {pods.map((pod) => (
              <div
                key={pod.id}
                className={`p-4 rounded-lg border transition-colors ${
                  pod.status === 'busy'
                    ? 'bg-amber-900/20 border-amber-700/50'
                    : 'bg-slate-700/30 border-slate-600/50'
                }`}
              >
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <h3 className="text-sm font-semibold text-slate-200">{pod.id}</h3>
                    <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
                      pod.pod_type === 'researcher'
                        ? 'bg-purple-600/30 text-purple-300 border border-purple-600/50'
                        : 'bg-blue-600/30 text-blue-300 border border-blue-600/50'
                    }`}>
                      {pod.pod_type || 'executor'}
                    </span>
                  </div>
                  <span
                    className={`px-2 py-0.5 rounded text-xs font-medium ${
                      pod.status === 'busy'
                        ? 'bg-amber-600 text-white'
                        : 'bg-slate-600 text-slate-300'
                    }`}
                  >
                    {pod.status}
                  </span>
                </div>

                {pod.current_task && pod.task_title ? (
                  <div className="mb-2">
                    <p className="text-xs text-slate-400 mb-0.5">Current Task:</p>
                    <p className="text-xs text-slate-200 truncate" title={pod.task_title}>
                      {pod.task_title}
                    </p>
                  </div>
                ) : (
                  <p className="text-xs text-slate-500 italic mb-2">No active task</p>
                )}

                <div className="flex items-center justify-between text-xs text-slate-400">
                  <span>Tasks: {pod.task_count}</span>
                  <span title={`Started: ${new Date(pod.started_at).toLocaleString()}`}>
                    Uptime:{' '}
                    {formatUptime(Date.now() - new Date(pod.started_at).getTime())}
                  </span>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-slate-500 italic text-sm py-4 text-center">
            No pods active
          </p>
        )}
      </section>

      {/* Two-column layout */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 sm:gap-6">
        {/* Insights */}
        <section className="card p-4 sm:p-6 lg:col-span-2">
          <h2 className="text-xs sm:text-sm font-medium text-slate-400 uppercase tracking-wider mb-4">
            Insights
          </h2>
          {insights ? (
            <div className="space-y-6">
              {/* Token Usage & Cost */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="p-4 rounded-lg bg-slate-700/30">
                  <div className="text-xs text-slate-400 mb-1">Total Token Usage</div>
                  <div className="text-2xl font-bold text-blue-400">
                    {insights.total_tokens.toLocaleString()}
                  </div>
                </div>
                <div className="p-4 rounded-lg bg-slate-700/30">
                  <div className="text-xs text-slate-400 mb-1">Total Cost</div>
                  <div className="text-2xl font-bold text-green-400">
                    ${insights.total_cost.toFixed(2)}
                  </div>
                </div>
              </div>

              {/* Activities per Project */}
              <div>
                <h3 className="text-xs font-medium text-slate-400 uppercase tracking-wider mb-3">
                  Activities per Project
                </h3>
                {insights.project_activities.length > 0 ? (
                  <div className="space-y-2">
                    {insights.project_activities.map((activity) => (
                      <div
                        key={activity.project_id}
                        className="flex items-center justify-between p-3 rounded-lg bg-slate-700/30 hover:bg-slate-700/60 cursor-pointer transition-colors"
                        onClick={() => navigate(`/projects/${activity.project_id}`)}
                      >
                        <div className="flex-1 min-w-0">
                          <h4 className="text-slate-100 font-medium truncate text-sm">
                            {activity.project_name}
                          </h4>
                        </div>
                        <div className="text-sm font-semibold text-slate-300">
                          {activity.task_count} {activity.task_count === 1 ? 'task' : 'tasks'}
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-slate-500 italic text-sm py-4 text-center">
                    No project activities yet
                  </p>
                )}
              </div>
            </div>
          ) : (
            <p className="text-slate-500 italic text-sm py-4 text-center">
              Loading insights...
            </p>
          )}
        </section>

        {/* Quick Info Panel */}
        <section className="space-y-4">
          {/* Active Projects */}
          <div
            className="card p-4 sm:p-5 cursor-pointer hover:border-slate-600 transition-colors touch-manipulation"
            onClick={() => navigate('/projects')}
          >
            <h2 className="text-xs sm:text-sm font-medium text-slate-400 uppercase tracking-wider mb-3">
              Projects
            </h2>
            <div className="text-xl sm:text-2xl font-bold text-slate-100 mb-1">
              {activeProjects}
            </div>
            <p className="text-xs text-slate-500">active projects</p>
          </div>

          {/* System Status */}
          <div className="card p-4 sm:p-5">
            <h2 className="text-xs sm:text-sm font-medium text-slate-400 uppercase tracking-wider mb-3">
              System
            </h2>
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-sm text-slate-400">WebSocket</span>
                <span className={`text-sm ${wsConnected ? 'text-green-400' : 'text-red-400'}`}>
                  {wsConnected ? 'Connected' : 'Disconnected'}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-slate-400">Tasks in Queue</span>
                <span className="text-sm text-slate-200">
                  {tasksByStatus.READY + tasksByStatus.PENDING}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-slate-400">Active Workers</span>
                <span className="text-sm text-slate-200">{tasksByStatus.RUNNING}</span>
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>
  )
}
