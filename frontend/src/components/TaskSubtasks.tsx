import { useNavigate } from 'react-router-dom'
import { Task } from '../lib/api'
import { StatusBadge } from './StatusBadge'
import SubtaskDAGVisualization from './SubtaskDAGVisualization'

interface TaskSubtasksProps {
  subtasks: Task[]
  dependencies: { dependent_id: string; dependency_id: string }[]
  showDAG: boolean
  expanded: boolean
  onToggleExpanded: () => void
}

export default function TaskSubtasks({ subtasks, dependencies, showDAG, expanded, onToggleExpanded }: TaskSubtasksProps) {
  const navigate = useNavigate()
  const completedCount = subtasks.filter((s) => s.status === 'COMPLETED').length
  if (subtasks.length === 0) return null

  return (
    <div className="card p-5">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-sm font-semibold text-content">
          Subtasks
          <span className="ml-2 text-xs font-normal text-content-faint">({completedCount}/{subtasks.length} completed)</span>
        </h2>
        <button onClick={onToggleExpanded} className="text-content-faint hover:text-content-secondary transition-colors" aria-label={expanded ? 'Collapse subtasks' : 'Expand subtasks'} aria-expanded={expanded}>
          <svg className={`w-4 h-4 transition-transform ${expanded ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </button>
      </div>
      {expanded ? (
        <>
          <div className="w-full bg-surface-hover rounded-full h-1.5 mb-4" role="progressbar" aria-valuenow={completedCount} aria-valuemin={0} aria-valuemax={subtasks.length}>
            <div className="bg-emerald-500 h-1.5 rounded-full transition-all" style={{ width: `${(completedCount / subtasks.length) * 100}%` }} />
          </div>
          {showDAG && (
            <div className="mb-4">
              <div className="flex items-center justify-between mb-2">
                <h3 className="text-xs font-semibold text-content-muted">Dependency Graph</h3>
                <span className="text-[10px] text-content-faint">{dependencies.length} dependencies</span>
              </div>
              <SubtaskDAGVisualization nodes={subtasks} edges={dependencies} />
            </div>
          )}
          <div className="space-y-1.5">
            {subtasks.map((sub) => (
              <button key={sub.id} type="button" className="flex items-center justify-between p-3 bg-surface-hover border border-line-subtle rounded-lg hover:border-line hover:bg-surface-hover transition-all w-full text-left" onClick={() => navigate(`/tasks/${sub.id}`)}>
                <div className="flex items-center gap-2 min-w-0">
                  <StatusBadge status={sub.status} />
                  <span className="text-xs text-content-secondary truncate">{sub.title}</span>
                </div>
                <span className="text-[10px] text-content-faint shrink-0 ml-2">P{sub.priority}</span>
              </button>
            ))}
          </div>
        </>
      ) : (
        <div className="text-xs text-content-faint">
          {completedCount} completed, {subtasks.filter((s) => s.status === 'RUNNING').length} running, {subtasks.filter((s) => s.status === 'PENDING' || s.status === 'READY').length} pending
        </div>
      )}
    </div>
  )
}
