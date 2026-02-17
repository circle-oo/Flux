// Shared utility functions

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

export function formatDate(iso?: string): string {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleString(undefined, {
      year: 'numeric',
      month: 'numeric',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      timeZoneName: 'short',
    })
  } catch {
    return iso
  }
}

export function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleTimeString(undefined, {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  } catch {
    return iso.slice(11, 19)
  }
}

export function formatUptime(ms: number): string {
  const totalMinutes = Math.floor(ms / 1000 / 60)
  if (totalMinutes < 60) return `${totalMinutes}m`
  const hours = Math.floor(totalMinutes / 60)
  const minutes = totalMinutes % 60
  if (hours < 24) return `${hours}h ${minutes}m`
  const days = Math.floor(hours / 24)
  return `${days}d ${hours % 24}h`
}

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
  return `${hours}h ${mins % 60}m`
}

export function formatCost(cost?: number | null): string {
  if (cost === undefined || cost === null) return '—'
  if (cost === 0) return '$0'
  return `$${cost.toFixed(4)}`
}

export function formatTokens(tokens?: number | null): string {
  if (tokens === undefined || tokens === null) return '—'
  if (tokens === 0) return '0'
  if (tokens >= 1000) return `${(tokens / 1000).toFixed(1)}k`
  return tokens.toString()
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const gb = bytes / (1024 * 1024 * 1024)
  if (gb >= 1) return `${gb.toFixed(1)} GB`
  const mb = bytes / (1024 * 1024)
  return `${mb.toFixed(0)} MB`
}

export function diskLevelColor(level: string): string {
  switch (level) {
    case 'ok': return 'text-emerald-500'
    case 'warning': return 'text-yellow-500'
    case 'block': return 'text-orange-500'
    case 'critical': return 'text-red-500'
    case 'force': return 'text-red-600'
    default: return 'text-content-muted'
  }
}

export function countByStatus(tasks: { status: string }[]): Record<string, number> {
  const counts: Record<string, number> = {}
  for (const task of tasks) {
    counts[task.status] = (counts[task.status] || 0) + 1
  }
  return counts
}

export const statusBadgeClass: Record<string, string> = {
  PENDING: 'badge-secondary',
  READY: 'badge-info',
  RUNNING: 'badge-warning',
  COMPLETED: 'badge-success',
  FAILED: 'badge-danger',
  RETRY: 'badge-warning',
  ARCHIVED: 'badge-secondary',
  DECOMPOSED: 'badge-purple',
  CANCELLED: 'badge-muted',
}

export const prStatusBadgeClass: Record<string, { label: string; className: string }> = {
  OPEN: { label: 'Open', className: 'badge-info' },
  APPROVED: { label: 'Approved', className: 'badge-success' },
  CHANGES_REQUESTED: { label: 'Changes Requested', className: 'badge-warning' },
  MERGED: { label: 'Merged', className: 'badge-secondary' },
  CLOSED: { label: 'Closed', className: 'badge-secondary' },
}

export const prStatusTextColor: Record<string, string> = {
  OPEN: 'text-emerald-600',
  MERGED: 'text-violet-600',
  CLOSED: 'text-rose-600',
  CHANGES_REQUESTED: 'text-amber-600',
}
