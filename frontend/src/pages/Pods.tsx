import { useEffect, useState } from 'react'
import { api, Pod, OrchestratorStatus } from '../lib/api'
import { useSettingsStore } from '../stores/settingsStore'
import { formatBytes, diskLevelColor } from '../lib/utils'
import PageHeader from '../components/PageHeader'
import PodCard from '../components/PodCard'
import LoadingState from '../components/LoadingState'

export default function Pods() {
  const podRefreshInterval = useSettingsStore((s) => s.podRefreshInterval)
  const [pods, setPods] = useState<Pod[]>([])
  const [orchStatus, setOrchStatus] = useState<OrchestratorStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const load = async () => {
      try {
        const [p, o] = await Promise.all([api.listPods(), api.getOrchestratorStatus()])
        setPods(p)
        setOrchStatus(o)
        setError(null)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load pod data')
      } finally {
        setLoading(false)
      }
    }
    load()
    const interval = setInterval(load, podRefreshInterval * 1000)
    return () => clearInterval(interval)
  }, [podRefreshInterval])

  if (loading) {
    return (
      <div className="p-5 sm:p-6 lg:p-8">
        <LoadingState message="Loading pods..." />
      </div>
    )
  }

  const busyPods = pods.filter((p) => p.status === 'busy').length
  const scaleStatus = orchStatus?.scale_status
  const diskStatus = orchStatus?.disk_status

  return (
    <div className="p-5 sm:p-6 lg:p-8 space-y-6 animate-fade-in">
      <PageHeader
        title="Pods"
        subtitle="Executor and triager pod management"
        action={
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-surface-hover border border-line">
            <div className={`w-2 h-2 rounded-full ${busyPods > 0 ? 'bg-amber-400 animate-pulse' : 'bg-emerald-400'}`} />
            <span className="text-xs text-content-muted">{busyPods}/{pods.length} active</span>
          </div>
        }
      />

      {error && (
        <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-600 text-sm">
          {error}
        </div>
      )}

      {/* Scale Status */}
      {scaleStatus && (
        <div className="card p-5">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">Scale Status</div>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
            <div>
              <div className="text-xs text-content-muted mb-1">Executor Pods</div>
              <div className="text-lg font-bold text-content tabular-nums">
                {scaleStatus.executor_pods} <span className="text-sm font-normal text-content-faint">/ {scaleStatus.max_executor_pods}</span>
              </div>
            </div>
            <div>
              <div className="text-xs text-content-muted mb-1">Triager Pods</div>
              <div className="text-lg font-bold text-content tabular-nums">
                {scaleStatus.triager_pods} <span className="text-sm font-normal text-content-faint">/ {scaleStatus.max_triager_pods}</span>
              </div>
            </div>
            {scaleStatus.max_researcher_pods > 0 && (
              <div>
                <div className="text-xs text-content-muted mb-1">Researcher Pods</div>
                <div className="text-lg font-bold text-content tabular-nums">
                  {scaleStatus.researcher_pods} <span className="text-sm font-normal text-content-faint">/ {scaleStatus.max_researcher_pods}</span>
                </div>
              </div>
            )}
            <div>
              <div className="text-xs text-content-muted mb-1">Queue State</div>
              <div className="text-sm font-medium text-content capitalize">{scaleStatus.queue_state || 'unknown'}</div>
            </div>
            <div>
              <div className="text-xs text-content-muted mb-1">Last Scale</div>
              <div className="text-sm font-medium text-content-secondary">
                {scaleStatus.last_scale_time
                  ? new Date(scaleStatus.last_scale_time).toLocaleTimeString()
                  : '--'}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Pod List */}
      <section className="card p-5">
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">
          Pod List
        </div>
        {pods.length > 0 ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {[...pods].sort((a, b) => a.id.localeCompare(b.id)).map((pod) => (
              <PodCard key={pod.id} pod={pod} />
            ))}
          </div>
        ) : (
          <p className="text-content-faint italic text-sm py-8 text-center">
            No pods active — pods will start when tasks are queued
          </p>
        )}
      </section>

      {/* Orchestrator Health */}
      {orchStatus && (
        <div className="card p-5">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">Orchestrator Health</div>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-4">
            <div>
              <div className="text-xs text-content-muted mb-1">Status</div>
              <div className={`text-sm font-medium ${orchStatus.running ? 'text-emerald-500' : 'text-rose-500'}`}>
                {orchStatus.running ? 'Running' : 'Stopped'}
              </div>
            </div>
            <div>
              <div className="text-xs text-content-muted mb-1">Uptime</div>
              <div className="text-sm font-medium text-content">{orchStatus.uptime || '--'}</div>
            </div>
            <div>
              <div className="text-xs text-content-muted mb-1">Tick Count</div>
              <div className="text-sm font-medium text-content tabular-nums">{orchStatus.tick_count}</div>
            </div>
            <div>
              <div className="text-xs text-content-muted mb-1">Rate Limited</div>
              <div className={`text-sm font-medium ${orchStatus.rate_limited ? 'text-rose-500' : 'text-emerald-500'}`}>
                {orchStatus.rate_limited ? 'Yes' : 'No'}
              </div>
            </div>
          </div>

          {/* Sub-Components Table */}
          {orchStatus.sub_components && orchStatus.sub_components.length > 0 && (
            <>
              <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3 mt-5">Sub-Components</div>
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="text-content-faint border-b border-line">
                      <th className="text-left py-2 pr-4 font-medium">Name</th>
                      <th className="text-left py-2 pr-4 font-medium">Status</th>
                      <th className="text-left py-2 pr-4 font-medium">Last Tick</th>
                      <th className="text-left py-2 font-medium">Last Error</th>
                    </tr>
                  </thead>
                  <tbody>
                    {orchStatus.sub_components.map((c) => (
                      <tr key={c.name} className="border-b border-line/50">
                        <td className="py-2 pr-4 font-mono text-content-secondary">{c.name}</td>
                        <td className="py-2 pr-4">
                          <span className={`inline-block w-2 h-2 rounded-full mr-1.5 ${c.healthy ? 'bg-emerald-400' : 'bg-red-400'}`} />
                          <span className={c.healthy ? 'text-emerald-500' : 'text-rose-500'}>
                            {c.healthy ? 'Healthy' : 'Error'}
                          </span>
                        </td>
                        <td className="py-2 pr-4 text-content-muted">
                          {c.last_tick ? new Date(c.last_tick).toLocaleTimeString() : '--'}
                        </td>
                        <td className="py-2 text-rose-500/80 max-w-[200px] truncate">{c.last_error || '--'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>
      )}

      {/* Disk Usage */}
      {diskStatus && diskStatus.total_bytes > 0 && (
        <div className="card p-5">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">Disk Usage</div>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-3">
            <div>
              <div className="text-xs text-content-muted mb-1">Available</div>
              <div className="text-sm font-medium text-content">{formatBytes(diskStatus.available_bytes)}</div>
            </div>
            <div>
              <div className="text-xs text-content-muted mb-1">Used</div>
              <div className="text-sm font-medium text-content">{formatBytes(diskStatus.used_bytes)}</div>
            </div>
            <div>
              <div className="text-xs text-content-muted mb-1">Total</div>
              <div className="text-sm font-medium text-content">{formatBytes(diskStatus.total_bytes)}</div>
            </div>
            <div>
              <div className="text-xs text-content-muted mb-1">Level</div>
              <div className={`text-sm font-medium uppercase ${diskLevelColor(diskStatus.level)}`}>
                {diskStatus.level}
              </div>
            </div>
          </div>
          <div className="w-full bg-surface-hover rounded-full h-2">
            <div
              className={`h-2 rounded-full transition-all ${
                diskStatus.level === 'ok' ? 'bg-emerald-500' :
                diskStatus.level === 'warning' ? 'bg-yellow-500' : 'bg-red-500'
              }`}
              style={{ width: `${(diskStatus.used_bytes / diskStatus.total_bytes * 100).toFixed(1)}%` }}
            />
          </div>
        </div>
      )}
    </div>
  )
}
