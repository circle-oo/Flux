interface PageHeaderProps {
  title: string
  subtitle: string
  count?: number
  action?: React.ReactNode
}

export default function PageHeader({ title, subtitle, count, action }: PageHeaderProps) {
  return (
    <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4 mb-2">
      <div>
        <div className="flex items-center gap-2.5 mb-1">
          <h1 className="text-[22px] leading-tight font-semibold tracking-[-0.02em] text-content">{title}</h1>
          {count !== undefined && count > 0 && (
            <span
              className="bg"
              style={{
                fontSize: 11,
                fontWeight: 700,
                padding: '3px 10px',
                borderRadius: 999,
                color: 'var(--t3)',
              }}
            >
              {count}
            </span>
          )}
        </div>
        <p className="text-sm text-content-faint">{subtitle}</p>
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  )
}
