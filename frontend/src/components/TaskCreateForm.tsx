import { useState } from 'react'
import { Project, Goal } from '../lib/api'

const MAX_DESCRIPTION_LENGTH = 51200

const priorityPresets = [
  { value: 10, label: 'Critical', color: 'text-rose-400' },
  { value: 25, label: 'High', color: 'text-amber-400' },
  { value: 40, label: 'Normal', color: 'text-cyan-400' },
  { value: 65, label: 'Low', color: 'text-white/40' },
  { value: 85, label: 'Backlog', color: 'text-white/20' },
]

interface TaskCreateFormProps {
  projects: Project[]
  currentGoal: Goal | null
  onSubmit: (task: { title: string; description: string; priority: number; project_id: string; goal_id?: string; tags?: string[] }) => Promise<void>
  onCancel: () => void
}

export default function TaskCreateForm({ projects, currentGoal, onSubmit, onCancel }: TaskCreateFormProps) {
  const [formData, setFormData] = useState({ title: '', description: '', priority: 40, project_id: '', goal_id: '', tags: '', prompt: '' })
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const activeProjects = projects.filter((p) => p.status === 'ACTIVE')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault(); setFormError(null)
    if (formData.description.length > MAX_DESCRIPTION_LENGTH) { setFormError(`Description exceeds maximum length of ${MAX_DESCRIPTION_LENGTH.toLocaleString()} characters`); return }
    try {
      await onSubmit({ title: formData.title, description: formData.description, priority: formData.priority, project_id: formData.project_id, goal_id: formData.goal_id || undefined, tags: formData.tags.split(',').map((s) => s.trim()).filter(Boolean) })
      setFormData({ title: '', description: '', priority: 40, project_id: '', goal_id: '', tags: '', prompt: '' }); setShowAdvanced(false); setFormError(null)
    } catch (error) { setFormError(error instanceof Error ? error.message : 'Failed to create task') }
  }

  return (
    <div className="card p-5 sm:p-6 animate-slide-up">
      <h2 className="text-base font-semibold text-white/90 mb-4">Create New Task</h2>
      <form onSubmit={handleSubmit} className="space-y-4">
        <div><label className="label">Title</label><input type="text" value={formData.title} onChange={(e) => setFormData({ ...formData, title: e.target.value })} className="input" placeholder="What needs to be done?" required autoFocus /></div>
        <div>
          <label className="label">Description</label>
          <textarea value={formData.description} onChange={(e) => setFormData({ ...formData, description: e.target.value })} className="input min-h-[120px] resize-y" placeholder="Describe the task in detail..." required />
          <div className="flex items-center justify-between mt-1">
            <p className="text-[10px] text-white/20">Be specific — this is what the executor agent reads.</p>
            <span className={`text-[10px] ${formData.description.length > MAX_DESCRIPTION_LENGTH ? 'text-rose-400' : 'text-white/20'}`}>{formData.description.length.toLocaleString()} / {MAX_DESCRIPTION_LENGTH.toLocaleString()}</span>
          </div>
        </div>
        <div>
          <label className="label">Priority <span className="text-white/20 normal-case tracking-normal">({formData.priority})</span></label>
          <div className="flex flex-wrap sm:flex-nowrap gap-1.5">
            {priorityPresets.map((preset) => (
              <button key={preset.value} type="button" onClick={() => setFormData({ ...formData, priority: preset.value })}
                className={`flex-1 px-2 py-2 rounded-lg text-xs font-medium border transition-all touch-manipulation min-w-[70px] ${formData.priority === preset.value ? 'bg-accent-600/15 border-accent-500/40 text-white ring-1 ring-accent-500/20' : 'bg-white/[0.03] border-white/[0.06] text-white/40 hover:border-white/[0.1]'}`}>
                <span className={preset.color}>{preset.label}</span>
              </button>
            ))}
          </div>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div><label className="label">Project</label><select value={formData.project_id} onChange={(e) => setFormData({ ...formData, project_id: e.target.value })} className="input" required><option value="">Select project</option>{activeProjects.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}</select></div>
          <div><label className="label">Goal <span className="text-white/15 normal-case tracking-normal">(optional)</span></label><select value={formData.goal_id} onChange={(e) => setFormData({ ...formData, goal_id: e.target.value })} className="input"><option value="">{currentGoal ? `Active: ${currentGoal.title}` : 'No active goal'}</option>{currentGoal && <option value={currentGoal.id}>{currentGoal.title}</option>}</select></div>
        </div>
        <div><label className="label">Tags <span className="text-white/15 normal-case tracking-normal">(comma-separated)</span></label><input type="text" value={formData.tags} onChange={(e) => setFormData({ ...formData, tags: e.target.value })} className="input" placeholder="frontend, urgent, needs-review" /></div>
        <div><button type="button" onClick={() => setShowAdvanced(!showAdvanced)} className="text-xs text-white/25 hover:text-white/50 transition-colors">{showAdvanced ? '- Hide advanced' : '+ Advanced options'}</button></div>
        {showAdvanced && (
          <div><label className="label">Additional Prompt <span className="text-white/15 normal-case tracking-normal">(optional)</span></label><textarea value={formData.prompt} onChange={(e) => setFormData({ ...formData, prompt: e.target.value })} className="input min-h-[80px] resize-y" placeholder="Extra instructions for the executor agent..." /><p className="text-[10px] text-white/20 mt-1">Appended to the executor prompt as additional context.</p></div>
        )}
        {formError && <div className="p-3 bg-rose-500/[0.06] border border-rose-500/15 rounded-lg text-xs text-rose-400">{formError}</div>}
        <div className="flex items-center gap-2 pt-1">
          <button type="submit" className="btn-primary">Create Task</button>
          <button type="button" onClick={onCancel} className="btn-secondary">Cancel</button>
        </div>
      </form>
    </div>
  )
}
