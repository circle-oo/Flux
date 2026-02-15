import { useEffect, useState } from 'react'
import { useTaskStore } from '../stores/taskStore'
import { useProjectStore } from '../stores/projectStore'
import { Task } from '../lib/api'

export default function Tasks() {
  const {
    tasks,
    isLoading,
    filters,
    setFilters,
    fetchTasks,
    createTask,
    cancelTask,
    retryTask,
  } = useTaskStore()
  const { projects, fetchProjects } = useProjectStore()
  const [showForm, setShowForm] = useState(false)
  const [formData, setFormData] = useState({
    title: '',
    description: '',
    type: 'CODING' as Task['type'],
    priority: 50,
    project_id: '',
    tags: '',
  })

  useEffect(() => {
    fetchTasks()
    fetchProjects()
  }, [fetchTasks, fetchProjects])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await createTask({
        ...formData,
        tags: formData.tags
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      })
      setShowForm(false)
      setFormData({
        title: '',
        description: '',
        type: 'CODING',
        priority: 50,
        project_id: '',
        tags: '',
      })
    } catch (error) {
      console.error('Failed to create task:', error)
    }
  }

  const handleCancel = async (id: string, title: string) => {
    if (confirm(`Cancel task: ${title}?`)) {
      try {
        await cancelTask(id)
      } catch (error) {
        console.error('Failed to cancel task:', error)
      }
    }
  }

  const handleRetry = async (id: string, title: string) => {
    if (confirm(`Retry task: ${title}?`)) {
      try {
        await retryTask(id)
      } catch (error) {
        console.error('Failed to retry task:', error)
      }
    }
  }

  const activeProjects = projects.filter((p) => p.status === 'ACTIVE')

  return (
    <div className="p-8 space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-slate-100 mb-2">Tasks</h1>
          <p className="text-slate-400">Manage and track system tasks</p>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="btn-primary"
        >
          {showForm ? 'Cancel' : '+ New Task'}
        </button>
      </div>

      {/* Filters */}
      <div className="card p-4 flex gap-4">
        <div className="flex-1">
          <label className="label">Status</label>
          <select
            value={filters.status || ''}
            onChange={(e) =>
              setFilters({ ...filters, status: e.target.value || undefined })
            }
            className="input"
          >
            <option value="">All</option>
            <option value="PENDING">Pending</option>
            <option value="READY">Ready</option>
            <option value="RUNNING">Running</option>
            <option value="COMPLETED">Completed</option>
            <option value="FAILED">Failed</option>
          </select>
        </div>
        <div className="flex-1">
          <label className="label">Project</label>
          <select
            value={filters.project_id || ''}
            onChange={(e) =>
              setFilters({
                ...filters,
                project_id: e.target.value || undefined,
              })
            }
            className="input"
          >
            <option value="">All</option>
            {activeProjects.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Create Form */}
      {showForm && (
        <div className="card p-6">
          <h2 className="text-xl font-semibold text-slate-100 mb-4">
            Create New Task
          </h2>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="label">Title</label>
              <input
                type="text"
                value={formData.title}
                onChange={(e) =>
                  setFormData({ ...formData, title: e.target.value })
                }
                className="input"
                required
              />
            </div>
            <div>
              <label className="label">Description</label>
              <textarea
                value={formData.description}
                onChange={(e) =>
                  setFormData({ ...formData, description: e.target.value })
                }
                className="input h-24 resize-none"
                required
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="label">Type</label>
                <select
                  value={formData.type}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      type: e.target.value as Task['type'],
                    })
                  }
                  className="input"
                >
                  <option value="CODING">Coding</option>
                  <option value="RESEARCH">Research</option>
                  <option value="DOCUMENT">Document</option>
                  <option value="MAINTENANCE">Maintenance</option>
                  <option value="DEPLOY">Deploy</option>
                  <option value="BUGFIX">Bug Fix</option>
                  <option value="PLANNING">Planning</option>
                </select>
              </div>
              <div>
                <label className="label">Priority (1-100)</label>
                <input
                  type="number"
                  min="1"
                  max="100"
                  value={formData.priority}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      priority: parseInt(e.target.value),
                    })
                  }
                  className="input"
                  required
                />
              </div>
            </div>
            <div>
              <label className="label">Project</label>
              <select
                value={formData.project_id}
                onChange={(e) =>
                  setFormData({ ...formData, project_id: e.target.value })
                }
                className="input"
                required
              >
                <option value="">Select project</option>
                {activeProjects.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="label">Tags (comma-separated)</label>
              <input
                type="text"
                value={formData.tags}
                onChange={(e) =>
                  setFormData({ ...formData, tags: e.target.value })
                }
                className="input"
                placeholder="frontend, urgent"
              />
            </div>
            <button type="submit" className="btn-primary">
              Create Task
            </button>
          </form>
        </div>
      )}

      {/* Tasks List */}
      <div className="space-y-4">
        {isLoading ? (
          <div className="text-slate-400">Loading...</div>
        ) : tasks.length === 0 ? (
          <div className="card p-6 text-center text-slate-400">
            No tasks found. Create one or adjust filters.
          </div>
        ) : (
          <div className="space-y-3">
            {tasks.map((task) => (
              <div key={task.id} className="card p-6">
                <div className="flex items-start justify-between mb-3">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-2">
                      <h3 className="text-lg font-medium text-slate-100">
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
                      <span className="badge-secondary">{task.type}</span>
                    </div>
                    <p className="text-slate-300 mb-3">{task.description}</p>
                    <div className="text-sm text-slate-500">
                      Priority: {task.priority} • Source: {task.source}
                      {task.pr_url && (
                        <>
                          {' '}
                          •{' '}
                          <a
                            href={task.pr_url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-blue-400 hover:underline"
                          >
                            PR
                          </a>
                        </>
                      )}
                    </div>
                    {task.tags.length > 0 && (
                      <div className="flex flex-wrap gap-2 mt-3">
                        {task.tags.map((tag, i) => (
                          <span key={i} className="badge-info text-xs">
                            {tag}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                  <div className="flex gap-2 ml-4">
                    {(task.status === 'FAILED' || task.status === 'RETRY') && (
                      <button
                        onClick={() => handleRetry(task.id, task.title)}
                        className="px-3 py-1.5 rounded text-sm font-medium bg-blue-600 text-white hover:bg-blue-500 transition-colors"
                      >
                        Retry
                      </button>
                    )}
                    {(task.status === 'READY' || task.status === 'RUNNING') && (
                      <button
                        onClick={() => handleCancel(task.id, task.title)}
                        className="btn-danger"
                      >
                        Cancel
                      </button>
                    )}
                  </div>
                </div>
                {task.error_log && (
                  <div className="mt-3 p-3 bg-red-900/30 border border-red-600 rounded text-sm text-red-200">
                    <strong>Error:</strong> {task.error_log}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
