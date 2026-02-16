import { Insights } from '../lib/api'

interface InsightsPanelProps {
  insights: Insights | null
  onProjectClick: (id: string) => void
}

export default function InsightsPanel({ insights, onProjectClick }: InsightsPanelProps) {
  if (!insights) return <p className="text-white/20 italic text-sm py-4 text-center">Loading insights...</p>

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div className="p-4 rounded-lg bg-white/[0.02] border border-white/[0.04]">
          <div className="text-[10px] text-white/30 mb-1 uppercase tracking-wider">Total Tokens</div>
          <div className="text-xl font-bold text-cyan-400">{insights.total_tokens.toLocaleString()}</div>
        </div>
        <div className="p-4 rounded-lg bg-white/[0.02] border border-white/[0.04]">
          <div className="text-[10px] text-white/30 mb-1 uppercase tracking-wider">Total Cost</div>
          <div className="text-xl font-bold text-emerald-400">${insights.total_cost.toFixed(2)}</div>
        </div>
      </div>
      <div>
        <div className="text-[11px] font-medium text-white/30 uppercase tracking-widest mb-3">Activities per Project</div>
        {insights.project_activities.length > 0 ? (
          <div className="space-y-1.5">
            {insights.project_activities.map((activity) => (
              <button key={activity.project_id} type="button" className="flex items-center justify-between w-full text-left p-3 rounded-lg bg-white/[0.02] hover:bg-white/[0.05] border border-transparent hover:border-white/[0.06] cursor-pointer transition-all" onClick={() => onProjectClick(activity.project_id)}>
                <h4 className="text-white/70 font-medium truncate text-sm">{activity.project_name}</h4>
                <span className="text-xs font-medium text-white/40 tabular-nums">{activity.task_count} {activity.task_count === 1 ? 'task' : 'tasks'}</span>
              </button>
            ))}
          </div>
        ) : <p className="text-white/20 italic text-sm py-4 text-center">No project activities yet</p>}
      </div>
    </div>
  )
}
