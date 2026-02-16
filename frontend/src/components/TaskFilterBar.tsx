import { useState } from 'react'
import { Project } from '../lib/api'

// Unified filter groups
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
    <div className="card p-3 sm:p-4 space-y-3">
      <div className="flex flex-col sm:flex-row sm:items-center gap-3 sm:gap-3">
        {/* Status filter groups */}
        <div className="flex items-center gap-1.5 flex-wrap overflow-x-auto scrollbar-hide">
          {statusFilterGroups.map((group) => (
            <button
              key={group.id}
              onClick={() => onFilterGroupChange(group.id)}
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
                onClick={() => onDetailedStatusFilter(sf.value)}
                className={
                  statusFilter === sf.value && !activeFilterGroup
                    ? 'btn-filter-active'
                    : 'btn-filter-inactive'
                }
              >
                {sf.label}
              </button>
            ))}
        </div>

        <div className="hidden sm:block w-px h-6 bg-slate-700" />

        {/* Project dropdown */}
        <select
          value={projectFilter || ''}
          onChange={(e) => onProjectFilter(e.target.value || undefined)}
          className="input w-full sm:w-auto text-sm py-2"
        >
          <option value="">All Projects</option>
          {activeProjects.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>

        <div className="hidden sm:block w-px h-6 bg-slate-700" />

        {/* Sort */}
        <div className="flex items-center gap-1.5 w-full sm:w-auto">
          <span className="text-xs text-slate-500 whitespace-nowrap">Sort:</span>
          {sortOptions.map((opt) => (
            <button
              key={opt.value}
              onClick={() => onSortChange(opt.value)}
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

        <div className="hidden sm:block w-px h-6 bg-slate-700" />

        {/* Show Subtasks Toggle */}
        <label className="flex items-center gap-2 cursor-pointer whitespace-nowrap">
          <input
            type="checkbox"
            checked={showSubtasksInList}
            onChange={onToggleSubtasks}
            className="w-4 h-4 rounded border-slate-600 bg-slate-700 text-blue-500 focus:ring-2 focus:ring-blue-500 focus:ring-offset-0 cursor-pointer"
          />
          <span className="text-xs text-slate-400">Show subtasks</span>
        </label>
      </div>

      {/* Result count */}
      {!isLoading && taskCount > 0 && (
        <p className="text-xs text-slate-500">
          Showing {visibleCount} task{visibleCount !== 1 ? 's' : ''}
        </p>
      )}
    </div>
  )
}
