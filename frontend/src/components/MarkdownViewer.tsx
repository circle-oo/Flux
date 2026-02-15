import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

interface MarkdownViewerProps {
  content: string
  className?: string
}

export default function MarkdownViewer({ content, className = '' }: MarkdownViewerProps) {
  return (
    <div className={`prose prose-invert prose-slate max-w-none ${className}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          // Customize heading styles
          h1: ({ children }) => (
            <h1 className="text-2xl font-bold text-slate-100 mt-6 mb-4 border-b border-slate-700 pb-2">
              {children}
            </h1>
          ),
          h2: ({ children }) => (
            <h2 className="text-xl font-bold text-slate-200 mt-5 mb-3">{children}</h2>
          ),
          h3: ({ children }) => (
            <h3 className="text-lg font-semibold text-slate-200 mt-4 mb-2">{children}</h3>
          ),
          // Customize paragraph styles
          p: ({ children }) => <p className="text-slate-300 mb-3 leading-relaxed">{children}</p>,
          // Customize list styles
          ul: ({ children }) => <ul className="list-disc list-inside text-slate-300 mb-3 space-y-1">{children}</ul>,
          ol: ({ children }) => <ol className="list-decimal list-inside text-slate-300 mb-3 space-y-1">{children}</ol>,
          li: ({ children }) => <li className="text-slate-300">{children}</li>,
          // Customize code styles
          code: ({ inline, children, ...props }: any) =>
            inline ? (
              <code className="bg-slate-800 text-pink-400 px-1.5 py-0.5 rounded text-sm font-mono" {...props}>
                {children}
              </code>
            ) : (
              <code className="block bg-slate-900 text-slate-300 p-3 rounded text-sm font-mono overflow-x-auto" {...props}>
                {children}
              </code>
            ),
          pre: ({ children }) => <pre className="bg-slate-900 rounded p-3 mb-3 overflow-x-auto">{children}</pre>,
          // Customize blockquote styles
          blockquote: ({ children }) => (
            <blockquote className="border-l-4 border-blue-500 pl-4 py-2 mb-3 text-slate-400 italic">
              {children}
            </blockquote>
          ),
          // Customize link styles
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
          // Customize table styles
          table: ({ children }) => (
            <div className="overflow-x-auto mb-3">
              <table className="min-w-full border border-slate-700">{children}</table>
            </div>
          ),
          thead: ({ children }) => <thead className="bg-slate-800">{children}</thead>,
          tbody: ({ children }) => <tbody className="divide-y divide-slate-700">{children}</tbody>,
          tr: ({ children }) => <tr className="border-b border-slate-700">{children}</tr>,
          th: ({ children }) => (
            <th className="px-4 py-2 text-left text-sm font-semibold text-slate-200">{children}</th>
          ),
          td: ({ children }) => <td className="px-4 py-2 text-sm text-slate-300">{children}</td>,
          // Customize horizontal rule
          hr: () => <hr className="border-slate-700 my-4" />,
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}
