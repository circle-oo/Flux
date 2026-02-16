import { Pod } from '../lib/api'
import { formatUptime } from '../lib/utils'

interface PodCardProps {
  pod: Pod
}

export default function PodCard({ pod }: PodCardProps) {
  return (
    <div className={`p-3.5 rounded-lg border transition-all ${
      pod.status === 'busy'
        ? 'bg-amber-50/50 border-amber-500/15'
        : 'bg-surface-hover border-line'
    }`}>
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <h3 className="text-xs font-semibold text-content font-mono">{pod.id}</h3>
          <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
            pod.pod_type === 'triager'
              ? 'bg-violet-50 text-violet-600'
              : 'bg-cyan-50 text-cyan-600'
          }`}>
            {pod.pod_type || 'executor'}
          </span>
        </div>
        <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
          pod.status === 'busy'
            ? 'bg-amber-50 text-amber-600'
            : 'bg-surface-active text-content-muted'
        }`}>
          {pod.status}
        </span>
      </div>
      {pod.current_task && pod.task_title ? (
        <div className="mb-2">
          <p className="text-[10px] text-content-faint mb-0.5">Current Task</p>
          <p className="text-xs text-content-secondary truncate" title={pod.task_title}>{pod.task_title}</p>
        </div>
      ) : (
        <p className="text-[10px] text-content-faint italic mb-2">No active task</p>
      )}
      <div className="flex items-center justify-between text-[10px] text-content-faint">
        <span>Tasks: {pod.task_count}</span>
        <span title={`Started: ${new Date(pod.started_at).toLocaleString()}`}>Uptime: {formatUptime(Date.now() - new Date(pod.started_at).getTime())}</span>
      </div>
    </div>
  )
}
