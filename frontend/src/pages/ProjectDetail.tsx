import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useProjectStore } from '../stores/projectStore'
import { useTaskStore } from '../stores/taskStore'
import { useGoalStore } from '../stores/goalStore'
import { Project } from '../lib/api'
import { StatusBadge } from '../components/StatusBadge'
import { formatDate } from '../lib/utils'

export default function ProjectDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { getProject } = useProjectStore()
  const { tasks, setFilters } = useTaskStore()
  const { goals } = useGoalStore()
  const [project, setProject] = useState<Project | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return

    const loadProject = async () => {
      setLoading(true)
      setError(null)
      try {
        const projectData = await getProject(id)
        setProject(projectData)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load project')
      } finally {
        setLoading(false)
      }
    }

    loadProject()
    setFilters({ project_id: id })
  }, [id, getProject, setFilters])

  if (loading) {
    return (
      <div className="p-4 sm:p-6 lg:p-8">
        <div className="text-slate-400">Loading project...</div>
      </div>
    )
  }

  if (error || !project) {
    return (
      <div className="p-4 sm:p-6 lg:p-8">
        <div className="card p-6">
          <h2 className="text-xl font-semibold text-red-400 mb-2">Error</h2>
          <p className="text-slate-300">{error || 'Project not found'}</p>
          <button onClick={() => navigate('/projects')} className="btn-primary mt-4">
            Back to Projects
          </button>
        </div>
      </div>
    )
  }

  const projectTasks = tasks.filter((t) => t.project_id === id)
  const goal = project.goal_id ? goals.find((g) => g.id === project.goal_id) : null

  const tasksByStatus = {
    PENDING: projectTasks.filter((t) => t.status === 'PENDING').length,
    READY: projectTasks.filter((t) => t.status === 'READY').length,
    RUNNING: projectTasks.filter((t) => t.status === 'RUNNING').length,
    COMPLETED: projectTasks.filter((t) => t.status === 'COMPLETED').length,
    FAILED: projectTasks.filter((t) => t.status === 'FAILED').length,
    DECOMPOSED: projectTasks.filter((t) => t.status === 'DECOMPOSED').length,
  }

  return (
    <div className="p-4 sm:p-6 lg:p-8 space-y-6 lg:space-y-8">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="flex-1 min-w-0">
          <button
            onClick={() => navigate('/projects')}
            className="text-slate-400 hover:text-slate-300 mb-3 flex items-center gap-2 text-sm"
          >
            ← Back to Projects
          </button>
          <div className="flex items-center gap-3 mb-2">
            <h1 className="text-2xl sm:text-3xl font-bold text-slate-100">
              {project.name}
            </h1>
            <span
              className={`${
                project.status === 'ACTIVE'
                  ? 'badge-success'
                  : project.status === 'PROPOSED'
                  ? 'badge-warning'
                  : project.status === 'REJECTED'
                  ? 'badge-danger'
                  : 'badge-secondary'
              }`}
            >
              {project.status}
            </span>
            <span className="badge-secondary">{project.type}</span>
          </div>
          <p className="text-sm sm:text-base text-slate-400">
            {project.description}
          </p>
        </div>
      </div>

      {/* Project Info Card */}
      <div className="card p-6">
        <h2 className="text-lg font-semibold text-slate-100 mb-4">
          Project Information
        </h2>
        <div className="space-y-4">
          {project.repo_url && (
            <div>
              <span className="text-sm text-slate-500 font-medium">Repository</span>
              <div className="mt-1">
                <a
                  href={project.repo_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-400 hover:underline text-sm"
                >
                  {project.repo_url}
                </a>
              </div>
            </div>
          )}

          <div>
            <span className="text-sm text-slate-500 font-medium">Vault Path</span>
            <div className="mt-1 text-sm text-slate-300 font-mono">
              {project.vault_path}
            </div>
          </div>

          {project.tech_stack.length > 0 && (
            <div>
              <span className="text-sm text-slate-500 font-medium">Tech Stack</span>
              <div className="flex flex-wrap gap-2 mt-2">
                {project.tech_stack.map((tech, i) => (
                  <span key={i} className="badge-info text-xs">
                    {tech}
                  </span>
                ))}
              </div>
            </div>
          )}

          {project.inspiration && (
            <div>
              <span className="text-sm text-slate-500 font-medium">Inspiration</span>
              <div className="mt-1 text-sm text-slate-300 italic">
                {project.inspiration}
              </div>
            </div>
          )}

          {goal && (
            <div>
              <span className="text-sm text-slate-500 font-medium">Linked Goal</span>
              <div
                className="mt-2 p-3 rounded-lg bg-slate-700/30 cursor-pointer hover:bg-slate-700/50 transition-colors"
                onClick={() => navigate('/goals')}
              >
                <h4 className="text-sm font-semibold text-blue-400 mb-1">
                  {goal.title}
                </h4>
                <p className="text-xs text-slate-400 line-clamp-2">
                  {goal.description}
                </p>
              </div>
            </div>
          )}

          <div className="grid grid-cols-2 gap-4 pt-2">
            <div>
              <span className="text-sm text-slate-500 font-medium">Created</span>
              <div className="mt-1 text-sm text-slate-300">
                {formatDate(project.created_at)}
              </div>
            </div>
            {project.updated_at && (
              <div>
                <span className="text-sm text-slate-500 font-medium">Updated</span>
                <div className="mt-1 text-sm text-slate-300">
                  {formatDate(project.updated_at)}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Task Statistics */}
      <div className="card p-6">
        <h2 className="text-lg font-semibold text-slate-100 mb-4">
          Task Statistics
        </h2>
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
          <div className="p-4 rounded-lg bg-slate-700/30">
            <div className="text-xs text-slate-400 mb-1">Total</div>
            <div className="text-2xl font-bold text-slate-100">
              {projectTasks.length}
            </div>
          </div>
          {tasksByStatus.PENDING > 0 && (
            <div className="p-4 rounded-lg bg-slate-700/30">
              <div className="text-xs text-slate-400 mb-1">Pending</div>
              <div className="text-2xl font-bold text-slate-400">
                {tasksByStatus.PENDING}
              </div>
            </div>
          )}
          {tasksByStatus.READY > 0 && (
            <div className="p-4 rounded-lg bg-slate-700/30">
              <div className="text-xs text-slate-400 mb-1">Ready</div>
              <div className="text-2xl font-bold text-blue-400">
                {tasksByStatus.READY}
              </div>
            </div>
          )}
          {tasksByStatus.RUNNING > 0 && (
            <div className="p-4 rounded-lg bg-slate-700/30">
              <div className="text-xs text-slate-400 mb-1">Running</div>
              <div className="text-2xl font-bold text-amber-400">
                {tasksByStatus.RUNNING}
              </div>
            </div>
          )}
          {tasksByStatus.COMPLETED > 0 && (
            <div className="p-4 rounded-lg bg-slate-700/30">
              <div className="text-xs text-slate-400 mb-1">Completed</div>
              <div className="text-2xl font-bold text-green-400">
                {tasksByStatus.COMPLETED}
              </div>
            </div>
          )}
          {tasksByStatus.FAILED > 0 && (
            <div className="p-4 rounded-lg bg-slate-700/30">
              <div className="text-xs text-slate-400 mb-1">Failed</div>
              <div className="text-2xl font-bold text-red-400">
                {tasksByStatus.FAILED}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Tasks List */}
      <div className="card p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-slate-100">Tasks</h2>
          <button
            onClick={() => navigate('/tasks?project_id=' + id)}
            className="text-sm text-blue-400 hover:text-blue-300"
          >
            View all →
          </button>
        </div>
        {projectTasks.length === 0 ? (
          <p className="text-slate-500 italic text-center py-8">
            No tasks for this project yet
          </p>
        ) : (
          <div className="space-y-2">
            {projectTasks.slice(0, 10).map((task) => (
              <div
                key={task.id}
                onClick={() => navigate(`/tasks/${task.id}`)}
                className="p-4 rounded-lg bg-slate-700/30 hover:bg-slate-700/60 cursor-pointer transition-colors"
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <h3 className="text-sm font-medium text-slate-100 truncate">
                        {task.title}
                      </h3>
                      <StatusBadge status={task.status} size="sm" />
                    </div>
                    <p className="text-xs text-slate-400 line-clamp-2">
                      {task.description}
                    </p>
                  </div>
                  <div className="text-xs text-slate-500 shrink-0">
                    {formatDate(task.created_at)}
                  </div>
                </div>
              </div>
            ))}
            {projectTasks.length > 10 && (
              <div className="text-center pt-2">
                <button
                  onClick={() => navigate('/tasks?project_id=' + id)}
                  className="text-sm text-blue-400 hover:text-blue-300"
                >
                  Show all {projectTasks.length} tasks →
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
