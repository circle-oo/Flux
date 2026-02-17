import { useState, useEffect } from 'react'
import { Task, TaskAttempt, TaskAttemptsResponse, TaskUsageResponse, api } from '../lib/api'
import { useWSStore } from '../stores/wsStore'
import InfoRow from './InfoRow'
import ContentRenderer from './ContentRenderer'
import { formatTokens, formatCost, formatDate, formatDuration, formatTime, prStatusTextColor } from '../lib/utils'

interface TaskExecutionProps { task: Task }

export default function TaskExecution({ task }: TaskExecutionProps) {
  const [attemptsData, setAttemptsData] = useState<TaskAttemptsResponse | null>(null)
  const [expandedAttempt, setExpandedAttempt] = useState<number | null>(null)
  const [usageData, setUsageData] = useState<TaskUsageResponse | null>(null)
  const [showUsageLog, setShowUsageLog] = useState(false)
  const liveUsage = useWSStore((s) => s.usageByTask[task.id])

  useEffect(() => {
    if ((task.retry_count ?? 0) > 0) {
      api.getTaskAttempts(task.id).then(setAttemptsData).catch(() => {})
    } else {
      setAttemptsData(null)
    }
  }, [task.id, task.retry_count])

  // Fetch usage events for all tasks (needed for triage vs execution breakdown)
  useEffect(() => {
    api.getTaskUsage(task.id).then(setUsageData).catch(() => {})
  }, [task.id, task.status])

  return (
    <>
      <div className="card p-5">
        <h2 className="text-sm font-semibold text-content mb-3">Timeline</h2>
        <InfoRow label="Created">{formatDate(task.created_at)}</InfoRow>
        <InfoRow label="Started">{formatDate(task.started_at)}</InfoRow>
        <InfoRow label="Completed">{formatDate(task.completed_at)}</InfoRow>
        <InfoRow label="Updated">{formatDate(task.updated_at)}</InfoRow>
        <InfoRow label="Duration">{formatDuration(task.started_at, task.completed_at)}</InfoRow>
      </div>
      <div className="card p-5">
        <h2 className="text-sm font-semibold text-content mb-3">Execution</h2>
        <InfoRow label="Executor"><span className="font-mono text-xs">{task.executor_id || '--'}</span></InfoRow>
        <InfoRow label="Model">{task.model || '--'}</InfoRow>
        <InfoRow label="Branch"><span className="font-mono text-xs">{task.branch_name || '--'}</span></InfoRow>
        <InfoRow label="Retry Count">{task.retry_count ?? 0}</InfoRow>
        <InfoRow label="Crash Recovery">{task.crash_recovery ? 'Yes' : '--'}</InfoRow>
        <InfoRow label="Tests Passed">{task.test_passed === null || task.test_passed === undefined ? '--' : task.test_passed ? 'Yes' : 'No'}</InfoRow>
        <InfoRow label="Diff">{task.diff_lines || task.files_changed ? `${task.diff_lines ?? 0} lines, ${task.files_changed ?? 0} files` : '--'}</InfoRow>
        {/* Token & Cost Breakdown */}
        {(() => {
          const triageTokens = usageData?.events.filter(e => e.source === 'triager').reduce((sum, e) => sum + e.tokens, 0) ?? 0
          const triageCost = usageData?.events.filter(e => e.source === 'triager').reduce((sum, e) => sum + e.cost_usd, 0) ?? 0
          const execTokens = (task.tokens_used ?? 0) - triageTokens
          const execCost = (task.cost_usd ?? 0) - triageCost
          const hasTriage = triageTokens > 0 || triageCost > 0

          return (
            <>
              {hasTriage ? (
                <>
                  <div className="border-t border-line-subtle mt-3 pt-3">
                    <span className="text-[10px] text-content-faint uppercase tracking-wider font-medium">Triage</span>
                  </div>
                  <InfoRow label="Tokens">{formatTokens(triageTokens)}</InfoRow>
                  <InfoRow label="Cost">{formatCost(triageCost)}</InfoRow>
                  <div className="border-t border-line-subtle mt-3 pt-3">
                    <span className="text-[10px] text-content-faint uppercase tracking-wider font-medium">Execution</span>
                  </div>
                  <InfoRow label="Tokens">{formatTokens(execTokens > 0 ? execTokens : 0)}</InfoRow>
                  <InfoRow label="Cost">{formatCost(execCost > 0 ? execCost : 0)}</InfoRow>
                </>
              ) : (
                <>
                  <InfoRow label="Tokens">{formatTokens(task.tokens_used)}</InfoRow>
                  <InfoRow label="Cost">{formatCost(task.cost_usd)}</InfoRow>
                </>
              )}
            </>
          )
        })()}
        {task.status === 'RUNNING' && liveUsage && (
          <div className="border-t border-line-subtle mt-3 pt-3">
            <div className="flex items-center gap-1.5 mb-2">
              <div className="w-1.5 h-1.5 rounded-full bg-emerald-400 shadow-sm shadow-emerald-500/30 animate-glow-pulse" />
              <span className="text-[10px] text-content-faint uppercase tracking-wider font-medium">Live Usage</span>
            </div>
            <InfoRow label="Live Tokens">
              <span className="text-primary-600 font-mono tabular-nums">{liveUsage.tokens.toLocaleString()}</span>
            </InfoRow>
            <InfoRow label="Live Cost">
              <span className="text-amber-600 font-mono tabular-nums">${liveUsage.cost.toFixed(4)}</span>
            </InfoRow>
          </div>
        )}
        {attemptsData && (
          <>
            <div className="border-t border-line mt-3 pt-3">
              <InfoRow label="Total Tokens (all attempts)">{formatTokens(attemptsData.total_tokens_used)}</InfoRow>
              <InfoRow label="Total Cost (all attempts)">{formatCost(attemptsData.total_cost_usd)}</InfoRow>
            </div>
          </>
        )}
      </div>
      {(task.pr_url || task.pr_status) && (
        <div className="card p-5">
          <h2 className="text-sm font-semibold text-content mb-3">Pull Request</h2>
          <InfoRow label="Status"><span className={prStatusTextColor[task.pr_status || ''] || 'text-content-muted'}>{task.pr_status || '--'}</span></InfoRow>
          <InfoRow label="URL">{task.pr_url ? <a href={task.pr_url} target="_blank" rel="noopener noreferrer" className="text-primary-600 hover:text-primary-500 break-all transition-colors text-xs">{task.pr_url}</a> : '--'}</InfoRow>
        </div>
      )}
      {task.plan && <div className="card p-5"><h2 className="text-sm font-semibold text-content mb-3">Plan</h2><ContentRenderer content={task.plan} /></div>}

      {attemptsData && attemptsData.attempts.length > 0 && (
        <div className="card p-5">
          <h2 className="text-sm font-semibold text-content mb-3">Attempt History</h2>
          <div className="space-y-2">
            {attemptsData.attempts.map((attempt) => (
              <AttemptCard
                key={attempt.id}
                attempt={attempt}
                expanded={expandedAttempt === attempt.id}
                onToggle={() => setExpandedAttempt(expandedAttempt === attempt.id ? null : attempt.id)}
              />
            ))}
          </div>
        </div>
      )}

      {usageData && usageData.events.length > 0 && (
        <div className="card p-5">
          <button
            onClick={() => setShowUsageLog(!showUsageLog)}
            className="w-full flex items-center justify-between text-left group"
          >
            <h2 className="text-sm font-semibold text-content">
              Usage Events
              <span className="ml-1.5 text-content-faint font-normal">({usageData.events.length})</span>
            </h2>
            <div className="flex items-center gap-3 text-xs text-content-muted tabular-nums">
              <span>{usageData.total_tokens.toLocaleString()} tokens</span>
              <span>${usageData.total_cost.toFixed(4)}</span>
              <span className="text-content-faint group-hover:text-content-muted transition-colors">{showUsageLog ? '▲' : '▼'}</span>
            </div>
          </button>
          {showUsageLog && (
            <div className="mt-3 space-y-1 max-h-60 overflow-y-auto">
              {usageData.events.map((evt) => (
                <div key={evt.id} className="flex items-center justify-between py-1.5 px-2 rounded-lg text-xs hover:bg-surface-hover transition-colors">
                  <div className="flex items-center gap-2">
                    <span className="text-content-faint font-mono tabular-nums">{formatTime(evt.recorded_at)}</span>
                    <span className={
                      evt.source === 'ccusage' ? 'badge-success' :
                      evt.source === 'executor' ? 'badge-info' :
                      'badge-purple'
                    }>{evt.source}</span>
                  </div>
                  <div className="flex items-center gap-3 text-content-muted font-mono tabular-nums">
                    <span>{evt.tokens.toLocaleString()}</span>
                    <span>${evt.cost_usd.toFixed(4)}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </>
  )
}

function AttemptCard({ attempt, expanded, onToggle }: { attempt: TaskAttempt; expanded: boolean; onToggle: () => void }) {
  const statusColor = attempt.status === 'FAILED' ? 'text-rose-600' : attempt.status === 'COMPLETED' ? 'text-emerald-600' : 'text-content-muted'

  return (
    <div className="border border-line rounded-lg overflow-hidden">
      <button
        onClick={onToggle}
        className="w-full flex items-center justify-between p-3 text-left hover:bg-surface-hover transition-colors"
      >
        <div className="flex items-center gap-3">
          <span className="text-xs font-mono text-content-muted">#{attempt.attempt}</span>
          <span className={`text-xs font-medium ${statusColor}`}>{attempt.status}</span>
          {attempt.model && <span className="text-xs text-content-muted">{attempt.model}</span>}
        </div>
        <div className="flex items-center gap-3 text-xs text-content-muted">
          <span>{formatTokens(attempt.tokens_used)} tokens</span>
          <span>{formatCost(attempt.cost_usd)}</span>
          <span>{expanded ? '▲' : '▼'}</span>
        </div>
      </button>
      {expanded && (
        <div className="border-t border-line p-3 space-y-2 text-xs">
          <InfoRow label="Executor"><span className="font-mono">{attempt.executor_id || '--'}</span></InfoRow>
          <InfoRow label="Branch"><span className="font-mono">{attempt.branch_name || '--'}</span></InfoRow>
          <InfoRow label="Started">{formatDate(attempt.started_at)}</InfoRow>
          <InfoRow label="Completed">{formatDate(attempt.completed_at)}</InfoRow>
          <InfoRow label="Tests">{attempt.test_passed === null ? '--' : attempt.test_passed ? 'Passed' : 'Failed'}</InfoRow>
          <InfoRow label="Diff">{attempt.diff_lines || attempt.files_changed ? `${attempt.diff_lines} lines, ${attempt.files_changed} files` : '--'}</InfoRow>
          {attempt.error_log && (
            <div className="mt-2">
              <span className="text-content-muted block mb-1">Error Log</span>
              <pre className="bg-rose-50 rounded p-2 text-rose-600 whitespace-pre-wrap break-words max-h-40 overflow-y-auto">{attempt.error_log}</pre>
            </div>
          )}
          {attempt.result && (
            <div className="mt-2">
              <span className="text-content-muted block mb-1">Result</span>
              <div className="bg-surface-hover rounded p-2 max-h-40 overflow-y-auto">
                <ContentRenderer content={attempt.result} />
              </div>
            </div>
          )}
          {attempt.triage_analysis && (
            <div className="mt-2">
              <span className="text-content-muted block mb-1">Triage Analysis</span>
              <div className="bg-surface-hover rounded p-2 max-h-40 overflow-y-auto">
                <ContentRenderer content={attempt.triage_analysis} />
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
