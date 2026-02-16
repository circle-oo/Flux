import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTaskStore } from '../stores/taskStore'
import { useWSStore } from '../stores/wsStore'
import { Task, api } from '../lib/api'
import LoadingState from '../components/LoadingState'
import MarkdownRenderer from '../components/MarkdownRenderer'
import TaskHeader from '../components/TaskHeader'
import TaskSubtasks from '../components/TaskSubtasks'
import TaskOverview from '../components/TaskOverview'
import TaskExecution from '../components/TaskExecution'
import TaskOutput from '../components/TaskOutput'
import { useConfirm } from '../hooks/useConfirm'
import { useToast } from '../components/Toast'

export default function TaskDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { getTask, cancelTask, retryTask, fetchSubtasks } = useTaskStore()
  const taskUpdateCounter = useWSStore((s) => s.taskUpdateCounter)
  const [task, setTask] = useState<Task | null>(null)
  const [subtasks, setSubtasks] = useState<Task[]>([])
  const [dependencies, setDependencies] = useState<{ dependent_id: string; dependency_id: string }[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [subtasksExpanded, setSubtasksExpanded] = useState(true)
  const [showDAG, setShowDAG] = useState(false)
  const [activeTab, setActiveTab] = useState<'overview' | 'execution' | 'output'>('overview')
  const { confirm, dialog } = useConfirm()
  const { toast } = useToast()

  const refreshTask = useCallback(() => {
    if (!id) return
    getTask(id).then((t) => {
      setTask(t)
      fetchSubtasks(t.id).then(setSubtasks)
      api.getSubtaskDependencies(t.id).then((data) => {
        setDependencies(data.edges)
        setShowDAG(data.edges.length > 0)
      }).catch(() => {})
    }).catch(() => {})
  }, [id, getTask, fetchSubtasks])

  useEffect(() => {
    if (!id) return
    setLoading(true)
    getTask(id)
      .then((t) => {
        setTask(t)
        fetchSubtasks(t.id).then(setSubtasks)
        api.getSubtaskDependencies(t.id).then((data) => {
          setDependencies(data.edges)
          setShowDAG(data.edges.length > 0)
        }).catch(() => {})
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [id, getTask, fetchSubtasks])

  useEffect(() => {
    if (!id || loading) return
    refreshTask()
  }, [taskUpdateCounter, id, loading, refreshTask])

  useEffect(() => {
    if (!task || !id) return
    const activeStatuses = ['RUNNING', 'PENDING', 'READY', 'DECOMPOSED']
    if (!activeStatuses.includes(task.status)) return

    const interval = setInterval(refreshTask, 5000)
    return () => clearInterval(interval)
  }, [task?.status, id, refreshTask])

  const handleRetry = async () => {
    if (!task) return
    const confirmed = await confirm({ title: 'Retry task?', description: task.title, confirmLabel: 'Retry' })
    if (!confirmed) return
    try {
      await retryTask(task.id)
      setTask(await getTask(task.id))
      toast('Task queued for retry', 'success')
    } catch (err) {
      toast(`Failed to retry task: ${err}`, 'error')
    }
  }

  const handleCancel = async () => {
    if (!task) return
    const confirmed = await confirm({ title: 'Cancel task?', description: task.title, confirmLabel: 'Cancel Task', variant: 'danger' })
    if (!confirmed) return
    try {
      await cancelTask(task.id)
      setTask(await getTask(task.id))
      toast('Task cancelled', 'success')
    } catch (err) {
      toast(`Failed to cancel task: ${err}`, 'error')
    }
  }

  if (loading) {
    return (
      <div className="p-4 sm:p-6 lg:p-8">
        <LoadingState message="Loading task..." />
      </div>
    )
  }

  if (error || !task) {
    return (
      <div className="p-8">
        <div className="text-red-400" role="alert">Error: {error || 'Task not found'}</div>
        <button onClick={() => navigate('/tasks')} className="mt-4 text-blue-400 hover:underline">
          Back to Tasks
        </button>
      </div>
    )
  }

  const tabs = [
    { id: 'overview' as const, label: 'Overview' },
    { id: 'execution' as const, label: 'Execution' },
    { id: 'output' as const, label: 'Output' },
  ]

  return (
    <div className="p-4 sm:p-6 lg:p-8 space-y-6 max-w-4xl">
      {dialog}

      <TaskHeader task={task} onRetry={handleRetry} onCancel={handleCancel} />

      {/* Triage Analysis */}
      {task.triage_analysis && (
        <div className="card p-6 border border-cyan-600/30">
          <div className="flex items-center gap-2 mb-3">
            <h2 className="text-lg font-semibold text-cyan-400">Triage Analysis</h2>
            <span className="bg-cyan-600/20 text-cyan-400 border border-cyan-600/30 px-2 py-0.5 rounded text-xs font-medium">AI</span>
          </div>
          <div className="text-sm text-slate-300 leading-relaxed">
            <MarkdownRenderer content={task.triage_analysis} />
          </div>
        </div>
      )}

      <TaskSubtasks
        subtasks={subtasks}
        dependencies={dependencies}
        showDAG={showDAG}
        expanded={subtasksExpanded}
        onToggleExpanded={() => setSubtasksExpanded(!subtasksExpanded)}
      />

      {/* Tab Navigation */}
      <div className="flex gap-1 border-b border-slate-700">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`px-4 py-2.5 text-sm font-medium transition-colors border-b-2 -mb-px ${
              activeTab === tab.id
                ? 'text-blue-400 border-blue-400'
                : 'text-slate-400 border-transparent hover:text-slate-200 hover:border-slate-600'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      {activeTab === 'overview' && <TaskOverview task={task} />}
      {activeTab === 'execution' && <TaskExecution task={task} />}
      {activeTab === 'output' && <TaskOutput task={task} />}
    </div>
  )
}
