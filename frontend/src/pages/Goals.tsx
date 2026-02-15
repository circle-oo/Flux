import { useEffect, useState } from 'react'
import { useGoalStore } from '../stores/goalStore'

export default function Goals() {
  const { goals, currentGoal, isLoading, fetchGoals, createGoal, activateGoal } =
    useGoalStore()
  const [showForm, setShowForm] = useState(false)
  const [formData, setFormData] = useState({
    title: '',
    description: '',
    priorities: '',
    metrics: '',
  })

  useEffect(() => {
    fetchGoals()
  }, [fetchGoals])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await createGoal({
        title: formData.title,
        description: formData.description,
        priorities: formData.priorities
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
        metrics: formData.metrics
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      })
      setShowForm(false)
      setFormData({ title: '', description: '', priorities: '', metrics: '' })
    } catch (error) {
      console.error('Failed to create goal:', error)
    }
  }

  const handleActivate = async (id: string) => {
    if (
      confirm(
        'Activating this goal will supersede the current goal. Continue?'
      )
    ) {
      try {
        await activateGoal(id)
      } catch (error) {
        console.error('Failed to activate goal:', error)
      }
    }
  }

  return (
    <div className="p-8 space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-slate-100 mb-2">Goals</h1>
          <p className="text-slate-400">Manage system goals and objectives</p>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="btn-primary"
        >
          {showForm ? 'Cancel' : '+ New Goal'}
        </button>
      </div>

      {/* Create Form */}
      {showForm && (
        <div className="card p-6">
          <h2 className="text-xl font-semibold text-slate-100 mb-4">
            Create New Goal
          </h2>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="label">Title</label>
              <input
                type="text"
                value={formData.title}
                onChange={(e) =>
                  setFormData({ ...formData, title: e.target.value })
                }
                className="input"
                required
              />
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
              <label className="label">
                Priorities (comma-separated)
              </label>
              <input
                type="text"
                value={formData.priorities}
                onChange={(e) =>
                  setFormData({ ...formData, priorities: e.target.value })
                }
                className="input"
                placeholder="High priority, Medium priority"
              />
            </div>
            <div>
              <label className="label">
                Metrics (comma-separated)
              </label>
              <input
                type="text"
                value={formData.metrics}
                onChange={(e) =>
                  setFormData({ ...formData, metrics: e.target.value })
                }
                className="input"
                placeholder="Code coverage, Performance"
              />
            </div>
            <button type="submit" className="btn-primary">
              Create Goal
            </button>
          </form>
        </div>
      )}

      {/* Current Goal */}
      {currentGoal && (
        <div className="card p-6 border-2 border-blue-500">
          <div className="flex items-start justify-between mb-4">
            <h2 className="text-xl font-semibold text-slate-100">
              Active Goal
            </h2>
            <span className="badge-success">ACTIVE</span>
          </div>
          <h3 className="text-lg font-medium text-blue-400 mb-2">
            {currentGoal.title}
          </h3>
          <p className="text-slate-300 mb-4">{currentGoal.description}</p>
          {currentGoal.priorities.length > 0 && (
            <div className="mb-2">
              <span className="text-sm font-medium text-slate-400">
                Priorities:
              </span>
              <div className="flex flex-wrap gap-2 mt-1">
                {currentGoal.priorities.map((p, i) => (
                  <span key={i} className="badge-info">
                    {p}
                  </span>
                ))}
              </div>
            </div>
          )}
          {currentGoal.metrics.length > 0 && (
            <div className="mb-2">
              <span className="text-sm font-medium text-slate-400">
                Metrics:
              </span>
              <div className="flex flex-wrap gap-2 mt-1">
                {currentGoal.metrics.map((m, i) => (
                  <span key={i} className="badge-secondary">
                    {m}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Goals List */}
      <div className="space-y-4">
        <h2 className="text-xl font-semibold text-slate-100">All Goals</h2>
        {isLoading ? (
          <div className="text-slate-400">Loading...</div>
        ) : goals.length === 0 ? (
          <div className="card p-6 text-center text-slate-400">
            No goals yet. Create one to get started.
          </div>
        ) : (
          <div className="space-y-3">
            {goals.map((goal) => (
              <div key={goal.id} className="card p-6">
                <div className="flex items-start justify-between mb-3">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-2">
                      <h3 className="text-lg font-medium text-slate-100">
                        {goal.title}
                      </h3>
                      <span
                        className={`badge ${
                          goal.status === 'ACTIVE'
                            ? 'badge-success'
                            : goal.status === 'COMPLETED'
                            ? 'badge-info'
                            : goal.status === 'SUPERSEDED'
                            ? 'badge-secondary'
                            : 'badge-warning'
                        }`}
                      >
                        {goal.status}
                      </span>
                    </div>
                    <p className="text-slate-300 mb-3">{goal.description}</p>
                    <div className="text-sm text-slate-500">
                      Created: {new Date(goal.created_at).toLocaleString()}
                      {goal.active_since && (
                        <> • Active: {new Date(goal.active_since).toLocaleString()}</>
                      )}
                    </div>
                  </div>
                  {goal.status !== 'ACTIVE' && (
                    <button
                      onClick={() => handleActivate(goal.id)}
                      className="btn-primary ml-4"
                    >
                      Activate
                    </button>
                  )}
                </div>
                {goal.priorities.length > 0 && (
                  <div className="flex flex-wrap gap-2 mt-3">
                    {goal.priorities.map((p, i) => (
                      <span key={i} className="badge-info">
                        {p}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
