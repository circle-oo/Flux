import MarkdownRenderer from './MarkdownRenderer'
import JsonViewer from './JsonViewer'

interface ContentRendererProps {
  content: string
  className?: string
  maxHeight?: string
}

function looksLikeJson(content: string): boolean {
  const trimmed = content.trim()
  return (trimmed.startsWith('{') && trimmed.endsWith('}')) ||
         (trimmed.startsWith('[') && trimmed.endsWith(']'))
}

export default function ContentRenderer({ content, className = '', maxHeight = '24rem' }: ContentRendererProps) {
  if (looksLikeJson(content)) {
    return <JsonViewer content={content} className={className} maxHeight={maxHeight} />
  }

  return (
    <div className={`bg-gray-50 rounded p-4 overflow-auto ${className}`} style={{ maxHeight }}>
      <MarkdownRenderer content={content} />
    </div>
  )
}
