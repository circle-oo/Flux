import { useEffect, useRef, useMemo, useState } from 'react'
import { useLogStore, LogEntry } from '../stores/logStore'
import PageHeader from '../components/PageHeader'

const levelColors: Record<string, string> = {
  DEBUG: 'bg-white/[0.06] text-white/50',
  INFO: 'bg-cyan-500/15 text-cyan-400',
  WARN: 'bg-amber-500/15 text-amber-400',
  ERROR: 'bg-rose-500/15 text-rose-400',
}

const componentColors: Record<string, string> = {
  executor: 'text-violet-400',
  triager: 'text-teal-400',
  manager: 'text-emerald-400',
  orchestrator: 'text-cyan-400',
  server: 'text-amber-400',
  main: 'text-white/40',
  shutdown: 'text-orange-400',
  github: 'text-pink-400',
}

function LevelBadge({ level }: { level: string }) {
  const cls = levelColors[level] || 'bg-white/[0.06] text-white/50'
  return (
    <span className={`inline-block px-1.5 py-0.5 rounded text-[10px] font-mono font-semibold ${cls}`}>
      {level}
    </span>
  )
}

function formatLogTime(iso: string): string {
  try {
    const d = new Date(iso)
    const hh = String(d.getHours()).padStart(2, '0')
    const mm = String(d.getMinutes()).padStart(2, '0')
    const ss = String(d.getSeconds()).padStart(2, '0')
    const ms = String(d.getMilliseconds()).padStart(3, '0')
    return `${hh}:${mm}:${ss}.${ms}`
  } catch {
    return iso
  }
}

function AttrCell({ attrs }: { attrs: Record<string, unknown> }) {
  const [expanded, setExpanded] = useState(false)
  const keys = Object.keys(attrs)
  if (keys.length === 0) return <span className="text-white/10">—</span>

  const preview = keys
    .slice(0, 3)
    .map((k) => `${k}=${JSON.stringify(attrs[k])}`)
    .join(' ')
  const hasMore = keys.length > 3

  if (!expanded) {
    return (
      <button
        onClick={() => setExpanded(true)}
        className="text-left text-white/30 hover:text-white/50 font-mono text-[10px] truncate max-w-md transition-colors"
        title="Click to expand"
        aria-expanded={false}
      >
        {preview}
        {hasMore && ' ...'}
      </button>
    )
  }

  return (
    <button
      onClick={() => setExpanded(false)}
      className="text-left font-mono text-[10px] text-white/50 whitespace-pre-wrap"
      aria-expanded={true}
    >
      {JSON.stringify(attrs, null, 2)}
    </button>
  )
}

function LogToolbar({
  filter,
  autoScroll,
  onFilterChange,
  onAutoScrollChange,
}: {
  filter: { level: string; search: string }
  autoScroll: boolean
  onFilterChange: (filter: Partial<{ level: string; search: string }>) => void
  onAutoScrollChange: (enabled: boolean) => void
}) {
  const levels = ['', 'DEBUG', 'INFO', 'WARN', 'ERROR']

  return (
    <div className="card p-3 flex items-center gap-3 mb-4">
      <div className="flex items-center gap-1" role="group" aria-label="Log level filter">
        {levels.map((lvl) => (
          <button
            key={lvl || 'ALL'}
            onClick={() => onFilterChange({ level: lvl })}
            aria-pressed={filter.level === lvl}
            className={`px-2.5 py-1 rounded text-[10px] font-semibold transition-all ${
              filter.level === lvl
                ? lvl === ''
                  ? 'bg-accent-600/20 text-accent-400 ring-1 ring-accent-500/20'
                  : (levelColors[lvl] || 'bg-white/[0.06] text-white')
                : 'bg-white/[0.03] text-white/30 hover:bg-white/[0.06] hover:text-white/50'
            }`}
          >
            {lvl || 'ALL'}
          </button>
        ))}
      </div>

      <label htmlFor="log-search" className="sr-only">Search logs</label>
      <input
        id="log-search"
        type="text"
        placeholder="Search logs..."
        value={filter.search}
        onChange={(e) => onFilterChange({ search: e.target.value })}
        className="input flex-1 text-xs min-h-[36px]"
      />

      <label className="flex items-center gap-2 text-xs text-white/30 cursor-pointer select-none whitespace-nowrap">
        <input
          type="checkbox"
          checked={autoScroll}
          onChange={(e) => onAutoScrollChange(e.target.checked)}
          className="rounded border-white/[0.1] bg-white/[0.04]"
        />
        Auto-scroll
      </label>
    </div>
  )
}

