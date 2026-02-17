import { PRStatusBadge } from './StatusBadge'
import MarkdownRenderer from './MarkdownRenderer'
import { timeAgo } from '../lib/utils'

interface PRListItemProps {
  pr: { id: string; title: string; description: string; pr_status?: string; pr_url?: string; diff_lines?: number; files_changed?: number; branch_name?: string; model?: string; created_at: string; tags: string[] }
  loading: boolean
  isDescriptionExpanded: boolean
  onToggleDescription: () => void
  onApprove: () => void
  onRequestChanges: () => void
  onClose: () => void
}

export default function PRListItem({ pr, loading, isDescriptionExpanded, onToggleDescription: onToggle, onApprove, onRequestChanges, onClose }: PRListItemProps) {
  return (
    <div className="gc p-4">
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-2 flex-wrap">
            <h3 className="text-sm font-semibold text-content truncate">{pr.title}</h3>
            <PRStatusBadge status={pr.pr_status} />
            {pr.model && <span className="badge-info text-[10px]">{pr.model}</span>}
          </div>

          {pr.description && (
            <button
              type="button"
              className={`mb-2 w-full text-left rounded-xl p-2 transition-colors hover:bg-surface-hover ${isDescriptionExpanded ? '' : 'max-h-16 overflow-hidden'}`}
              onClick={onToggle}
              aria-expanded={isDescriptionExpanded}
            >
              <MarkdownRenderer content={pr.description} />
              {!isDescriptionExpanded && <div className="text-[10px] text-content-faint mt-0.5">Click to expand</div>}
            </button>
          )}

          <div className="flex items-center gap-2.5 text-[11px] text-content-faint flex-wrap">
            {pr.diff_lines !== undefined && <span>Δ {pr.diff_lines} lines / {pr.files_changed} files</span>}
            {pr.branch_name && <span className="font-mono text-content-faint bg-surface-hover px-1.5 py-0.5 rounded text-[10px]">{pr.branch_name}</span>}
            <span>{timeAgo(pr.created_at)}</span>
          </div>

          <div className="flex items-center gap-3 mt-2 flex-wrap">
            {pr.pr_url && (
              <a href={pr.pr_url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1.5 text-primary-600 hover:text-primary-500 text-[11px] transition-colors">
                View on GitHub
              </a>
            )}
            {pr.tags.length > 0 && pr.tags.map((tag, i) => <span key={i} className="badge-info">{tag}</span>)}
          </div>
        </div>

        <div className="flex flex-col gap-1.5 shrink-0">
          {pr.pr_status === 'OPEN' && (
            <>
              <button onClick={onApprove} className="btn-sm btn-success whitespace-nowrap" disabled={loading}>Approve & Merge</button>
              <button onClick={onRequestChanges} className="btn-sm btn-warning whitespace-nowrap" disabled={loading}>Request Changes</button>
            </>
          )}
          {pr.pr_status !== 'MERGED' && pr.pr_status !== 'CLOSED' && (
            <button onClick={onClose} className="btn-sm btn-secondary whitespace-nowrap" disabled={loading}>Close PR</button>
          )}
        </div>
      </div>
    </div>
  )
}
