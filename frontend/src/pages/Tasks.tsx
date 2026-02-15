import { useEffect, useState, useMemo, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTaskStore } from '../stores/taskStore'
import { useProjectStore } from '../stores/projectStore'
import { useGoalStore } from '../stores/goalStore'
import { Task, api } from '../lib/api'

function timeAgo(iso: string): string {
  const now = Date.now()
  const then = new Date(iso).getTime()
  const seconds = Math.floor((now - then) / 1000)
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 7) return `${days}d ago`
  return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

function duration(startIso: string, endIso: string): string {
  const ms = new Date(endIso).getTime() - new Date(startIso).getTime()
  const seconds = Math.floor(ms / 1000)
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  const remainMins = minutes % 60
  return remainMins > 0 ? `${hours}h ${remainMins}m` : `${hours}h`
}

const priorityPresets = [
  { value: 1, label: 'Critical', color: 'text-red-400' },
  { value: 10, label: 'High', color: 'text-amber-400' },
  { value: 30, label: 'Medium', color: 'text-blue-400' },
  { value: 50, label: 'Normal', color: 'text-slate-400' },
  { value: 80, label: 'Low', color: 'text-slate-500' },
]

const typeDescriptions: Record<string, string> = {
  CODING: 'Implementation, features, refactoring',
  BUGFIX: 'Fix a bug or regression',
  RESEARCH: 'Investigation, analysis, no code changes',
  DOCUMENT: 'Documentation, README, comments',
  MAINTENANCE: 'Deps, cleanup, CI/CD, infra',
  DEPLOY: 'Deployment, release, rollback',
  PLANNING: 'Architecture, design, strategy',
}

const mainStatusFilters = [
  { value: '', label: 'All' },
  { value: 'READY', label: 'Ready' },
  { value: 'RUNNING', label: 'Running' },
  { value: 'COMPLETED', label: 'Completed' },
  { value: 'FAILED', label: 'Failed' },
]

const moreStatusFilters = [
  { value: 'PENDING', label: 'Pending' },
  { value: 'DECOMPOSED', label: 'Decomposed' },
  { value: 'CANCELLED', label: 'Cancelled' },
  { value: 'RETRY', label: 'Retry' },
  { value: 'ARCHIVED', label: 'Archived' },
]

type SortOption = 'priority' | 'newest' | 'updated' | 'status'

const sortOptions: { value: SortOption; label: string }[] = [
  { value: 'priority', label: 'Priority' },
  { value: 'newest', label: 'Newest' },
  { value: 'updated', label: 'Updated' },
  { value: 'status', label: 'Status' },
]

