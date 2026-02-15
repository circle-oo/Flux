import { useEffect, useState } from 'react'
import { usePRStore } from '../stores/prStore'
import MarkdownRenderer from '../components/MarkdownRenderer'

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

const statusFilters = [
  { value: '', label: 'All' },
  { value: 'OPEN', label: 'Open' },
  { value: 'MERGED', label: 'Merged' },
  { value: 'CHANGES_REQUESTED', label: 'Changes Requested' },
  { value: 'CLOSED', label: 'Closed' },
]

export default function PRs() {
  const { pendingPRs, loading, error, statusFilter, setStatusFilter, fetchPendingPRs, approvePR, requestChanges, closePR } =
    usePRStore()
  const [expandedDescriptions, setExpandedDescriptions] = useState<Set<string>>(new Set())

  useEffect(() => {
    fetchPendingPRs()
  }, [fetchPendingPRs])

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
    if (confirm(`Request changes for PR: ${title}?`)) {
      try {
        await requestChanges(taskId)
      } catch (error) {
        console.error('Failed to request changes:', error)
      }
    }
  }

  const handleClose = async (taskId: string, title: string) => {
    if (confirm(`Close PR: ${title}?\n\nThis will close the PR on GitHub without merging.`)) {
      try {
        await closePR(taskId)
      } catch (error) {
        console.error('Failed to close PR:', error)
      }
    }
  }

  const toggleDescription = (id: string) => {
    setExpandedDescriptions((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const getPRStatusBadge = (status?: string) => {
    if (!status) return null

    const statusMap: Record<string, { label: string; className: string }> = {
      OPEN: { label: 'Open', className: 'badge-info' },
      APPROVED: { label: 'Approved', className: 'badge-success' },
      CHANGES_REQUESTED: { label: 'Changes Requested', className: 'badge-warning' },
      MERGED: { label: 'Merged', className: 'badge-secondary' },
      CLOSED: { label: 'Closed', className: 'bg-slate-600 text-slate-200 px-2 py-1 rounded text-xs font-medium' },
    }

    const config = statusMap[status] || { label: status, className: 'badge-secondary' }
    return <span className={`badge ${config.className}`}>{config.label}</span>
  }

  const handleStatusFilter = (value: string) => {
    const newFilter = statusFilter === value ? '' : value
    setStatusFilter(newFilter)
  }

  return (
    <div className="p-4 sm:p-6 lg:p-8 space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 sm:gap-3 mb-2">
            <h1 className="text-2xl sm:text-3xl font-bold text-slate-100">Pull Requests</h1>
            {pendingPRs.length > 0 && (
              <span className="badge badge-info text-sm sm:text-lg px-2 sm:px-3 py-0.5 sm:py-1">
                {pendingPRs.length}
              </span>
            )}
          </div>
          <p className="text-sm sm:text-base text-slate-400">Review and manage pull requests</p>
        </div>
        <button
          onClick={() => fetchPendingPRs()}
          className="btn-secondary"
          disabled={loading}
        >
          {loading ? 'Refreshing...' : 'Refresh'}
        </button>
      </div>

      {/* Filter buttons */}
      <div className="card p-4">
        <div className="flex items-center gap-1.5 flex-wrap">
          {statusFilters.map((sf) => (
            <button
              key={sf.value}
              onClick={() => handleStatusFilter(sf.value)}
              className={
                statusFilter === sf.value
                  ? 'btn-filter-active'
                  : 'btn-filter-inactive'
              }
              disabled={loading}
            >
              {sf.label}
            </button>
          ))}
        </div>
      </div>

      {/* Error Message */}
      {error && (
        <div className="card p-4 bg-red-900/30 border border-red-600">
          <p className="text-red-200">{error}</p>
        </div>
      )}

      {/* PRs List */}
      <div className="space-y-3">
        {loading && pendingPRs.length === 0 ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-slate-400">Loading pull requests...</div>
          </div>
        ) : pendingPRs.length === 0 ? (
          <div className="card p-12 text-center">
            <div className="text-6xl mb-4">&#10003;</div>
            <h2 className="text-xl font-semibold text-slate-300 mb-2">
              All caught up!
            </h2>
            <p className="text-slate-400">No PRs pending review</p>
          </div>
        ) : (
          <div className="space-y-3">
            {pendingPRs.map((pr) => (
              <div key={pr.id} className="card p-4">
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1 min-w-0">
                    {/* Title + badges row */}
                    <div className="flex items-center gap-2 mb-1.5">
                      <h3 className="text-base font-medium text-slate-100 truncate">
                        {pr.title}
                      </h3>
                      {getPRStatusBadge(pr.pr_status)}
                      <span className="badge-secondary text-xs shrink-0">{pr.type}</span>
                    </div>

                    {/* Description (clamped, expandable) */}
                    {pr.description && (
                      <div
                        className={`mb-2 cursor-pointer hover:bg-slate-700/20 rounded transition-colors ${
                          expandedDescriptions.has(pr.id) ? '' : 'max-h-12 overflow-hidden'
                        }`}
                        onClick={() => toggleDescription(pr.id)}
                      >
                        <MarkdownRenderer content={pr.description} />
                        {!expandedDescriptions.has(pr.id) && (
                          <div className="text-xs text-slate-500 mt-0.5">Click to expand</div>
                        )}
                      </div>
                    )}

                    {/* Meta row: stats + branch + model + time */}
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
                      {pr.model && (
                        <span className="text-slate-600">{pr.model}</span>
                      )}
                      {pr.created_at && (
                        <span>{timeAgo(pr.created_at)}</span>
                      )}
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
                          <svg
                            className="w-3.5 h-3.5"
                            fill="currentColor"
                            viewBox="0 0 16 16"
                          >
                            <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                          </svg>
                          View on GitHub
                        </a>
                      )}
                      {pr.tags.length > 0 &&
                        pr.tags.map((tag, i) => (
                          <span key={i} className="badge-info text-[10px]">
                            {tag}
                          </span>
                        ))}
                    </div>
                  </div>

                  {/* Action Buttons */}
                  <div className="flex flex-col gap-2 shrink-0">
                    {pr.pr_status === 'OPEN' && (
                      <>
                        <button
                          onClick={() => handleApprove(pr.id, pr.title)}
                          className="btn-success whitespace-nowrap text-sm py-1.5 px-3"
                          disabled={loading}
                        >
                          Approve & Merge
                        </button>
                        <button
                          onClick={() => handleRequestChanges(pr.id, pr.title)}
                          className="btn-warning whitespace-nowrap text-sm py-1.5 px-3"
                          disabled={loading}
                        >
                          Request Changes
                        </button>
                      </>
                    )}
                    {pr.pr_status !== 'MERGED' && pr.pr_status !== 'CLOSED' && (
                      <button
                        onClick={() => handleClose(pr.id, pr.title)}
                        className="btn-secondary whitespace-nowrap text-sm py-1.5 px-3"
                        disabled={loading}
                      >
                        Close PR
                      </button>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
