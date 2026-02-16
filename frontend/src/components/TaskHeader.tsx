import { Task } from '../lib/api'
import { StatusBadge } from './StatusBadge'
import BackButton from './BackButton'

interface TaskHeaderProps {
  task: Task
  onRetry: () => void
  onCancel: () => void
}

export default function TaskHeader({ task, onRetry, onCancel }: TaskHeaderProps) {
  return (
    <div className="flex items-start justify-between">
      <div>
        <BackButton to="/tasks" label="Back to Tasks" />
        <h1 className="text-xl font-bold text-content mb-2 tracking-tight">{task.title}</h1>
        <div className="flex items-center gap-2">
          <StatusBadge status={task.status} />
          <span className="text-xs text-content-faint font-mono">{task.id.slice(0, 8)}</span>
        </div>
      </div>
      <div className="flex gap-2">
        {(task.status === 'FAILED' || task.status === 'RETRY') && <button onClick={onRetry} className="btn-sm btn-primary">Retry</button>}
        {(task.status === 'READY' || task.status === 'RUNNING' || task.status === 'DECOMPOSED') && <button onClick={onCancel} className="btn-sm btn-danger">Cancel</button>}
      </div>
    </div>
  )
}
