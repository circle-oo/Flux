import type { DeployStatusResponse } from '../lib/api'
import { timeAgo } from '../lib/utils'

function StateIndicator({ state }: { state: string }) {
  const colors: Record<string, string> = { idle: 'bg-emerald-400', checking: 'bg-cyan-400 animate-pulse', updating: 'bg-amber-400 animate-pulse', restarting: 'bg-violet-400 animate-pulse', error: 'bg-rose-400', disabled: 'bg-surface-active' }
  return <div className="flex items-center gap-2"><div className={`w-2 h-2 rounded-full ${colors[state] || 'bg-surface-active'}`} aria-hidden="true" /><span className="text-xs text-content-secondary capitalize">{state}</span></div>
}

interface DeployStatusCardProps { status: DeployStatusResponse; wsConnected: boolean; isAutoMode: boolean; isDeploying: boolean; isLoading: boolean; isCheckingRemote: boolean; onDeploy: () => void; onCheckRemote: () => void }

export default function DeployStatusCard({ status, wsConnected, isAutoMode, isDeploying, isLoading, isCheckingRemote, onDeploy, onCheckRemote }: DeployStatusCardProps) {
  const updater = status.updater
  return (
    <section className="card p-5">
      <div className="flex items-center justify-between mb-4">
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest">Deploy Status</div>
        {isAutoMode && <button onClick={onDeploy} disabled={isDeploying || isLoading} className="btn-sm btn-secondary">{isDeploying ? 'Deploying...' : 'Deploy Now'}</button>}
      </div>
      <div className="space-y-5">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div><div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">Version</div><div className="text-sm font-mono text-content-secondary">{status.version}</div></div>
          <div><div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">State</div><StateIndicator state={updater?.state || 'disabled'} /></div>
          <div><div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">WebSocket</div><div className="flex items-center gap-2"><div className={`w-2 h-2 rounded-full ${wsConnected ? 'bg-emerald-400' : 'bg-rose-400'}`} aria-hidden="true" /><span className="text-xs text-content-secondary">{wsConnected ? 'Connected' : 'Disconnected'}</span></div></div>
          <div><div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">Total Deploys</div><div className="text-sm text-content-secondary">{updater?.update_count ?? 0}</div></div>
        </div>
        {updater?.local_commit && (
          <div className="border-t border-line-subtle pt-4">
            <div className="flex items-center justify-between mb-3"><h3 className="text-xs font-medium text-content-muted">Git</h3><button onClick={onCheckRemote} disabled={isCheckingRemote || isLoading} className="text-[10px] px-2.5 py-1 bg-surface-hover hover:bg-surface-active disabled:bg-surface-hover disabled:text-content-faint text-content-muted rounded transition-colors" title="Fetch latest remote commit">{isCheckingRemote ? 'Checking...' : 'Check Remote'}</button></div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div><div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">Local Commit</div><span className="text-xs font-mono text-content-muted bg-surface-hover px-2 py-0.5 rounded">{updater.local_commit.substring(0, 8)}</span></div>
              {updater.remote_commit && <div><div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">Remote Commit</div><span className={`text-xs font-mono px-2 py-0.5 rounded ${updater.local_commit === updater.remote_commit ? 'text-emerald-600 bg-emerald-500/10' : 'text-amber-600 bg-amber-500/10'}`}>{updater.remote_commit.substring(0, 8)}{updater.local_commit !== updater.remote_commit && ' (update available)'}</span></div>}
            </div>
          </div>
        )}
        <div className="border-t border-line-subtle pt-4">
          <h3 className="text-xs font-medium text-content-muted mb-3">Activity</h3>
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
            <div><div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">Last Check</div><div className="text-xs text-content-muted">{updater?.last_check_at ? timeAgo(updater.last_check_at) : 'Never'}</div></div>
            <div><div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">Last Deploy</div><div className="text-xs text-content-muted">{updater?.last_update_at ? timeAgo(updater.last_update_at) : 'Never'}</div></div>
            <div><div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">Next Check</div><div className="text-xs text-content-muted">{updater?.next_check_at ? timeAgo(updater.next_check_at) : '-'}</div></div>
          </div>
        </div>
        {updater?.last_error && (
          <div className="border-t border-line-subtle pt-4">
            <h3 className="text-xs font-medium text-rose-600 mb-2">Last Error</h3>
            <div className="p-3 bg-rose-50/50 border border-rose-500/15 rounded-lg text-xs text-rose-600 font-mono" role="alert">{updater.last_error}</div>
          </div>
        )}
      </div>
    </section>
  )
}
