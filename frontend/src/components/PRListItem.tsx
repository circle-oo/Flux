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
    <div className="card p-4">
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1.5">
            <h3 className="text-sm font-medium text-content truncate">{pr.title}</h3>
            <PRStatusBadge status={pr.pr_status} />
          </div>
          {pr.description && (
            <button type="button" className={`mb-2 w-full text-left hover:bg-surface-hover rounded-lg p-1 -ml-1 transition-colors ${isDescriptionExpanded ? '' : 'max-h-12 overflow-hidden'}`} onClick={onToggle} aria-expanded={isDescriptionExpanded} aria-label={isDescriptionExpanded ? 'Collapse description' : 'Expand description'}>
              <MarkdownRenderer content={pr.description} />
              {!isDescriptionExpanded && <div className="text-[10px] text-content-faint mt-0.5">Click to expand</div>}
            </button>
          )}
          <div className="flex items-center gap-2.5 text-[11px] text-content-faint flex-wrap">
            {pr.diff_lines !== undefined && <span><span className="text-emerald-600">+</span><span className="text-rose-600">-</span> {pr.diff_lines}L / {pr.files_changed}F</span>}
            {pr.branch_name && <span className="font-mono text-content-faint bg-surface-hover px-1.5 py-0.5 rounded text-[10px]">{pr.branch_name}</span>}
            {pr.model && <span className="text-content-faint">{pr.model}</span>}
            {pr.created_at && <span>{timeAgo(pr.created_at)}</span>}
          </div>
          <div className="flex items-center gap-3 mt-2 flex-wrap">
            {pr.pr_url && (
              <a href={pr.pr_url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1.5 text-primary-400 hover:text-primary-300 text-[11px] transition-colors">
                <svg className="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 16 16" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" /></svg>
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
          {pr.pr_status !== 'MERGED' && pr.pr_status !== 'CLOSED' && <button onClick={onClose} className="btn-sm btn-secondary whitespace-nowrap" disabled={loading}>Close PR</button>}
        </div>
      </div>
    </div>
  )
}
