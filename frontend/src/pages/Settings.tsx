import { useEffect } from 'react'
import { useDeployStore } from '../stores/deployStore'
import { useWSStore } from '../stores/wsStore'
import PageHeader from '../components/PageHeader'
import LoadingState from '../components/LoadingState'
import ErrorBanner from '../components/ErrorBanner'
import DeploymentModeCard from '../components/DeploymentModeCard'
import DeployStatusCard from '../components/DeployStatusCard'
import SystemInfoCard from '../components/SystemInfoCard'
import { useConfirm } from '../hooks/useConfirm'

export default function Settings() {
  const { status, isLoading, isDeploying, isCheckingRemote, error, fetchStatus, triggerDeploy, checkRemoteCommit } = useDeployStore()
  const wsConnected = useWSStore((s) => s.connected)
  const { confirm, dialog } = useConfirm()

  useEffect(() => {
    fetchStatus()
    const interval = setInterval(fetchStatus, 30000)
    return () => clearInterval(interval)
  }, [fetchStatus])

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

  return (
    <div className="p-5 sm:p-6 lg:p-8 space-y-6 animate-fade-in">
      {dialog}
      <PageHeader
        title="Settings"
        subtitle="System configuration and deployment"
      />

      {error && <ErrorBanner message={error} />}

      {isDeploying && (
        <div className="p-3 bg-accent-500/10 border border-accent-500/20 rounded-lg text-sm text-accent-300" role="status">
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
  )
}
