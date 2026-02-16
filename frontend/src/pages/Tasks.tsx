import { useEffect, useState, useMemo, useCallback } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
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
  { value: 10, label: 'Critical', color: 'text-red-400' },
  { value: 25, label: 'High', color: 'text-amber-400' },
  { value: 40, label: 'Normal', color: 'text-blue-400' },
  { value: 65, label: 'Low', color: 'text-slate-400' },
  { value: 85, label: 'Backlog', color: 'text-slate-500' },
]

const MAX_DESCRIPTION_LENGTH = 51200 // Must match backend maxDescriptionLength

const typeDescriptions: Record<string, string> = {
  CODING: 'Implementation, features, refactoring',
  BUGFIX: 'Fix a bug or regression',
  RESEARCH: 'Investigation, analysis, no code changes',
  DOCUMENT: 'Documentation, README, comments',
  MAINTENANCE: 'Deps, cleanup, CI/CD, infra',
  DEPLOY: 'Deployment, release, rollback',
  PLANNING: 'Architecture, design, strategy',
}

// Unified filter groups
const statusFilterGroups = [
  { id: 'all', label: 'All', statuses: [] as string[] },
  { id: 'pending', label: 'Pending', statuses: ['PENDING'] },
  { id: 'active', label: 'Active', statuses: ['READY', 'RUNNING', 'DECOMPOSED'] },
  { id: 'terminal', label: 'Done', statuses: ['COMPLETED', 'CANCELLED', 'ARCHIVED'] },
  { id: 'failed', label: 'Failed', statuses: ['FAILED'] },
]

