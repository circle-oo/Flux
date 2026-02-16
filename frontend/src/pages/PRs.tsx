import { useEffect, useState } from 'react'
import { usePRStore } from '../stores/prStore'
import PageHeader from '../components/PageHeader'
import EmptyState from '../components/EmptyState'
import LoadingState from '../components/LoadingState'
import ErrorBanner from '../components/ErrorBanner'
import PRListItem from '../components/PRListItem'
import { useConfirm } from '../hooks/useConfirm'
import { useToast } from '../components/Toast'

const statusFilters = [
  { value: '', label: 'Actionable', description: 'Open & Changes Requested' },
  { value: 'ALL', label: 'All' },
  { value: 'OPEN', label: 'Open' },
  { value: 'MERGED', label: 'Merged' },
  { value: 'CHANGES_REQUESTED', label: 'Changes Requested' },
  { value: 'CLOSED', label: 'Closed' },
]

export default function PRs() {
  const {
    pendingPRs,
    loading,
    error,
    statusFilter,
    sortBy,
    sortOrder,
    setStatusFilter,
    setSortBy,
    fetchPendingPRs,
    approvePR,
    requestChanges,
    closePR,
  } = usePRStore()
  const [expandedDescriptions, setExpandedDescriptions] = useState<Set<string>>(new Set())
  const { confirm, dialog } = useConfirm()
  const { toast } = useToast()

  useEffect(() => {
    fetchPendingPRs()
  }, [fetchPendingPRs])

  const handleApprove = async (taskId: string, title: string) => {
    const confirmed = await confirm({ title: 'Approve and merge?', description: title, confirmLabel: 'Approve & Merge' })
    if (confirmed) {
      try { await approvePR(taskId); toast('PR approved and merged', 'success') } catch (error) { toast(`Failed to approve PR: ${error}`, 'error') }
    }
  }

  const handleRequestChanges = async (taskId: string, title: string) => {
    const confirmed = await confirm({ title: 'Request changes?', description: title, confirmLabel: 'Request Changes', variant: 'danger' })
    if (confirmed) {
      try { await requestChanges(taskId); toast('Changes requested', 'success') } catch (error) { toast(`Failed to request changes: ${error}`, 'error') }
    }
  }

  const handleClose = async (taskId: string, title: string) => {
    const confirmed = await confirm({ title: 'Close PR?', description: `${title}\n\nThis will close the PR on GitHub without merging.`, confirmLabel: 'Close PR', variant: 'danger' })
    if (confirmed) {
      try { await closePR(taskId); toast('PR closed', 'success') } catch (error) { toast(`Failed to close PR: ${error}`, 'error') }
    }
  }

  const toggleDescription = (id: string) => {
    setExpandedDescriptions((prev) => {
      const next = new Set(prev)
      if (next.has(id)) { next.delete(id) } else { next.add(id) }
      return next
    })
  }

  return (
    <div className="p-5 sm:p-6 lg:p-8 space-y-5 animate-fade-in">
      {dialog}
      <PageHeader
        title="Pull Requests"
        subtitle="Review and manage pull requests"
        count={pendingPRs.length}
        action={
          <button onClick={() => fetchPendingPRs()} className="btn-secondary" disabled={loading}>
            {loading ? 'Refreshing...' : 'Refresh'}
          </button>
        }
      />

      {/* Filters */}
      <div className="card p-4 space-y-3">
        <div>
          <label className="text-[11px] text-content-faint mb-1.5 block uppercase tracking-widest font-medium" id="pr-status-filter-label">Status</label>
          <div className="flex items-center gap-1.5 flex-wrap" role="group" aria-labelledby="pr-status-filter-label">
            {statusFilters.map((sf) => (
              <button
                key={sf.value}
                onClick={() => { if (statusFilter !== sf.value) setStatusFilter(sf.value) }}
                className={statusFilter === sf.value ? 'btn-filter-active' : 'btn-filter-inactive'}
                disabled={loading}
                aria-pressed={statusFilter === sf.value}
              >
                {sf.label}
              </button>
            ))}
          </div>
        </div>

        <div>
          <label className="text-[11px] text-content-faint mb-1.5 block uppercase tracking-widest font-medium" id="pr-sort-label">Sort by</label>
          <div className="flex items-center gap-2 flex-wrap" role="group" aria-labelledby="pr-sort-label">
            <button
              onClick={() => setSortBy('created_at')}
              className={`${sortBy === 'created_at' ? 'btn-filter-active' : 'btn-filter-inactive'} flex items-center gap-1`}
              disabled={loading}
              aria-pressed={sortBy === 'created_at'}
            >
              Created
              {sortBy === 'created_at' && <span className="text-[10px]">{sortOrder === 'desc' ? '\u2193' : '\u2191'}</span>}
            </button>
            <button
              onClick={() => setSortBy('updated_at')}
              className={`${sortBy === 'updated_at' ? 'btn-filter-active' : 'btn-filter-inactive'} flex items-center gap-1`}
              disabled={loading}
              aria-pressed={sortBy === 'updated_at'}
            >
              Updated
              {sortBy === 'updated_at' && <span className="text-[10px]">{sortOrder === 'desc' ? '\u2193' : '\u2191'}</span>}
            </button>
          </div>
        </div>
      </div>

      {error && <ErrorBanner message={error} />}

      <div className="space-y-2.5">
        {loading && pendingPRs.length === 0 ? (
          <LoadingState message="Loading pull requests..." />
        ) : pendingPRs.length === 0 ? (
          <EmptyState icon="&#10003;" title="All caught up!" message="No PRs pending review" />
        ) : (
          <div className="space-y-2.5">
            {pendingPRs.map((pr) => (
              <PRListItem
                key={pr.id}
                pr={pr}
                loading={loading}
                isDescriptionExpanded={expandedDescriptions.has(pr.id)}
                onToggleDescription={() => toggleDescription(pr.id)}
                onApprove={() => handleApprove(pr.id, pr.title)}
                onRequestChanges={() => handleRequestChanges(pr.id, pr.title)}
                onClose={() => handleClose(pr.id, pr.title)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
