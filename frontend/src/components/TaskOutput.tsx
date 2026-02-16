import { Task } from '../lib/api'
import ContentRenderer from './ContentRenderer'
import MarkdownRenderer from './MarkdownRenderer'

interface TaskOutputProps { task: Task }

export default function TaskOutput({ task }: TaskOutputProps) {
  return (
    <>
      {task.error_log && (
        <div className="card p-5 ring-1 ring-rose-500/20" role="alert">
          <h2 className="text-sm font-semibold text-rose-400 mb-3">Error</h2>
          <pre className="text-xs text-rose-300/80 bg-rose-500/[0.04] rounded-lg p-4 overflow-auto whitespace-pre-wrap border border-rose-500/10">{task.error_log}</pre>
        </div>
      )}
      {task.result && (
        <div className="card p-5">
          <h2 className="text-sm font-semibold text-white/80 mb-3">Result</h2>
          {(() => {
            try {
              const parsed = JSON.parse(task.result)
              if (parsed.result && typeof parsed.result === 'string') {
                return (<><div className="mb-4"><MarkdownRenderer content={parsed.result} /></div><details className="mt-4"><summary className="text-xs text-white/30 cursor-pointer hover:text-white/50 transition-colors">Show full output</summary><div className="mt-2"><ContentRenderer content={task.result} /></div></details></>)
              }
            } catch {}
            return <ContentRenderer content={task.result} />
          })()}
        </div>
      )}
      {task.prompt && <div className="card p-5"><h2 className="text-sm font-semibold text-white/80 mb-3">Prompt</h2><ContentRenderer content={task.prompt} /></div>}
      {!task.error_log && !task.result && !task.prompt && <div className="card p-5 text-center text-white/20 text-sm">No output yet</div>}
    </>
  )
}
