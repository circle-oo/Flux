// Shared utility functions

/**
 * Formats an ISO timestamp as a human-readable relative time string.
 * Examples: "just now", "5m ago", "2h ago", "3d ago", "Jan 15"
 */
export function timeAgo(iso: string): string {
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

/**
 * Formats the duration between two ISO timestamps.
 * Examples: "45s", "12m", "2h 15m"
 */
export function duration(startIso: string, endIso: string): string {
  const ms = new Date(endIso).getTime() - new Date(startIso).getTime()
  const seconds = Math.floor(ms / 1000)
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  const remainMins = minutes % 60
  return remainMins > 0 ? `${hours}h ${remainMins}m` : `${hours}h`
}

/**
 * Formats an ISO timestamp as a full locale date string.
 * Returns "—" for missing values.
 */
export function formatDate(iso?: string): string {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

/**
 * Formats a duration in milliseconds as a human-readable uptime string.
 * Examples: "45m", "2h 15m", "3d 5h"
 */
export function formatUptime(ms: number): string {
  const totalMinutes = Math.floor(ms / 1000 / 60)
  if (totalMinutes < 60) return `${totalMinutes}m`
  const hours = Math.floor(totalMinutes / 60)
  const minutes = totalMinutes % 60
  if (hours < 24) return `${hours}h ${minutes}m`
  const days = Math.floor(hours / 24)
  const remainingHours = hours % 24
  return `${days}d ${remainingHours}h`
}

/**
 * Formats a duration between two ISO timestamps, handling optional/in-progress values.
 * If end is missing, uses Date.now().
 */
export function formatDuration(start?: string, end?: string): string {
  if (!start) return '—'
  const s = new Date(start).getTime()
  const e = end ? new Date(end).getTime() : Date.now()
  const diff = e - s
  if (diff < 0) return '—'
  const secs = Math.floor(diff / 1000)
  if (secs < 60) return `${secs}s`
  const mins = Math.floor(secs / 60)
  const remSecs = secs % 60
  if (mins < 60) return `${mins}m ${remSecs}s`
  const hours = Math.floor(mins / 60)
  const remMins = mins % 60
  return `${hours}h ${remMins}m`
}

/**
 * Formats a cost value as a dollar string.
 */
export function formatCost(cost?: number): string {
  if (cost === undefined || cost === null || cost === 0) return '—'
  return `$${cost.toFixed(4)}`
}

/**
 * Formats a token count with k suffix for thousands.
 */
export function formatTokens(tokens?: number): string {
  if (!tokens) return '—'
  if (tokens >= 1000) return `${(tokens / 1000).toFixed(1)}k`
  return tokens.toString()
}

// Status badge class mappings
export const statusBadgeClass: Record<string, string> = {
  PENDING: 'badge-secondary',
  READY: 'badge-info',
  RUNNING: 'badge-warning',
  COMPLETED: 'badge-success',
  FAILED: 'badge-danger',
  RETRY: 'badge-warning',
  ARCHIVED: 'badge-secondary',
  DECOMPOSED: 'bg-purple-600 text-white px-2 py-1 rounded text-xs font-semibold',
  CANCELLED: 'bg-slate-500 text-white px-2 py-1 rounded text-xs font-semibold',
}

export const prStatusBadgeClass: Record<string, { label: string; className: string }> = {
  OPEN: { label: 'Open', className: 'badge-info' },
  APPROVED: { label: 'Approved', className: 'badge-success' },
  CHANGES_REQUESTED: { label: 'Changes Requested', className: 'badge-warning' },
  MERGED: { label: 'Merged', className: 'badge-secondary' },
  CLOSED: { label: 'Closed', className: 'bg-slate-600 text-slate-200 px-2 py-1 rounded text-xs font-medium' },
}

export const prStatusTextColor: Record<string, string> = {
  OPEN: 'text-green-400',
  MERGED: 'text-purple-400',
  CLOSED: 'text-red-400',
  CHANGES_REQUESTED: 'text-amber-400',
}
