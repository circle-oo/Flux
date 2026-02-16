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
  { value: 'READY', label: 'Ready' }, { value: 'RUNNING', label: 'Running' }, { value: 'PENDING', label: 'Pending' },
  { value: 'DECOMPOSED', label: 'Decomposed' }, { value: 'COMPLETED', label: 'Completed' }, { value: 'CANCELLED', label: 'Cancelled' },
  { value: 'RETRY', label: 'Retry' }, { value: 'ARCHIVED', label: 'Archived' }, { value: 'FAILED', label: 'Failed' },
]

export type SortOption = 'priority' | 'newest' | 'updated' | 'status'

const sortOptions: { value: SortOption; label: string }[] = [
  { value: 'priority', label: 'Priority' }, { value: 'newest', label: 'Newest' }, { value: 'updated', label: 'Updated' }, { value: 'status', label: 'Status' },
]

export { statusFilterGroups }

interface TaskFilterBarProps {
  activeFilterGroup: string; statusFilter?: string; projectFilter?: string; sortBy: SortOption; showSubtasksInList: boolean
  taskCount: number; visibleCount: number; isLoading: boolean; activeProjects: Project[]
  onFilterGroupChange: (groupId: string) => void; onDetailedStatusFilter: (value: string) => void
  onProjectFilter: (projectId: string | undefined) => void; onSortChange: (sort: SortOption) => void; onToggleSubtasks: () => void
}

export default function TaskFilterBar({ activeFilterGroup, statusFilter, projectFilter, sortBy, showSubtasksInList, taskCount, visibleCount, isLoading, activeProjects, onFilterGroupChange, onDetailedStatusFilter, onProjectFilter, onSortChange, onToggleSubtasks }: TaskFilterBarProps) {
  const [showDetailedFilters, setShowDetailedFilters] = useState(false)

  return (
    <div className="card p-3 sm:p-4 space-y-3">
      <div className="flex flex-col sm:flex-row sm:items-center gap-3">
        <div className="flex items-center gap-1.5 flex-wrap overflow-x-auto scrollbar-hide">
          {statusFilterGroups.map((group) => (
            <button key={group.id} onClick={() => onFilterGroupChange(group.id)} className={activeFilterGroup === group.id ? 'btn-filter-active' : 'btn-filter-inactive'}>{group.label}</button>
          ))}
          <button onClick={() => setShowDetailedFilters(!showDetailedFilters)} className="btn-filter-inactive text-content-faint">{showDetailedFilters ? 'Less' : 'More'}</button>
          {showDetailedFilters && detailedStatusFilters.map((sf) => (
            <button key={sf.value} onClick={() => onDetailedStatusFilter(sf.value)} className={statusFilter === sf.value && !activeFilterGroup ? 'btn-filter-active' : 'btn-filter-inactive'}>{sf.label}</button>
          ))}
        </div>

        <div className="hidden sm:block w-px h-5 bg-surface-active" />

        <select value={projectFilter || ''} onChange={(e) => onProjectFilter(e.target.value || undefined)} className="input w-full sm:w-auto text-xs py-1.5 min-h-[36px]">
          <option value="">All Projects</option>
          {activeProjects.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
        </select>

        <div className="hidden sm:block w-px h-5 bg-surface-active" />

        <div className="flex items-center gap-1.5 w-full sm:w-auto">
          <span className="text-[10px] text-content-faint whitespace-nowrap uppercase tracking-wider">Sort:</span>
          {sortOptions.map((opt) => <button key={opt.value} onClick={() => onSortChange(opt.value)} className={sortBy === opt.value ? 'btn-filter-active' : 'btn-filter-inactive'}>{opt.label}</button>)}
        </div>

        <div className="hidden sm:block w-px h-5 bg-surface-active" />

        <label className="flex items-center gap-2 cursor-pointer whitespace-nowrap">
          <input type="checkbox" checked={showSubtasksInList} onChange={onToggleSubtasks} className="w-3.5 h-3.5 rounded border-line-hover bg-surface-hover text-primary-500 focus:ring-2 focus:ring-primary-500 focus:ring-offset-0 cursor-pointer" />
          <span className="text-[10px] text-content-faint uppercase tracking-wider">Subtasks</span>
        </label>
      </div>

      {!isLoading && taskCount > 0 && (
        <p className="text-[10px] text-content-faint">Showing {visibleCount} task{visibleCount !== 1 ? 's' : ''}</p>
      )}
    </div>
  )
}
