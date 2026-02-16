import type { UpdaterStatus } from '../lib/api'

interface DeploymentModeCardProps {
  updater: UpdaterStatus | undefined
  isAutoMode: boolean
  isDeploying: boolean
  isLoading: boolean
  onDeploy: () => void
}

export default function DeploymentModeCard({ updater, isAutoMode, isDeploying, isLoading, onDeploy }: DeploymentModeCardProps) {
  return (
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
                    onClick={onDeploy}
                    disabled={isDeploying || isLoading}
                    className="btn-sm btn-primary"
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
  )
}
