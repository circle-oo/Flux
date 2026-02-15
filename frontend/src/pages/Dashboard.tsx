import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useGoalStore } from '../stores/goalStore'
import { useTaskStore } from '../stores/taskStore'
import { useProjectStore } from '../stores/projectStore'
import { useWSStore } from '../stores/wsStore'
import { api, Pod } from '../lib/api'

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
      className={`card p-5 ${onClick ? 'cursor-pointer hover:bg-slate-700/50 transition-colors' : ''}`}
      onClick={onClick}
    >
      <div className="text-sm font-medium text-slate-400 mb-1">{label}</div>
      <div className={`text-3xl font-bold ${color}`}>{value}</div>
    </div>
  )
}

export default function Dashboard() {
  const navigate = useNavigate()
  const { currentGoal, fetchCurrentGoal } = useGoalStore()
  const { tasks, fetchTasks } = useTaskStore()
  const { projects, fetchProjects } = useProjectStore()
  const wsConnected = useWSStore((s) => s.connected)
  const wsReconnecting = useWSStore((s) => s.reconnecting)
  const [pods, setPods] = useState<Pod[]>([])

  useEffect(() => {
    fetchCurrentGoal()
    fetchTasks()
    fetchProjects()
    fetchPods()

    // Poll pod status every 10 seconds
    const interval = setInterval(fetchPods, 10000)
    return () => clearInterval(interval)
  }, [fetchCurrentGoal, fetchTasks, fetchProjects])

  async function fetchPods() {
    try {
      const data = await api.listPods()
      setPods(data)
    } catch (error) {
      console.error('Failed to fetch pods:', error)
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
  const recentTasks = tasks.slice(0, 10)

  return (
    <div className="p-8 space-y-8">
      {/* Header with system status */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-slate-100 mb-2">Dashboard</h1>
          <p className="text-slate-400">System overview and status</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-800 border border-slate-700">
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
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
        <StatCard
          label="Pending"
          value={tasksByStatus.PENDING}
          color="text-slate-400"
          onClick={() => navigate('/tasks')}
        />
        <StatCard
          label="Ready"
          value={tasksByStatus.READY}
          color="text-blue-400"
          onClick={() => navigate('/tasks')}
        />
        <StatCard
          label="Running"
          value={tasksByStatus.RUNNING}
          color="text-amber-400"
          onClick={() => navigate('/tasks')}
        />
        <StatCard
          label="Completed"
          value={tasksByStatus.COMPLETED}
          color="text-green-400"
          onClick={() => navigate('/tasks')}
        />
        <StatCard
          label="Failed"
          value={tasksByStatus.FAILED}
          color="text-red-400"
          onClick={() => navigate('/tasks')}
        />
        {tasksByStatus.DECOMPOSED > 0 && (
          <StatCard
            label="Decomposed"
            value={tasksByStatus.DECOMPOSED}
            color="text-purple-400"
            onClick={() => navigate('/tasks')}
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
      <section className="card p-6">
        <h2 className="text-sm font-medium text-slate-400 uppercase tracking-wider mb-4">
          Executor Pods
        </h2>
        {pods.length > 0 ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
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
                  <h3 className="text-sm font-semibold text-slate-200">{pod.id}</h3>
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
                    {Math.floor(
                      (Date.now() - new Date(pod.started_at).getTime()) / 1000 / 60
                    )}
                    m
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
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Recent Tasks */}
        <section className="card p-6 lg:col-span-2">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-sm font-medium text-slate-400 uppercase tracking-wider">
              Recent Tasks
            </h2>
            <button
              onClick={() => navigate('/tasks')}
              className="text-xs text-blue-400 hover:text-blue-300"
            >
              View all
            </button>
          </div>
          {recentTasks.length > 0 ? (
            <div className="space-y-2">
              {recentTasks.map((task) => (
                <div
                  key={task.id}
                  className="flex items-center justify-between p-3 rounded-lg bg-slate-700/30 hover:bg-slate-700/60 cursor-pointer transition-colors"
                  onClick={() => navigate(`/tasks/${task.id}`)}
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-0.5">
                      <h3 className="text-slate-100 font-medium truncate text-sm">
                        {task.title}
                      </h3>
                      <span
                        className={`badge text-[10px] ${
                          task.status === 'COMPLETED'
                            ? 'badge-success'
                            : task.status === 'FAILED'
                            ? 'badge-danger'
                            : task.status === 'RUNNING'
                            ? 'badge-warning'
                            : task.status === 'READY'
                            ? 'badge-info'
                            : task.status === 'DECOMPOSED'
                            ? 'bg-purple-600 text-white px-2 py-1 rounded text-xs font-semibold'
                            : task.status === 'CANCELLED'
                            ? 'bg-slate-500 text-white px-2 py-1 rounded text-xs font-semibold'
                            : 'badge-secondary'
                        }`}
                      >
                        {task.status}
                      </span>
                    </div>
                    <p className="text-xs text-slate-500">
                      {task.type} · P{task.priority}
                      {task.status === 'RUNNING' && task.executor_id && (
                        <> · {task.executor_id}</>
                      )}
                      {task.status === 'RUNNING' && task.model && (
                        <> · {task.model}</>
                      )}
                      {task.pr_url && ' · PR'}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-slate-500 italic text-sm py-4 text-center">
              No tasks yet
            </p>
          )}
        </section>

        {/* Quick Info Panel */}
        <section className="space-y-4">
          {/* Active Projects */}
          <div
            className="card p-5 cursor-pointer hover:border-slate-600 transition-colors"
            onClick={() => navigate('/projects')}
          >
            <h2 className="text-sm font-medium text-slate-400 uppercase tracking-wider mb-3">
              Projects
            </h2>
            <div className="text-2xl font-bold text-slate-100 mb-1">
              {activeProjects}
            </div>
            <p className="text-xs text-slate-500">active projects</p>
          </div>

          {/* System Status */}
          <div className="card p-5">
            <h2 className="text-sm font-medium text-slate-400 uppercase tracking-wider mb-3">
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
