export default function SystemInfoCard() {
  return (
    <section className="card p-4 sm:p-6">
      <h2 className="text-lg font-semibold text-slate-100 mb-4">System Info</h2>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
        <div>
          <div className="text-xs text-slate-500 mb-1">Process</div>
          <div className="text-slate-300">launchd (KeepAlive)</div>
        </div>
        <div>
          <div className="text-xs text-slate-500 mb-1">Restart Strategy</div>
          <div className="text-slate-300">SIGTERM + auto-restart</div>
        </div>
        <div>
          <div className="text-xs text-slate-500 mb-1">Build System</div>
          <div className="text-slate-300">make build (Go + Vite)</div>
        </div>
      </div>
    </section>
  )
}
