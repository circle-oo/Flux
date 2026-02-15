import { useEffect } from 'react'
import { useGoalStore } from '../stores/goalStore'
import { useTaskStore } from '../stores/taskStore'
import { useProjectStore } from '../stores/projectStore'

export default function Dashboard() {
  const { currentGoal, fetchCurrentGoal } = useGoalStore()
  const { tasks, fetchTasks } = useTaskStore()
  const { projects, fetchProjects } = useProjectStore()

  useEffect(() => {
    fetchCurrentGoal()
    fetchTasks()
    fetchProjects()
  }, [fetchCurrentGoal, fetchTasks, fetchProjects])

  const tasksByStatus = {
    READY: tasks.filter((t) => t.status === 'READY').length,
    RUNNING: tasks.filter((t) => t.status === 'RUNNING').length,
    COMPLETED: tasks.filter((t) => t.status === 'COMPLETED').length,
    FAILED: tasks.filter((t) => t.status === 'FAILED').length,
  }

  const activeProjects = projects.filter((p) => p.status === 'ACTIVE').length
  const recentTasks = tasks.slice(0, 10)

  return (
    <div className="p-8 space-y-8">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-slate-100 mb-2">Dashboard</h1>
        <p className="text-slate-400">System overview and status</p>
      </div>

      {/* Current Goal */}
      <section className="card p-6">
        <h2 className="text-xl font-semibold text-slate-100 mb-4">
          Active Goal
        </h2>
        {currentGoal ? (
          <div>
            <h3 className="text-lg font-medium text-blue-400 mb-2">
              {currentGoal.title}
            </h3>
            <p className="text-slate-300 mb-4">{currentGoal.description}</p>
            {currentGoal.priorities.length > 0 && (
              <div className="mb-2">
                <span className="text-sm font-medium text-slate-400">
                  Priorities:
                </span>
                <div className="flex flex-wrap gap-2 mt-1">
                  {currentGoal.priorities.map((p, i) => (
                    <span key={i} className="badge-info">
                      {p}
                    </span>
                  ))}
                </div>
              </div>
            )}
            {currentGoal.active_since && (
              <p className="text-sm text-slate-500">
                Active since: {new Date(currentGoal.active_since).toLocaleString()}
              </p>
            )}
          </div>
        ) : (
          <p className="text-slate-400 italic">No active goal set</p>
        )}
      </section>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <div className="card p-6">
          <div className="text-sm font-medium text-slate-400 mb-1">
            Ready Tasks
          </div>
          <div className="text-3xl font-bold text-blue-400">
            {tasksByStatus.READY}
          </div>
        </div>

        <div className="card p-6">
          <div className="text-sm font-medium text-slate-400 mb-1">
            Running
          </div>
          <div className="text-3xl font-bold text-amber-400">
            {tasksByStatus.RUNNING}
          </div>
        </div>

        <div className="card p-6">
          <div className="text-sm font-medium text-slate-400 mb-1">
            Completed
          </div>
          <div className="text-3xl font-bold text-green-400">
            {tasksByStatus.COMPLETED}
          </div>
        </div>

        <div className="card p-6">
          <div className="text-sm font-medium text-slate-400 mb-1">
            Active Projects
          </div>
          <div className="text-3xl font-bold text-slate-100">
            {activeProjects}
          </div>
        </div>
      </div>

      {/* Recent Tasks */}
      <section className="card p-6">
        <h2 className="text-xl font-semibold text-slate-100 mb-4">
          Recent Tasks
        </h2>
        {recentTasks.length > 0 ? (
          <div className="space-y-3">
            {recentTasks.map((task) => (
              <div
                key={task.id}
                className="flex items-center justify-between p-3 bg-slate-700/50 rounded-lg"
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <h3 className="text-slate-100 font-medium truncate">
                      {task.title}
                    </h3>
                    <span
                      className={`badge ${
                        task.status === 'COMPLETED'
                          ? 'badge-success'
                          : task.status === 'FAILED'
                          ? 'badge-danger'
                          : task.status === 'RUNNING'
                          ? 'badge-warning'
                          : task.status === 'READY'
                          ? 'badge-info'
                          : 'badge-secondary'
                      }`}
                    >
                      {task.status}
                    </span>
                  </div>
                  <p className="text-sm text-slate-400">
                    {task.type} • Priority {task.priority}
                  </p>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-slate-400 italic">No tasks yet</p>
        )}
      </section>

      {/* System Status */}
      <section className="card p-6">
        <h2 className="text-xl font-semibold text-slate-100 mb-4">
          System Status
        </h2>
        <div className="flex items-center gap-3">
          <div className="w-3 h-3 bg-green-500 rounded-full animate-pulse"></div>
          <span className="text-slate-300">System operational</span>
        </div>
      </section>
    </div>
  )
}
