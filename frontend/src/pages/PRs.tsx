import { useEffect, useState } from 'react'
import { usePRStore } from '../stores/prStore'
import { PRStatusBadge } from '../components/StatusBadge'
import PageHeader from '../components/PageHeader'
import EmptyState from '../components/EmptyState'
import LoadingState from '../components/LoadingState'
import MarkdownRenderer from '../components/MarkdownRenderer'
import { timeAgo } from '../lib/utils'

const statusFilters = [
  { value: '', label: 'Actionable', description: 'Open & Changes Requested' },
  { value: 'ALL', label: 'All' },
  { value: 'OPEN', label: 'Open' },
  { value: 'MERGED', label: 'Merged' },
  { value: 'CHANGES_REQUESTED', label: 'Changes Requested' },
  { value: 'CLOSED', label: 'Closed' },
]

function PRListItem({
  pr,
  loading,
  isDescriptionExpanded,
  onToggleDescription: onToggle,
  onApprove,
  onRequestChanges,
  onClose,
}: {
  pr: { id: string; title: string; description: string; pr_status?: string; pr_url?: string; type: string; diff_lines?: number; files_changed?: number; branch_name?: string; model?: string; created_at: string; tags: string[] }
  loading: boolean
  isDescriptionExpanded: boolean
  onToggleDescription: () => void
  onApprove: () => void
  onRequestChanges: () => void
  onClose: () => void
}) {
  return (
    <div className="card p-4">
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          {/* Title + badges row */}
          <div className="flex items-center gap-2 mb-1.5">
            <h3 className="text-base font-medium text-slate-100 truncate">{pr.title}</h3>
            <PRStatusBadge status={pr.pr_status} />
            <span className="badge-secondary text-xs shrink-0">{pr.type}</span>
          </div>

          {/* Description (clamped, expandable) */}
          {pr.description && (
            <div
              className={`mb-2 cursor-pointer hover:bg-slate-700/20 rounded transition-colors ${
                isDescriptionExpanded ? '' : 'max-h-12 overflow-hidden'
              }`}
              onClick={onToggle}
            >
              <MarkdownRenderer content={pr.description} />
              {!isDescriptionExpanded && (
                <div className="text-xs text-slate-500 mt-0.5">Click to expand</div>
              )}
            </div>
          )}

          {/* Meta row */}
          <div className="flex items-center gap-3 text-xs text-slate-500 flex-wrap">
            {pr.diff_lines !== undefined && (
              <span>
                <span className="text-green-400">+</span>
                <span className="text-red-400">-</span>
                {' '}{pr.diff_lines}L / {pr.files_changed}F
              </span>
            )}
            {pr.branch_name && (
              <span className="font-mono text-slate-500 bg-slate-700/50 px-1.5 py-0.5 rounded text-[11px]">
                {pr.branch_name}
              </span>
            )}
            {pr.model && <span className="text-slate-600">{pr.model}</span>}
            {pr.created_at && <span>{timeAgo(pr.created_at)}</span>}
          </div>

          {/* GitHub link + tags row */}
          <div className="flex items-center gap-3 mt-2 flex-wrap">
            {pr.pr_url && (
              <a
                href={pr.pr_url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 text-blue-400 hover:text-blue-300 text-xs"
              >
                <svg className="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 16 16">
                  <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                </svg>
                View on GitHub
              </a>
            )}
            {pr.tags.length > 0 && pr.tags.map((tag, i) => (
              <span key={i} className="badge-info text-[10px]">{tag}</span>
            ))}
          </div>
        </div>

        {/* Action Buttons */}
        <div className="flex flex-col gap-2 shrink-0">
          {pr.pr_status === 'OPEN' && (
            <>
              <button onClick={onApprove} className="btn-success whitespace-nowrap text-sm py-1.5 px-3" disabled={loading}>
                Approve & Merge
              </button>
              <button onClick={onRequestChanges} className="btn-warning whitespace-nowrap text-sm py-1.5 px-3" disabled={loading}>
                Request Changes
              </button>
            </>
          )}
          {pr.pr_status !== 'MERGED' && pr.pr_status !== 'CLOSED' && (
            <button onClick={onClose} className="btn-secondary whitespace-nowrap text-sm py-1.5 px-3" disabled={loading}>
              Close PR
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

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
          <label className="text-xs text-slate-400 mb-1.5 block">Status</label>
          <div className="flex items-center gap-1.5 flex-wrap">
            {statusFilters.map((sf) => (
              <button
                key={sf.value}
                onClick={() => { if (statusFilter !== sf.value) setStatusFilter(sf.value) }}
                className={statusFilter === sf.value ? 'btn-filter-active' : 'btn-filter-inactive'}
                disabled={loading}
              >
                {sf.label}
              </button>
            ))}
          </div>
        </div>

        <div>
          <label className="text-xs text-slate-400 mb-1.5 block">Sort by</label>
          <div className="flex items-center gap-2 flex-wrap">
            <button
              onClick={() => setSortBy('created_at')}
              className={`${sortBy === 'created_at' ? 'btn-filter-active' : 'btn-filter-inactive'} flex items-center gap-1`}
              disabled={loading}
            >
              Created
              {sortBy === 'created_at' && <span className="text-xs">{sortOrder === 'desc' ? '↓' : '↑'}</span>}
            </button>
            <button
              onClick={() => setSortBy('updated_at')}
              className={`${sortBy === 'updated_at' ? 'btn-filter-active' : 'btn-filter-inactive'} flex items-center gap-1`}
              disabled={loading}
            >
              Updated
              {sortBy === 'updated_at' && <span className="text-xs">{sortOrder === 'desc' ? '↓' : '↑'}</span>}
            </button>
          </div>
        </div>
      </div>

      {error && (
        <div className="card p-4 bg-red-900/30 border border-red-600">
          <p className="text-red-200">{error}</p>
        </div>
      )}

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
