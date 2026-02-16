import { Pod } from '../lib/api'
import { formatUptime } from '../lib/utils'

interface PodCardProps {
  pod: Pod
}

export default function PodCard({ pod }: PodCardProps) {
  return (
    <div className={`p-3.5 rounded-lg border transition-all ${
      pod.status === 'busy'
        ? 'bg-amber-500/[0.04] border-amber-500/15'
        : 'bg-white/[0.02] border-white/[0.06]'
    }`}>
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <h3 className="text-xs font-semibold text-white/80 font-mono">{pod.id}</h3>
          <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
            pod.pod_type === 'researcher'
              ? 'bg-violet-500/15 text-violet-400'
              : 'bg-cyan-500/15 text-cyan-400'
          }`}>
            {pod.pod_type || 'executor'}
          </span>
        </div>
        <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
          pod.status === 'busy'
            ? 'bg-amber-500/15 text-amber-400'
            : 'bg-white/[0.06] text-white/40'
        }`}>
          {pod.status}
        </span>
      </div>
      {pod.current_task && pod.task_title ? (
        <div className="mb-2">
          <p className="text-[10px] text-white/30 mb-0.5">Current Task</p>
          <p className="text-xs text-white/70 truncate" title={pod.task_title}>{pod.task_title}</p>
        </div>
      ) : (
        <p className="text-[10px] text-white/20 italic mb-2">No active task</p>
      )}
      <div className="flex items-center justify-between text-[10px] text-white/25">
        <span>Tasks: {pod.task_count}</span>
        <span title={`Started: ${new Date(pod.started_at).toLocaleString()}`}>Uptime: {formatUptime(Date.now() - new Date(pod.started_at).getTime())}</span>
      </div>
    </div>
  )
}
