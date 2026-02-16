interface InfoRowProps {
  label: string
  children: React.ReactNode
}

export default function InfoRow({ label, children }: InfoRowProps) {
  return (
    <div className="flex items-start py-2.5 border-b border-line-subtle">
      <span className="w-32 shrink-0 text-[11px] text-content-faint font-medium uppercase tracking-wider">{label}</span>
      <span className="text-sm text-content-secondary min-w-0">{children}</span>
    </div>
  )
}
