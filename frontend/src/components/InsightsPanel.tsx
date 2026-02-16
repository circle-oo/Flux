import { Insights } from '../lib/api'

interface InsightsPanelProps {
  insights: Insights | null
  onProjectClick: (id: string) => void
}

export default function InsightsPanel({ insights, onProjectClick }: InsightsPanelProps) {
  if (!insights) return <p className="text-content-faint italic text-sm py-4 text-center">Loading insights...</p>

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div className="p-4 rounded-lg bg-surface-hover border border-line-subtle">
          <div className="text-[10px] text-content-faint mb-1 uppercase tracking-wider">Total Tokens</div>
          <div className="text-xl font-bold text-cyan-600">{insights.total_tokens.toLocaleString()}</div>
        </div>
        <div className="p-4 rounded-lg bg-surface-hover border border-line-subtle">
          <div className="text-[10px] text-content-faint mb-1 uppercase tracking-wider">Total Cost</div>
          <div className="text-xl font-bold text-emerald-600">${insights.total_cost.toFixed(2)}</div>
        </div>
      </div>
      <div>
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3">Activities per Project</div>
        {insights.project_activities.length > 0 ? (
          <div className="space-y-1.5">
            {insights.project_activities.map((activity) => (
              <button key={activity.project_id} type="button" className="flex items-center justify-between w-full text-left p-3 rounded-lg bg-surface-hover hover:bg-surface-hover border border-transparent hover:border-line cursor-pointer transition-all" onClick={() => onProjectClick(activity.project_id)}>
                <h4 className="text-content-secondary font-medium truncate text-sm">{activity.project_name}</h4>
                <span className="text-xs font-medium text-content-muted tabular-nums">{activity.task_count} {activity.task_count === 1 ? 'task' : 'tasks'}</span>
              </button>
            ))}
          </div>
        ) : <p className="text-content-faint italic text-sm py-4 text-center">No project activities yet</p>}
      </div>
    </div>
  )
}
