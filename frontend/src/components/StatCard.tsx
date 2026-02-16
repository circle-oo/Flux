interface StatCardProps {
  label: string
  value: number
  color: string
  onClick?: () => void
}

export default function StatCard({ label, value, color, onClick }: StatCardProps) {
  const Wrapper = onClick ? 'button' : 'div'
  return (
    <Wrapper
      className={`card p-4 text-left w-full ${onClick ? 'cursor-pointer hover:bg-surface-hover transition-all touch-manipulation group' : ''}`}
      onClick={onClick}
      {...(onClick ? { type: 'button' as const } : {})}
    >
      <div className="text-[11px] font-medium text-content-faint mb-1 uppercase tracking-wider">{label}</div>
      <div className={`text-2xl font-bold tabular-nums ${color} ${onClick ? 'group-hover:scale-105 transition-transform origin-left' : ''}`}>{value}</div>
    </Wrapper>
  )
}
