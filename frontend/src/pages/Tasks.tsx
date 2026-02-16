import { useEffect, useState, useMemo } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useTaskStore } from '../stores/taskStore'
import { useProjectStore } from '../stores/projectStore'
import { useGoalStore } from '../stores/goalStore'
import PageHeader from '../components/PageHeader'
import TaskCreateForm from '../components/TaskCreateForm'
import TaskFilterBar, { SortOption, statusFilterGroups } from '../components/TaskFilterBar'
import TaskListItem from '../components/TaskListItem'
import { useConfirm } from '../hooks/useConfirm'
import { useToast } from '../components/Toast'
import { useSubtaskExpansion } from '../hooks/useSubtaskExpansion'

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
  const [showSubtasksInList, setShowSubtasksInList] = useState<boolean>(() => {
    const stored = localStorage.getItem('flux-show-subtasks-in-list')
    return stored ? JSON.parse(stored) : false
  })
  const { confirm, dialog } = useConfirm()
  const { toast } = useToast()
  const {
    subtaskCounts,
    expandedTasks,
    loadedSubtasks,
    loadingSubtasks,
    fetchSubtaskCounts,
    toggleSubtasks,
  } = useSubtaskExpansion()

  useEffect(() => {
    const statusParam = searchParams.get('status')
    if (statusParam) {
      const validStatuses = ['PENDING', 'READY', 'RUNNING', 'DECOMPOSED', 'COMPLETED', 'CANCELLED', 'RETRY', 'ARCHIVED', 'FAILED']
      if (validStatuses.includes(statusParam)) {
        setFilters({ status: statusParam })
        setActiveFilterGroup('')
      }
    }
  }, [])

  useEffect(() => {
    fetchTasks()
    fetchProjects()
    fetchCurrentGoal()
  }, [fetchTasks, fetchProjects, fetchCurrentGoal])

  useEffect(() => {
    if (tasks.length > 0) fetchSubtaskCounts(tasks)
  }, [tasks, fetchSubtaskCounts])

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
    const confirmed = await confirm({ title: 'Cancel task?', description: title, confirmLabel: 'Cancel Task', variant: 'danger' })
    if (confirmed) {
      try { await cancelTask(id); toast('Task cancelled', 'success') } catch (error) { toast(`Failed to cancel task: ${error}`, 'error') }
    }
  }

  const handleRetry = async (id: string, title: string) => {
    const confirmed = await confirm({ title: 'Retry task?', description: title, confirmLabel: 'Retry' })
    if (confirmed) {
      try { await retryTask(id); toast('Task queued for retry', 'success') } catch (error) { toast(`Failed to retry task: ${error}`, 'error') }
    }
  }

  const handleArchive = async (id: string, title: string) => {
    const confirmed = await confirm({ title: 'Archive task?', description: title, confirmLabel: 'Archive' })
    if (confirmed) {
      try { await archiveTask(id); toast('Task archived', 'success') } catch (error) { toast(`Failed to archive task: ${error}`, 'error') }
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
    <div className="p-5 sm:p-6 lg:p-8 space-y-5 animate-fade-in">
      {dialog}
      <PageHeader
        title="Tasks"
        subtitle="Manage and track system tasks"
        count={tasks.length}
        action={
          <button onClick={() => setShowForm(!showForm)} className="btn-primary whitespace-nowrap">
            {showForm ? 'Cancel' : 'New Task'}
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

      <div className="space-y-2.5">
        {isLoading ? (
          <div className="text-content-faint text-sm py-8 text-center">Loading tasks...</div>
        ) : tasks.length === 0 ? (
          <div className="card p-8 text-center text-content-faint text-sm">
            No tasks found. Create one or adjust filters.
          </div>
        ) : (
          <div className="space-y-2">
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
