interface InfoRowProps {
  label: string
  children: React.ReactNode
}

export default function InfoRow({ label, children }: InfoRowProps) {
  return (
    <div className="flex items-start py-2 border-b border-slate-700/50">
      <span className="w-36 shrink-0 text-sm text-slate-500 font-medium">{label}</span>
      <span className="text-sm text-slate-200 min-w-0">{children}</span>
    </div>
  )
}
