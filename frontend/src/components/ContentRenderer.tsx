import MarkdownRenderer from './MarkdownRenderer'
import JsonViewer from './JsonViewer'

interface ContentRendererProps {
  content: string
  className?: string
}

function detectContentType(content: string): 'json' | 'markdown' | 'plain' {
  const trimmed = content.trim()

  // Detect JSON: starts with { or [
  if ((trimmed.startsWith('{') && trimmed.endsWith('}')) ||
      (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
    try {
      JSON.parse(trimmed)
      return 'json'
    } catch {
      // Not valid JSON, fall through
    }
  }

  // Detect markdown: look for common markdown patterns
  const markdownPatterns = [
    /^#{1,6}\s/m,           // Headers
    /^\s*[-*+]\s/m,         // Unordered lists
    /^\s*\d+\.\s/m,         // Ordered lists
    /\[.+\]\(.+\)/,         // Links
    /```[\s\S]*?```/,       // Code blocks
    /\*\*.+\*\*/,           // Bold
    /^\s*>/m,               // Blockquotes
    /^\s*\|.+\|/m,          // Tables
    /^\s*---+\s*$/m,        // Horizontal rules
    /^\s*- \[[ x]\]/m,     // Task lists
  ]

  const matchCount = markdownPatterns.filter((p) => p.test(trimmed)).length
  if (matchCount >= 2) return 'markdown'

  return 'plain'
}

export default function ContentRenderer({ content, className = '' }: ContentRendererProps) {
  if (!content) return <span className="text-slate-500">—</span>

  const type = detectContentType(content)

  switch (type) {
    case 'json':
      return <JsonViewer content={content} className={className} />
    case 'markdown':
      return (
        <div className={`bg-slate-900/50 rounded p-4 overflow-auto max-h-96 ${className}`}>
          <MarkdownRenderer content={content} />
        </div>
      )
    case 'plain':
    default:
      return (
        <pre className={`text-sm text-slate-300 bg-slate-900/50 rounded p-4 overflow-auto whitespace-pre-wrap max-h-96 ${className}`}>
          {content}
        </pre>
      )
  }
}
