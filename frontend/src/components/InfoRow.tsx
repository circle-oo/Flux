interface InfoRowProps {
  label: string
  children: React.ReactNode
}

export default function InfoRow({ label, children }: InfoRowProps) {
  return (
    <div className="flex items-start py-2.5 border-b border-white/[0.04]">
      <span className="w-32 shrink-0 text-[11px] text-white/30 font-medium uppercase tracking-wider">{label}</span>
      <span className="text-sm text-white/70 min-w-0">{children}</span>
    </div>
  )
}
