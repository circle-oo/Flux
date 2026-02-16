interface PageHeaderProps {
  title: string
  subtitle: string
  count?: number
  action?: React.ReactNode
}

export default function PageHeader({ title, subtitle, count, action }: PageHeaderProps) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
      <div>
        <div className="flex items-center gap-3 mb-1">
          <h1 className="text-2xl font-bold text-slate-100">{title}</h1>
          {count !== undefined && count > 0 && (
            <span className="badge badge-info text-sm px-2.5 py-0.5">
              {count}
            </span>
          )}
        </div>
        <p className="text-sm text-slate-400">{subtitle}</p>
      </div>
      {action}
    </div>
  )
}
