import type { UpdaterStatus } from '../lib/api'

interface DeploymentModeCardProps { updater: UpdaterStatus | undefined; isAutoMode: boolean; isDeploying: boolean; isLoading: boolean; onDeploy: () => void }

export default function DeploymentModeCard({ updater, isAutoMode, isDeploying, isLoading, onDeploy }: DeploymentModeCardProps) {
  return (
    <section className="card p-5">
      <div className="text-[11px] font-medium text-white/30 uppercase tracking-widest mb-4">Deployment Mode</div>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div className={`relative rounded-lg border-2 p-4 transition-all ${isAutoMode ? 'border-accent-500/40 bg-accent-500/[0.04]' : 'border-white/[0.06] bg-white/[0.02]'}`}>
          <div className="flex items-start gap-3">
            <div className={`mt-0.5 w-4 h-4 rounded-full border-2 flex items-center justify-center flex-shrink-0 ${isAutoMode ? 'border-accent-500' : 'border-white/[0.1]'}`} aria-hidden="true">{isAutoMode && <div className="w-2 h-2 rounded-full bg-accent-500" />}</div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-1"><span className="font-medium text-white/90 text-sm">Automatic</span>{isAutoMode && <span className="badge badge-info">Active</span>}</div>
              <p className="text-[11px] text-white/30">Polls for changes and deploys automatically</p>
              {isAutoMode && updater && (
                <div className="mt-3 space-y-1.5 text-[11px] text-white/30">
                  <div className="flex items-center justify-between"><span>Branch</span><span className="font-mono text-white/50 bg-white/[0.04] px-1.5 py-0.5 rounded text-[10px]">{updater.branch || 'main'}</span></div>
                  <div className="flex items-center justify-between"><span>Interval</span><span className="text-white/50">{updater.check_interval || '-'}</span></div>
                  <div className="flex items-center justify-between"><span>Status</span><span className={`font-medium ${updater.running ? 'text-emerald-400' : 'text-white/25'}`}>{updater.running ? 'Running' : 'Stopped'}</span></div>
                </div>
              )}
            </div>
          </div>
        </div>
        <div className={`relative rounded-lg border-2 p-4 transition-all ${!isAutoMode ? 'border-accent-500/40 bg-accent-500/[0.04]' : 'border-white/[0.06] bg-white/[0.02]'}`}>
          <div className="flex items-start gap-3">
            <div className={`mt-0.5 w-4 h-4 rounded-full border-2 flex items-center justify-center flex-shrink-0 ${!isAutoMode ? 'border-accent-500' : 'border-white/[0.1]'}`} aria-hidden="true">{!isAutoMode && <div className="w-2 h-2 rounded-full bg-accent-500" />}</div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-1"><span className="font-medium text-white/90 text-sm">Manual</span>{!isAutoMode && <span className="badge badge-info">Active</span>}</div>
              <p className="text-[11px] text-white/30">Deploy on demand via the button below</p>
              {!isAutoMode && <div className="mt-3"><button onClick={onDeploy} disabled={isDeploying || isLoading} className="btn-sm btn-primary">{isDeploying ? 'Deploying...' : 'Deploy Now'}</button></div>}
            </div>
          </div>
        </div>
      </div>
      <p className="text-[10px] text-white/15 mt-3">Deployment mode is configured in <span className="font-mono">config.yaml</span> under <span className="font-mono">auto_update.enabled</span>.</p>
    </section>
  )
}
