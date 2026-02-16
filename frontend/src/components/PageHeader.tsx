interface PageHeaderProps {
  title: string
  subtitle: string
  count?: number
  action?: React.ReactNode
}

export default function PageHeader({ title, subtitle, count, action }: PageHeaderProps) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-1">
      <div>
        <div className="flex items-center gap-3 mb-0.5">
          <h1 className="text-xl font-semibold text-white/90 tracking-tight">{title}</h1>
          {count !== undefined && count > 0 && (
            <span className="text-xs font-medium text-white/30 bg-white/[0.04] px-2 py-0.5 rounded-full">
              {count}
            </span>
          )}
        </div>
        <p className="text-xs text-white/30">{subtitle}</p>
      </div>
      {action}
    </div>
  )
}
