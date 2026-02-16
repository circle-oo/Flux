import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useProjectStore } from '../stores/projectStore'
import { useGoalStore } from '../stores/goalStore'
import { Project } from '../lib/api'
import PageHeader from '../components/PageHeader'
import LoadingState from '../components/LoadingState'
import EmptyState from '../components/EmptyState'
import { useConfirm } from '../hooks/useConfirm'
import { useToast } from '../components/Toast'

export default function Projects() {
  const navigate = useNavigate()
  const {
    projects,
    isLoading,
    fetchProjects,
    createProject,
    approveProject,
    rejectProject,
  } = useProjectStore()
  const { goals, fetchGoals } = useGoalStore()
  const [showForm, setShowForm] = useState(false)
  const { confirm, dialog } = useConfirm()
  const { toast } = useToast()
  const [formData, setFormData] = useState({
    name: '',
    type: 'LIBRARY' as Project['type'],
    description: '',
    repo_url: '',
    tech_stack: '',
    inspiration: '',
    goal_id: '',
  })

  useEffect(() => {
    fetchProjects()
    fetchGoals()
  }, [fetchProjects, fetchGoals])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await createProject({
        name: formData.name,
        type: formData.type,
        description: formData.description,
        tech_stack: formData.tech_stack
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
        inspiration: formData.inspiration || undefined,
        repo_url: formData.repo_url || undefined,
        goal_id: formData.goal_id || undefined,
      })
      setShowForm(false)
      setFormData({
        name: '',
        type: 'LIBRARY',
        description: '',
        repo_url: '',
        tech_stack: '',
        inspiration: '',
        goal_id: '',
      })
    } catch (error) {
      console.error('Failed to create project:', error)
    }
  }

  const handleApprove = async (id: string, name: string) => {
    const confirmed = await confirm({ title: 'Approve project?', description: name, confirmLabel: 'Approve' })
    if (confirmed) {
      try { await approveProject(id); toast('Project approved', 'success') } catch (error) { toast(`Failed: ${error}`, 'error') }
    }
  }

  const handleReject = async (id: string, name: string) => {
    const confirmed = await confirm({ title: 'Reject project?', description: name, confirmLabel: 'Reject', variant: 'danger' })
    if (confirmed) {
      try { await rejectProject(id); toast('Project rejected', 'success') } catch (error) { toast(`Failed: ${error}`, 'error') }
    }
  }

  const proposedProjects = projects.filter((p) => p.status === 'PROPOSED')
  const activeProjects = projects.filter((p) => p.status === 'ACTIVE')
  const otherProjects = projects.filter(
    (p) => p.status !== 'PROPOSED' && p.status !== 'ACTIVE'
  )

  return (
    <div className="p-5 sm:p-6 lg:p-8 space-y-6 animate-fade-in">
      {dialog}
      <PageHeader
        title="Projects"
        subtitle="Register and manage projects"
        action={
          <button
            onClick={() => setShowForm(!showForm)}
            className="btn-primary whitespace-nowrap"
          >
            {showForm ? 'Cancel' : 'Register Project'}
          </button>
        }
      />

      {showForm && (
        <div className="card p-5 sm:p-6 animate-slide-up">
          <h2 className="text-base font-semibold text-white/90 mb-4">
            Register New Project
          </h2>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label htmlFor="project-name" className="label">Name</label>
                <input
                  id="project-name"
                  type="text"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  className="input"
                  required
                  autoFocus
                />
              </div>
              <div>
                <label htmlFor="project-type" className="label">Type</label>
                <select
                  id="project-type"
                  value={formData.type}
                  onChange={(e) => setFormData({ ...formData, type: e.target.value as Project['type'] })}
                  className="input"
                >
                  <option value="REPO">Repo</option>
                  <option value="SERVICE">Service</option>
                  <option value="LIBRARY">Library</option>
                  <option value="TOOL">Tool</option>
                </select>
              </div>
            </div>
            <div>
              <label htmlFor="project-description" className="label">Description</label>
              <textarea
                id="project-description"
                value={formData.description}
                onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                className="input h-24 resize-none"
                required
              />
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label htmlFor="project-repo" className="label">Repository URL</label>
                <input
                  id="project-repo"
                  type="text"
                  value={formData.repo_url}
                  onChange={(e) => setFormData({ ...formData, repo_url: e.target.value })}
                  className="input"
                  placeholder="https://github.com/..."
                />
              </div>
              <div>
                <label htmlFor="project-techstack" className="label">Tech Stack (comma-separated)</label>
                <input
                  id="project-techstack"
                  type="text"
                  value={formData.tech_stack}
                  onChange={(e) => setFormData({ ...formData, tech_stack: e.target.value })}
                  className="input"
                  placeholder="Go, React, SQLite"
                  required
                />
              </div>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label htmlFor="project-inspiration" className="label">Inspiration</label>
                <input
                  id="project-inspiration"
                  type="text"
                  value={formData.inspiration}
                  onChange={(e) => setFormData({ ...formData, inspiration: e.target.value })}
                  className="input"
                  placeholder="Inspired by..."
                />
              </div>
              <div>
                <label htmlFor="project-goal" className="label">Link to Goal</label>
                <select
                  id="project-goal"
                  value={formData.goal_id}
                  onChange={(e) => setFormData({ ...formData, goal_id: e.target.value })}
                  className="input"
                >
                  <option value="">None</option>
                  {goals
                    .filter((g) => g.status === 'ACTIVE')
                    .map((goal) => (
                      <option key={goal.id} value={goal.id}>
                        {goal.title}
                      </option>
                    ))}
                </select>
              </div>
            </div>
            <button type="submit" className="btn-primary">
              Register Project
            </button>
          </form>
        </div>
      )}

      {proposedProjects.length > 0 && (
        <div className="space-y-3">
          <div className="text-[11px] font-medium text-white/30 uppercase tracking-widest">Proposed</div>
          {proposedProjects.map((project) => (
            <div
              key={project.id}
              className="card p-5 ring-1 ring-amber-500/20"
            >
              <div className="flex items-start justify-between mb-3">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1.5">
                    <h3 className="text-base font-medium text-white/90">
                      {project.name}
                    </h3>
                    <span className="badge-warning">Proposed</span>
                    <span className="badge-secondary">{project.type}</span>
                  </div>
                  <p className="text-sm text-white/50 mb-2">
                    {project.description}
                  </p>
                  <div className="text-xs text-white/20 mb-2">
                    Created: {new Date(project.created_at).toLocaleString()}
                  </div>
                  {project.repo_url && (
                    <div className="mb-2">
                      <a
                        href={project.repo_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-accent-400 hover:text-accent-300 text-xs transition-colors"
                      >
                        {project.repo_url}
                      </a>
                    </div>
                  )}
                  {project.tech_stack.length > 0 && (
                    <div className="flex flex-wrap gap-1.5">
                      {project.tech_stack.map((tech, i) => (
                        <span key={i} className="badge-info">
                          {tech}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
                <div className="flex gap-2 ml-4">
                  <button
                    onClick={() => handleApprove(project.id, project.name)}
                    className="btn-sm btn-success"
                  >
                    Approve
                  </button>
                  <button
                    onClick={() => handleReject(project.id, project.name)}
                    className="btn-sm btn-danger"
                  >
                    Reject
                  </button>
                </div>
              </div>
              {project.inspiration && (
                <div className="mt-3 text-xs text-white/30 italic">
                  Inspiration: {project.inspiration}
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      <div className="space-y-3">
        <div className="text-[11px] font-medium text-white/30 uppercase tracking-widest">Active Projects</div>
        {isLoading ? (
          <LoadingState message="Loading projects..." />
        ) : activeProjects.length === 0 ? (
          <EmptyState title="No active projects" message="Register a project to get started." />
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {activeProjects.map((project) => (
              <button
                key={project.id}
                type="button"
                className="card p-5 w-full text-left group"
                onClick={() => navigate(`/projects/${project.id}`)}
              >
                <div className="flex items-center gap-2 mb-1.5">
                  <h3 className="text-sm font-medium text-white/90 group-hover:text-accent-400 transition-colors truncate">
                    {project.name}
                  </h3>
                  <span className="badge-success">Active</span>
                  <span className="badge-secondary">{project.type}</span>
                </div>
                <p className="text-xs text-white/40 mb-3 line-clamp-2">
                  {project.description}
                </p>
                {project.tech_stack.length > 0 && (
                  <div className="flex flex-wrap gap-1.5">
                    {project.tech_stack.map((tech, i) => (
                      <span key={i} className="badge-info">
                        {tech}
                      </span>
                    ))}
                  </div>
                )}
              </button>
            ))}
          </div>
        )}
      </div>

      {otherProjects.length > 0 && (
        <div className="space-y-3">
          <div className="text-[11px] font-medium text-white/30 uppercase tracking-widest">Other Projects</div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {otherProjects.map((project) => (
              <button
                key={project.id}
                type="button"
                className="card p-5 w-full text-left opacity-60 hover:opacity-100 transition-all group"
                onClick={() => navigate(`/projects/${project.id}`)}
              >
                <div className="flex items-center gap-2 mb-1.5">
                  <h3 className="text-sm font-medium text-white/90 truncate">
                    {project.name}
                  </h3>
                  <span
                    className={`badge ${
                      project.status === 'ARCHIVED'
                        ? 'badge-secondary'
                        : 'badge-danger'
                    }`}
                  >
                    {project.status}
                  </span>
                  <span className="badge-secondary">{project.type}</span>
                </div>
                <p className="text-xs text-white/40 line-clamp-2">
                  {project.description}
                </p>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