function LogRow({ entry }: { entry: LogEntry }) {
  return (
    <tr
      className={`border-b border-white/[0.03] hover:bg-white/[0.02] ${
        entry.level === 'ERROR'
          ? 'bg-rose-500/[0.03]'
          : entry.level === 'WARN'
          ? 'bg-amber-500/[0.03]'
          : ''
      }`}
    >
      <td className="px-3 py-1.5 text-white/20 whitespace-nowrap font-mono text-[10px]">
        {formatLogTime(entry.time)}
      </td>
      <td className="px-3 py-1.5"><LevelBadge level={entry.level} /></td>
      <td className={`px-3 py-1.5 font-mono text-[10px] font-semibold ${componentColors[entry.attrs.component as string] || 'text-white/30'}`}>
        {(entry.attrs.component as string) || '—'}
      </td>
      <td className="px-3 py-1.5 text-white/70 text-xs">{entry.msg}</td>
      <td className="px-3 py-1.5"><AttrCell attrs={entry.attrs} /></td>
    </tr>
  )
}

export default function Logs() {
  const {
    logs,
    filter,
    autoScroll,
    paused,
    fetchRecentLogs,
    clearLogs,
    setFilter,
    setAutoScroll,
    setPaused,
  } = useLogStore()

  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    fetchRecentLogs()
  }, [fetchRecentLogs])

  useEffect(() => {
    if (autoScroll && !paused) {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
    }
  }, [logs.length, autoScroll, paused])

  const filteredLogs = useMemo(() => {
    return logs.filter((entry: LogEntry) => {
      if (filter.level && entry.level !== filter.level) return false
      if (filter.search) {
        const q = filter.search.toLowerCase()
        const matchMsg = entry.msg.toLowerCase().includes(q)
        const matchAttrs = JSON.stringify(entry.attrs).toLowerCase().includes(q)
        if (!matchMsg && !matchAttrs) return false
      }
      return true
    })
  }, [logs, filter])

  return (
    <div className="p-5 sm:p-6 lg:p-8 flex flex-col h-full animate-fade-in">
      <PageHeader
        title="Logs"
        subtitle={`Real-time system logs (${filteredLogs.length} entries)`}
        count={filteredLogs.length}
        action={
          <div className="flex items-center gap-2">
            <button
              onClick={() => setPaused(!paused)}
              className={`btn-sm ${
                paused ? 'btn-warning' : 'btn-secondary'
              }`}
              aria-pressed={paused}
            >
              {paused ? 'Resume' : 'Pause'}
            </button>
            <button
              onClick={clearLogs}
              className="btn-sm btn-secondary"
            >
              Clear
            </button>
          </div>
        }
      />

      <LogToolbar
        filter={filter}
        autoScroll={autoScroll}
        onFilterChange={setFilter}
        onAutoScrollChange={setAutoScroll}
      />

      <div className="card flex-1 overflow-auto min-h-0">
        <table className="w-full text-sm">
          <thead className="sticky top-0 bg-surface-100/95 backdrop-blur-sm z-10">
            <tr className="text-left text-white/30 border-b border-white/[0.06]">
              <th scope="col" className="px-3 py-2 w-24 font-medium text-[10px] uppercase tracking-wider">Time</th>
              <th scope="col" className="px-3 py-2 w-16 font-medium text-[10px] uppercase tracking-wider">Level</th>
              <th scope="col" className="px-3 py-2 w-20 font-medium text-[10px] uppercase tracking-wider">Component</th>
              <th scope="col" className="px-3 py-2 font-medium text-[10px] uppercase tracking-wider">Message</th>
              <th scope="col" className="px-3 py-2 font-medium text-[10px] uppercase tracking-wider">Attributes</th>
            </tr>
          </thead>
          <tbody className="font-mono text-xs">
            {filteredLogs.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-white/20 text-sm">
                  {logs.length === 0
                    ? 'No log entries yet. Logs will appear here in real-time.'
                    : 'No entries match the current filters.'}
                </td>
              </tr>
            ) : (
              filteredLogs.map((entry, i) => (
                <LogRow key={i} entry={entry} />
              ))
            )}
          </tbody>
        </table>
        <div ref={bottomRef} />
      </div>
    </div>
  )
}
