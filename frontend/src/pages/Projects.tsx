import { useEffect, useState } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { useGoalStore } from '../stores/goalStore'
import { Project } from '../lib/api'

export default function Projects() {
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
    if (confirm(`Approve project: ${name}?`)) {
      try {
        await approveProject(id)
      } catch (error) {
        console.error('Failed to approve project:', error)
      }
    }
  }

  const handleReject = async (id: string, name: string) => {
    if (confirm(`Reject project: ${name}?`)) {
      try {
        await rejectProject(id)
      } catch (error) {
        console.error('Failed to reject project:', error)
      }
    }
  }

  const proposedProjects = projects.filter((p) => p.status === 'PROPOSED')
  const activeProjects = projects.filter((p) => p.status === 'ACTIVE')
  const otherProjects = projects.filter(
    (p) => p.status !== 'PROPOSED' && p.status !== 'ACTIVE'
  )

  return (
    <div className="p-8 space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-slate-100 mb-2">Projects</h1>
          <p className="text-slate-400">Register and manage projects</p>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="btn-primary"
        >
          {showForm ? 'Cancel' : '+ Register Project'}
        </button>
      </div>

      {/* Create Form */}
      {showForm && (
        <div className="card p-6">
          <h2 className="text-xl font-semibold text-slate-100 mb-4">
            Register New Project
          </h2>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="label">Name</label>
              <input
                type="text"
                value={formData.name}
                onChange={(e) =>
                  setFormData({ ...formData, name: e.target.value })
                }
                className="input"
                required
              />
            </div>
            <div>
              <label className="label">Type</label>
              <select
                value={formData.type}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    type: e.target.value as Project['type'],
                  })
                }
                className="input"
              >
                <option value="REPO">Repo</option>
                <option value="SERVICE">Service</option>
                <option value="LIBRARY">Library</option>
                <option value="TOOL">Tool</option>
              </select>
            </div>
            <div>
              <label className="label">Description</label>
              <textarea
                value={formData.description}
                onChange={(e) =>
                  setFormData({ ...formData, description: e.target.value })
                }
                className="input h-24 resize-none"
                required
              />
            </div>
            <div>
              <label className="label">Repository URL (optional)</label>
              <input
                type="text"
                value={formData.repo_url}
                onChange={(e) =>
                  setFormData({ ...formData, repo_url: e.target.value })
                }
                className="input"
                placeholder="https://github.com/..."
              />
            </div>
            <div>
              <label className="label">Tech Stack (comma-separated)</label>
              <input
                type="text"
                value={formData.tech_stack}
                onChange={(e) =>
                  setFormData({ ...formData, tech_stack: e.target.value })
                }
                className="input"
                placeholder="Go, React, SQLite"
                required
              />
            </div>
            <div>
              <label className="label">Inspiration (optional)</label>
              <input
                type="text"
                value={formData.inspiration}
                onChange={(e) =>
                  setFormData({ ...formData, inspiration: e.target.value })
                }
                className="input"
                placeholder="Inspired by..."
              />
            </div>
            <div>
              <label className="label">Link to Goal (optional)</label>
              <select
                value={formData.goal_id}
                onChange={(e) =>
                  setFormData({ ...formData, goal_id: e.target.value })
                }
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
            <button type="submit" className="btn-primary">
              Register Project
            </button>
          </form>
        </div>
      )}

      {/* Proposed Projects */}
      {proposedProjects.length > 0 && (
        <div className="space-y-4">
          <h2 className="text-xl font-semibold text-slate-100">
            Proposed Projects
          </h2>
          <div className="space-y-3">
            {proposedProjects.map((project) => (
              <div
                key={project.id}
                className="card p-6 border-2 border-amber-500"
              >
                <div className="flex items-start justify-between mb-3">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-2">
                      <h3 className="text-lg font-medium text-slate-100">
                        {project.name}
                      </h3>
                      <span className="badge-warning">PROPOSED</span>
                      <span className="badge-secondary">{project.type}</span>
                    </div>
                    <p className="text-slate-300 mb-3">
                      {project.description}
                    </p>
                    <div className="text-sm text-slate-500 mb-3">
                      Created: {new Date(project.created_at).toLocaleString()}
                    </div>
                    {project.repo_url && (
                      <div className="mb-2">
                        <a
                          href={project.repo_url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-blue-400 hover:underline text-sm"
                        >
                          {project.repo_url}
                        </a>
                      </div>
                    )}
                    {project.tech_stack.length > 0 && (
                      <div className="flex flex-wrap gap-2">
                        {project.tech_stack.map((tech, i) => (
                          <span key={i} className="badge-info text-xs">
                            {tech}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                  <div className="flex gap-2 ml-4">
                    <button
                      onClick={() => handleApprove(project.id, project.name)}
                      className="btn-success"
                    >
                      Approve
                    </button>
                    <button
                      onClick={() => handleReject(project.id, project.name)}
                      className="btn-danger"
                    >
                      Reject
                    </button>
                  </div>
                </div>
                {project.inspiration && (
                  <div className="mt-3 text-sm text-slate-400 italic">
                    Inspiration: {project.inspiration}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Active Projects */}
      <div className="space-y-4">
        <h2 className="text-xl font-semibold text-slate-100">
          Active Projects
        </h2>
        {isLoading ? (
          <div className="text-slate-400">Loading...</div>
        ) : activeProjects.length === 0 ? (
          <div className="card p-6 text-center text-slate-400">
            No active projects yet.
          </div>
        ) : (
          <div className="space-y-3">
            {activeProjects.map((project) => (
              <div key={project.id} className="card p-6">
                <div className="flex items-start justify-between mb-3">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-2">
                      <h3 className="text-lg font-medium text-slate-100">
                        {project.name}
                      </h3>
                      <span className="badge-success">ACTIVE</span>
                      <span className="badge-secondary">{project.type}</span>
                    </div>
                    <p className="text-slate-300 mb-3">
                      {project.description}
                    </p>
                    {project.repo_url && (
                      <div className="mb-2">
                        <a
                          href={project.repo_url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-blue-400 hover:underline text-sm"
                        >
                          {project.repo_url}
                        </a>
                      </div>
                    )}
                    {project.tech_stack.length > 0 && (
                      <div className="flex flex-wrap gap-2 mt-3">
                        {project.tech_stack.map((tech, i) => (
                          <span key={i} className="badge-info text-xs">
                            {tech}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Other Projects */}
      {otherProjects.length > 0 && (
        <div className="space-y-4">
          <h2 className="text-xl font-semibold text-slate-100">
            Other Projects
          </h2>
          <div className="space-y-3">
            {otherProjects.map((project) => (
              <div key={project.id} className="card p-6 opacity-75">
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-2">
                      <h3 className="text-lg font-medium text-slate-100">
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
                    <p className="text-slate-300">{project.description}</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
