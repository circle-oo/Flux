import { useEffect, useState } from 'react'
import { usePRStore } from '../stores/prStore'
import PageHeader from '../components/PageHeader'
import EmptyState from '../components/EmptyState'
import LoadingState from '../components/LoadingState'
import ErrorBanner from '../components/ErrorBanner'
import PRListItem from '../components/PRListItem'

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

  useEffect(() => {
    fetchPendingPRs()
  }, [fetchPendingPRs])

  const handleApprove = async (taskId: string, title: string) => {
    if (confirm(`Approve and merge PR: ${title}?`)) {
      try { await approvePR(taskId) } catch (error) { console.error('Failed to approve PR:', error) }
    }
  }

  const handleRequestChanges = async (taskId: string, title: string) => {
    if (confirm(`Request changes for PR: ${title}?`)) {
      try { await requestChanges(taskId) } catch (error) { console.error('Failed to request changes:', error) }
    }
  }

  const handleClose = async (taskId: string, title: string) => {
    if (confirm(`Close PR: ${title}?\n\nThis will close the PR on GitHub without merging.`)) {
      try { await closePR(taskId) } catch (error) { console.error('Failed to close PR:', error) }
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
    <div className="p-4 sm:p-6 lg:p-8 space-y-6">
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

      {/* Filter and Sort Controls */}
      <div className="card p-4 space-y-3">
        <div>
          <label className="text-xs text-slate-400 mb-1.5 block" id="pr-status-filter-label">Status</label>
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
          <label className="text-xs text-slate-400 mb-1.5 block" id="pr-sort-label">Sort by</label>
          <div className="flex items-center gap-2 flex-wrap" role="group" aria-labelledby="pr-sort-label">
            <button
              onClick={() => setSortBy('created_at')}
              className={`${sortBy === 'created_at' ? 'btn-filter-active' : 'btn-filter-inactive'} flex items-center gap-1`}
              disabled={loading}
              aria-pressed={sortBy === 'created_at'}
            >
              Created
              {sortBy === 'created_at' && <span className="text-xs">{sortOrder === 'desc' ? '↓' : '↑'}</span>}
            </button>
            <button
              onClick={() => setSortBy('updated_at')}
              className={`${sortBy === 'updated_at' ? 'btn-filter-active' : 'btn-filter-inactive'} flex items-center gap-1`}
              disabled={loading}
              aria-pressed={sortBy === 'updated_at'}
            >
              Updated
              {sortBy === 'updated_at' && <span className="text-xs">{sortOrder === 'desc' ? '↓' : '↑'}</span>}
            </button>
          </div>
        </div>
      </div>

      {error && <ErrorBanner message={error} />}

      {/* PRs List */}
      <div className="space-y-3">
        {loading && pendingPRs.length === 0 ? (
          <LoadingState message="Loading pull requests..." />
        ) : pendingPRs.length === 0 ? (
          <EmptyState icon="&#10003;" title="All caught up!" message="No PRs pending review" />
        ) : (
          <div className="space-y-3">
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
