import { Task } from '../lib/api'
import InfoRow from './InfoRow'
import ContentRenderer from './ContentRenderer'
import { formatTokens, formatCost, prStatusTextColor } from '../lib/utils'

interface TaskExecutionProps {
  task: Task
}

export default function TaskExecution({ task }: TaskExecutionProps) {
  return (
    <>
      <div className="card p-6">
        <h2 className="text-lg font-semibold text-slate-200 mb-3">Execution</h2>
        <InfoRow label="Executor"><span className="font-mono text-xs">{task.executor_id || '--'}</span></InfoRow>
        <InfoRow label="Model">{task.model || '--'}</InfoRow>
        <InfoRow label="Branch"><span className="font-mono text-xs">{task.branch_name || '--'}</span></InfoRow>
        <InfoRow label="Retry Count">{task.retry_count ?? 0}</InfoRow>
        <InfoRow label="Crash Recovery">{task.crash_recovery ? 'Yes' : '--'}</InfoRow>
        <InfoRow label="Tests Passed">
          {task.test_passed === null || task.test_passed === undefined ? '--' : task.test_passed ? 'Yes' : 'No'}
        </InfoRow>
        <InfoRow label="Diff">
          {task.diff_lines || task.files_changed
            ? `${task.diff_lines ?? 0} lines, ${task.files_changed ?? 0} files`
            : '--'}
        </InfoRow>
        <InfoRow label="Tokens">{formatTokens(task.tokens_used)}</InfoRow>
        <InfoRow label="Cost">{formatCost(task.cost_usd)}</InfoRow>
      </div>

      {(task.pr_url || task.pr_status) && (
        <div className="card p-6">
          <h2 className="text-lg font-semibold text-slate-200 mb-3">Pull Request</h2>
          <InfoRow label="Status">
            <span className={prStatusTextColor[task.pr_status || ''] || 'text-slate-400'}>
              {task.pr_status || '--'}
            </span>
          </InfoRow>
          <InfoRow label="URL">
            {task.pr_url ? (
              <a href={task.pr_url} target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:underline break-all">
                {task.pr_url}
              </a>
            ) : '--'}
          </InfoRow>
        </div>
      )}

      {task.plan && (
        <div className="card p-6">
          <h2 className="text-lg font-semibold text-slate-200 mb-3">Plan</h2>
          <ContentRenderer content={task.plan} />
        </div>
      )}
    </>
  )
}
