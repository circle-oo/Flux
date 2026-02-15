import { useState, useCallback } from 'react'

interface JsonViewerProps {
  content: string
  className?: string
  defaultExpanded?: boolean
}

function JsonValue({ value, depth, defaultExpanded }: { value: unknown; depth: number; defaultExpanded: boolean }) {
  if (value === null) return <span className="text-slate-500 italic">null</span>
  if (value === undefined) return <span className="text-slate-500 italic">undefined</span>

  switch (typeof value) {
    case 'string':
      return <span className="text-green-400">&quot;{value}&quot;</span>
    case 'number':
      return <span className="text-amber-400">{String(value)}</span>
    case 'boolean':
      return <span className="text-purple-400">{String(value)}</span>
    case 'object':
      if (Array.isArray(value)) {
        return <JsonArray items={value} depth={depth} defaultExpanded={defaultExpanded} />
      }
      return <JsonObject data={value as Record<string, unknown>} depth={depth} defaultExpanded={defaultExpanded} />
    default:
      return <span className="text-slate-300">{String(value)}</span>
  }
}

function JsonArray({ items, depth, defaultExpanded }: { items: unknown[]; depth: number; defaultExpanded: boolean }) {
  const [expanded, setExpanded] = useState(defaultExpanded && depth < 2)
  const toggle = useCallback(() => setExpanded((e) => !e), [])

  if (items.length === 0) return <span className="text-slate-500">[]</span>

  if (!expanded) {
    return (
      <button onClick={toggle} className="text-slate-400 hover:text-slate-200 transition-colors">
        <span className="text-slate-500">[</span>
        <span className="text-slate-500 mx-1">{items.length} items</span>
        <span className="text-slate-500">]</span>
      </button>
    )
  }

  return (
    <span>
      <button onClick={toggle} className="text-slate-500 hover:text-slate-300 transition-colors">[</button>
      <div className="ml-4 border-l border-slate-700/50 pl-2">
        {items.map((item, i) => (
          <div key={i} className="py-0.5">
            <JsonValue value={item} depth={depth + 1} defaultExpanded={defaultExpanded} />
            {i < items.length - 1 && <span className="text-slate-600">,</span>}
          </div>
        ))}
      </div>
      <button onClick={toggle} className="text-slate-500 hover:text-slate-300 transition-colors">]</button>
    </span>
  )
}

function JsonObject({ data, depth, defaultExpanded }: { data: Record<string, unknown>; depth: number; defaultExpanded: boolean }) {
  const [expanded, setExpanded] = useState(defaultExpanded && depth < 2)
  const toggle = useCallback(() => setExpanded((e) => !e), [])
  const keys = Object.keys(data)

  if (keys.length === 0) return <span className="text-slate-500">{'{}'}</span>

  if (!expanded) {
    return (
      <button onClick={toggle} className="text-slate-400 hover:text-slate-200 transition-colors">
        <span className="text-slate-500">{'{'}</span>
        <span className="text-slate-500 mx-1">{keys.length} keys</span>
        <span className="text-slate-500">{'}'}</span>
      </button>
    )
  }

  return (
    <span>
      <button onClick={toggle} className="text-slate-500 hover:text-slate-300 transition-colors">{'{'}</button>
      <div className="ml-4 border-l border-slate-700/50 pl-2">
        {keys.map((key, i) => (
          <div key={key} className="py-0.5">
            <span className="text-sky-400">&quot;{key}&quot;</span>
            <span className="text-slate-500">: </span>
            <JsonValue value={data[key]} depth={depth + 1} defaultExpanded={defaultExpanded} />
            {i < keys.length - 1 && <span className="text-slate-600">,</span>}
          </div>
        ))}
      </div>
      <button onClick={toggle} className="text-slate-500 hover:text-slate-300 transition-colors">{'}'}</button>
    </span>
  )
}

export default function JsonViewer({ content, className = '', defaultExpanded = true }: JsonViewerProps) {
  let parsed: unknown
  try {
    parsed = JSON.parse(content)
  } catch {
    return (
      <pre className={`text-sm text-slate-300 bg-slate-900/50 rounded p-4 overflow-auto whitespace-pre-wrap font-mono ${className}`}>
        {content}
      </pre>
    )
  }

  return (
    <div className={`text-xs font-mono bg-slate-900/50 rounded p-4 overflow-auto max-h-96 ${className}`}>
      <JsonValue value={parsed} depth={0} defaultExpanded={defaultExpanded} />
    </div>
  )
}
