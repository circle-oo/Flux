import { Pod } from '../lib/api'
import { formatUptime } from '../lib/utils'

interface PodCardProps {
  pod: Pod
}

export default function PodCard({ pod }: PodCardProps) {
  return (
    <div
      className={`p-4 rounded-lg border transition-colors ${
        pod.status === 'busy'
          ? 'bg-amber-900/20 border-amber-700/50'
          : 'bg-slate-700/30 border-slate-600/50'
      }`}
    >
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-semibold text-slate-200">{pod.id}</h3>
          <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
            pod.pod_type === 'researcher'
              ? 'bg-purple-600/30 text-purple-300 border border-purple-600/50'
              : 'bg-blue-600/30 text-blue-300 border border-blue-600/50'
          }`}>
            {pod.pod_type || 'executor'}
          </span>
        </div>
        <span
          className={`px-2 py-0.5 rounded text-xs font-medium ${
            pod.status === 'busy'
              ? 'bg-amber-600 text-white'
              : 'bg-slate-600 text-slate-300'
          }`}
        >
          {pod.status}
        </span>
      </div>

      {pod.current_task && pod.task_title ? (
        <div className="mb-2">
          <p className="text-xs text-slate-400 mb-0.5">Current Task:</p>
          <p className="text-xs text-slate-200 truncate" title={pod.task_title}>
            {pod.task_title}
          </p>
        </div>
      ) : (
        <p className="text-xs text-slate-500 italic mb-2">No active task</p>
      )}

      <div className="flex items-center justify-between text-xs text-slate-400">
        <span>Tasks: {pod.task_count}</span>
        <span title={`Started: ${new Date(pod.started_at).toLocaleString()}`}>
          Uptime: {formatUptime(Date.now() - new Date(pod.started_at).getTime())}
        </span>
      </div>
    </div>
  )
}
