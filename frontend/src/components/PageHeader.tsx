interface PageHeaderProps {
  title: string
  subtitle: string
  count?: number
  action?: React.ReactNode
}

export default function PageHeader({ title, subtitle, count, action }: PageHeaderProps) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
      <div>
        <div className="flex items-center gap-2 sm:gap-3 mb-1 sm:mb-2">
          <h1 className="text-2xl sm:text-3xl font-bold text-slate-100">{title}</h1>
          {count !== undefined && count > 0 && (
            <span className="badge badge-info text-sm sm:text-lg px-2 sm:px-3 py-0.5 sm:py-1">
              {count}
            </span>
          )}
        </div>
        <p className="text-sm sm:text-base text-slate-400">{subtitle}</p>
      </div>
      {action}
    </div>
  )
}
