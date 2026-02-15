import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTaskStore } from '../stores/taskStore'
import { Task } from '../lib/api'
import ContentRenderer from '../components/ContentRenderer'
import MarkdownRenderer from '../components/MarkdownRenderer'

const subtaskStatusColor: Record<string, string> = {
  PENDING: 'badge-secondary',
  READY: 'badge-info',
  RUNNING: 'badge-warning',
  COMPLETED: 'badge-success',
  FAILED: 'badge-danger',
  RETRY: 'badge-warning',
  DECOMPOSED: 'bg-purple-600 text-white px-2 py-1 rounded text-xs font-semibold',
  CANCELLED: 'bg-slate-500 text-white px-2 py-1 rounded text-xs font-semibold',
  ARCHIVED: 'badge-secondary',
}

const statusColor: Record<string, string> = {
  PENDING: 'badge-secondary',
  READY: 'badge-info',
  RUNNING: 'badge-warning',
  COMPLETED: 'badge-success',
  FAILED: 'badge-danger',
  RETRY: 'badge-warning',
  ARCHIVED: 'badge-secondary',
  DECOMPOSED: 'bg-purple-600 text-white px-2 py-1 rounded text-xs font-semibold',
  CANCELLED: 'bg-slate-500 text-white px-2 py-1 rounded text-xs font-semibold',
}

const prStatusColor: Record<string, string> = {
  OPEN: 'text-green-400',
  MERGED: 'text-purple-400',
  CLOSED: 'text-red-400',
  CHANGES_REQUESTED: 'text-amber-400',
}

function formatDate(iso?: string) {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function formatDuration(start?: string, end?: string) {
  if (!start) return '—'
  const s = new Date(start).getTime()
  const e = end ? new Date(end).getTime() : Date.now()
  const diff = e - s
  if (diff < 0) return '—'
  const secs = Math.floor(diff / 1000)
  if (secs < 60) return `${secs}s`
  const mins = Math.floor(secs / 60)
  const remSecs = secs % 60
  if (mins < 60) return `${mins}m ${remSecs}s`
  const hours = Math.floor(mins / 60)
  const remMins = mins % 60
  return `${hours}h ${remMins}m`
}

function formatCost(cost?: number) {
  if (cost === undefined || cost === null || cost === 0) return '—'
  return `$${cost.toFixed(4)}`
}

function formatTokens(tokens?: number) {
  if (!tokens) return '—'
  if (tokens >= 1000) return `${(tokens / 1000).toFixed(1)}k`
  return tokens.toString()
}

function InfoRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-start py-2 border-b border-slate-700/50">
      <span className="w-36 shrink-0 text-sm text-slate-500 font-medium">{label}</span>
      <span className="text-sm text-slate-200 min-w-0">{children}</span>
    </div>
  )
}

