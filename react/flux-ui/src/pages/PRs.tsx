import { useEffect } from 'react'
import { usePRStore } from '../stores/prStore'

const STATUS_TABS = [
  { value: 'ALL' as const, label: 'All' },
  { value: 'OPEN' as const, label: 'Open' },
  { value: 'APPROVED' as const, label: 'Approved' },
  { value: 'CHANGES_REQUESTED' as const, label: 'Changes Requested' },
  { value: 'MERGED' as const, label: 'Merged' },
  { value: 'CLOSED' as const, label: 'Closed' },
]

export default function PRs() {
  const {
    prs,
    loading,
    error,
    statusFilter,
    setStatusFilter,
    fetchPRs,
    approvePR,
    requestChanges,
    closePR,
    resolveConflicts,
  } = usePRStore()

  useEffect(() => {
    fetchPRs()
  }, [fetchPRs])

  const handleApprove = async (taskId: string, title: string) => {
    if (confirm(`Approve and merge PR: ${title}?`)) {
      try {
        await approvePR(taskId)
      } catch (error) {
        console.error('Failed to approve PR:', error)
      }
    }
  }

  const handleRequestChanges = async (taskId: string, title: string) => {
    if (confirm(`Request changes for PR: ${title}?\nThis will create a fix task from PR comments.`)) {
      try {
        await requestChanges(taskId)
      } catch (error) {
        console.error('Failed to request changes:', error)
      }
    }
  }

  const handleClose = async (taskId: string, title: string) => {
    if (confirm(`Close PR without merging: ${title}?\nThis action can be undone by reopening the PR on GitHub.`)) {
      try {
        await closePR(taskId)
      } catch (error) {
        console.error('Failed to close PR:', error)
      }
    }
  }

  const handleResolveConflicts = async (taskId: string, title: string) => {
    if (confirm(`Create a conflict resolution task for: ${title}?\nThis will create a new task to resolve merge conflicts on the PR branch.`)) {
      try {
        await resolveConflicts(taskId)
      } catch (error) {
        console.error('Failed to create conflict resolution task:', error)
      }
    }
  }

  const getPRStatusBadge = (status?: string) => {
    if (!status) return null

    const statusMap: Record<string, { label: string; className: string }> = {
      OPEN: { label: 'Open', className: 'badge-info' },
      APPROVED: { label: 'Approved', className: 'badge-success' },
      CHANGES_REQUESTED: { label: 'Changes Requested', className: 'badge-warning' },
      MERGED: { label: 'Merged', className: 'badge-success' },
      CLOSED: { label: 'Closed', className: 'badge-secondary' },
    }

    const config = statusMap[status] || { label: status, className: 'badge-secondary' }
    return <span className={`badge ${config.className}`}>{config.label}</span>
  }

  const formatDate = (dateString: string) => {
    const date = new Date(dateString)
    return date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  const isActionable = (status?: string) => status === 'OPEN'
  const isCloseable = (status?: string) => status === 'OPEN' || status === 'CHANGES_REQUESTED'
  const isTerminal = (status?: string) => status === 'MERGED' || status === 'CLOSED'

  return (
    <div className="p-8 space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-3 mb-2">
            <h1 className="text-3xl font-bold text-slate-100">Pull Requests</h1>
            {prs.length > 0 && (
              <span className="badge badge-info text-lg px-3 py-1">
                {prs.length}
              </span>
            )}
          </div>
          <p className="text-slate-400">Review, merge, and manage pull requests</p>
        </div>
        <button
          onClick={() => fetchPRs()}
          className="btn-secondary"
          disabled={loading}
        >
          {loading ? 'Refreshing...' : 'Refresh'}
        </button>
      </div>

      {/* Status Filter Tabs */}
      <div className="flex gap-1 p-1 bg-slate-800 rounded-lg border border-slate-700 overflow-x-auto">
        {STATUS_TABS.map((tab) => (
          <button
            key={tab.value}
            onClick={() => setStatusFilter(tab.value)}
            className={`px-4 py-2 text-sm font-medium rounded-md transition-colors whitespace-nowrap ${
              statusFilter === tab.value
                ? 'bg-blue-600 text-white'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-700'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Error Message */}
      {error && (
        <div className="card p-4 bg-red-900/30 border border-red-600">
          <p className="text-red-200">{error}</p>
        </div>
      )}

      {/* PRs List */}
      <div className="space-y-4">
        {loading && prs.length === 0 ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-slate-400">Loading pull requests...</div>
          </div>
        ) : prs.length === 0 ? (
          <div className="card p-12 text-center">
            <div className="text-6xl mb-4">
              {statusFilter === 'ALL' ? '✓' : '📭'}
            </div>
            <h2 className="text-xl font-semibold text-slate-300 mb-2">
              {statusFilter === 'ALL'
                ? 'No pull requests'
                : `No ${statusFilter.toLowerCase().replace('_', ' ')} PRs`}
            </h2>
            <p className="text-slate-400">
              {statusFilter === 'ALL'
                ? 'No PRs have been created yet'
                : 'Try a different filter to see more PRs'}
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            {prs.map((pr) => (
              <div
                key={pr.id}
                className={`card p-6 ${isTerminal(pr.pr_status) ? 'opacity-75' : ''}`}
              >
                <div className="flex items-start justify-between mb-4">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-2">
                      <h3 className="text-lg font-medium text-slate-100">
                        {pr.title}
                      </h3>
                      {getPRStatusBadge(pr.pr_status)}
                      <span className="badge-secondary text-xs">{pr.type}</span>
                    </div>
                    <p className="text-slate-300 mb-3">{pr.description}</p>

                    {/* Diff Stats */}
                    <div className="flex items-center gap-4 text-sm text-slate-400 mb-3">
                      {pr.diff_lines !== undefined && (
                        <div className="flex items-center gap-1">
                          <span className="text-green-400">+</span>
                          <span className="text-red-400">-</span>
                          <span>{pr.diff_lines} lines</span>
                        </div>
                      )}
                      {pr.files_changed !== undefined && (
                        <div>{pr.files_changed} files changed</div>
                      )}
                      {pr.created_at && (
                        <div>Created {formatDate(pr.created_at)}</div>
                      )}
                    </div>

                    {/* GitHub PR Link */}
                    {pr.pr_url && (
                      <a
                        href={pr.pr_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-2 text-blue-400 hover:text-blue-300 text-sm"
                      >
                        <svg
                          className="w-4 h-4"
                          fill="currentColor"
                          viewBox="0 0 16 16"
                        >
                          <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                        </svg>
                        View on GitHub
                        <svg
                          className="w-3 h-3"
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                          />
                        </svg>
                      </a>
                    )}

                    {/* Tags */}
                    {pr.tags.length > 0 && (
                      <div className="flex flex-wrap gap-2 mt-3">
                        {pr.tags.map((tag, i) => (
                          <span key={i} className="badge-info text-xs">
                            {tag}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>

                  {/* Action Buttons */}
                  <div className="flex flex-col gap-2 ml-4">
                    {isActionable(pr.pr_status) && (
                      <>
                        <button
                          onClick={() => handleApprove(pr.id, pr.title)}
                          className="btn-success whitespace-nowrap"
                          disabled={loading}
                        >
                          ✓ Approve & Merge
                        </button>
                        <button
                          onClick={() => handleRequestChanges(pr.id, pr.title)}
                          className="btn-warning whitespace-nowrap"
                          disabled={loading}
                        >
                          Request Changes
                        </button>
                        <button
                          onClick={() => handleResolveConflicts(pr.id, pr.title)}
                          className="btn-secondary whitespace-nowrap"
                          disabled={loading}
                        >
                          🔧 Resolve Conflicts
                        </button>
                      </>
                    )}
                    {isCloseable(pr.pr_status) && (
                      <button
                        onClick={() => handleClose(pr.id, pr.title)}
                        className="btn-danger whitespace-nowrap"
                        disabled={loading}
                      >
                        Close PR
                      </button>
                    )}
                  </div>
                </div>

                {/* Auto-merge Indicator */}
                {pr.requires_test === false && pr.pr_status === 'OPEN' && (
                  <div className="mt-3 p-2 bg-blue-900/30 border border-blue-600 rounded text-sm text-blue-200 flex items-center gap-2">
                    <svg
                      className="w-4 h-4"
                      fill="currentColor"
                      viewBox="0 0 20 20"
                    >
                      <path
                        fillRule="evenodd"
                        d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                        clipRule="evenodd"
                      />
                    </svg>
                    Eligible for auto-merge
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
