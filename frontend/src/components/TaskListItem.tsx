import { useNavigate } from 'react-router-dom'
import { Task, Project } from '../lib/api'
import { StatusBadge } from './StatusBadge'
import { timeAgo, duration } from '../lib/utils'

interface TaskListItemProps {
  task: Task
  project?: Project
  subtaskCount?: number
  isExpanded?: boolean
  isLoadingSubtasks?: boolean
  subtasks?: Task[]
  onToggleSubtasks?: (taskId: string, e: React.MouseEvent) => void
  onRetry: (id: string, title: string) => void
  onCancel: (id: string, title: string) => void
  onArchive: (id: string, title: string) => void
}

export default function TaskListItem({ task, project, subtaskCount, isExpanded, isLoadingSubtasks, subtasks, onToggleSubtasks, onRetry, onCancel, onArchive }: TaskListItemProps) {
  const navigate = useNavigate()

  return (
    <div
      className="gc p-4 hover:-translate-y-[1px] transition-transform cursor-pointer"
      onClick={() => navigate(`/tasks/${task.id}`)}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1.5 flex-wrap">
            <h3 className="text-sm font-semibold text-content hover:text-primary-600 transition-colors truncate">
              {task.triage_title || task.title}
            </h3>
            <StatusBadge status={task.status} />
            <span className="badge-secondary text-[10px]">P{task.priority}</span>
            {task.model && <span className="badge-info text-[10px]">{task.model}</span>}
            {subtaskCount && onToggleSubtasks && (
              <button
                onClick={(e) => onToggleSubtasks(task.id, e)}
                className="btn-filter-inactive text-[10px] px-2 py-1"
              >
                {isExpanded ? '▼' : '▶'} {subtaskCount} subtask{subtaskCount !== 1 ? 's' : ''}
              </button>
            )}
          </div>

          {task.triage_description && (
            <p className="text-xs text-content-muted mb-2 line-clamp-2 border-l-2 border-primary-300/40 pl-2">
              {task.triage_description}
            </p>
          )}

          <div className="flex flex-wrap items-center gap-2.5 text-[11px] text-content-faint">
            {project && <span className="text-content-muted">{project.name}</span>}
            <span>{task.source}</span>
            {task.executor_id && task.status === 'RUNNING' && <span className="text-amber-500">{task.executor_id}</span>}
            {task.diff_lines ? <span>{task.diff_lines}L / {task.files_changed}F</span> : null}
            {task.pr_url && (
              <a href={task.pr_url} target="_blank" rel="noopener noreferrer" className="text-primary-600 hover:text-primary-500 transition-colors" onClick={(e) => e.stopPropagation()}>
                PR {task.pr_status === 'MERGED' ? '(merged)' : ''}
              </a>
            )}
            {task.cost_usd ? <span>${task.cost_usd.toFixed(3)}</span> : null}
          </div>

          <div className="flex items-center gap-2.5 text-[10px] text-content-faint mt-1">
            <span>Created {timeAgo(task.created_at)}</span>
            {task.started_at && <span>Started {timeAgo(task.started_at)}</span>}
            {task.completed_at && <span>Done {timeAgo(task.completed_at)}</span>}
            {task.started_at && task.completed_at && <span>({duration(task.started_at, task.completed_at)})</span>}
          </div>

          {task.tags.length > 0 && (
            <div className="flex flex-wrap gap-1.5 mt-2">
              {task.tags.map((tag, i) => <span key={i} className="badge-info">{tag}</span>)}
            </div>
          )}
        </div>

        <div className="flex flex-col sm:flex-row gap-1.5 shrink-0" onClick={(e) => e.stopPropagation()}>
          {(task.status === 'FAILED' || task.status === 'RETRY') && <button onClick={() => onRetry(task.id, task.title)} className="btn-sm btn-primary">Retry</button>}
          {(task.status === 'READY' || task.status === 'RUNNING' || task.status === 'DECOMPOSED') && <button onClick={() => onCancel(task.id, task.title)} className="btn-sm btn-danger">Cancel</button>}
          {(task.status === 'COMPLETED' || task.status === 'FAILED' || task.status === 'CANCELLED') && <button onClick={() => onArchive(task.id, task.title)} className="btn-sm btn-secondary">Archive</button>}
        </div>
      </div>

      {task.error_log && (
        <div className="mt-3 p-3 rounded-xl text-xs" style={{ background: 'color-mix(in srgb, var(--err) 8%, transparent)', color: 'var(--err)', border: '1px solid color-mix(in srgb, var(--err) 24%, transparent)' }} role="alert">
          <strong>Error:</strong> {task.error_log}
        </div>
      )}

      {isExpanded && (
        <div className="mt-3 pl-3 border-l-2 border-primary-300/30 space-y-1.5">
          {isLoadingSubtasks ? <div className="text-[10px] text-content-faint">Loading subtasks...</div> : subtasks && subtasks.length > 0 ? (
            subtasks.map((subtask) => (
              <button type="button" key={subtask.id} className="w-full text-left gi rounded-xl p-2.5 cursor-pointer"
                onClick={(e) => { e.stopPropagation(); navigate(`/tasks/${subtask.id}`) }}>
                <div className="flex items-center gap-2 mb-0.5">
                  <h4 className="text-xs font-medium text-content-secondary truncate">{subtask.triage_title || subtask.title}</h4>
                  <StatusBadge status={subtask.status} size="sm" />
                </div>
                <div className="flex items-center gap-2 text-[10px] text-content-faint">
                  <span>P{subtask.priority}</span>
                  {subtask.started_at && <span>Started {timeAgo(subtask.started_at)}</span>}
                  {subtask.completed_at && <span>Done {timeAgo(subtask.completed_at)}</span>}
                  {subtask.started_at && subtask.completed_at && <span>({duration(subtask.started_at, subtask.completed_at)})</span>}
                </div>
              </button>
            ))
          ) : <div className="text-[10px] text-content-faint">No subtasks found</div>}
        </div>
      )}
    </div>
  )
}
