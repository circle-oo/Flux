import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

interface MarkdownRendererProps {
  content: string
  className?: string
}

export default function MarkdownRenderer({ content, className = '' }: MarkdownRendererProps) {
  return (
    <div className={`prose prose-invert prose-sm max-w-none ${className}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          h1: ({ children }) => (
            <h1 className="text-xl font-bold text-slate-100 mt-4 mb-2 pb-1 border-b border-slate-700">
              {children}
            </h1>
          ),
          h2: ({ children }) => (
            <h2 className="text-lg font-semibold text-slate-200 mt-3 mb-2 pb-1 border-b border-slate-700/50">
              {children}
            </h2>
          ),
          h3: ({ children }) => (
            <h3 className="text-base font-semibold text-slate-200 mt-3 mb-1">{children}</h3>
          ),
          h4: ({ children }) => (
            <h4 className="text-sm font-semibold text-slate-300 mt-2 mb-1">{children}</h4>
          ),
          p: ({ children }) => <p className="text-sm text-slate-300 my-1.5 leading-relaxed">{children}</p>,
          a: ({ href, children }) => (
            <a
              href={href}
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-400 hover:text-blue-300 underline"
            >
              {children}
            </a>
          ),
          ul: ({ children }) => <ul className="list-disc list-inside my-1.5 space-y-0.5 text-sm text-slate-300">{children}</ul>,
          ol: ({ children }) => <ol className="list-decimal list-inside my-1.5 space-y-0.5 text-sm text-slate-300">{children}</ol>,
          li: ({ children }) => <li className="text-sm text-slate-300">{children}</li>,
          code: ({ className: codeClassName, children, ...props }) => {
            const isInline = !codeClassName
            if (isInline) {
              return (
                <code className="px-1.5 py-0.5 bg-slate-700/80 text-sky-300 rounded text-xs font-mono">
                  {children}
                </code>
              )
            }
            return (
              <code className={`block bg-slate-900/80 rounded p-3 text-xs font-mono text-slate-300 overflow-x-auto ${codeClassName || ''}`} {...props}>
                {children}
              </code>
            )
          },
          pre: ({ children }) => <pre className="my-2 rounded overflow-hidden">{children}</pre>,
          blockquote: ({ children }) => (
            <blockquote className="border-l-2 border-slate-600 pl-3 my-2 text-slate-400 italic">
              {children}
            </blockquote>
          ),
          table: ({ children }) => (
            <div className="overflow-x-auto my-2">
              <table className="min-w-full text-sm border border-slate-700 rounded">{children}</table>
            </div>
          ),
          thead: ({ children }) => <thead className="bg-slate-800">{children}</thead>,
          th: ({ children }) => (
            <th className="px-3 py-1.5 text-left text-xs font-semibold text-slate-300 border-b border-slate-700">
              {children}
            </th>
          ),
          td: ({ children }) => (
            <td className="px-3 py-1.5 text-sm text-slate-400 border-b border-slate-700/50">{children}</td>
          ),
          hr: () => <hr className="my-3 border-slate-700" />,
          strong: ({ children }) => <strong className="font-semibold text-slate-200">{children}</strong>,
          em: ({ children }) => <em className="italic text-slate-300">{children}</em>,
          del: ({ children }) => <del className="line-through text-slate-500">{children}</del>,
          input: ({ checked, ...props }) => (
            <input
              type="checkbox"
              checked={checked}
              readOnly
              className="mr-1.5 rounded border-slate-600"
              {...props}
            />
          ),
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}