const statusOrder: Record<string, number> = {
  RUNNING: 0,
  READY: 1,
  RETRY: 2,
  PENDING: 3,
  DECOMPOSED: 4,
  FAILED: 5,
  CANCELLED: 6,
  COMPLETED: 7,
  ARCHIVED: 8,
}

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
    archiveTask,
  } = useTaskStore()
  const { projects, fetchProjects } = useProjectStore()
  const { currentGoal, fetchCurrentGoal } = useGoalStore()
  const navigate = useNavigate()
  const [showForm, setShowForm] = useState(false)
  const [showMoreStatuses, setShowMoreStatuses] = useState(false)
  const [sortBy, setSortBy] = useState<SortOption>('priority')
  const [formData, setFormData] = useState({
    title: '',
    description: '',
    type: 'CODING' as Task['type'],
    priority: 50,
    project_id: '',
    goal_id: '',
    tags: '',
    prompt: '',
  })
  const [showAdvanced, setShowAdvanced] = useState(false)

  const [subtaskCounts, setSubtaskCounts] = useState<Record<string, number>>({})

  useEffect(() => {
    fetchTasks()
    fetchProjects()
    fetchCurrentGoal()
  }, [fetchTasks, fetchProjects, fetchCurrentGoal])

  // Fetch subtask counts for DECOMPOSED parent tasks
  const fetchSubtaskCounts = useCallback(async () => {
    const parents = tasks.filter((t) => t.status === 'DECOMPOSED' || t.tags?.includes('build-failure'))
    const counts: Record<string, number> = {}
    for (const parent of parents) {
      try {
        const subs = await api.listSubtasks(parent.id)
        if (subs.length > 0) counts[parent.id] = subs.length
      } catch { /* ignore */ }
    }
    setSubtaskCounts(counts)
  }, [tasks])

  useEffect(() => {
    if (tasks.length > 0) fetchSubtaskCounts()
  }, [tasks, fetchSubtaskCounts])

  // Filter out archived tasks and subtasks by default (unless explicitly filtering)
  const visibleTasks = useMemo(() => {
    let filtered = tasks
    // Hide subtasks from top-level list
    if (!filters.status) {
      filtered = filtered.filter((t) => !t.parent_id && t.status !== 'ARCHIVED')
    } else if (filters.status === 'ARCHIVED') {
      // Show all when filtering archived
    } else {
      filtered = filtered.filter((t) => !t.parent_id)
    }
    return filtered
  }, [tasks, filters.status])

  const sortedTasks = useMemo(() => {
    const sorted = [...visibleTasks]
    switch (sortBy) {
      case 'priority':
        sorted.sort((a, b) => a.priority - b.priority || new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
        break
      case 'newest':
        sorted.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
        break
      case 'updated':
        sorted.sort((a, b) => {
          const aTime = new Date(a.updated_at || a.created_at).getTime()
          const bTime = new Date(b.updated_at || b.created_at).getTime()
          return bTime - aTime
        })
        break
      case 'status':
        sorted.sort((a, b) => (statusOrder[a.status] ?? 99) - (statusOrder[b.status] ?? 99))
        break
    }
    return sorted
  }, [visibleTasks, sortBy])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await createTask({
        title: formData.title,
        description: formData.description,
        type: formData.type,
        priority: formData.priority,
        project_id: formData.project_id,
        goal_id: formData.goal_id || undefined,
        tags: formData.tags
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      })
      setShowForm(false)
      setShowAdvanced(false)
      setFormData({
        title: '',
        description: '',
        type: 'CODING',
        priority: 50,
        project_id: '',
        goal_id: '',
        tags: '',
        prompt: '',
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

  const handleArchive = async (id: string, title: string) => {
    if (confirm(`Archive task: ${title}?`)) {
      try {
        await archiveTask(id)
      } catch (error) {
        console.error('Failed to archive task:', error)
      }
    }
  }

  const handleStatusFilter = (value: string) => {
    const newStatus = filters.status === value ? undefined : value || undefined
    setFilters({ ...filters, status: newStatus })
  }

  const activeProjects = projects.filter((p) => p.status === 'ACTIVE')
  const projectMap = Object.fromEntries(projects.map((p) => [p.id, p]))

  return (
    <div className="p-8 space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-3 mb-2">
            <h1 className="text-3xl font-bold text-slate-100">Tasks</h1>
            {tasks.length > 0 && (
              <span className="badge badge-info text-lg px-3 py-1">
                {tasks.length}
              </span>
            )}
          </div>
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
      <div className="card p-4 space-y-3">
        <div className="flex items-center gap-3 flex-wrap">
          {/* Status filter buttons */}
          <div className="flex items-center gap-1.5 flex-wrap">
            {mainStatusFilters.map((sf) => (
              <button
                key={sf.value}
                onClick={() => handleStatusFilter(sf.value)}
                className={
                  (filters.status || '') === sf.value
                    ? 'btn-filter-active'
                    : 'btn-filter-inactive'
                }
              >
                {sf.label}
              </button>
            ))}
            {/* More statuses toggle */}
            <button
              onClick={() => setShowMoreStatuses(!showMoreStatuses)}
              className="btn-filter-inactive text-slate-500"
            >
              {showMoreStatuses ? 'Less' : 'More'}
            </button>
            {showMoreStatuses &&
              moreStatusFilters.map((sf) => (
                <button
                  key={sf.value}
                  onClick={() => handleStatusFilter(sf.value)}
                  className={
                    filters.status === sf.value
                      ? 'btn-filter-active'
                      : 'btn-filter-inactive'
                  }
                >
                  {sf.label}
                </button>
              ))}
          </div>

          <div className="w-px h-6 bg-slate-700" />

          {/* Project dropdown (kept as-is since there can be many) */}
          <select
            value={filters.project_id || ''}
            onChange={(e) =>
              setFilters({
                ...filters,
                project_id: e.target.value || undefined,
              })
            }
            className="input w-auto text-xs py-1.5"
          >
            <option value="">All Projects</option>
            {activeProjects.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>

          <div className="w-px h-6 bg-slate-700" />

          {/* Sort */}
          <div className="flex items-center gap-1.5">
            <span className="text-xs text-slate-500">Sort:</span>
            {sortOptions.map((opt) => (
              <button
                key={opt.value}
                onClick={() => setSortBy(opt.value)}
                className={
                  sortBy === opt.value
                    ? 'btn-filter-active'
                    : 'btn-filter-inactive'
                }
              >
                {opt.label}
              </button>
            ))}
          </div>
        </div>

        {/* Result count */}
        {!isLoading && tasks.length > 0 && (
          <p className="text-xs text-slate-500">
            Showing {sortedTasks.length} task{sortedTasks.length !== 1 ? 's' : ''}
          </p>
        )}
      </div>

      {/* Create Form */}
      {showForm && (
        <div className="card p-6">
          <h2 className="text-xl font-semibold text-slate-100 mb-4">
            Create New Task
          </h2>
          <form onSubmit={handleSubmit} className="space-y-5">
            {/* Title */}
            <div>
              <label className="label">Title</label>
              <input
                type="text"
                value={formData.title}
                onChange={(e) =>
                  setFormData({ ...formData, title: e.target.value })
                }
                className="input"
                placeholder="What needs to be done?"
                required
                autoFocus
              />
            </div>

            {/* Description */}
            <div>
              <label className="label">Description</label>
              <textarea
                value={formData.description}
                onChange={(e) =>
                  setFormData({ ...formData, description: e.target.value })
                }
                className="input min-h-[120px] resize-y"
                placeholder="Describe the task in detail. Include acceptance criteria, context, and any constraints. This becomes the executor's primary input."
                required
              />
              <p className="text-xs text-slate-500 mt-1">
                Be specific — this is what the executor agent reads to understand the work.
              </p>
            </div>

            {/* Type + Priority row */}
            <div className="grid grid-cols-2 gap-4">
              {/* Type */}
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
                  {Object.entries(typeDescriptions).map(([value, desc]) => (
                    <option key={value} value={value}>
                      {value.charAt(0) + value.slice(1).toLowerCase()} — {desc}
                    </option>
                  ))}
                </select>
              </div>

              {/* Priority */}
              <div>
                <label className="label">
                  Priority
                  <span className="text-slate-500 font-normal ml-1">
                    ({formData.priority})
                  </span>
                </label>
                <div className="flex gap-2">
                  {priorityPresets.map((preset) => (
                    <button
                      key={preset.value}
                      type="button"
                      onClick={() =>
                        setFormData({ ...formData, priority: preset.value })
                      }
                      className={`flex-1 px-2 py-2 rounded-lg text-xs font-medium border transition-colors ${
                        formData.priority === preset.value
                          ? 'bg-slate-600 border-blue-500 text-white'
                          : 'bg-slate-700 border-slate-600 text-slate-400 hover:border-slate-500'
                      }`}
                    >
                      <span className={preset.color}>{preset.label}</span>
                    </button>
                  ))}
                </div>
              </div>
            </div>

            {/* Project + Goal row */}
            <div className="grid grid-cols-2 gap-4">
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
                <label className="label">
                  Goal
                  <span className="text-slate-500 font-normal ml-1">(optional)</span>
                </label>
                <select
                  value={formData.goal_id}
                  onChange={(e) =>
                    setFormData({ ...formData, goal_id: e.target.value })
                  }
                  className="input"
                >
                  <option value="">
                    {currentGoal ? `Active: ${currentGoal.title}` : 'No active goal'}
                  </option>
                  {currentGoal && (
                    <option value={currentGoal.id}>
                      {currentGoal.title}
                    </option>
                  )}
                </select>
              </div>
            </div>

            {/* Tags */}
            <div>
              <label className="label">
                Tags
                <span className="text-slate-500 font-normal ml-1">(comma-separated)</span>
              </label>
              <input
                type="text"
                value={formData.tags}
                onChange={(e) =>
                  setFormData({ ...formData, tags: e.target.value })
                }
                className="input"
                placeholder="frontend, urgent, needs-review"
              />
            </div>

            {/* Advanced toggle */}
            <div>
              <button
                type="button"
                onClick={() => setShowAdvanced(!showAdvanced)}
                className="text-sm text-slate-500 hover:text-slate-300 transition-colors"
              >
                {showAdvanced ? '- Hide advanced' : '+ Advanced options'}
              </button>
            </div>

            {/* Advanced: Prompt */}
            {showAdvanced && (
              <div>
                <label className="label">
                  Additional Prompt
                  <span className="text-slate-500 font-normal ml-1">(optional)</span>
                </label>
                <textarea
                  value={formData.prompt}
                  onChange={(e) =>
                    setFormData({ ...formData, prompt: e.target.value })
                  }
                  className="input min-h-[80px] resize-y"
                  placeholder="Extra instructions for the executor agent, e.g. 'Use the existing auth middleware' or 'Follow the pattern in services/users.go'"
                />
                <p className="text-xs text-slate-500 mt-1">
                  Appended to the executor prompt as additional context.
                </p>
              </div>
            )}

            {/* Submit */}
            <div className="flex items-center gap-3 pt-2">
              <button type="submit" className="btn-primary">
                Create Task
              </button>
              <button
                type="button"
                onClick={() => setShowForm(false)}
                className="btn-secondary"
              >
                Cancel
              </button>
            </div>
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
            {sortedTasks.map((task) => {
              const project = projectMap[task.project_id]
              return (
                <div
                  key={task.id}
                  className="card p-5 hover:border-slate-600 transition-colors cursor-pointer"
                  onClick={() => navigate(`/tasks/${task.id}`)}
                >
                  <div className="flex items-start justify-between">
                    <div className="flex-1 min-w-0">
                      {/* Title row */}
                      <div className="flex items-center gap-2 mb-1.5">
                        <h3 className="text-base font-medium text-slate-100 hover:text-blue-400 transition-colors truncate">
                          {task.title}
                        </h3>
                        <span
                          className={`badge shrink-0 ${
                            task.status === 'COMPLETED'
                              ? 'badge-success'
                              : task.status === 'FAILED'
                              ? 'badge-danger'
                              : task.status === 'RUNNING'
                              ? 'badge-warning'
                              : task.status === 'READY'
                              ? 'badge-info'
                              : task.status === 'RETRY'
                              ? 'badge-warning'
                              : task.status === 'DECOMPOSED'
                              ? 'bg-purple-600 text-white px-2 py-1 rounded text-xs font-semibold'
                              : task.status === 'CANCELLED'
                              ? 'bg-slate-500 text-white px-2 py-1 rounded text-xs font-semibold'
                              : 'badge-secondary'
                          }`}
                        >
                          {task.status}
                        </span>
                        <span className="badge-secondary shrink-0">{task.type}</span>
                        {subtaskCounts[task.id] && (
                          <span className="text-xs text-purple-400 bg-purple-900/30 px-1.5 py-0.5 rounded shrink-0">
                            {subtaskCounts[task.id]} subtask{subtaskCounts[task.id] !== 1 ? 's' : ''}
                          </span>
                        )}
                      </div>

                      {/* Description (clamped) */}
                      <p className="text-sm text-slate-400 mb-2 line-clamp-2">
                        {task.description}
                      </p>

                      {/* Meta row */}
                      <div className="flex items-center gap-3 text-xs text-slate-500">
                        {project && (
                          <span className="text-slate-400">
                            {project.name}
                          </span>
                        )}
                        <span>P{task.priority}</span>
                        <span>{task.source}</span>
                        {task.executor_id && task.status === 'RUNNING' && (
                          <span className="text-amber-500">{task.executor_id}</span>
                        )}
                        {task.model && (
                          <span className="text-slate-600">{task.model}</span>
                        )}
                        {task.diff_lines ? (
                          <span>
                            {task.diff_lines}L / {task.files_changed}F
                          </span>
                        ) : null}
                        {task.pr_url && (
                          <a
                            href={task.pr_url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-blue-400 hover:underline"
                            onClick={(e) => e.stopPropagation()}
                          >
                            PR {task.pr_status === 'MERGED' ? '(merged)' : ''}
                          </a>
                        )}
                        {task.cost_usd ? (
                          <span>${task.cost_usd.toFixed(3)}</span>
                        ) : null}
                      </div>

                      {/* Timestamps row */}
                      <div className="flex items-center gap-3 text-xs text-slate-600 mt-1">
                        <span>Created {timeAgo(task.created_at)}</span>
                        {task.started_at && (
                          <span>Started {timeAgo(task.started_at)}</span>
                        )}
                        {task.completed_at && (
                          <span>Done {timeAgo(task.completed_at)}</span>
                        )}
                        {task.started_at && task.completed_at && (
                          <span className="text-slate-500">
                            ({duration(task.started_at, task.completed_at)})
                          </span>
                        )}
                      </div>

                      {/* Tags */}
                      {task.tags.length > 0 && (
                        <div className="flex flex-wrap gap-1.5 mt-2">
                          {task.tags.map((tag, i) => (
                            <span key={i} className="badge-info text-[10px]">
                              {tag}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>

                    {/* Actions */}
                    <div className="flex gap-2 ml-4 shrink-0" onClick={(e) => e.stopPropagation()}>
                      {(task.status === 'FAILED' || task.status === 'RETRY') && (
                        <button
                          onClick={() => handleRetry(task.id, task.title)}
                          className="px-3 py-1.5 rounded text-sm font-medium bg-blue-600 text-white hover:bg-blue-500 transition-colors"
                        >
                          Retry
                        </button>
                      )}
                      {(task.status === 'READY' || task.status === 'RUNNING' || task.status === 'DECOMPOSED') && (
                        <button
                          onClick={() => handleCancel(task.id, task.title)}
                          className="btn-danger"
                        >
                          Cancel
                        </button>
                      )}
                      {(task.status === 'COMPLETED' || task.status === 'FAILED' || task.status === 'CANCELLED') && (
                        <button
                          onClick={() => handleArchive(task.id, task.title)}
                          className="px-3 py-1.5 rounded text-sm font-medium bg-slate-600 text-slate-300 hover:bg-slate-500 transition-colors"
                        >
                          Archive
                        </button>
                      )}
                    </div>
                  </div>

                  {/* Error */}
                  {task.error_log && (
                    <div className="mt-3 p-3 bg-red-900/30 border border-red-600 rounded text-sm text-red-200 line-clamp-3">
                      <strong>Error:</strong> {task.error_log}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
