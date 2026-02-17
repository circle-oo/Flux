import { useState } from 'react'
import { Project } from '../lib/api'

const statusFilterGroups = [
  { id: 'all', label: 'All', statuses: [] as string[] },
  { id: 'pending', label: 'Pending', statuses: ['PENDING'] },
  { id: 'active', label: 'Active', statuses: ['READY', 'RUNNING', 'DECOMPOSED'] },
  { id: 'terminal', label: 'Done', statuses: ['COMPLETED', 'CANCELLED', 'ARCHIVED'] },
  { id: 'failed', label: 'Failed', statuses: ['FAILED'] },
]

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

export type SortOption = 'priority' | 'newest' | 'updated' | 'status'

const sortOptions: { value: SortOption; label: string }[] = [
  { value: 'priority', label: 'Priority' },
  { value: 'newest', label: 'Newest' },
  { value: 'updated', label: 'Updated' },
  { value: 'status', label: 'Status' },
]

export { statusFilterGroups }

interface TaskFilterBarProps {
  activeFilterGroup: string
  statusFilter?: string
  projectFilter?: string
  sortBy: SortOption
  showSubtasksInList: boolean
  taskCount: number
  visibleCount: number
  isLoading: boolean
  activeProjects: Project[]
  onFilterGroupChange: (groupId: string) => void
  onDetailedStatusFilter: (value: string) => void
  onProjectFilter: (projectId: string | undefined) => void
  onSortChange: (sort: SortOption) => void
  onToggleSubtasks: () => void
}

export default function TaskFilterBar({
  activeFilterGroup,
  statusFilter,
  projectFilter,
  sortBy,
  showSubtasksInList,
  taskCount,
  visibleCount,
  isLoading,
  activeProjects,
  onFilterGroupChange,
  onDetailedStatusFilter,
  onProjectFilter,
  onSortChange,
  onToggleSubtasks,
}: TaskFilterBarProps) {
  const [showDetailedFilters, setShowDetailedFilters] = useState(false)

  return (
    <div className="gc p-4 sm:p-5 space-y-4">
      <div className="flex flex-wrap items-center gap-1.5">
        {statusFilterGroups.map((group) => (
          <button
            key={group.id}
            onClick={() => onFilterGroupChange(group.id)}
            className={activeFilterGroup === group.id ? 'btn-filter-active' : 'btn-filter-inactive'}
          >
            {group.label}
          </button>
        ))}
        <button onClick={() => setShowDetailedFilters((v) => !v)} className="btn-filter-inactive text-content-faint">
          {showDetailedFilters ? 'Hide details' : 'More filters'}
        </button>
      </div>

      {showDetailedFilters && (
        <div className="flex flex-wrap items-center gap-1.5">
          {detailedStatusFilters.map((sf) => (
            <button
              key={sf.value}
              onClick={() => onDetailedStatusFilter(sf.value)}
              className={statusFilter === sf.value && !activeFilterGroup ? 'btn-filter-active' : 'btn-filter-inactive'}
            >
              {sf.label}
            </button>
          ))}
        </div>
      )}

      <div className="grid grid-cols-1 xl:grid-cols-[1.2fr_1fr_auto] gap-3 xl:items-center">
        <div className="flex items-center gap-2 min-w-0">
          <span className="text-[11px] uppercase tracking-[0.1em] text-content-faint shrink-0">Project</span>
          <select
            value={projectFilter || ''}
            onChange={(e) => onProjectFilter(e.target.value || undefined)}
            className="input min-h-[38px] py-2 text-xs"
          >
            <option value="">All Projects</option>
            {activeProjects.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
          </select>
        </div>

        <div className="flex items-center gap-1.5 flex-wrap">
          <span className="text-[11px] uppercase tracking-[0.1em] text-content-faint">Sort</span>
          {sortOptions.map((opt) => (
            <button
              key={opt.value}
              onClick={() => onSortChange(opt.value)}
              className={sortBy === opt.value ? 'btn-filter-active' : 'btn-filter-inactive'}
            >
              {opt.label}
            </button>
          ))}
        </div>

        <label className="flex items-center gap-2 text-xs text-content-muted cursor-pointer select-none">
          <input
            type="checkbox"
            checked={showSubtasksInList}
            onChange={onToggleSubtasks}
            className="w-3.5 h-3.5 rounded border-line-hover bg-surface-hover text-primary-500"
          />
          Show subtasks
        </label>
      </div>

      {!isLoading && taskCount > 0 && (
        <div className="text-[11px] text-content-faint">
          Showing <span className="text-content-secondary font-medium">{visibleCount}</span> of {taskCount} tasks
        </div>
      )}
    </div>
  )
}
