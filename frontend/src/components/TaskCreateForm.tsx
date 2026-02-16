import { useState } from 'react'
import { Project, Goal } from '../lib/api'

const MAX_DESCRIPTION_LENGTH = 51200 // Must match backend maxDescriptionLength

const priorityPresets = [
  { value: 10, label: 'Critical', color: 'text-red-400' },
  { value: 25, label: 'High', color: 'text-amber-400' },
  { value: 40, label: 'Normal', color: 'text-blue-400' },
  { value: 65, label: 'Low', color: 'text-slate-400' },
  { value: 85, label: 'Backlog', color: 'text-slate-500' },
]

interface TaskCreateFormProps {
  projects: Project[]
  currentGoal: Goal | null
  onSubmit: (task: {
    title: string
    description: string
    priority: number
    project_id: string
    goal_id?: string
    tags?: string[]
  }) => Promise<void>
  onCancel: () => void
}

export default function TaskCreateForm({ projects, currentGoal, onSubmit, onCancel }: TaskCreateFormProps) {
  const [formData, setFormData] = useState({
    title: '',
    description: '',
    priority: 40,
    project_id: '',
    goal_id: '',
    tags: '',
    prompt: '',
  })
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const activeProjects = projects.filter((p) => p.status === 'ACTIVE')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)

    if (formData.description.length > MAX_DESCRIPTION_LENGTH) {
      setFormError(`Description exceeds maximum length of ${MAX_DESCRIPTION_LENGTH.toLocaleString()} characters (${formData.description.length.toLocaleString()} provided)`)
      return
    }

    try {
      await onSubmit({
        title: formData.title,
        description: formData.description,
        priority: formData.priority,
        project_id: formData.project_id,
        goal_id: formData.goal_id || undefined,
        tags: formData.tags
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      })
      setFormData({
        title: '',
        description: '',
        priority: 40,
        project_id: '',
        goal_id: '',
        tags: '',
        prompt: '',
      })
      setShowAdvanced(false)
      setFormError(null)
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to create task'
      setFormError(message)
    }
  }

  return (
    <div className="card p-4 sm:p-6">
      <h2 className="text-lg sm:text-xl font-semibold text-slate-100 mb-4">
        Create New Task
      </h2>
      <form onSubmit={handleSubmit} className="space-y-4 sm:space-y-5">
        {/* Title */}
        <div>
          <label className="label">Title</label>
          <input
            type="text"
            value={formData.title}
            onChange={(e) => setFormData({ ...formData, title: e.target.value })}
            className="input"
            placeholder="What needs to be done?"
            required
            autoFocus
          />
        </div>

        {/* Description */}
        <div>
          <label className="label">Description</label>
          <textarea
            value={formData.description}
            onChange={(e) => setFormData({ ...formData, description: e.target.value })}
            className="input min-h-[120px] resize-y"
            placeholder="Describe the task in detail. Include acceptance criteria, context, and any constraints. This becomes the executor's primary input."
            required
          />
          <div className="flex items-center justify-between mt-1">
            <p className="text-xs text-slate-500">
              Be specific — this is what the executor agent reads to understand the work.
            </p>
            <span className={`text-xs ${formData.description.length > MAX_DESCRIPTION_LENGTH ? 'text-red-400' : 'text-slate-500'}`}>
              {formData.description.length.toLocaleString()} / {MAX_DESCRIPTION_LENGTH.toLocaleString()}
            </span>
          </div>
        </div>

        {/* Priority */}
        <div>
          <label className="label">
            Priority
            <span className="text-slate-500 font-normal ml-1">({formData.priority})</span>
          </label>
          <div className="flex flex-wrap sm:flex-nowrap gap-2">
            {priorityPresets.map((preset) => (
              <button
                key={preset.value}
                type="button"
                onClick={() => setFormData({ ...formData, priority: preset.value })}
                className={`flex-1 px-2 py-2.5 rounded-lg text-xs sm:text-sm font-medium border transition-colors touch-manipulation min-w-[80px] ${
                  formData.priority === preset.value
                    ? 'bg-slate-600 border-blue-500 text-white'
                    : 'bg-slate-700 border-slate-600 text-slate-400 hover:border-slate-500'
                }`}
              >
                <span className={preset.color}>{preset.label}</span>
              </button>
            ))}
          </div>
        </div>

        {/* Project + Goal row */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <label className="label">Project</label>
            <select
              value={formData.project_id}
              onChange={(e) => setFormData({ ...formData, project_id: e.target.value })}
              className="input"
              required
            >
              <option value="">Select project</option>
              {activeProjects.map((p) => (
                <option key={p.id} value={p.id}>{p.name}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="label">
              Goal
              <span className="text-slate-500 font-normal ml-1">(optional)</span>
            </label>
            <select
              value={formData.goal_id}
              onChange={(e) => setFormData({ ...formData, goal_id: e.target.value })}
              className="input"
            >
              <option value="">
                {currentGoal ? `Active: ${currentGoal.title}` : 'No active goal'}
              </option>
              {currentGoal && (
                <option value={currentGoal.id}>{currentGoal.title}</option>
              )}
            </select>
          </div>
        </div>

        {/* Tags */}
        <div>
          <label className="label">
            Tags
            <span className="text-slate-500 font-normal ml-1">(comma-separated)</span>
          </label>
          <input
            type="text"
            value={formData.tags}
            onChange={(e) => setFormData({ ...formData, tags: e.target.value })}
            className="input"
            placeholder="frontend, urgent, needs-review"
          />
        </div>

        {/* Advanced toggle */}
        <div>
          <button
            type="button"
            onClick={() => setShowAdvanced(!showAdvanced)}
            className="text-sm text-slate-500 hover:text-slate-300 transition-colors"
          >
            {showAdvanced ? '- Hide advanced' : '+ Advanced options'}
          </button>
        </div>

        {/* Advanced: Prompt */}
        {showAdvanced && (
          <div>
            <label className="label">
              Additional Prompt
              <span className="text-slate-500 font-normal ml-1">(optional)</span>
            </label>
            <textarea
              value={formData.prompt}
              onChange={(e) => setFormData({ ...formData, prompt: e.target.value })}
              className="input min-h-[80px] resize-y"
              placeholder="Extra instructions for the executor agent, e.g. 'Use the existing auth middleware' or 'Follow the pattern in services/users.go'"
            />
            <p className="text-xs text-slate-500 mt-1">
              Appended to the executor prompt as additional context.
            </p>
          </div>
        )}

        {/* Error display */}
        {formError && (
          <div className="p-3 bg-red-900/30 border border-red-600 rounded text-sm text-red-200">
            {formError}
          </div>
        )}

        {/* Submit */}
        <div className="flex items-center gap-3 pt-2">
          <button type="submit" className="btn-primary">Create Task</button>
          <button type="button" onClick={onCancel} className="btn-secondary">Cancel</button>
        </div>
      </form>
    </div>
  )
}
