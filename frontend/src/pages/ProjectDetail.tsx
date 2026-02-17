import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useProjectStore } from '../stores/projectStore'
import { useTaskStore } from '../stores/taskStore'
import { useGoalStore } from '../stores/goalStore'
import { Project } from '../lib/api'
import { StatusBadge } from '../components/StatusBadge'
import { formatDate, countByStatus } from '../lib/utils'
import BackButton from '../components/BackButton'
import LoadingState from '../components/LoadingState'
import StatCard from '../components/StatCard'

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
      <div className="p-5 sm:p-6 lg:p-8">
        <LoadingState message="Loading project..." />
      </div>
    )
  }

  if (error || !project) {
    return (
      <div className="p-5 sm:p-6 lg:p-8">
        <div className="card p-6">
          <h2 className="text-base font-semibold text-rose-600 mb-2">Error</h2>
          <p className="text-sm text-content-secondary">{error || 'Project not found'}</p>
          <button onClick={() => navigate('/projects')} className="btn-primary mt-4">
            Back to Projects
          </button>
        </div>
      </div>
    )
  }

  const projectTasks = tasks.filter((t) => t.project_id === id)
  const goal = project.goal_id ? goals.find((g) => g.id === project.goal_id) : null
  const taskCounts = countByStatus(projectTasks)

  return (
    <div className="p-5 sm:p-6 lg:p-8 space-y-6 animate-fade-in">
      <div>
        <BackButton to="/projects" label="Back to Projects" />
        <div className="flex items-center gap-3 mb-2 mt-1">
          <h1 className="text-xl sm:text-2xl font-bold text-content">
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
        <p className="text-sm text-content-muted">{project.description}</p>
      </div>

      <div className="card p-5">
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">Project Information</div>
        <div className="space-y-4">
          {project.repo_url && (
            <div>
              <span className="text-[11px] text-content-faint uppercase tracking-wider font-medium">Repository</span>
              <div className="mt-1">
                <a
                  href={project.repo_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary-600 hover:text-primary-500 text-sm transition-colors"
                >
                  {project.repo_url}
                </a>
              </div>
            </div>
          )}

          <div>
            <span className="text-[11px] text-content-faint uppercase tracking-wider font-medium">Vault Path</span>
            <div className="mt-1 text-sm text-content-secondary font-mono">
              {project.vault_path}
            </div>
          </div>

          {project.tech_stack.length > 0 && (
            <div>
              <span className="text-[11px] text-content-faint uppercase tracking-wider font-medium">Tech Stack</span>
              <div className="flex flex-wrap gap-1.5 mt-2">
                {project.tech_stack.map((tech, i) => (
                  <span key={i} className="badge-info">
                    {tech}
                  </span>
                ))}
              </div>
            </div>
          )}

          {project.inspiration && (
            <div>
              <span className="text-[11px] text-content-faint uppercase tracking-wider font-medium">Inspiration</span>
              <div className="mt-1 text-sm text-content-muted italic">
                {project.inspiration}
              </div>
            </div>
          )}

          {goal && (
            <div>
              <span className="text-[11px] text-content-faint uppercase tracking-wider font-medium">Linked Goal</span>
              <button
                type="button"
                className="mt-2 p-3 rounded-lg bg-surface-hover hover:bg-surface-active border border-line transition-colors w-full text-left"
                onClick={() => navigate('/goals')}
              >
                <h4 className="text-sm font-semibold text-primary-600 mb-1">
                  {goal.title}
                </h4>
                <p className="text-xs text-content-muted line-clamp-2">
                  {goal.description}
                </p>
              </button>
            </div>
          )}

          <div className="grid grid-cols-2 gap-4 pt-2">
            <div>
              <span className="text-[11px] text-content-faint uppercase tracking-wider font-medium">Created</span>
              <div className="mt-1 text-sm text-content-secondary">
                {formatDate(project.created_at)}
              </div>
            </div>
            {project.updated_at && (
              <div>
                <span className="text-[11px] text-content-faint uppercase tracking-wider font-medium">Updated</span>
                <div className="mt-1 text-sm text-content-secondary">
                  {formatDate(project.updated_at)}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="card p-5">
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">Task Statistics</div>
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
          <StatCard label="Total" value={projectTasks.length} color="text-content" />
          {(taskCounts['PENDING'] || 0) > 0 && (
            <StatCard label="Pending" value={taskCounts['PENDING'] || 0} color="text-content-muted" />
          )}
          {(taskCounts['READY'] || 0) > 0 && (
            <StatCard label="Ready" value={taskCounts['READY'] || 0} color="text-cyan-600" />
          )}
          {(taskCounts['RUNNING'] || 0) > 0 && (
            <StatCard label="Running" value={taskCounts['RUNNING'] || 0} color="text-amber-600" />
          )}
          {(taskCounts['COMPLETED'] || 0) > 0 && (
            <StatCard label="Completed" value={taskCounts['COMPLETED'] || 0} color="text-emerald-600" />
          )}
          {(taskCounts['FAILED'] || 0) > 0 && (
            <StatCard label="Failed" value={taskCounts['FAILED'] || 0} color="text-rose-600" />
          )}
        </div>
      </div>

      <div className="card p-5">
        <div className="flex items-center justify-between mb-4">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest">Tasks</div>
          <button
            onClick={() => navigate('/tasks?project_id=' + id)}
            className="text-xs text-primary-600 hover:text-primary-500 transition-colors"
          >
            View all →
          </button>
        </div>
        {projectTasks.length === 0 ? (
          <p className="text-content-faint italic text-center py-8 text-sm">
            No tasks for this project yet
          </p>
        ) : (
          <div className="space-y-1.5">
            {projectTasks.slice(0, 10).map((task) => (
              <button
                key={task.id}
                type="button"
                onClick={() => navigate(`/tasks/${task.id}`)}
                className="p-3 rounded-lg bg-surface-hover hover:bg-surface-hover border border-transparent hover:border-line transition-all w-full text-left"
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <h3 className="text-sm font-medium text-content truncate">
                        {task.title}
                      </h3>
                      <StatusBadge status={task.status} size="sm" />
                    </div>
                    <p className="text-xs text-content-faint line-clamp-2">
                      {task.description}
                    </p>
                  </div>
                  <div className="text-xs text-content-faint shrink-0">
                    {formatDate(task.created_at)}
                  </div>
                </div>
              </button>
            ))}
            {projectTasks.length > 10 && (
              <div className="text-center pt-2">
                <button
                  onClick={() => navigate('/tasks?project_id=' + id)}
                  className="text-xs text-primary-600 hover:text-primary-500 transition-colors"
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
