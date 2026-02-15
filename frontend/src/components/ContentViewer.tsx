import { useState } from 'react'
import MarkdownViewer from './MarkdownViewer'
import JsonViewer from './JsonViewer'

interface ContentViewerProps {
  content: string
  title?: string
  defaultMode?: 'auto' | 'markdown' | 'json' | 'raw'
  maxHeight?: string
}

function detectContentType(content: string): 'json' | 'markdown' | 'raw' {
  // Try to detect JSON
  const trimmed = content.trim()
  if (
    (trimmed.startsWith('{') && trimmed.endsWith('}')) ||
    (trimmed.startsWith('[') && trimmed.endsWith(']'))
  ) {
    try {
      JSON.parse(content)
      return 'json'
    } catch {
      // Not valid JSON, continue detection
    }
  }

  // Check for markdown indicators
  const markdownPatterns = [
    /^#{1,6}\s/m, // Headers
    /^\*\s/m, // Unordered list
    /^\d+\.\s/m, // Ordered list
    /\[.*\]\(.*\)/, // Links
    /```/, // Code blocks
    /^\>\s/m, // Blockquotes
    /\*\*.*\*\*/, // Bold
    /\_.*\_/, // Italic
    /^\|.*\|/m, // Tables
  ]

  const hasMarkdown = markdownPatterns.some((pattern) => pattern.test(content))
  if (hasMarkdown) {
    return 'markdown'
  }

  return 'raw'
}

export default function ContentViewer({
  content,
  title,
  defaultMode = 'auto',
  maxHeight = 'max-h-96',
}: ContentViewerProps) {
  const detectedType = defaultMode === 'auto' ? detectContentType(content) : defaultMode
  const [viewMode, setViewMode] = useState<'markdown' | 'json' | 'raw'>(detectedType)

  return (
    <div>
      {/* View Mode Selector */}
      <div className="flex items-center justify-between mb-3">
        {title && <h2 className="text-lg font-semibold text-slate-200">{title}</h2>}
        <div className="flex gap-1">
          <button
            onClick={() => setViewMode('markdown')}
            className={`px-3 py-1 text-xs font-medium rounded transition-colors ${
              viewMode === 'markdown'
                ? 'bg-blue-600 text-white'
                : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
            }`}
          >
            Markdown
          </button>
          <button
            onClick={() => setViewMode('json')}
            className={`px-3 py-1 text-xs font-medium rounded transition-colors ${
              viewMode === 'json'
                ? 'bg-blue-600 text-white'
                : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
            }`}
          >
            JSON
          </button>
          <button
            onClick={() => setViewMode('raw')}
            className={`px-3 py-1 text-xs font-medium rounded transition-colors ${
              viewMode === 'raw'
                ? 'bg-blue-600 text-white'
                : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
            }`}
          >
            Raw
          </button>
        </div>
      </div>

      {/* Content Display */}
      <div className={`overflow-auto ${maxHeight}`}>
        {viewMode === 'markdown' && <MarkdownViewer content={content} />}
        {viewMode === 'json' && <JsonViewer content={content} />}
        {viewMode === 'raw' && (
          <pre className="text-sm text-slate-300 bg-slate-900/50 rounded p-4 whitespace-pre-wrap">
            {content}
          </pre>
        )}
      </div>
    </div>
  )
}
