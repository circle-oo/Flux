interface EmptyStateProps {
  icon?: string
  title: string
  message: string
}

export default function EmptyState({ icon, title, message }: EmptyStateProps) {
  return (
    <div className="card p-12 text-center">
      {icon && <div className="text-6xl mb-4">{icon}</div>}
      <h2 className="text-xl font-semibold text-slate-300 mb-2">{title}</h2>
      <p className="text-slate-400">{message}</p>
    </div>
  )
}
