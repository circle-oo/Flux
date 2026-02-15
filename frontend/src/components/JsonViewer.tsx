import JsonView from '@uiw/react-json-view'

interface JsonViewerProps {
  content: string
  className?: string
}

export default function JsonViewer({ content, className = '' }: JsonViewerProps) {
  let parsed: unknown
  let error: string | null = null

  try {
    parsed = JSON.parse(content)
  } catch (e) {
    error = e instanceof Error ? e.message : 'Invalid JSON'
  }

  if (error) {
    return (
      <div className={`bg-red-900/20 border border-red-600 rounded p-4 ${className}`}>
        <p className="text-red-300 text-sm font-semibold mb-2">Invalid JSON</p>
        <pre className="text-red-200 text-xs font-mono whitespace-pre-wrap">{content}</pre>
      </div>
    )
  }

  return (
    <div className={`bg-slate-900/50 rounded p-4 overflow-auto ${className}`}>
      <JsonView
        value={parsed as object}
        collapsed={2}
        style={{
          backgroundColor: 'transparent',
          fontSize: '13px',
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
        }}
      />
    </div>
  )
}
