import { useEffect } from 'react'
import { useDeployStore } from '../stores/deployStore'
import { useWSStore } from '../stores/wsStore'

function timeAgo(iso: string): string {
  const now = Date.now()
  const then = new Date(iso).getTime()
  const seconds = Math.floor((now - then) / 1000)
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 7) return `${days}d ago`
  return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

function StateIndicator({ state }: { state: string }) {
  const colors: Record<string, string> = {
    idle: 'bg-green-500',
    checking: 'bg-blue-500 animate-pulse',
    updating: 'bg-amber-500 animate-pulse',
    restarting: 'bg-purple-500 animate-pulse',
    error: 'bg-red-500',
    disabled: 'bg-slate-500',
  }

  return (
    <div className="flex items-center gap-2">
      <div className={`w-2.5 h-2.5 rounded-full ${colors[state] || 'bg-slate-500'}`} />
      <span className="text-sm text-slate-300 capitalize">{state}</span>
    </div>
  )
}

export default function Settings() {
  const { status, isLoading, isDeploying, isCheckingRemote, error, fetchStatus, triggerDeploy, checkRemoteCommit } = useDeployStore()
  const wsConnected = useWSStore((s) => s.connected)

  useEffect(() => {
    fetchStatus()
    // Poll status every 30 seconds
    const interval = setInterval(fetchStatus, 30000)
    return () => clearInterval(interval)
  }, [fetchStatus])

  const handleDeploy = async () => {
    if (!confirm('Deploy now? This will pull latest changes, rebuild, and restart Flux.')) {
      return
    }
    await triggerDeploy()
  }

  const updater = status?.updater

  return (
    <div className="p-8 space-y-8">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-slate-100 mb-2">Settings</h1>
        <p className="text-slate-400">System configuration and deployment</p>
      </div>

      {/* Deploy Section */}
      <section className="card p-6">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h2 className="text-lg font-semibold text-slate-100 mb-1">Deployment</h2>
            <p className="text-sm text-slate-400">
              Manage auto-deployment and trigger manual deploys
            </p>
          </div>
          <button
            onClick={handleDeploy}
            disabled={isDeploying || isLoading}
            className="btn-primary"
          >
            {isDeploying ? 'Deploying...' : 'Deploy Now'}
          </button>
        </div>

        {error && (
          <div className="mb-4 p-3 bg-red-900/30 border border-red-600 rounded text-sm text-red-200">
            {error}
          </div>
        )}

        {isDeploying && (
          <div className="mb-4 p-3 bg-blue-900/30 border border-blue-600 rounded text-sm text-blue-200">
            Deploy in progress. Pulling latest code, rebuilding, and restarting...
            The page will reload automatically when the new version is ready.
          </div>
        )}

        {isLoading && !status ? (
          <div className="text-slate-400 text-sm">Loading deploy status...</div>
        ) : status ? (
          <div className="space-y-6">
            {/* Status Overview */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div>
                <div className="text-xs text-slate-500 mb-1">Version</div>
                <div className="text-sm font-mono text-slate-200">{status.version}</div>
              </div>
              <div>
                <div className="text-xs text-slate-500 mb-1">State</div>
                <StateIndicator state={updater?.state || 'disabled'} />
              </div>
              <div>
                <div className="text-xs text-slate-500 mb-1">WebSocket</div>
                <div className="flex items-center gap-2">
                  <div className={`w-2.5 h-2.5 rounded-full ${wsConnected ? 'bg-green-500' : 'bg-red-500'}`} />
                  <span className="text-sm text-slate-300">{wsConnected ? 'Connected' : 'Disconnected'}</span>
                </div>
              </div>
              <div>
                <div className="text-xs text-slate-500 mb-1">Deploys</div>
                <div className="text-sm text-slate-200">{updater?.update_count ?? 0}</div>
              </div>
            </div>

            {/* Auto-Deploy Config */}
            <div className="border-t border-slate-700 pt-4">
              <h3 className="text-sm font-medium text-slate-300 mb-3">Auto-Deploy</h3>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div>
                  <div className="text-xs text-slate-500 mb-1">Enabled</div>
                  <span className={`badge ${updater?.enabled ? 'badge-success' : 'badge-secondary'}`}>
                    {updater?.enabled ? 'Yes' : 'No'}
                  </span>
                </div>
                <div>
                  <div className="text-xs text-slate-500 mb-1">Branch</div>
                  <span className="text-sm font-mono text-slate-300 bg-slate-700/50 px-2 py-0.5 rounded">
                    {updater?.branch || 'main'}
                  </span>
                </div>
                <div>
                  <div className="text-xs text-slate-500 mb-1">Check Interval</div>
                  <div className="text-sm text-slate-300">{updater?.check_interval || '-'}</div>
                </div>
                <div>
                  <div className="text-xs text-slate-500 mb-1">Running</div>
                  <span className={`badge ${updater?.running ? 'badge-success' : 'badge-secondary'}`}>
                    {updater?.running ? 'Active' : 'Stopped'}
                  </span>
                </div>
              </div>
            </div>

            {/* Git Info */}
            {updater?.local_commit && (
              <div className="border-t border-slate-700 pt-4">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-sm font-medium text-slate-300">Git</h3>
                  <button
                    onClick={checkRemoteCommit}
                    disabled={isCheckingRemote || isLoading}
                    className="text-xs px-3 py-1 bg-slate-700 hover:bg-slate-600 disabled:bg-slate-800 disabled:text-slate-500 text-slate-300 rounded transition-colors"
                    title="Fetch latest remote commit hash"
                  >
                    {isCheckingRemote ? 'Checking...' : 'Check Remote'}
                  </button>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <div className="text-xs text-slate-500 mb-1">Local Commit</div>
                    <span className="text-sm font-mono text-slate-300 bg-slate-700/50 px-2 py-0.5 rounded">
                      {updater.local_commit.substring(0, 8)}
                    </span>
                  </div>
                  {updater.remote_commit && (
                    <div>
                      <div className="text-xs text-slate-500 mb-1">Remote Commit</div>
                      <span className={`text-sm font-mono px-2 py-0.5 rounded ${
                        updater.local_commit === updater.remote_commit
                          ? 'text-green-400 bg-green-900/30'
                          : 'text-amber-400 bg-amber-900/30'
                      }`}>
                        {updater.remote_commit.substring(0, 8)}
                        {updater.local_commit !== updater.remote_commit && ' (update available)'}
                      </span>
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* Timing Info */}
            <div className="border-t border-slate-700 pt-4">
              <h3 className="text-sm font-medium text-slate-300 mb-3">Activity</h3>
              <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
                <div>
                  <div className="text-xs text-slate-500 mb-1">Last Check</div>
                  <div className="text-sm text-slate-300">
                    {updater?.last_check_at ? timeAgo(updater.last_check_at) : 'Never'}
                  </div>
                </div>
                <div>
                  <div className="text-xs text-slate-500 mb-1">Last Deploy</div>
                  <div className="text-sm text-slate-300">
                    {updater?.last_update_at ? timeAgo(updater.last_update_at) : 'Never'}
                  </div>
                </div>
                <div>
                  <div className="text-xs text-slate-500 mb-1">Next Check</div>
                  <div className="text-sm text-slate-300">
                    {updater?.next_check_at ? timeAgo(updater.next_check_at) : '-'}
                  </div>
                </div>
              </div>
            </div>

            {/* Error Display */}
            {updater?.last_error && (
              <div className="border-t border-slate-700 pt-4">
                <h3 className="text-sm font-medium text-red-400 mb-2">Last Error</h3>
                <div className="p-3 bg-red-900/30 border border-red-600 rounded text-sm text-red-200 font-mono">
                  {updater.last_error}
                </div>
              </div>
            )}
          </div>
        ) : null}
      </section>

      {/* System Info */}
      <section className="card p-6">
        <h2 className="text-lg font-semibold text-slate-100 mb-4">System Info</h2>
        <div className="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
          <div>
            <div className="text-xs text-slate-500 mb-1">Process</div>
            <div className="text-slate-300">launchd (KeepAlive)</div>
          </div>
          <div>
            <div className="text-xs text-slate-500 mb-1">Restart Strategy</div>
            <div className="text-slate-300">SIGTERM + auto-restart</div>
          </div>
          <div>
            <div className="text-xs text-slate-500 mb-1">Build System</div>
            <div className="text-slate-300">make build (Go + Vite)</div>
          </div>
        </div>
      </section>
    </div>
  )
}
