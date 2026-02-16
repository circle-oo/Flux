interface StatCardProps {
  label: string
  value: number
  color: string
  onClick?: () => void
  delta?: number
  deltaLabel?: string
}

export default function StatCard({ label, value, color, onClick, delta, deltaLabel }: StatCardProps) {
  const Wrapper = onClick ? 'button' : 'div'
  return (
    <Wrapper
      className={`card p-4 text-left w-full ${onClick ? 'cursor-pointer hover:bg-surface-hover transition-all touch-manipulation group' : ''}`}
      onClick={onClick}
      {...(onClick ? { type: 'button' as const } : {})}
    >
      <div className="text-[11px] font-medium text-content-faint mb-1 uppercase tracking-wider">{label}</div>
      <div className="flex items-baseline gap-2">
        <div className={`text-2xl font-bold tabular-nums ${color} ${onClick ? 'group-hover:scale-105 transition-transform origin-left' : ''}`}>{value}</div>
        {delta !== undefined && delta !== 0 && (
          <span className={`text-xs font-semibold tabular-nums ${delta > 0 ? 'text-emerald-600' : 'text-rose-600'}`}>
            {delta > 0 ? '↑' : '↓'}{Math.abs(delta)}
          </span>
        )}
      </div>
      {deltaLabel && delta !== undefined && (
        <div className="text-[10px] text-content-faint mt-0.5">{deltaLabel}</div>
      )}
    </Wrapper>
  )
}
