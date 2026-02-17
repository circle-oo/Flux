import { useEffect, useState } from 'react'
import { useDeployStore } from '../stores/deployStore'
import { useWSStore } from '../stores/wsStore'
import { useSettingsStore, DashboardLayout } from '../stores/settingsStore'
import { api, OrchestratorStatus } from '../lib/api'
import { formatBytes, diskLevelColor } from '../lib/utils'
import PageHeader from '../components/PageHeader'
import LoadingState from '../components/LoadingState'
import ErrorBanner from '../components/ErrorBanner'
import DeploymentModeCard from '../components/DeploymentModeCard'
import DeployStatusCard from '../components/DeployStatusCard'
import SystemInfoCard from '../components/SystemInfoCard'
import { useConfirm } from '../hooks/useConfirm'

type SettingsTab = 'appearance' | 'system' | 'configuration' | 'orchestrator'

export default function Settings() {
  const { status, isLoading, isDeploying, isCheckingRemote, error, fetchStatus, triggerDeploy, checkRemoteCommit } = useDeployStore()
  const wsConnected = useWSStore((s) => s.connected)
  const { confirm, dialog } = useConfirm()
  const { dashboardLayout, setDashboardLayout } = useSettingsStore()
  const [tab, setTab] = useState<SettingsTab>('appearance')
  const [config, setConfig] = useState<Record<string, unknown> | null>(null)
  const [configLoading, setConfigLoading] = useState(false)
  const [configError, setConfigError] = useState<string | null>(null)
  const [orchStatus, setOrchStatus] = useState<OrchestratorStatus | null>(null)
  const [orchLoading, setOrchLoading] = useState(false)
  const [orchError, setOrchError] = useState<string | null>(null)

  useEffect(() => {
    fetchStatus()
    const interval = setInterval(fetchStatus, 30000)
    return () => clearInterval(interval)
  }, [fetchStatus])

  useEffect(() => {
    if (tab === 'configuration' && !config) {
      setConfigLoading(true)
      api.getConfig()
        .then(setConfig)
        .catch((e) => setConfigError(e.message))
        .finally(() => setConfigLoading(false))
    }
  }, [tab, config])

  useEffect(() => {
    if (tab !== 'orchestrator') return
    const load = () => {
      setOrchLoading(true)
      api.getOrchestratorStatus()
        .then(setOrchStatus)
        .catch((e) => setOrchError(e.message))
        .finally(() => setOrchLoading(false))
    }
    load()
    const interval = setInterval(load, 10000)
    return () => clearInterval(interval)
  }, [tab])

  const handleDeploy = async () => {
    const confirmed = await confirm({
      title: 'Deploy now?',
      description: 'This will pull latest changes, rebuild, and restart Flux.',
      confirmLabel: 'Deploy',
      variant: 'danger',
    })
    if (!confirmed) return
    await triggerDeploy()
  }

  const updater = status?.updater
  const isAutoMode = updater?.enabled ?? false

  const tabs: { id: SettingsTab; label: string }[] = [
    { id: 'appearance', label: 'Appearance' },
    { id: 'system', label: 'System' },
    { id: 'configuration', label: 'Configuration' },
    { id: 'orchestrator', label: 'Orchestrator' },
  ]

  return (
    <div className="page-shell space-y-6 animate-fade-in">
      {dialog}
      <PageHeader
        title="Settings"
        subtitle="Preferences, deployment, and configuration"
      />

      {/* Tab Bar */}
      <div className="flex gap-1 p-1 bg-surface-hover rounded-lg w-fit">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={`px-4 py-2 rounded-md text-sm font-medium transition-all ${
              tab === t.id
                ? 'bg-surface text-content shadow-sm'
                : 'text-content-muted hover:text-content-secondary'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {/* Appearance Tab */}
      {tab === 'appearance' && (
        <div className="space-y-6">
          {/* Dashboard Layout */}
          <div className="card p-5">
            <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">Dashboard Layout</div>
            <div className="flex gap-4">
              {([
                { id: 'bento' as DashboardLayout, label: 'Bento Grid', desc: 'Asymmetric layout with hero tiles' },
                { id: 'classic' as DashboardLayout, label: 'Classic', desc: 'Original flat grid layout' },
              ]).map((l) => (
                <button
                  key={l.id}
                  onClick={() => setDashboardLayout(l.id)}
                  className={`flex-1 px-4 py-3 rounded-lg border text-left transition-all ${
                    dashboardLayout === l.id
                      ? 'border-primary-500/40 bg-primary-50 ring-1 ring-primary-200/50'
                      : 'border-line hover:border-line-hover hover:bg-surface-hover'
                  }`}
                >
                  <div className={`text-sm font-medium mb-0.5 ${dashboardLayout === l.id ? 'text-content' : 'text-content-muted'}`}>{l.label}</div>
                  <div className="text-xs text-content-faint">{l.desc}</div>
                </button>
              ))}
            </div>
          </div>

          {/* Keyboard Shortcut Hint */}
          <div className="card p-5">
            <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3">Keyboard Shortcuts</div>
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs text-content-muted">Command Palette</span>
                <kbd className="px-2 py-0.5 text-[11px] font-medium bg-surface-hover border border-line rounded text-content-faint">⌘K</kbd>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* System Tab */}
      {tab === 'system' && (
        <div className="space-y-6">
          {error && <ErrorBanner message={error} />}

          {isDeploying && (
            <div className="p-3 bg-primary-500/10 border border-primary-500/20 rounded-lg text-sm text-primary-600" role="status">
              Deploy in progress. Pulling latest code, rebuilding, and restarting...
            </div>
          )}

          {isLoading && !status ? (
            <LoadingState message="Loading deploy status..." />
          ) : status ? (
            <>
              <DeploymentModeCard
                updater={updater}
                isAutoMode={isAutoMode}
                isDeploying={isDeploying}
                isLoading={isLoading}
                onDeploy={handleDeploy}
              />

              <DeployStatusCard
                status={status}
                wsConnected={wsConnected}
                isAutoMode={isAutoMode}
                isDeploying={isDeploying}
                isLoading={isLoading}
                isCheckingRemote={isCheckingRemote}
                onDeploy={handleDeploy}
                onCheckRemote={checkRemoteCommit}
              />
            </>
          ) : null}

          <SystemInfoCard />
        </div>
      )}

      {/* Configuration Tab */}
      {tab === 'configuration' && (
        <div className="space-y-4">
          {configError && <ErrorBanner message={configError} />}
          {configLoading && <LoadingState message="Loading configuration..." />}
          {config && <ConfigView data={config} />}
        </div>
      )}

      {/* Orchestrator Tab */}
      {tab === 'orchestrator' && (
        <div className="space-y-6">
          {orchError && <ErrorBanner message={orchError} />}
          {orchLoading && !orchStatus && <LoadingState message="Loading orchestrator status..." />}
          {orchStatus && <OrchestratorView status={orchStatus} />}
        </div>
      )}
    </div>
  )
}

function ConfigView({ data }: { data: Record<string, unknown> }) {
  const sections = Object.entries(data).filter(([, v]) => typeof v === 'object' && v !== null && !Array.isArray(v))
  const topLevel = Object.entries(data).filter(([, v]) => typeof v !== 'object' || v === null)

  return (
    <div className="space-y-4">
      {topLevel.length > 0 && (
        <ConfigSection title="General" entries={topLevel} />
      )}
      {sections.map(([key, value]) => (
        <ConfigSection key={key} title={key} entries={Object.entries(value as Record<string, unknown>)} />
      ))}
    </div>
  )
}

function ConfigSection({ title, entries }: { title: string; entries: [string, unknown][] }) {
  return (
    <div className="card p-5">
      <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3">{title}</div>
      <div className="space-y-2">
        {entries.map(([key, value]) => (
          <div key={key} className="flex items-start justify-between gap-4">
            <span className="text-xs text-content-muted font-mono shrink-0">{key}</span>
            <span className="text-xs text-content-secondary font-mono text-right break-all">
              {formatConfigValue(value)}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

function OrchestratorView({ status }: { status: OrchestratorStatus }) {
  return (
    <div className="space-y-6">
      {/* System Health */}
      <div className="card p-5">
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">System Health</div>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
          <div>
            <div className="text-xs text-content-muted mb-1">Status</div>
            <div className={`text-sm font-medium ${status.running ? 'text-emerald-500' : 'text-rose-500'}`}>
              {status.running ? 'Running' : 'Stopped'}
            </div>
          </div>
          <div>
            <div className="text-xs text-content-muted mb-1">Uptime</div>
            <div className="text-sm font-medium text-content">{status.uptime || '--'}</div>
          </div>
          <div>
            <div className="text-xs text-content-muted mb-1">Tick Count</div>
            <div className="text-sm font-medium text-content">{status.tick_count}</div>
          </div>
          <div>
            <div className="text-xs text-content-muted mb-1">Rate Limited</div>
            <div className={`text-sm font-medium ${status.rate_limited ? 'text-rose-500' : 'text-emerald-500'}`}>
              {status.rate_limited ? 'Yes' : 'No'}
            </div>
            {status.rate_limited && status.rate_limit_until && (
              <div className="text-[10px] text-rose-500/70 mt-0.5">
                Until {new Date(status.rate_limit_until).toLocaleTimeString()}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Sub-Components */}
      {status.sub_components && status.sub_components.length > 0 && (
        <div className="card p-5">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">Sub-Components</div>
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
                {status.sub_components.map((c) => (
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
        </div>
      )}

      {/* Scale Status */}
      {status.scale_status && (
        <div className="card p-5">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">Scale Status</div>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
            <div>
              <div className="text-xs text-content-muted mb-1">Executor Pods</div>
              <div className="text-sm font-medium text-content tabular-nums">
                {status.scale_status.executor_pods} <span className="text-content-faint">/ {status.scale_status.max_executor_pods}</span>
              </div>
            </div>
            <div>
              <div className="text-xs text-content-muted mb-1">Triager Pods</div>
              <div className="text-sm font-medium text-content tabular-nums">
                {status.scale_status.triager_pods} <span className="text-content-faint">/ {status.scale_status.max_triager_pods}</span>
              </div>
            </div>
            {status.scale_status.max_researcher_pods > 0 && (
              <div>
                <div className="text-xs text-content-muted mb-1">Researcher Pods</div>
                <div className="text-sm font-medium text-content tabular-nums">
                  {status.scale_status.researcher_pods} <span className="text-content-faint">/ {status.scale_status.max_researcher_pods}</span>
                </div>
              </div>
            )}
            <div>
              <div className="text-xs text-content-muted mb-1">Queue State</div>
              <div className="text-sm font-medium text-content capitalize">{status.scale_status.queue_state || '--'}</div>
            </div>
            <div>
              <div className="text-xs text-content-muted mb-1">Last Scale</div>
              <div className="text-sm font-medium text-content-secondary">
                {status.scale_status.last_scale_time
                  ? new Date(status.scale_status.last_scale_time).toLocaleTimeString()
                  : '--'}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Disk Usage */}
      {status.disk_status && (
        <div className="card p-5">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">Disk Usage</div>
          <div className="grid grid-cols-4 gap-4">
            <div>
              <div className="text-xs text-content-muted mb-1">Available</div>
              <div className="text-sm font-medium text-content">{formatBytes(status.disk_status.available_bytes)}</div>
            </div>
            <div>
              <div className="text-xs text-content-muted mb-1">Used</div>
              <div className="text-sm font-medium text-content">{formatBytes(status.disk_status.used_bytes)}</div>
            </div>
            <div>
              <div className="text-xs text-content-muted mb-1">Total</div>
              <div className="text-sm font-medium text-content">{formatBytes(status.disk_status.total_bytes)}</div>
            </div>
            <div>
              <div className="text-xs text-content-muted mb-1">Level</div>
              <div className={`text-sm font-medium uppercase ${diskLevelColor(status.disk_status.level)}`}>
                {status.disk_status.level}
              </div>
            </div>
          </div>
          {status.disk_status.total_bytes > 0 && (
            <div className="mt-3">
              <div className="w-full bg-surface-hover rounded-full h-2">
                <div
                  className={`h-2 rounded-full transition-all ${
                    status.disk_status.level === 'ok' ? 'bg-emerald-500' :
                    status.disk_status.level === 'warning' ? 'bg-yellow-500' : 'bg-red-500'
                  }`}
                  style={{ width: `${(status.disk_status.used_bytes / status.disk_status.total_bytes * 100).toFixed(1)}%` }}
                />
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function formatConfigValue(value: unknown): string {
  if (value === null || value === undefined) return '—'
  if (value === '***') return '***'
  if (typeof value === 'object') {
    if (Array.isArray(value)) {
      if (value.length === 0) return '[]'
      return value.map((v) => (typeof v === 'object' ? JSON.stringify(v) : String(v))).join(', ')
    }
    return JSON.stringify(value)
  }
  return String(value)
}
