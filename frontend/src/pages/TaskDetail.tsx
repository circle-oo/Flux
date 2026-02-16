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
import { useTaskStream } from '../hooks/useTaskStream'

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

  // Connect-RPC SSE stream for real-time task events (additive — WS still active)
  const { events: streamEvents, isRunning: streamRunning } = useTaskStream(id)

  // Refresh task when stream delivers terminal events
  useEffect(() => {
    if (!streamRunning && streamEvents.length > 0 && id) {
      refreshTask()
    }
  }, [streamRunning, streamEvents.length, id])

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

  if (loading) return <div className="p-5 sm:p-6 lg:p-8"><LoadingState message="Loading task..." /></div>

  if (error || !task) {
    return (
      <div className="p-5 sm:p-6 lg:p-8 animate-fade-in">
        <div className="text-rose-600 text-sm" role="alert">Error: {error || 'Task not found'}</div>
        <button onClick={() => navigate('/tasks')} className="mt-4 text-primary-400 hover:text-primary-300 text-sm transition-colors">
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
    <div className="p-5 sm:p-6 lg:p-8 space-y-5 max-w-4xl animate-fade-in">
      {dialog}

      <TaskHeader task={task} onRetry={handleRetry} onCancel={handleCancel} />

      {/* Triage Analysis */}
      {task.triage_analysis && (
        <div className="card p-5 ring-1 ring-cyan-200/60 animate-slide-up">
          <div className="flex items-center gap-2 mb-3">
            <h2 className="text-sm font-semibold text-cyan-600">Triage Analysis</h2>
            <span className="badge-info">AI</span>
          </div>
          <div className="text-sm text-content-secondary leading-relaxed">
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
      <div className="flex gap-0.5 border-b border-line">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`px-4 py-2.5 text-xs font-medium tracking-wide uppercase transition-all border-b-2 -mb-px ${
              activeTab === tab.id
                ? 'text-primary-400 border-primary-400'
                : 'text-content-faint border-transparent hover:text-content-secondary hover:border-line-hover'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === 'overview' && <TaskOverview task={task} />}
      {activeTab === 'execution' && <TaskExecution task={task} />}
      {activeTab === 'output' && <TaskOutput task={task} />}
    </div>
  )
}