export default function TaskDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { getTask, cancelTask, retryTask, fetchSubtasks } = useTaskStore()
  const [task, setTask] = useState<Task | null>(null)
  const [subtasks, setSubtasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    setLoading(true)
    getTask(id)
      .then((t) => {
        setTask(t)
        // Fetch subtasks if task has any potential children
        fetchSubtasks(t.id).then(setSubtasks)
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [id, getTask, fetchSubtasks])

  const handleRetry = async () => {
    if (!task || !confirm(`Retry task: ${task.title}?`)) return
    try {
      await retryTask(task.id)
      const updated = await getTask(task.id)
      setTask(updated)
    } catch (err) {
      console.error('Failed to retry task:', err)
    }
  }

  const handleCancel = async () => {
    if (!task || !confirm(`Cancel task: ${task.title}?`)) return
    try {
      await cancelTask(task.id)
      const updated = await getTask(task.id)
      setTask(updated)
    } catch (err) {
      console.error('Failed to cancel task:', err)
    }
  }

  if (loading) {
    return (
      <div className="p-8">
        <div className="text-slate-400">Loading task...</div>
      </div>
    )
  }

  if (error || !task) {
    return (
      <div className="p-8">
        <div className="text-red-400">Error: {error || 'Task not found'}</div>
        <button onClick={() => navigate('/tasks')} className="mt-4 text-blue-400 hover:underline">
          Back to Tasks
        </button>
      </div>
    )
  }

  return (
    <div className="p-8 space-y-6 max-w-4xl">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <button
            onClick={() => navigate('/tasks')}
            className="text-sm text-slate-500 hover:text-slate-300 mb-2 inline-block"
          >
            &larr; Back to Tasks
          </button>
          <h1 className="text-2xl font-bold text-slate-100 mb-2">{task.title}</h1>
          <div className="flex items-center gap-2">
            <span className={`badge ${statusColor[task.status] || 'badge-secondary'}`}>
              {task.status}
            </span>
            <span className="badge-secondary">{task.type}</span>
            <span className="text-sm text-slate-500 font-mono">{task.id.slice(0, 8)}</span>
          </div>
        </div>
        <div className="flex gap-2">
          {(task.status === 'FAILED' || task.status === 'RETRY') && (
            <button
              onClick={handleRetry}
              className="px-4 py-2 rounded text-sm font-medium bg-blue-600 text-white hover:bg-blue-500 transition-colors"
            >
              Retry
            </button>
          )}
          {(task.status === 'READY' || task.status === 'RUNNING' || task.status === 'DECOMPOSED') && (
            <button onClick={handleCancel} className="btn-danger">
              Cancel
            </button>
          )}
        </div>
      </div>

      {/* Description */}
      <div className="card p-6">
        <div className="flex items-center gap-2 mb-3">
          <h2 className="text-lg font-semibold text-slate-200">Description</h2>
          {task.triage_analysis && (
            <span className="bg-cyan-600/20 text-cyan-400 border border-cyan-600/30 px-2 py-0.5 rounded text-xs font-medium">
              Triaged
            </span>
          )}
        </div>
        {task.description ? (
          <MarkdownRenderer content={task.description} />
        ) : (
          <p className="text-slate-400">—</p>
        )}
      </div>

      {/* Triage Analysis */}
      {task.triage_analysis && (
        <div className="card p-6 border border-cyan-600/30 bg-cyan-950/10">
          <h2 className="text-lg font-semibold text-cyan-400 mb-3">Triage Analysis</h2>
          <div className="text-sm text-slate-300 leading-relaxed">
            <MarkdownRenderer content={task.triage_analysis} />
          </div>
        </div>
      )}

      {/* Overview */}
      <div className="card p-6">
        <h2 className="text-lg font-semibold text-slate-200 mb-3">Overview</h2>
        <InfoRow label="Priority">{task.priority}</InfoRow>
        <InfoRow label="Source">{task.source}</InfoRow>
        <InfoRow label="Project ID">
          <span className="font-mono text-xs">{task.project_id || '—'}</span>
        </InfoRow>
        <InfoRow label="Goal ID">
          <span className="font-mono text-xs">{task.goal_id || '—'}</span>
        </InfoRow>
        {task.parent_id && (
          <InfoRow label="Parent Task">
            <button
              onClick={() => navigate(`/tasks/${task.parent_id}`)}
              className="font-mono text-xs text-blue-400 hover:underline"
            >
              {task.parent_id.slice(0, 8)}
            </button>
            {task.depth !== undefined && (
              <span className="ml-2 text-slate-500">(depth {task.depth})</span>
            )}
          </InfoRow>
        )}
        {task.depends_on.length > 0 && (
          <InfoRow label="Depends On">
            <div className="flex flex-wrap gap-1">
              {task.depends_on.map((dep) => (
                <button
                  key={dep}
                  onClick={() => navigate(`/tasks/${dep}`)}
                  className="font-mono text-xs text-blue-400 hover:underline"
                >
                  {dep.slice(0, 8)}
                </button>
              ))}
            </div>
          </InfoRow>
        )}
        {task.tags.length > 0 && (
          <InfoRow label="Tags">
            <div className="flex flex-wrap gap-1">
              {task.tags.map((tag, i) => (
                <span key={i} className="badge-info text-xs">
                  {tag}
                </span>
              ))}
            </div>
          </InfoRow>
        )}
      </div>

      {/* Subtasks */}
      {subtasks.length > 0 && (
        <div className="card p-6">
          <h2 className="text-lg font-semibold text-slate-200 mb-3">
            Subtasks
            <span className="ml-2 text-sm font-normal text-slate-400">
              ({subtasks.filter((s) => s.status === 'COMPLETED').length}/{subtasks.length} completed)
            </span>
          </h2>
          {/* Progress bar */}
          <div className="w-full bg-slate-700 rounded-full h-2 mb-4">
            <div
              className="bg-green-500 h-2 rounded-full transition-all"
              style={{
                width: `${(subtasks.filter((s) => s.status === 'COMPLETED').length / subtasks.length) * 100}%`,
              }}
            />
          </div>
          <div className="space-y-2">
            {subtasks.map((sub) => (
              <div
                key={sub.id}
                className="flex items-center justify-between p-3 bg-slate-800 rounded-lg hover:bg-slate-750 cursor-pointer transition-colors"
                onClick={() => navigate(`/tasks/${sub.id}`)}
              >
                <div className="flex items-center gap-2 min-w-0">
                  <span className={`badge shrink-0 ${subtaskStatusColor[sub.status] || 'badge-secondary'}`}>
                    {sub.status}
                  </span>
                  <span className="text-sm text-slate-200 truncate">{sub.title}</span>
                </div>
                <span className="text-xs text-slate-500 shrink-0 ml-2">P{sub.priority}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Execution */}
      <div className="card p-6">
        <h2 className="text-lg font-semibold text-slate-200 mb-3">Execution</h2>
        <InfoRow label="Executor">
          <span className="font-mono text-xs">{task.executor_id || '—'}</span>
        </InfoRow>
        <InfoRow label="Model">{task.model || '—'}</InfoRow>
        <InfoRow label="Branch">
          <span className="font-mono text-xs">{task.branch_name || '—'}</span>
        </InfoRow>
        <InfoRow label="Retry Count">{task.retry_count ?? 0}</InfoRow>
        <InfoRow label="Crash Recovery">
          {task.crash_recovery ? 'Yes' : '—'}
        </InfoRow>
        <InfoRow label="Tests Passed">
          {task.test_passed === null || task.test_passed === undefined
            ? '—'
            : task.test_passed
            ? 'Yes'
            : 'No'}
        </InfoRow>
        <InfoRow label="Diff">
          {task.diff_lines || task.files_changed
            ? `${task.diff_lines ?? 0} lines, ${task.files_changed ?? 0} files`
            : '—'}
        </InfoRow>
        <InfoRow label="Tokens">{formatTokens(task.tokens_used)}</InfoRow>
        <InfoRow label="Cost">{formatCost(task.cost_usd)}</InfoRow>
      </div>

      {/* PR */}
      {(task.pr_url || task.pr_status) && (
        <div className="card p-6">
          <h2 className="text-lg font-semibold text-slate-200 mb-3">Pull Request</h2>
          <InfoRow label="Status">
            <span className={prStatusColor[task.pr_status || ''] || 'text-slate-400'}>
              {task.pr_status || '—'}
            </span>
          </InfoRow>
          <InfoRow label="URL">
            {task.pr_url ? (
              <a
                href={task.pr_url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-blue-400 hover:underline break-all"
              >
                {task.pr_url}
              </a>
            ) : (
              '—'
            )}
          </InfoRow>
        </div>
      )}

      {/* Timeline */}
      <div className="card p-6">
        <h2 className="text-lg font-semibold text-slate-200 mb-3">Timeline</h2>
        <InfoRow label="Created">{formatDate(task.created_at)}</InfoRow>
        <InfoRow label="Started">{formatDate(task.started_at)}</InfoRow>
        <InfoRow label="Completed">{formatDate(task.completed_at)}</InfoRow>
        <InfoRow label="Updated">{formatDate(task.updated_at)}</InfoRow>
        <InfoRow label="Duration">{formatDuration(task.started_at, task.completed_at)}</InfoRow>
      </div>

      {/* Plan */}
      {task.plan && (
        <div className="card p-6">
          <h2 className="text-lg font-semibold text-slate-200 mb-3">Plan</h2>
          <ContentRenderer content={task.plan} />
        </div>
      )}

      {/* Error */}
      {task.error_log && (
        <div className="card p-6 border border-red-600/50">
          <h2 className="text-lg font-semibold text-red-400 mb-3">Error</h2>
          <pre className="text-sm text-red-200 bg-red-900/20 rounded p-4 overflow-auto whitespace-pre-wrap">
            {task.error_log}
          </pre>
        </div>
      )}

      {/* Result */}
      {task.result && (
        <div className="card p-6">
          <h2 className="text-lg font-semibold text-slate-200 mb-3">Result</h2>
          <ContentRenderer content={task.result} />
        </div>
      )}

      {/* Prompt */}
      {task.prompt && (
        <div className="card p-6">
          <h2 className="text-lg font-semibold text-slate-200 mb-3">Prompt</h2>
          <ContentRenderer content={task.prompt} />
        </div>
      )}
    </div>
  )
}
