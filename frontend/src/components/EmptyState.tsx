interface EmptyStateProps {
  icon?: string
  title: string
  message: string
}

export default function EmptyState({ icon, title, message }: EmptyStateProps) {
  return (
    <div className="gc p-10 text-center animate-fade-in" style={{ alignItems: 'center' }}>
      {icon && <div className="text-5xl opacity-40 mb-2">{icon}</div>}
      <h2 className="text-base font-semibold text-content mb-1">{title}</h2>
      <p className="text-sm text-content-faint max-w-md">{message}</p>
    </div>
  )
}
