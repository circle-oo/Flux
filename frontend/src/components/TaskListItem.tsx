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
      className="card p-4 hover:border-white/[0.12] transition-all cursor-pointer touch-manipulation"
      onClick={() => navigate(`/tasks/${task.id}`)}
    >
      <div className="flex items-start justify-between">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <h3 className="text-sm font-medium text-white/90 hover:text-accent-400 transition-colors truncate">
              {task.triage_title || task.title}
            </h3>
            <StatusBadge status={task.status} />
            {task.triage_analysis && (
              <span className="bg-cyan-500/10 text-cyan-400 ring-1 ring-cyan-500/20 px-1.5 py-0.5 rounded-full text-[10px] font-medium shrink-0">Triaged</span>
            )}
            {subtaskCount && onToggleSubtasks && (
              <button
                onClick={(e) => onToggleSubtasks(task.id, e)}
                className="text-[10px] text-violet-400 bg-violet-500/10 px-1.5 py-0.5 rounded-full shrink-0 hover:bg-violet-500/20 transition-colors flex items-center gap-1 ring-1 ring-violet-500/20"
              >
                <span className="text-[9px]">{isExpanded ? '▼' : '▶'}</span>
                {subtaskCount} subtask{subtaskCount !== 1 ? 's' : ''}
              </button>
            )}
          </div>

          {task.triage_description && (
            <p className="text-xs text-cyan-400/60 mb-1.5 line-clamp-2 border-l-2 border-cyan-500/20 pl-2">
              {task.triage_description}
            </p>
          )}

          <div className="flex flex-wrap items-center gap-2.5 text-[11px] text-white/25">
            {project && <span className="text-white/40">{project.name}</span>}
            <span>P{task.priority}</span>
            <span>{task.source}</span>
            {task.executor_id && task.status === 'RUNNING' && <span className="text-amber-400/70">{task.executor_id}</span>}
            {task.model && <span className="text-white/15">{task.model}</span>}
            {task.diff_lines ? <span>{task.diff_lines}L / {task.files_changed}F</span> : null}
            {task.pr_url && (
              <a href={task.pr_url} target="_blank" rel="noopener noreferrer" className="text-accent-400 hover:text-accent-300 transition-colors" onClick={(e) => e.stopPropagation()}>
                PR {task.pr_status === 'MERGED' ? '(merged)' : ''}
              </a>
            )}
            {task.cost_usd ? <span>${task.cost_usd.toFixed(3)}</span> : null}
          </div>

          <div className="flex items-center gap-2.5 text-[10px] text-white/15 mt-1">
            <span>Created {timeAgo(task.created_at)}</span>
            {task.started_at && <span>Started {timeAgo(task.started_at)}</span>}
            {task.completed_at && <span>Done {timeAgo(task.completed_at)}</span>}
            {task.started_at && task.completed_at && <span className="text-white/20">({duration(task.started_at, task.completed_at)})</span>}
          </div>

          {task.tags.length > 0 && (
            <div className="flex flex-wrap gap-1.5 mt-2">
              {task.tags.map((tag, i) => <span key={i} className="badge-info">{tag}</span>)}
            </div>
          )}
        </div>

        <div className="flex flex-col sm:flex-row gap-1.5 ml-0 sm:ml-4 mt-2 sm:mt-0 shrink-0" onClick={(e) => e.stopPropagation()}>
          {(task.status === 'FAILED' || task.status === 'RETRY') && <button onClick={() => onRetry(task.id, task.title)} className="btn-sm btn-primary">Retry</button>}
          {(task.status === 'READY' || task.status === 'RUNNING' || task.status === 'DECOMPOSED') && <button onClick={() => onCancel(task.id, task.title)} className="btn-sm btn-danger">Cancel</button>}
          {(task.status === 'COMPLETED' || task.status === 'FAILED' || task.status === 'CANCELLED') && <button onClick={() => onArchive(task.id, task.title)} className="btn-sm btn-secondary">Archive</button>}
        </div>
      </div>

      {task.error_log && (
        <div className="mt-3 p-3 bg-rose-500/[0.06] border border-rose-500/15 rounded-lg text-xs text-rose-300 line-clamp-3" role="alert">
          <strong className="text-rose-400">Error:</strong> {task.error_log}
        </div>
      )}

      {isExpanded && (
        <div className="mt-3 pl-3 border-l-2 border-violet-500/20 space-y-1.5">
          {isLoadingSubtasks ? <div className="text-[10px] text-white/20">Loading subtasks...</div> : subtasks && subtasks.length > 0 ? (
            subtasks.map((subtask) => (
              <button type="button" key={subtask.id} className="w-full text-left bg-white/[0.02] border border-white/[0.04] rounded-lg p-2.5 cursor-pointer hover:border-white/[0.08] hover:bg-white/[0.04] transition-all"
                onClick={(e) => { e.stopPropagation(); navigate(`/tasks/${subtask.id}`) }}>
                <div className="flex items-center gap-2 mb-0.5">
                  <h4 className="text-xs font-medium text-white/70 truncate">{subtask.triage_title || subtask.title}</h4>
                  <StatusBadge status={subtask.status} size="sm" />
                </div>
                <div className="flex items-center gap-2 text-[10px] text-white/20">
                  <span>P{subtask.priority}</span>
                  {subtask.started_at && <span>Started {timeAgo(subtask.started_at)}</span>}
                  {subtask.completed_at && <span>Done {timeAgo(subtask.completed_at)}</span>}
                  {subtask.started_at && subtask.completed_at && <span className="text-white/25">({duration(subtask.started_at, subtask.completed_at)})</span>}
                </div>
              </button>
            ))
          ) : <div className="text-[10px] text-white/20">No subtasks found</div>}
        </div>
      )}
    </div>
  )
}
