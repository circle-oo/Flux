import { useEffect, useRef, useMemo, useState } from 'react'
import { useLogStore, LogEntry } from '../stores/logStore'

const levelColors: Record<string, string> = {
  DEBUG: 'bg-slate-600 text-slate-200',
  INFO: 'bg-blue-600 text-blue-100',
  WARN: 'bg-amber-600 text-amber-100',
  ERROR: 'bg-red-600 text-red-100',
}

const componentColors: Record<string, string> = {
  executor: 'text-purple-400',
  triager: 'text-teal-400',
  manager: 'text-green-400',
  orchestrator: 'text-cyan-400',
  server: 'text-yellow-400',
  main: 'text-slate-400',
  shutdown: 'text-orange-400',
  github: 'text-pink-400',
}

function levelBadge(level: string) {
  const cls = levelColors[level] || 'bg-slate-600 text-slate-200'
  return (
    <span className={`inline-block px-2 py-0.5 rounded text-xs font-mono font-semibold ${cls}`}>
      {level}
    </span>
  )
}

function formatTime(iso: string) {
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
  if (keys.length === 0) return <span className="text-slate-600">—</span>

  const preview = keys
    .slice(0, 3)
    .map((k) => `${k}=${JSON.stringify(attrs[k])}`)
    .join(' ')
  const hasMore = keys.length > 3

  if (!expanded) {
    return (
      <button
        onClick={() => setExpanded(true)}
        className="text-left text-slate-400 hover:text-slate-200 font-mono text-xs truncate max-w-md"
        title="Click to expand"
      >
        {preview}
        {hasMore && ' ...'}
      </button>
    )
  }

  return (
    <button
      onClick={() => setExpanded(false)}
      className="text-left font-mono text-xs text-slate-300 whitespace-pre-wrap"
    >
      {JSON.stringify(attrs, null, 2)}
    </button>
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

  // Auto-scroll on new logs
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

  const levels = ['', 'DEBUG', 'INFO', 'WARN', 'ERROR']

  return (
    <div className="p-4 sm:p-6 lg:p-8 flex flex-col h-full">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between mb-6 gap-4">
        <div>
          <h1 className="text-2xl sm:text-3xl font-bold text-slate-100 mb-2">Logs</h1>
          <p className="text-sm sm:text-base text-slate-400">
            Real-time system logs ({filteredLogs.length} entries)
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setPaused(!paused)}
            className={`px-3 py-1.5 rounded text-sm font-medium transition-colors ${
              paused
                ? 'bg-amber-600 text-white hover:bg-amber-500'
                : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
            }`}
          >
            {paused ? 'Resume' : 'Pause'}
          </button>
          <button
            onClick={clearLogs}
            className="px-3 py-1.5 rounded text-sm font-medium bg-slate-700 text-slate-300 hover:bg-slate-600 transition-colors"
          >
            Clear
          </button>
        </div>
      </div>

      {/* Toolbar */}
      <div className="card p-4 flex items-center gap-4 mb-4">
        {/* Level filter buttons */}
        <div className="flex items-center gap-1">
          {levels.map((lvl) => (
            <button
              key={lvl || 'ALL'}
              onClick={() => setFilter({ level: lvl })}
              className={`px-3 py-1 rounded text-xs font-semibold transition-colors ${
                filter.level === lvl
                  ? lvl === ''
                    ? 'bg-blue-600 text-white'
                    : (levelColors[lvl] || 'bg-slate-600 text-white')
                  : 'bg-slate-700 text-slate-400 hover:bg-slate-600'
              }`}
            >
              {lvl || 'ALL'}
            </button>
          ))}
        </div>

        {/* Search */}
        <input
          type="text"
          placeholder="Search logs..."
          value={filter.search}
          onChange={(e) => setFilter({ search: e.target.value })}
          className="input flex-1"
        />

        {/* Auto-scroll toggle */}
        <label className="flex items-center gap-2 text-sm text-slate-400 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
            className="rounded border-slate-600"
          />
          Auto-scroll
        </label>
      </div>

      {/* Log table */}
      <div className="card flex-1 overflow-auto min-h-0">
        <table className="w-full text-sm">
          <thead className="sticky top-0 bg-slate-800 z-10">
            <tr className="text-left text-slate-400 border-b border-slate-700">
              <th className="px-4 py-2 w-28 font-medium">Time</th>
              <th className="px-4 py-2 w-20 font-medium">Level</th>
              <th className="px-4 py-2 w-24 font-medium">Component</th>
              <th className="px-4 py-2 font-medium">Message</th>
              <th className="px-4 py-2 font-medium">Attributes</th>
            </tr>
          </thead>
          <tbody className="font-mono text-xs">
            {filteredLogs.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-slate-500">
                  {logs.length === 0
                    ? 'No log entries yet. Logs will appear here in real-time.'
                    : 'No entries match the current filters.'}
                </td>
              </tr>
            ) : (
              filteredLogs.map((entry, i) => (
                <tr
                  key={i}
                  className={`border-b border-slate-700/50 hover:bg-slate-700/30 ${
                    entry.level === 'ERROR'
                      ? 'bg-red-900/10'
                      : entry.level === 'WARN'
                      ? 'bg-amber-900/10'
                      : ''
                  }`}
                >
                  <td className="px-4 py-1.5 text-slate-500 whitespace-nowrap">
                    {formatTime(entry.time)}
                  </td>
                  <td className="px-4 py-1.5">{levelBadge(entry.level)}</td>
                  <td className={`px-4 py-1.5 font-mono text-xs font-semibold ${componentColors[entry.attrs.component as string] || 'text-slate-500'}`}>
                    {(entry.attrs.component as string) || '—'}
                  </td>
                  <td className="px-4 py-1.5 text-slate-200">{entry.msg}</td>
                  <td className="px-4 py-1.5">
                    <AttrCell attrs={entry.attrs} />
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
        <div ref={bottomRef} />
      </div>
    </div>
  )
}
