export default function SystemInfoCard() {
  return (
    <section className="card p-5">
      <div className="text-[11px] font-medium text-white/30 uppercase tracking-widest mb-4">System Info</div>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
        <div><div className="text-[10px] text-white/25 uppercase tracking-wider mb-1">Process</div><div className="text-white/60 text-xs">launchd (KeepAlive)</div></div>
        <div><div className="text-[10px] text-white/25 uppercase tracking-wider mb-1">Restart Strategy</div><div className="text-white/60 text-xs">SIGTERM + auto-restart</div></div>
        <div><div className="text-[10px] text-white/25 uppercase tracking-wider mb-1">Build System</div><div className="text-white/60 text-xs">make build (Go + Vite)</div></div>
      </div>
    </section>
  )
}
