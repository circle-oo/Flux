import { useEffect, useState } from 'react'
import { useGoalStore } from '../stores/goalStore'
import PageHeader from '../components/PageHeader'
import LoadingState from '../components/LoadingState'
import EmptyState from '../components/EmptyState'
import { useConfirm } from '../hooks/useConfirm'
import { useToast } from '../components/Toast'

export default function Goals() {
  const { goals, currentGoal, isLoading, fetchGoals, createGoal, activateGoal } =
    useGoalStore()
  const [showForm, setShowForm] = useState(false)
  const { confirm, dialog } = useConfirm()
  const { toast } = useToast()
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
    const confirmed = await confirm({
      title: 'Activate goal?',
      description: 'Activating this goal will supersede the current goal.',
      confirmLabel: 'Activate',
    })
    if (confirmed) {
      try { await activateGoal(id); toast('Goal activated', 'success') } catch (error) { toast(`Failed to activate goal: ${error}`, 'error') }
    }
  }

  return (
    <div className="p-5 sm:p-6 lg:p-8 space-y-6 animate-fade-in">
      {dialog}
      <PageHeader
        title="Goals"
        subtitle="Manage system goals and objectives"
        action={
          <button
            onClick={() => setShowForm(!showForm)}
            className="btn-primary whitespace-nowrap"
          >
            {showForm ? 'Cancel' : 'New Goal'}
          </button>
        }
      />

      {/* Create Form */}
      {showForm && (
        <div className="card p-5 sm:p-6 animate-slide-up">
          <h2 className="text-base font-semibold text-content mb-4">Create New Goal</h2>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="goal-title" className="label">Title</label>
              <input
                id="goal-title"
                type="text"
                value={formData.title}
                onChange={(e) => setFormData({ ...formData, title: e.target.value })}
                className="input"
                required
                autoFocus
              />
            </div>
            <div>
              <label htmlFor="goal-description" className="label">Description</label>
              <textarea
                id="goal-description"
                value={formData.description}
                onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                className="input h-24 resize-none"
                required
              />
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label htmlFor="goal-priorities" className="label">Priorities (comma-separated)</label>
                <input
                  id="goal-priorities"
                  type="text"
                  value={formData.priorities}
                  onChange={(e) => setFormData({ ...formData, priorities: e.target.value })}
                  className="input"
                  placeholder="High priority, Medium priority"
                />
              </div>
              <div>
                <label htmlFor="goal-metrics" className="label">Metrics (comma-separated)</label>
                <input
                  id="goal-metrics"
                  type="text"
                  value={formData.metrics}
                  onChange={(e) => setFormData({ ...formData, metrics: e.target.value })}
                  className="input"
                  placeholder="Code coverage, Performance"
                />
              </div>
            </div>
            <button type="submit" className="btn-primary">Create Goal</button>
          </form>
        </div>
      )}

      {/* Current Goal */}
      {currentGoal && (
        <div className="card p-5 sm:p-6 relative overflow-hidden ring-1 ring-primary-500/30">
          <div className="absolute inset-0 bg-gradient-to-r from-primary-600/5 to-primary-400/5 pointer-events-none" />
          <div className="relative">
            <div className="flex items-start justify-between mb-3">
              <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest">Active Goal</div>
              <span className="badge-success">Active</span>
            </div>
            <h3 className="text-lg font-semibold text-content mb-2">{currentGoal.title}</h3>
            <p className="text-sm text-content-muted mb-4">{currentGoal.description}</p>
            {currentGoal.priorities.length > 0 && (
              <div className="mb-3">
                <span className="text-[11px] font-medium text-content-faint uppercase tracking-wider">Priorities</span>
                <div className="flex flex-wrap gap-1.5 mt-1.5">
                  {currentGoal.priorities.map((p, i) => (
                    <span key={i} className="badge-info">{p}</span>
                  ))}
                </div>
              </div>
            )}
            {currentGoal.metrics.length > 0 && (
              <div>
                <span className="text-[11px] font-medium text-content-faint uppercase tracking-wider">Metrics</span>
                <div className="flex flex-wrap gap-1.5 mt-1.5">
                  {currentGoal.metrics.map((m, i) => (
                    <span key={i} className="badge-secondary">{m}</span>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Goals List */}
      <div className="space-y-4">
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest">All Goals</div>
        {isLoading ? (
          <LoadingState message="Loading goals..." />
        ) : goals.length === 0 ? (
          <EmptyState title="No goals yet" message="Create one to get started." />
        ) : (
          <div className="space-y-3">
            {goals.map((goal) => (
              <div key={goal.id} className="card p-5 animate-slide-up">
                <div className="flex items-start justify-between mb-3">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1.5">
                      <h3 className="text-base font-medium text-content">{goal.title}</h3>
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
                    <p className="text-sm text-content-muted mb-2">{goal.description}</p>
                    <div className="text-xs text-content-faint">
                      Created: {new Date(goal.created_at).toLocaleString()}
                      {goal.active_since && (
                        <> · Active: {new Date(goal.active_since).toLocaleString()}</>
                      )}
                    </div>
                  </div>
                  {goal.status !== 'ACTIVE' && (
                    <button
                      onClick={() => handleActivate(goal.id)}
                      className="btn-sm btn-primary ml-4"
                    >
                      Activate
                    </button>
                  )}
                </div>
                {goal.priorities.length > 0 && (
                  <div className="flex flex-wrap gap-1.5 mt-3 pt-3 border-t border-line-subtle">
                    {goal.priorities.map((p, i) => (
                      <span key={i} className="badge-info">{p}</span>
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
