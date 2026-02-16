import { useEffect } from 'react'
import { useDeployStore } from '../stores/deployStore'
import { useWSStore } from '../stores/wsStore'
import { timeAgo } from '../lib/utils'
import PageHeader from '../components/PageHeader'
import LoadingState from '../components/LoadingState'
import ErrorBanner from '../components/ErrorBanner'

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
      <div className={`w-2.5 h-2.5 rounded-full ${colors[state] || 'bg-slate-500'}`} aria-hidden="true" />
      <span className="text-sm text-slate-300 capitalize">{state}</span>
    </div>
  )
}

export default function Settings() {
  const { status, isLoading, isDeploying, isCheckingRemote, error, fetchStatus, triggerDeploy, checkRemoteCommit } = useDeployStore()
  const wsConnected = useWSStore((s) => s.connected)

  useEffect(() => {
    fetchStatus()
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
  const isAutoMode = updater?.enabled ?? false

  return (
    <div className="p-4 sm:p-6 lg:p-8 space-y-6 lg:space-y-8">
      <PageHeader
        title="Settings"
        subtitle="System configuration and deployment"
      />

      {error && <ErrorBanner message={error} />}

      {isDeploying && (
        <div className="p-3 bg-blue-900/30 border border-blue-600 rounded-lg text-sm text-blue-200" role="status">
          Deploy in progress. Pulling latest code, rebuilding, and restarting...
          The page will reload automatically when the new version is ready.
        </div>
      )}

      {isLoading && !status ? (
        <LoadingState message="Loading deploy status..." />
      ) : status ? (
        <>
          {/* Deployment Mode Card */}
          <section className="card p-4 sm:p-6">
            <h2 className="text-lg font-semibold text-slate-100 mb-4">Deployment Mode</h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              {/* Automatic Mode */}
              <div
                className={`relative rounded-lg border-2 p-4 transition-colors ${
                  isAutoMode
                    ? 'border-blue-500 bg-blue-950/30'
                    : 'border-slate-700 bg-slate-800/50'
                }`}
              >
                <div className="flex items-start gap-3">
                  <div className={`mt-0.5 w-4 h-4 rounded-full border-2 flex items-center justify-center flex-shrink-0 ${
                    isAutoMode ? 'border-blue-500' : 'border-slate-600'
                  }`} aria-hidden="true">
                    {isAutoMode && <div className="w-2 h-2 rounded-full bg-blue-500" />}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="font-medium text-slate-100">Automatic</span>
                      {isAutoMode && (
                        <span className="badge badge-info">Active</span>
                      )}
                    </div>
                    <p className="text-xs text-slate-400">
                      Polls for changes and deploys automatically
                    </p>
                    {isAutoMode && updater && (
                      <div className="mt-3 space-y-1.5 text-xs text-slate-400">
                        <div className="flex items-center justify-between">
                          <span>Branch</span>
                          <span className="font-mono text-slate-300 bg-slate-700/50 px-1.5 py-0.5 rounded">
                            {updater.branch || 'main'}
                          </span>
                        </div>
                        <div className="flex items-center justify-between">
                          <span>Interval</span>
                          <span className="text-slate-300">{updater.check_interval || '-'}</span>
                        </div>
                        <div className="flex items-center justify-between">
                          <span>Status</span>
                          <span className={`font-medium ${updater.running ? 'text-green-400' : 'text-slate-500'}`}>
                            {updater.running ? 'Running' : 'Stopped'}
                          </span>
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              </div>

              {/* Manual Mode */}
              <div
                className={`relative rounded-lg border-2 p-4 transition-colors ${
                  !isAutoMode
                    ? 'border-blue-500 bg-blue-950/30'
                    : 'border-slate-700 bg-slate-800/50'
                }`}
              >
                <div className="flex items-start gap-3">
                  <div className={`mt-0.5 w-4 h-4 rounded-full border-2 flex items-center justify-center flex-shrink-0 ${
                    !isAutoMode ? 'border-blue-500' : 'border-slate-600'
                  }`} aria-hidden="true">
                    {!isAutoMode && <div className="w-2 h-2 rounded-full bg-blue-500" />}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="font-medium text-slate-100">Manual</span>
                      {!isAutoMode && (
                        <span className="badge badge-info">Active</span>
                      )}
                    </div>
                    <p className="text-xs text-slate-400">
                      Deploy on demand via the button below
                    </p>
                    {!isAutoMode && (
                      <div className="mt-3">
                        <button
                          onClick={handleDeploy}
                          disabled={isDeploying || isLoading}
                          className="btn-primary text-sm !py-1.5 !px-3 !min-h-0"
                        >
                          {isDeploying ? 'Deploying...' : 'Deploy Now'}
                        </button>
                      </div>
                    )}
                  </div>
                </div>
              </div>
            </div>
            <p className="text-xs text-slate-500 mt-3">
              Deployment mode is configured in <span className="font-mono">config.yaml</span> under <span className="font-mono">auto_update.enabled</span>.
            </p>
          </section>

          {/* Status & Details Card */}
          <section className="card p-4 sm:p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold text-slate-100">Deploy Status</h2>
              {isAutoMode && (
                <button
                  onClick={handleDeploy}
                  disabled={isDeploying || isLoading}
                  className="btn-secondary text-sm !py-1.5 !px-3 !min-h-0"
                >
                  {isDeploying ? 'Deploying...' : 'Deploy Now'}
                </button>
              )}
            </div>

            <div className="space-y-5">
              {/* Overview */}
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
                    <div className={`w-2.5 h-2.5 rounded-full ${wsConnected ? 'bg-green-500' : 'bg-red-500'}`} aria-hidden="true" />
                    <span className="text-sm text-slate-300">{wsConnected ? 'Connected' : 'Disconnected'}</span>
                  </div>
                </div>
                <div>
                  <div className="text-xs text-slate-500 mb-1">Total Deploys</div>
                  <div className="text-sm text-slate-200">{updater?.update_count ?? 0}</div>
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

              {/* Activity */}
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
                  <div className="p-3 bg-red-900/30 border border-red-600 rounded text-sm text-red-200 font-mono" role="alert">
                    {updater.last_error}
                  </div>
                </div>
              )}
            </div>
          </section>
        </>
      ) : null}

      {/* System Info */}
      <section className="card p-4 sm:p-6">
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
