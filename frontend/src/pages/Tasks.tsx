import { useEffect, useState, useMemo, useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useTaskStore } from '../stores/taskStore'
import { useProjectStore } from '../stores/projectStore'
import { useGoalStore } from '../stores/goalStore'
import { Task, api } from '../lib/api'
import PageHeader from '../components/PageHeader'
import TaskCreateForm from '../components/TaskCreateForm'
import TaskFilterBar, { SortOption, statusFilterGroups } from '../components/TaskFilterBar'
import TaskListItem from '../components/TaskListItem'

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
  const [searchParams] = useSearchParams()
  const [showForm, setShowForm] = useState(false)
  const [sortBy, setSortBy] = useState<SortOption>('newest')
  const [activeFilterGroup, setActiveFilterGroup] = useState<string>('active')

  const [subtaskCounts, setSubtaskCounts] = useState<Record<string, number>>({})
  const [expandedTasks, setExpandedTasks] = useState<Set<string>>(new Set())
  const [loadedSubtasks, setLoadedSubtasks] = useState<Record<string, Task[]>>({})
  const [loadingSubtasks, setLoadingSubtasks] = useState<Set<string>>(new Set())
  const [showSubtasksInList, setShowSubtasksInList] = useState<boolean>(() => {
    const stored = localStorage.getItem('flux-show-subtasks-in-list')
    return stored ? JSON.parse(stored) : false
  })

  // Initialize filters from URL params on mount
  useEffect(() => {
    const statusParam = searchParams.get('status')
    if (statusParam) {
      const validStatuses = ['PENDING', 'READY', 'RUNNING', 'DECOMPOSED', 'COMPLETED', 'CANCELLED', 'RETRY', 'ARCHIVED', 'FAILED']
      if (validStatuses.includes(statusParam)) {
        setFilters({ status: statusParam })
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
    const group = statusFilterGroups.find((g) => g.id === activeFilterGroup)

    if (group && group.statuses.length > 0) {
      filtered = filtered.filter((t) => {
        const matchesStatus = group.statuses.includes(t.status)
        const isSubtask = !!t.parent_id
        return matchesStatus && (showSubtasksInList || !isSubtask)
      })
    } else if (filters.status) {
      filtered = filtered.filter((t) => {
        const matchesStatus = filters.status === 'ARCHIVED' || t.status === filters.status
        const isSubtask = !!t.parent_id
        return matchesStatus && (showSubtasksInList || !isSubtask)
      })
    } else {
      filtered = filtered.filter((t) => {
        const isSubtask = !!t.parent_id
        return t.status !== 'ARCHIVED' && (showSubtasksInList || !isSubtask)
      })
    }

    return filtered
  }, [tasks, filters.status, activeFilterGroup, showSubtasksInList])

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

  const handleSubmit = async (taskData: Parameters<typeof createTask>[0]) => {
    await createTask(taskData)
    setShowForm(false)
  }

  const handleCancel = async (id: string, title: string) => {
    if (confirm(`Cancel task: ${title}?`)) {
      try { await cancelTask(id) } catch (error) { console.error('Failed to cancel task:', error) }
    }
  }

  const handleRetry = async (id: string, title: string) => {
    if (confirm(`Retry task: ${title}?`)) {
      try { await retryTask(id) } catch (error) { console.error('Failed to retry task:', error) }
    }
  }

  const handleArchive = async (id: string, title: string) => {
    if (confirm(`Archive task: ${title}?`)) {
      try { await archiveTask(id) } catch (error) { console.error('Failed to archive task:', error) }
    }
  }

  const toggleSubtasks = async (taskId: string, e: React.MouseEvent) => {
    e.stopPropagation()

    if (expandedTasks.has(taskId)) {
      const newExpanded = new Set(expandedTasks)
      newExpanded.delete(taskId)
      setExpandedTasks(newExpanded)
    } else {
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
    setFilters({ ...filters, status: undefined })
  }

  const handleDetailedStatusFilter = (value: string) => {
    const newStatus = filters.status === value ? undefined : value || undefined
    setFilters({ ...filters, status: newStatus })
    setActiveFilterGroup('')
  }

  const handleToggleSubtasks = () => {
    const newValue = !showSubtasksInList
    setShowSubtasksInList(newValue)
    localStorage.setItem('flux-show-subtasks-in-list', JSON.stringify(newValue))
  }

  const activeProjects = projects.filter((p) => p.status === 'ACTIVE')
  const projectMap = Object.fromEntries(projects.map((p) => [p.id, p]))

  return (
    <div className="p-4 sm:p-6 lg:p-8 space-y-6 lg:space-y-8">
      <PageHeader
        title="Tasks"
        subtitle="Manage and track system tasks"
        count={tasks.length}
        action={
          <button onClick={() => setShowForm(!showForm)} className="btn-primary whitespace-nowrap">
            {showForm ? 'Cancel' : '+ New Task'}
          </button>
        }
      />

      <TaskFilterBar
        activeFilterGroup={activeFilterGroup}
        statusFilter={filters.status}
        projectFilter={filters.project_id}
        sortBy={sortBy}
        showSubtasksInList={showSubtasksInList}
        taskCount={tasks.length}
        visibleCount={sortedTasks.length}
        isLoading={isLoading}
        activeProjects={activeProjects}
        onFilterGroupChange={handleFilterGroupChange}
        onDetailedStatusFilter={handleDetailedStatusFilter}
        onProjectFilter={(pid) => setFilters({ ...filters, project_id: pid })}
        onSortChange={setSortBy}
        onToggleSubtasks={handleToggleSubtasks}
      />

      {showForm && (
        <TaskCreateForm
          projects={projects}
          currentGoal={currentGoal}
          onSubmit={handleSubmit}
          onCancel={() => setShowForm(false)}
        />
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
            {sortedTasks.map((task) => (
              <TaskListItem
                key={task.id}
                task={task}
                project={projectMap[task.project_id]}
                subtaskCount={subtaskCounts[task.id]}
                isExpanded={expandedTasks.has(task.id)}
                isLoadingSubtasks={loadingSubtasks.has(task.id)}
                subtasks={loadedSubtasks[task.id]}
                onToggleSubtasks={toggleSubtasks}
                onRetry={handleRetry}
                onCancel={handleCancel}
                onArchive={handleArchive}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
