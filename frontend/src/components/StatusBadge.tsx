import { statusBadgeClass, prStatusBadgeClass } from '../lib/utils'

interface StatusBadgeProps {
  status: string
  size?: 'sm' | 'md'
}

export function StatusBadge({ status, size = 'md' }: StatusBadgeProps) {
  const cls = statusBadgeClass[status] || 'badge-secondary'
  const sizeClass = size === 'sm' ? 'text-xs' : ''
  return <span className={`badge ${cls} ${sizeClass}`}>{status}</span>
}

interface PRStatusBadgeProps {
  status?: string
}

export function PRStatusBadge({ status }: PRStatusBadgeProps) {
  if (!status) return null
  const config = prStatusBadgeClass[status] || { label: status, className: 'badge-secondary' }
  return <span className={`badge ${config.className}`}>{config.label}</span>
}
