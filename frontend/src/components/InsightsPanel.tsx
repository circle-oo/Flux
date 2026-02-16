import { Insights } from '../lib/api'

interface InsightsPanelProps {
  insights: Insights | null
  onProjectClick: (id: string) => void
}

export default function InsightsPanel({ insights, onProjectClick }: InsightsPanelProps) {
  if (!insights) {
    return <p className="text-slate-500 italic text-sm py-4 text-center">Loading insights...</p>
  }

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div className="p-4 rounded-lg bg-slate-700/30">
          <div className="text-xs text-slate-400 mb-1">Total Token Usage</div>
          <div className="text-2xl font-bold text-blue-400">{insights.total_tokens.toLocaleString()}</div>
        </div>
        <div className="p-4 rounded-lg bg-slate-700/30">
          <div className="text-xs text-slate-400 mb-1">Total Cost</div>
          <div className="text-2xl font-bold text-green-400">${insights.total_cost.toFixed(2)}</div>
        </div>
      </div>

      <div>
        <h3 className="text-xs font-medium text-slate-400 uppercase tracking-wider mb-3">
          Activities per Project
        </h3>
        {insights.project_activities.length > 0 ? (
          <div className="space-y-2">
            {insights.project_activities.map((activity) => (
              <button
                key={activity.project_id}
                type="button"
                className="flex items-center justify-between w-full text-left p-3 rounded-lg bg-slate-700/30 hover:bg-slate-700/60 cursor-pointer transition-colors"
                onClick={() => onProjectClick(activity.project_id)}
              >
                <div className="flex-1 min-w-0">
                  <h4 className="text-slate-100 font-medium truncate text-sm">{activity.project_name}</h4>
                </div>
                <div className="text-sm font-semibold text-slate-300">
                  {activity.task_count} {activity.task_count === 1 ? 'task' : 'tasks'}
                </div>
              </button>
            ))}
          </div>
        ) : (
          <p className="text-slate-500 italic text-sm py-4 text-center">No project activities yet</p>
        )}
      </div>
    </div>
  )
}
