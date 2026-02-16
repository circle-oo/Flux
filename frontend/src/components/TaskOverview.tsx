import { useNavigate } from 'react-router-dom'
import { Task } from '../lib/api'
import InfoRow from './InfoRow'
import MarkdownRenderer from './MarkdownRenderer'
import { formatDate, formatDuration } from '../lib/utils'

interface TaskOverviewProps { task: Task }

export default function TaskOverview({ task }: TaskOverviewProps) {
  const navigate = useNavigate()
  return (
    <>
      <div className="card p-5">
        <div className="flex items-center gap-2 mb-3">
          <h2 className="text-sm font-semibold text-white/80">Description</h2>
          {task.triage_description && <span className="bg-white/[0.04] text-white/30 px-1.5 py-0.5 rounded text-[10px] font-medium">User</span>}
        </div>
        {task.description ? <MarkdownRenderer content={task.description} /> : <p className="text-white/30">--</p>}
      </div>
      {task.triage_description && (
        <div className="card p-5 ring-1 ring-cyan-500/15">
          <div className="flex items-center gap-2 mb-3">
            <h2 className="text-sm font-semibold text-cyan-400">Triage Description</h2>
            <span className="badge-info">AI</span>
          </div>
          <div className="text-sm text-white/60 leading-relaxed"><MarkdownRenderer content={task.triage_description} /></div>
        </div>
      )}
      <div className="card p-5">
        <h2 className="text-sm font-semibold text-white/80 mb-3">Details</h2>
        <InfoRow label="Priority">{task.priority}</InfoRow>
        <InfoRow label="Source">{task.source}</InfoRow>
        <InfoRow label="Project ID"><span className="font-mono text-xs">{task.project_id || '--'}</span></InfoRow>
        <InfoRow label="Goal ID"><span className="font-mono text-xs">{task.goal_id || '--'}</span></InfoRow>
        {task.parent_id && <InfoRow label="Parent Task"><button onClick={() => navigate(`/tasks/${task.parent_id}`)} className="font-mono text-xs text-accent-400 hover:text-accent-300 transition-colors">{task.parent_id.slice(0, 8)}</button>{task.depth !== undefined && <span className="ml-2 text-white/30">(depth {task.depth})</span>}</InfoRow>}
        {task.depends_on.length > 0 && <InfoRow label="Depends On"><div className="flex flex-wrap gap-1">{task.depends_on.map((dep) => (<button key={dep} onClick={() => navigate(`/tasks/${dep}`)} className="font-mono text-xs text-accent-400 hover:text-accent-300 transition-colors">{dep.slice(0, 8)}</button>))}</div></InfoRow>}
        {task.tags.length > 0 && <InfoRow label="Tags"><div className="flex flex-wrap gap-1">{task.tags.map((tag, i) => (<span key={i} className="badge-info">{tag}</span>))}</div></InfoRow>}
      </div>
      <div className="card p-5">
        <h2 className="text-sm font-semibold text-white/80 mb-3">Timeline</h2>
        <InfoRow label="Created">{formatDate(task.created_at)}</InfoRow>
        <InfoRow label="Started">{formatDate(task.started_at)}</InfoRow>
        <InfoRow label="Completed">{formatDate(task.completed_at)}</InfoRow>
        <InfoRow label="Updated">{formatDate(task.updated_at)}</InfoRow>
        <InfoRow label="Duration">{formatDuration(task.started_at, task.completed_at)}</InfoRow>
      </div>
    </>
  )
}
