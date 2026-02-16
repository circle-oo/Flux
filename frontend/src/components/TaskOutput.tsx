import { Task } from '../lib/api'
import ContentRenderer from './ContentRenderer'
import MarkdownRenderer from './MarkdownRenderer'

interface TaskOutputProps {
  task: Task
}

export default function TaskOutput({ task }: TaskOutputProps) {
  return (
    <>
      {task.error_log && (
        <div className="card p-6 border border-red-600/50" role="alert">
          <h2 className="text-lg font-semibold text-red-400 mb-3">Error</h2>
          <pre className="text-sm text-red-200 bg-red-900/20 rounded p-4 overflow-auto whitespace-pre-wrap">
            {task.error_log}
          </pre>
        </div>
      )}

      {task.result && (
        <div className="card p-6">
          <h2 className="text-lg font-semibold text-slate-200 mb-3">Result</h2>
          {(() => {
            try {
              const parsed = JSON.parse(task.result)
              if (parsed.result && typeof parsed.result === 'string') {
                return (
                  <>
                    <div className="mb-4">
                      <MarkdownRenderer content={parsed.result} />
                    </div>
                    <details className="mt-4">
                      <summary className="text-sm text-slate-400 cursor-pointer hover:text-slate-300">
                        Show full output
                      </summary>
                      <div className="mt-2">
                        <ContentRenderer content={task.result} />
                      </div>
                    </details>
                  </>
                )
              }
            } catch {
              // Not JSON, fall through to default rendering
            }
            return <ContentRenderer content={task.result} />
          })()}
        </div>
      )}

      {task.prompt && (
        <div className="card p-6">
          <h2 className="text-lg font-semibold text-slate-200 mb-3">Prompt</h2>
          <ContentRenderer content={task.prompt} />
        </div>
      )}

      {!task.error_log && !task.result && !task.prompt && (
        <div className="card p-6 text-center text-slate-500">
          No output yet
        </div>
      )}
    </>
  )
}
