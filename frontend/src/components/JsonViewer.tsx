import { useState } from 'react'

interface JsonViewerProps {
  content: string
  className?: string
  maxHeight?: string
}

function tryParseJson(content: string): unknown | null {
  try {
    return JSON.parse(content)
  } catch {
    return null
  }
}

function syntaxHighlight(json: string): string {
  return json.replace(
    /("(\\u[\da-fA-F]{4}|\\[^u]|[^"\\])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g,
    (match) => {
      let cls = 'json-number'
      if (match.startsWith('"')) {
        cls = match.endsWith(':') ? 'json-key' : 'json-string'
      } else if (/true|false/.test(match)) {
        cls = 'json-boolean'
      } else if (match === 'null') {
        cls = 'json-null'
      }
      return `<span class="${cls}">${match}</span>`
    }
  )
}

export default function JsonViewer({ content, className = '', maxHeight = '24rem' }: JsonViewerProps) {
  const [collapsed, setCollapsed] = useState(false)
  const parsed = tryParseJson(content)

  if (parsed === null) {
    // Not valid JSON, render as plain text
    return (
      <pre className={`text-sm text-content-secondary bg-gray-50 rounded p-4 overflow-auto whitespace-pre-wrap ${className}`} style={{ maxHeight }}>
        {content}
      </pre>
    )
  }

  const formatted = JSON.stringify(parsed, null, 2)
  const highlighted = syntaxHighlight(formatted)

  return (
    <div className={`relative ${className}`}>
      <button
        onClick={() => setCollapsed(!collapsed)}
        className="absolute top-2 right-2 text-xs text-content-muted hover:text-content-secondary bg-surface-active px-2 py-1 rounded z-10"
      >
        {collapsed ? 'Expand' : 'Collapse'}
      </button>
      {collapsed ? (
        <pre className="text-sm text-content-muted bg-gray-50 rounded p-4 overflow-auto">
          {Array.isArray(parsed) ? `[...] (${(parsed as unknown[]).length} items)` : `{...} (${Object.keys(parsed as object).length} keys)`}
        </pre>
      ) : (
        <pre
          className="json-viewer text-sm bg-gray-50 rounded p-4 overflow-auto"
          style={{ maxHeight }}
          dangerouslySetInnerHTML={{ __html: highlighted }}
        />
      )}
    </div>
  )
}
