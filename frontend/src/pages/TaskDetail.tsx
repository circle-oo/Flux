import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTaskStore } from '../stores/taskStore'
import { useWSStore } from '../stores/wsStore'
import { Task, api } from '../lib/api'
import LoadingState from '../components/LoadingState'
import TaskHeader from '../components/TaskHeader'
import TaskSubtasks from '../components/TaskSubtasks'
import TaskTriage from '../components/TaskTriage'
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
  const [activeTab, setActiveTab] = useState<'triage' | 'execution' | 'output'>('triage')
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

  if (loading) return <div className="p-5 sm:p-6 lg:p-8"><LoadingState message="Loading task..." /></div>

  if (error || !task) {
    return (
      <div className="p-5 sm:p-6 lg:p-8 animate-fade-in">
        <div className="text-rose-600 text-sm" role="alert">Error: {error || 'Task not found'}</div>
        <button onClick={() => navigate('/tasks')} className="mt-4 text-primary-600 hover:text-primary-500 text-sm transition-colors">
          Back to Tasks
        </button>
      </div>
    )
  }

  const tabs = [
    { id: 'triage' as const, label: 'Triage' },
    { id: 'execution' as const, label: 'Execution' },
    { id: 'output' as const, label: 'Output' },
  ]

  return (
    <div className="p-5 sm:p-6 lg:p-8 space-y-5 max-w-4xl animate-fade-in">
      {dialog}

      <TaskHeader task={task} onRetry={handleRetry} onCancel={handleCancel} />

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
                ? 'text-primary-600 border-primary-400'
                : 'text-content-faint border-transparent hover:text-content-secondary hover:border-line-hover'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === 'triage' && <TaskTriage task={task} />}
      {activeTab === 'execution' && <TaskExecution task={task} />}
      {activeTab === 'output' && <TaskOutput task={task} />}
    </div>
  )
}
