interface StatCardProps {
  label: string
  value: number
  color: string
  onClick?: () => void
  delta?: number
  deltaLabel?: string
}

function resolveValueColor(color: string): string {
  if (color.includes('emerald')) return 'var(--ok)'
  if (color.includes('rose')) return 'var(--err)'
  if (color.includes('amber')) return 'var(--warn)'
  if (color.includes('violet')) return 'var(--p600)'
  if (color.includes('cyan')) return 'var(--info)'
  return 'var(--t1)'
}

export default function StatCard({ label, value, color, onClick, delta, deltaLabel }: StatCardProps) {
  const valueColor = resolveValueColor(color)
  const Wrapper = onClick ? 'button' : 'div'

  return (
    <Wrapper
      {...(onClick ? { type: 'button' as const, onClick } : {})}
      className={`gc p-4 text-left w-full ${onClick ? 'cursor-pointer hover:-translate-y-[1px] transition-transform' : ''}`}
    >
      <span className="text-[10px] uppercase tracking-[0.08em] text-content-faint font-semibold">{label}</span>
      <div className="mt-2 flex items-end gap-2">
        <div className="text-3xl font-bold tabular-nums leading-none" style={{ color: valueColor }}>{value}</div>
        {delta !== undefined && delta !== 0 && (
          <span className={`text-xs font-semibold tabular-nums ${delta > 0 ? 'text-emerald-500' : 'text-rose-500'}`}>
            {delta > 0 ? '↑' : '↓'} {Math.abs(delta)}
          </span>
        )}
      </div>
      {deltaLabel && delta !== undefined && (
        <div className="text-[11px] text-content-faint mt-1">{deltaLabel}</div>
      )}
    </Wrapper>
  )
}
