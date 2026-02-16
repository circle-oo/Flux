interface EmptyStateProps {
  icon?: string
  title: string
  message: string
}

export default function EmptyState({ icon, title, message }: EmptyStateProps) {
  return (
    <div className="card p-12 text-center animate-fade-in">
      {icon && <div className="text-5xl mb-4 opacity-30">{icon}</div>}
      <h2 className="text-base font-semibold text-content-muted mb-1">{title}</h2>
      <p className="text-sm text-content-faint">{message}</p>
    </div>
  )
}
