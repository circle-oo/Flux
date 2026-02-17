import { useNavigate } from 'react-router-dom'
import { Task } from '../lib/api'
import InfoRow from './InfoRow'
import MarkdownRenderer from './MarkdownRenderer'

interface TaskTriageProps { task: Task }

export default function TaskTriage({ task }: TaskTriageProps) {
  const navigate = useNavigate()
  const hasTriage = task.triage_analysis || task.triage_description || task.triage_title

  return (
    <>
      {/* AI Triage Analysis */}
      {task.triage_analysis && (
        <div className="card p-5 ring-1 ring-cyan-200/60">
          <div className="flex items-center gap-2 mb-3">
            <h2 className="text-sm font-semibold text-cyan-600">Analysis</h2>
            <span className="badge-info">AI</span>
          </div>
          <div className="text-sm text-content-secondary leading-relaxed">
            <MarkdownRenderer content={task.triage_analysis} />
          </div>
        </div>
      )}

      {/* AI Triage Description */}
      {task.triage_description && (
        <div className="card p-5 ring-1 ring-cyan-500/15">
          <div className="flex items-center gap-2 mb-3">
            <h2 className="text-sm font-semibold text-cyan-600">Task Brief</h2>
            <span className="badge-info">AI</span>
          </div>
          <div className="text-sm text-content-secondary leading-relaxed">
            <MarkdownRenderer content={task.triage_description} />
          </div>
        </div>
      )}

      {/* Original User Description */}
      <div className="card p-5">
        <div className="flex items-center gap-2 mb-3">
          <h2 className="text-sm font-semibold text-content">Original Description</h2>
          <span className="bg-surface-hover text-content-faint px-1.5 py-0.5 rounded text-[10px] font-medium">User</span>
        </div>
        {task.description ? <MarkdownRenderer content={task.description} /> : <p className="text-content-faint">--</p>}
      </div>

      {/* Task Metadata */}
      <div className="card p-5">
        <h2 className="text-sm font-semibold text-content mb-3">Details</h2>
        <InfoRow label="Priority">{task.priority}</InfoRow>
        <InfoRow label="Source">{task.source}</InfoRow>
        <InfoRow label="Project ID"><span className="font-mono text-xs">{task.project_id || '--'}</span></InfoRow>
        <InfoRow label="Goal ID"><span className="font-mono text-xs">{task.goal_id || '--'}</span></InfoRow>
        {task.parent_id && <InfoRow label="Parent Task"><button onClick={() => navigate(`/tasks/${task.parent_id}`)} className="font-mono text-xs text-primary-600 hover:text-primary-500 transition-colors">{task.parent_id.slice(0, 8)}</button>{task.depth !== undefined && <span className="ml-2 text-content-faint">(depth {task.depth})</span>}</InfoRow>}
        {task.depends_on.length > 0 && <InfoRow label="Depends On"><div className="flex flex-wrap gap-1">{task.depends_on.map((dep) => (<button key={dep} onClick={() => navigate(`/tasks/${dep}`)} className="font-mono text-xs text-primary-600 hover:text-primary-500 transition-colors">{dep.slice(0, 8)}</button>))}</div></InfoRow>}
        {task.tags.length > 0 && <InfoRow label="Tags"><div className="flex flex-wrap gap-1">{task.tags.map((tag, i) => (<span key={i} className="badge-info">{tag}</span>))}</div></InfoRow>}
      </div>

      {!hasTriage && (
        <div className="card p-5 text-center text-content-faint text-sm">Not yet triaged</div>
      )}
    </>
  )
}
