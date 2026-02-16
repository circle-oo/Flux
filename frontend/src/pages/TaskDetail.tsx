import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTaskStore } from '../stores/taskStore'
import { useWSStore } from '../stores/wsStore'
import { Task, api } from '../lib/api'
import { StatusBadge } from '../components/StatusBadge'
import InfoRow from '../components/InfoRow'
import ContentRenderer from '../components/ContentRenderer'
import MarkdownRenderer from '../components/MarkdownRenderer'
import BackButton from '../components/BackButton'
import LoadingState from '../components/LoadingState'
import SubtaskDAGVisualization from '../components/SubtaskDAGVisualization'
import { formatDate, formatDuration, formatCost, formatTokens, prStatusTextColor } from '../lib/utils'

export default function TaskDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { getTask, cancelTask, retryTask, fetchSubtasks } = useTaskStore()
  const taskUpdateCounter = useWSStore((s) => s.taskUpdateCounter)
  const [task, setTask] = useState<Task | null>(null)
  const [subtasks, setSubtasks] = useState<Task[]>([])
  const [dependencies, setDependencies] = useState<{ dependent_id: string; dependency_id: string }[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [subtasksExpanded, setSubtasksExpanded] = useState(true)
  const [showDAG, setShowDAG] = useState(false)

  const refreshTask = useCallback(() => {
    if (!id) return
    getTask(id).then((t) => {
      setTask(t)
      fetchSubtasks(t.id).then(setSubtasks)
      // Fetch dependency graph
      api.getSubtaskDependencies(t.id).then((data) => {
        setDependencies(data.edges)
        setShowDAG(data.edges.length > 0)
      }).catch(() => {})
    }).catch(() => {})
  }, [id, getTask, fetchSubtasks])

  useEffect(() => {
    if (!id) return
    setLoading(true)
    getTask(id)
      .then((t) => {
        setTask(t)
        fetchSubtasks(t.id).then(setSubtasks)
        // Fetch dependency graph
        api.getSubtaskDependencies(t.id).then((data) => {
          setDependencies(data.edges)
          setShowDAG(data.edges.length > 0)
        }).catch(() => {})
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [id, getTask, fetchSubtasks])

  // Refresh task when WebSocket broadcasts a task update
  useEffect(() => {
    if (!id || loading) return
    refreshTask()
  }, [taskUpdateCounter, id, loading, refreshTask])

  // Auto-refresh running/pending/decomposed tasks every 5s
  useEffect(() => {
    if (!task || !id) return
    const activeStatuses = ['RUNNING', 'PENDING', 'READY', 'DECOMPOSED']
    if (!activeStatuses.includes(task.status)) return

    const interval = setInterval(refreshTask, 5000)
    return () => clearInterval(interval)
  }, [task?.status, id, refreshTask])

  const handleRetry = async () => {
    if (!task || !confirm(`Retry task: ${task.title}?`)) return
    try {
      await retryTask(task.id)
      setTask(await getTask(task.id))
    } catch (err) {
      console.error('Failed to retry task:', err)
    }
  }

  const handleCancel = async () => {
    if (!task || !confirm(`Cancel task: ${task.title}?`)) return
    try {
      await cancelTask(task.id)
      setTask(await getTask(task.id))
    } catch (err) {
      console.error('Failed to cancel task:', err)
    }
  }

  if (loading) {
    return (
      <div className="p-4 sm:p-6 lg:p-8">
        <LoadingState message="Loading task..." />
      </div>
    )
  }

  if (error || !task) {
    return (
      <div className="p-8">
        <div className="text-red-400" role="alert">Error: {error || 'Task not found'}</div>
        <button onClick={() => navigate('/tasks')} className="mt-4 text-blue-400 hover:underline">
          Back to Tasks
        </button>
      </div>
    )
  }

  const completedCount = subtasks.filter((s) => s.status === 'COMPLETED').length

  return (
    <div className="p-4 sm:p-6 lg:p-8 space-y-6 max-w-4xl">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <BackButton to="/tasks" label="Back to Tasks" />
          <h1 className="text-2xl font-bold text-slate-100 mb-2">{task.title}</h1>
          <div className="flex items-center gap-2">
            <StatusBadge status={task.status} />
            <span className="text-sm text-slate-500 font-mono">{task.id.slice(0, 8)}</span>
          </div>
        </div>
        <div className="flex gap-2">
          {(task.status === 'FAILED' || task.status === 'RETRY') && (
            <button onClick={handleRetry} className="btn-primary">
              Retry
            </button>
          )}
          {(task.status === 'READY' || task.status === 'RUNNING' || task.status === 'DECOMPOSED') && (
            <button onClick={handleCancel} className="btn-danger">Cancel</button>
          )}
        </div>
      </div>

      {/* Triage Analysis */}
      {task.triage_analysis && (
        <div className="card p-6 border border-cyan-600/30">
          <div className="flex items-center gap-2 mb-3">
            <h2 className="text-lg font-semibold text-cyan-400">Triage Analysis</h2>
            <span className="bg-cyan-600/20 text-cyan-400 border border-cyan-600/30 px-2 py-0.5 rounded text-xs font-medium">AI</span>
          </div>
          <div className="text-sm text-slate-300 leading-relaxed">
            <MarkdownRenderer content={task.triage_analysis} />
          </div>
        </div>
      )}

      {/* Subtasks */}
      {subtasks.length > 0 && (
        <div className="card p-6">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-lg font-semibold text-slate-200">
              Subtasks
              <span className="ml-2 text-sm font-normal text-slate-400">
                ({completedCount}/{subtasks.length} completed)
              </span>
            </h2>
            <button
              onClick={() => setSubtasksExpanded(!subtasksExpanded)}
              className="text-slate-400 hover:text-slate-200 transition-colors"
              aria-label={subtasksExpanded ? 'Collapse subtasks' : 'Expand subtasks'}
              aria-expanded={subtasksExpanded}
            >
              <svg
                className={`w-5 h-5 transition-transform ${subtasksExpanded ? 'rotate-180' : ''}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
                aria-hidden="true"
              >
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
              </svg>
            </button>
          </div>
          {subtasksExpanded ? (
            <>
              <div className="w-full bg-slate-700 rounded-full h-2 mb-4" role="progressbar" aria-valuenow={completedCount} aria-valuemin={0} aria-valuemax={subtasks.length}>
                <div
                  className="bg-green-500 h-2 rounded-full transition-all"
                  style={{ width: `${(completedCount / subtasks.length) * 100}%` }}
                />
              </div>

              {/* DAG Visualization - show only if dependencies exist */}
              {showDAG && (
                <div className="mb-4">
                  <div className="flex items-center justify-between mb-2">
                    <h3 className="text-sm font-semibold text-slate-300">Dependency Graph</h3>
                    <span className="text-xs text-slate-500">{dependencies.length} dependencies</span>
                  </div>
                  <SubtaskDAGVisualization nodes={subtasks} edges={dependencies} />
                </div>
              )}

              <div className="space-y-2">
                {subtasks.map((sub) => (
                  <button
                    key={sub.id}
                    type="button"
                    className="flex items-center justify-between p-3 bg-slate-800 rounded-lg hover:bg-slate-750 transition-colors w-full text-left"
                    onClick={() => navigate(`/tasks/${sub.id}`)}
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <StatusBadge status={sub.status} />
                      <span className="text-sm text-slate-200 truncate">{sub.title}</span>
                    </div>
                    <span className="text-xs text-slate-500 shrink-0 ml-2">P{sub.priority}</span>
                  </button>
                ))}
              </div>
            </>
          ) : (
            <div className="text-sm text-slate-400">
              {completedCount} completed,{' '}
              {subtasks.filter((s) => s.status === 'RUNNING').length} running,{' '}
              {subtasks.filter((s) => s.status === 'PENDING' || s.status === 'READY').length} pending
            </div>
          )}
        </div>
      )}

      {/* Description */}
      <div className="card p-6">
        <div className="flex items-center gap-2 mb-3">
          <h2 className="text-lg font-semibold text-slate-200">Description</h2>
          {task.triage_description && (
            <span className="bg-slate-600/40 text-slate-400 border border-slate-600/30 px-2 py-0.5 rounded text-xs font-medium">User</span>
          )}
        </div>
        {task.description ? (
          <MarkdownRenderer content={task.description} />
        ) : (
          <p className="text-slate-400">—</p>
        )}
      </div>

      {/* Triage Description */}
      {task.triage_description && (
        <div className="card p-6 border border-cyan-600/30">
          <div className="flex items-center gap-2 mb-3">
            <h2 className="text-lg font-semibold text-cyan-400">Triage Description</h2>
            <span className="bg-cyan-600/20 text-cyan-400 border border-cyan-600/30 px-2 py-0.5 rounded text-xs font-medium">AI</span>
          </div>
          <div className="text-sm text-slate-300 leading-relaxed">
            <MarkdownRenderer content={task.triage_description} />
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
            <button onClick={() => navigate(`/tasks/${task.parent_id}`)} className="font-mono text-xs text-blue-400 hover:underline">
              {task.parent_id.slice(0, 8)}
            </button>
            {task.depth !== undefined && <span className="ml-2 text-slate-500">(depth {task.depth})</span>}
          </InfoRow>
        )}
        {task.depends_on.length > 0 && (
          <InfoRow label="Depends On">
            <div className="flex flex-wrap gap-1">
              {task.depends_on.map((dep) => (
                <button key={dep} onClick={() => navigate(`/tasks/${dep}`)} className="font-mono text-xs text-blue-400 hover:underline">
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
                <span key={i} className="badge-info text-xs">{tag}</span>
              ))}
            </div>
          </InfoRow>
        )}
      </div>

      {/* Execution */}
      <div className="card p-6">
        <h2 className="text-lg font-semibold text-slate-200 mb-3">Execution</h2>
        <InfoRow label="Executor"><span className="font-mono text-xs">{task.executor_id || '—'}</span></InfoRow>
        <InfoRow label="Model">{task.model || '—'}</InfoRow>
        <InfoRow label="Branch"><span className="font-mono text-xs">{task.branch_name || '—'}</span></InfoRow>
        <InfoRow label="Retry Count">{task.retry_count ?? 0}</InfoRow>
        <InfoRow label="Crash Recovery">{task.crash_recovery ? 'Yes' : '—'}</InfoRow>
        <InfoRow label="Tests Passed">
          {task.test_passed === null || task.test_passed === undefined ? '—' : task.test_passed ? 'Yes' : 'No'}
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
            <span className={prStatusTextColor[task.pr_status || ''] || 'text-slate-400'}>
              {task.pr_status || '—'}
            </span>
          </InfoRow>
          <InfoRow label="URL">
            {task.pr_url ? (
              <a href={task.pr_url} target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:underline break-all">
                {task.pr_url}
              </a>
            ) : '—'}
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
        <div className="card p-6 border border-red-600/50" role="alert">
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
          {(() => {
            try {
              const parsed = JSON.parse(task.result)
              if (parsed.result && typeof parsed.result === 'string') {
                return (
                  <>
                    <div className="mb-4">
                      <MarkdownRenderer content={parsed.result} />
                    </div>
                    <details className="mt-4">
                      <summary className="text-sm text-slate-400 cursor-pointer hover:text-slate-300">
                        Show full output
                      </summary>
                      <div className="mt-2">
                        <ContentRenderer content={task.result} />
                      </div>
                    </details>
                  </>
                )
              }
            } catch {
              // Not JSON, fall through to default rendering
            }
            return <ContentRenderer content={task.result} />
          })()}
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
