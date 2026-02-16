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
      className={`card p-4 sm:p-5 text-left w-full ${onClick ? 'cursor-pointer hover:bg-slate-700/50 transition-colors touch-manipulation' : ''}`}
      onClick={onClick}
      {...(onClick ? { type: 'button' as const } : {})}
    >
      <div className="text-xs sm:text-sm font-medium text-slate-400 mb-1">{label}</div>
      <div className={`text-2xl sm:text-3xl font-bold ${color}`}>{value}</div>
    </Wrapper>
  )
}
