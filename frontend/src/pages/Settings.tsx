import { useEffect, useState } from 'react'
import { useDeployStore } from '../stores/deployStore'
import { useWSStore } from '../stores/wsStore'
import { useSettingsStore, Theme, DashboardLayout } from '../stores/settingsStore'
import { api } from '../lib/api'
import PageHeader from '../components/PageHeader'
import LoadingState from '../components/LoadingState'
import ErrorBanner from '../components/ErrorBanner'
import DeploymentModeCard from '../components/DeploymentModeCard'
import DeployStatusCard from '../components/DeployStatusCard'
import SystemInfoCard from '../components/SystemInfoCard'
import { useConfirm } from '../hooks/useConfirm'

type SettingsTab = 'appearance' | 'system' | 'configuration'

export default function Settings() {
  const { status, isLoading, isDeploying, isCheckingRemote, error, fetchStatus, triggerDeploy, checkRemoteCommit } = useDeployStore()
  const wsConnected = useWSStore((s) => s.connected)
  const { confirm, dialog } = useConfirm()
  const { theme, setTheme, dashboardLayout, setDashboardLayout } = useSettingsStore()
  const [tab, setTab] = useState<SettingsTab>('appearance')
  const [config, setConfig] = useState<Record<string, unknown> | null>(null)
  const [configLoading, setConfigLoading] = useState(false)
  const [configError, setConfigError] = useState<string | null>(null)

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
  ]

  return (
    <div className="p-5 sm:p-6 lg:p-8 space-y-6 animate-fade-in">
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
          {/* Point Color */}
          <div className="card p-5">
            <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">Point Color</div>
            <div className="flex gap-4">
              {([
                { id: 'blue' as Theme, label: 'Light Blue', color: 'bg-[#4B6EF5]', ring: 'ring-[#4B6EF5]' },
                { id: 'green' as Theme, label: 'Sage Green', color: 'bg-[#1DB960]', ring: 'ring-[#1DB960]' },
              ]).map((t) => (
                <button
                  key={t.id}
                  onClick={() => setTheme(t.id)}
                  className={`flex items-center gap-3 px-4 py-3 rounded-lg border transition-all ${
                    theme === t.id
                      ? 'border-primary-500/40 bg-primary-50 ring-1 ring-primary-200/50'
                      : 'border-line hover:border-line-hover hover:bg-surface-hover'
                  }`}
                >
                  <div className={`w-6 h-6 rounded-full ${t.color} ${
                    theme === t.id ? `ring-2 ${t.ring} ring-offset-2 ring-offset-surface` : ''
                  }`} />
                  <span className={`text-sm font-medium ${theme === t.id ? 'text-content' : 'text-content-muted'}`}>{t.label}</span>
                </button>
              ))}
            </div>
          </div>

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
            <div className="p-3 bg-primary-500/10 border border-primary-500/20 rounded-lg text-sm text-primary-300" role="status">
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