// Individual status filters for advanced filtering
const detailedStatusFilters = [
  { value: 'READY', label: 'Ready' },
  { value: 'RUNNING', label: 'Running' },
  { value: 'PENDING', label: 'Pending' },
  { value: 'DECOMPOSED', label: 'Decomposed' },
  { value: 'COMPLETED', label: 'Completed' },
  { value: 'CANCELLED', label: 'Cancelled' },
  { value: 'RETRY', label: 'Retry' },
  { value: 'ARCHIVED', label: 'Archived' },
  { value: 'FAILED', label: 'Failed' },
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
  const [searchParams] = useSearchParams()
  const [showForm, setShowForm] = useState(false)
  const [showDetailedFilters, setShowDetailedFilters] = useState(false)
  const [sortBy, setSortBy] = useState<SortOption>('newest')
  const [activeFilterGroup, setActiveFilterGroup] = useState<string>('active')
  const [formData, setFormData] = useState({
    title: '',
    description: '',
    type: 'CODING' as Task['type'],
    priority: 40,
    project_id: '',
    goal_id: '',
    tags: '',
    prompt: '',
  })
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const [subtaskCounts, setSubtaskCounts] = useState<Record<string, number>>({})
  const [expandedTasks, setExpandedTasks] = useState<Set<string>>(new Set())
  const [loadedSubtasks, setLoadedSubtasks] = useState<Record<string, Task[]>>({})
  const [loadingSubtasks, setLoadingSubtasks] = useState<Set<string>>(new Set())

  // Initialize filters from URL params on mount
  useEffect(() => {
    const statusParam = searchParams.get('status')
    if (statusParam) {
      // Valid status values
      const validStatuses = ['PENDING', 'READY', 'RUNNING', 'DECOMPOSED', 'COMPLETED', 'CANCELLED', 'RETRY', 'ARCHIVED', 'FAILED']
      if (validStatuses.includes(statusParam)) {
        setFilters({ status: statusParam })
        // Clear the group filter when coming from URL
        setActiveFilterGroup('')
      }
    }
  }, []) // Run only on mount

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

  // Filter tasks based on active group or individual status filter
  const visibleTasks = useMemo(() => {
    let filtered = tasks

    // Find the active group to determine which statuses to show
    const group = statusFilterGroups.find((g) => g.id === activeFilterGroup)

    if (group && group.statuses.length > 0) {
      // Filter by group statuses
      filtered = filtered.filter((t) => !t.parent_id && group.statuses.includes(t.status))
    } else if (filters.status) {
      // Individual status filter (detailed filter mode)
      filtered = filtered.filter((t) => !t.parent_id && (
        filters.status === 'ARCHIVED' || t.status === filters.status
      ))
    } else {
      // 'All' group or no filter - hide archived by default, hide subtasks
      filtered = filtered.filter((t) => !t.parent_id && t.status !== 'ARCHIVED')
    }

    return filtered
  }, [tasks, filters.status, activeFilterGroup])

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
    setFormError(null)

    // Client-side validation
    if (formData.description.length > MAX_DESCRIPTION_LENGTH) {
      setFormError(`Description exceeds maximum length of ${MAX_DESCRIPTION_LENGTH.toLocaleString()} characters (${formData.description.length.toLocaleString()} provided)`)
      return
    }

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
      setFormError(null)
      setFormData({
        title: '',
        description: '',
        type: 'CODING',
        priority: 40,
        project_id: '',
        goal_id: '',
        tags: '',
        prompt: '',
      })
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to create task'
      setFormError(message)
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

  const toggleSubtasks = async (taskId: string, e: React.MouseEvent) => {
    e.stopPropagation()

    if (expandedTasks.has(taskId)) {
      // Collapse
      const newExpanded = new Set(expandedTasks)
      newExpanded.delete(taskId)
      setExpandedTasks(newExpanded)
    } else {
      // Expand - fetch subtasks if not already loaded
      const newExpanded = new Set(expandedTasks)
      newExpanded.add(taskId)
      setExpandedTasks(newExpanded)

      if (!loadedSubtasks[taskId]) {
        const newLoading = new Set(loadingSubtasks)
        newLoading.add(taskId)
        setLoadingSubtasks(newLoading)

        try {
          const subtasks = await api.listSubtasks(taskId)
          setLoadedSubtasks({ ...loadedSubtasks, [taskId]: subtasks })
        } catch (error) {
          console.error('Failed to load subtasks:', error)
        } finally {
          const newLoading = new Set(loadingSubtasks)
          newLoading.delete(taskId)
          setLoadingSubtasks(newLoading)
        }
      }
    }
  }

  const handleFilterGroupChange = (groupId: string) => {
    setActiveFilterGroup(groupId)
    const group = statusFilterGroups.find((g) => g.id === groupId)
    if (!group) return

    // Clear the backend status filter to fetch all tasks
    // The client-side filtering logic will handle showing only the tasks matching the group
    setFilters({ ...filters, status: undefined })
  }

  const handleDetailedStatusFilter = (value: string) => {
    const newStatus = filters.status === value ? undefined : value || undefined
    setFilters({ ...filters, status: newStatus })
    // Clear group selection when using detailed filters
    setActiveFilterGroup('')
  }

  const activeProjects = projects.filter((p) => p.status === 'ACTIVE')
  const projectMap = Object.fromEntries(projects.map((p) => [p.id, p]))

  return (
    <div className="p-4 sm:p-6 lg:p-8 space-y-6 lg:space-y-8">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 sm:gap-3 mb-2">
            <h1 className="text-2xl sm:text-3xl font-bold text-slate-100">Tasks</h1>
            {tasks.length > 0 && (
              <span className="badge badge-info text-sm sm:text-lg px-2 sm:px-3 py-0.5 sm:py-1">
                {tasks.length}
              </span>
            )}
          </div>
          <p className="text-sm sm:text-base text-slate-400">Manage and track system tasks</p>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="btn-primary whitespace-nowrap"
        >
          {showForm ? 'Cancel' : '+ New Task'}
        </button>
      </div>

      {/* Filters */}
      <div className="card p-3 sm:p-4 space-y-3">
        <div className="flex flex-col sm:flex-row sm:items-center gap-3 sm:gap-3">
          {/* Status filter groups */}
          <div className="flex items-center gap-1.5 flex-wrap overflow-x-auto scrollbar-hide">
            {statusFilterGroups.map((group) => (
              <button
                key={group.id}
                onClick={() => handleFilterGroupChange(group.id)}
                className={
                  activeFilterGroup === group.id
                    ? 'btn-filter-active'
                    : 'btn-filter-inactive'
                }
              >
                {group.label}
              </button>
            ))}
            {/* Detailed filters toggle */}
            <button
              onClick={() => setShowDetailedFilters(!showDetailedFilters)}
              className="btn-filter-inactive text-slate-500"
            >
              {showDetailedFilters ? 'Less' : 'More'}
            </button>
            {showDetailedFilters &&
              detailedStatusFilters.map((sf) => (
                <button
                  key={sf.value}
                  onClick={() => handleDetailedStatusFilter(sf.value)}
                  className={
                    filters.status === sf.value && !activeFilterGroup
                      ? 'btn-filter-active'
                      : 'btn-filter-inactive'
                  }
                >
                  {sf.label}
                </button>
              ))}
          </div>

          <div className="hidden sm:block w-px h-6 bg-slate-700" />

          {/* Project dropdown (kept as-is since there can be many) */}
          <select
            value={filters.project_id || ''}
            onChange={(e) =>
              setFilters({
                ...filters,
                project_id: e.target.value || undefined,
              })
            }
            className="input w-full sm:w-auto text-sm py-2"
          >
            <option value="">All Projects</option>
            {activeProjects.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>

          <div className="hidden sm:block w-px h-6 bg-slate-700" />

          {/* Sort */}
          <div className="flex items-center gap-1.5 w-full sm:w-auto">
            <span className="text-xs text-slate-500 whitespace-nowrap">Sort:</span>
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
        <div className="card p-4 sm:p-6">
          <h2 className="text-lg sm:text-xl font-semibold text-slate-100 mb-4">
            Create New Task
          </h2>
          <form onSubmit={handleSubmit} className="space-y-4 sm:space-y-5">
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
              <div className="flex items-center justify-between mt-1">
                <p className="text-xs text-slate-500">
                  Be specific — this is what the executor agent reads to understand the work.
                </p>
                <span className={`text-xs ${formData.description.length > MAX_DESCRIPTION_LENGTH ? 'text-red-400' : 'text-slate-500'}`}>
                  {formData.description.length.toLocaleString()} / {MAX_DESCRIPTION_LENGTH.toLocaleString()}
                </span>
              </div>
            </div>

            {/* Type + Priority row */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
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
                <div className="flex flex-wrap sm:flex-nowrap gap-2">
                  {priorityPresets.map((preset) => (
                    <button
                      key={preset.value}
                      type="button"
                      onClick={() =>
                        setFormData({ ...formData, priority: preset.value })
                      }
                      className={`flex-1 px-2 py-2.5 rounded-lg text-xs sm:text-sm font-medium border transition-colors touch-manipulation min-w-[80px] ${
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
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
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

            {/* Error display */}
            {formError && (
              <div className="p-3 bg-red-900/30 border border-red-600 rounded text-sm text-red-200">
                {formError}
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
      <div className="space-y-3 sm:space-y-4">
        {isLoading ? (
          <div className="text-slate-400">Loading...</div>
        ) : tasks.length === 0 ? (
          <div className="card p-4 sm:p-6 text-center text-slate-400">
            No tasks found. Create one or adjust filters.
          </div>
        ) : (
          <div className="space-y-3">
            {sortedTasks.map((task) => {
              const project = projectMap[task.project_id]
              return (
                <div
                  key={task.id}
                  className="card p-4 sm:p-5 hover:border-slate-600 transition-colors cursor-pointer touch-manipulation"
                  onClick={() => navigate(`/tasks/${task.id}`)}
                >
                  <div className="flex items-start justify-between">
                    <div className="flex-1 min-w-0">
                      {/* Title row */}
                      <div className="flex items-center gap-2 mb-1.5">
                        <h3 className="text-base font-medium text-slate-100 hover:text-blue-400 transition-colors truncate">
                          {task.triage_title || task.title}
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
                        {task.triage_analysis && (
                          <span className="bg-cyan-600/20 text-cyan-400 border border-cyan-600/30 px-1.5 py-0.5 rounded text-[10px] font-medium shrink-0">
                            Triaged
                          </span>
                        )}
                        {subtaskCounts[task.id] && (
                          <button
                            onClick={(e) => toggleSubtasks(task.id, e)}
                            className="text-xs text-purple-400 bg-purple-900/30 px-1.5 py-0.5 rounded shrink-0 hover:bg-purple-900/50 transition-colors flex items-center gap-1"
                          >
                            <span className="text-[10px]">
                              {expandedTasks.has(task.id) ? '▼' : '▶'}
                            </span>
                            {subtaskCounts[task.id]} subtask{subtaskCounts[task.id] !== 1 ? 's' : ''}
                          </button>
                        )}
                      </div>

                      {/* Triage Description (preview) */}
                      {task.triage_description && (
                        <p className="text-sm text-cyan-400/80 mb-1.5 line-clamp-2 border-l-2 border-cyan-600/40 pl-2">
                          {task.triage_description}
                        </p>
                      )}

                      {/* Meta row */}
                      <div className="flex flex-wrap items-center gap-3 text-xs text-slate-500">
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
                    <div className="flex flex-col sm:flex-row gap-2 ml-0 sm:ml-4 mt-3 sm:mt-0 shrink-0" onClick={(e) => e.stopPropagation()}>
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

                  {/* Subtasks */}
                  {expandedTasks.has(task.id) && (
                    <div className="mt-3 pl-4 border-l-2 border-purple-600/30 space-y-2">
                      {loadingSubtasks.has(task.id) ? (
                        <div className="text-xs text-slate-500">Loading subtasks...</div>
                      ) : loadedSubtasks[task.id]?.length > 0 ? (
                        loadedSubtasks[task.id].map((subtask) => (
                          <div
                            key={subtask.id}
                            className="bg-slate-800/50 border border-slate-700 rounded p-3 cursor-pointer hover:border-slate-600 transition-colors"
                            onClick={(e) => {
                              e.stopPropagation()
                              navigate(`/tasks/${subtask.id}`)
                            }}
                          >
                            <div className="flex items-start gap-2">
                              <div className="flex-1 min-w-0">
                                <div className="flex items-center gap-2 mb-1">
                                  <h4 className="text-sm font-medium text-slate-300 truncate">
                                    {subtask.triage_title || subtask.title}
                                  </h4>
                                  <span
                                    className={`badge shrink-0 text-[10px] ${
                                      subtask.status === 'COMPLETED'
                                        ? 'badge-success'
                                        : subtask.status === 'FAILED'
                                        ? 'badge-danger'
                                        : subtask.status === 'RUNNING'
                                        ? 'badge-warning'
                                        : subtask.status === 'READY'
                                        ? 'badge-info'
                                        : subtask.status === 'RETRY'
                                        ? 'badge-warning'
                                        : subtask.status === 'CANCELLED'
                                        ? 'bg-slate-500 text-white px-1.5 py-0.5 rounded'
                                        : 'badge-secondary'
                                    }`}
                                  >
                                    {subtask.status}
                                  </span>
                                  <span className="badge-secondary shrink-0 text-[10px]">
                                    {subtask.type}
                                  </span>
                                </div>
                                <div className="flex items-center gap-2 text-[10px] text-slate-600">
                                  <span>P{subtask.priority}</span>
                                  {subtask.started_at && (
                                    <span>Started {timeAgo(subtask.started_at)}</span>
                                  )}
                                  {subtask.completed_at && (
                                    <span>Done {timeAgo(subtask.completed_at)}</span>
                                  )}
                                  {subtask.started_at && subtask.completed_at && (
                                    <span className="text-slate-500">
                                      ({duration(subtask.started_at, subtask.completed_at)})
                                    </span>
                                  )}
                                </div>
                              </div>
                            </div>
                          </div>
                        ))
                      ) : (
                        <div className="text-xs text-slate-500">No subtasks found</div>
                      )}
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
