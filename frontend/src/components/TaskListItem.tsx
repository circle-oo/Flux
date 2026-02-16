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

export default function TaskListItem({
  task,
  project,
  subtaskCount,
  isExpanded,
  isLoadingSubtasks,
  subtasks,
  onToggleSubtasks,
  onRetry,
  onCancel,
  onArchive,
}: TaskListItemProps) {
  const navigate = useNavigate()

  return (
    <div
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
            <StatusBadge status={task.status} />
            {task.triage_analysis && (
              <span className="bg-cyan-600/20 text-cyan-400 border border-cyan-600/30 px-2 py-0.5 rounded-md text-xs font-medium shrink-0">
                Triaged
              </span>
            )}
            {subtaskCount && onToggleSubtasks && (
              <button
                onClick={(e) => onToggleSubtasks(task.id, e)}
                className="text-xs text-purple-400 bg-purple-900/30 px-1.5 py-0.5 rounded shrink-0 hover:bg-purple-900/50 transition-colors flex items-center gap-1"
              >
                <span className="text-xs">
                  {isExpanded ? '▼' : '▶'}
                </span>
                {subtaskCount} subtask{subtaskCount !== 1 ? 's' : ''}
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
            {project && <span className="text-slate-400">{project.name}</span>}
            <span>P{task.priority}</span>
            <span>{task.source}</span>
            {task.executor_id && task.status === 'RUNNING' && (
              <span className="text-amber-500">{task.executor_id}</span>
            )}
            {task.model && <span className="text-slate-600">{task.model}</span>}
            {task.diff_lines ? (
              <span>{task.diff_lines}L / {task.files_changed}F</span>
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
            {task.cost_usd ? <span>${task.cost_usd.toFixed(3)}</span> : null}
          </div>

          {/* Timestamps row */}
          <div className="flex items-center gap-3 text-xs text-slate-600 mt-1">
            <span>Created {timeAgo(task.created_at)}</span>
            {task.started_at && <span>Started {timeAgo(task.started_at)}</span>}
            {task.completed_at && <span>Done {timeAgo(task.completed_at)}</span>}
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
                <span key={i} className="badge-info">{tag}</span>
              ))}
            </div>
          )}
        </div>

        {/* Actions */}
        <div className="flex flex-col sm:flex-row gap-2 ml-0 sm:ml-4 mt-3 sm:mt-0 shrink-0" onClick={(e) => e.stopPropagation()}>
          {(task.status === 'FAILED' || task.status === 'RETRY') && (
            <button
              onClick={() => onRetry(task.id, task.title)}
              className="btn-sm btn-primary"
            >
              Retry
            </button>
          )}
          {(task.status === 'READY' || task.status === 'RUNNING' || task.status === 'DECOMPOSED') && (
            <button
              onClick={() => onCancel(task.id, task.title)}
              className="btn-danger"
            >
              Cancel
            </button>
          )}
          {(task.status === 'COMPLETED' || task.status === 'FAILED' || task.status === 'CANCELLED') && (
            <button
              onClick={() => onArchive(task.id, task.title)}
              className="btn-sm btn-secondary"
            >
              Archive
            </button>
          )}
        </div>
      </div>

      {/* Error */}
      {task.error_log && (
        <div className="mt-3 p-3 bg-red-900/30 border border-red-600 rounded text-sm text-red-200 line-clamp-3" role="alert">
          <strong>Error:</strong> {task.error_log}
        </div>
      )}

      {/* Subtasks */}
      {isExpanded && (
        <div className="mt-3 pl-4 border-l-2 border-purple-600/30 space-y-2">
          {isLoadingSubtasks ? (
            <div className="text-xs text-slate-500">Loading subtasks...</div>
          ) : subtasks && subtasks.length > 0 ? (
            subtasks.map((subtask) => (
              <button
                type="button"
                key={subtask.id}
                className="w-full text-left bg-slate-800/50 border border-slate-700 rounded p-3 cursor-pointer hover:border-slate-600 transition-colors"
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
                      <StatusBadge status={subtask.status} size="sm" />
                    </div>
                    <div className="flex items-center gap-2 text-xs text-slate-600">
                      <span>P{subtask.priority}</span>
                      {subtask.started_at && <span>Started {timeAgo(subtask.started_at)}</span>}
                      {subtask.completed_at && <span>Done {timeAgo(subtask.completed_at)}</span>}
                      {subtask.started_at && subtask.completed_at && (
                        <span className="text-slate-500">
                          ({duration(subtask.started_at, subtask.completed_at)})
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              </button>
            ))
          ) : (
            <div className="text-xs text-slate-500">No subtasks found</div>
          )}
        </div>
      )}
    </div>
  )
}
