import { Command } from 'cmdk'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTaskStore } from '../stores/taskStore'
import { useProjectStore } from '../stores/projectStore'
import { useGoalStore } from '../stores/goalStore'

interface CommandRow {
  id: string
  label: string
  action: () => void
  meta?: string
  group: string
}

function Shortcut({ keys }: { keys: string }) {
  return (
    <kbd
      style={{
        fontSize: 10,
        color: 'var(--t3)',
        border: '1px solid var(--div)',
        borderRadius: 8,
        padding: '2px 7px',
        background: 'color-mix(in srgb, var(--gi-bg) 70%, transparent)',
      }}
    >
      {keys}
    </kbd>
  )
}

export default function CommandPalette() {
  const [open, setOpen] = useState(false)
  const navigate = useNavigate()
  const tasks = useTaskStore((s) => s.tasks)
  const projects = useProjectStore((s) => s.projects)
  const goals = useGoalStore((s) => s.goals)

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setOpen((prev) => !prev)
      }
      if (e.key === '/' && !(e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement)) {
        e.preventDefault()
        setOpen(true)
      }
      if (e.key === 'Escape') setOpen(false)
    }
    function onOpenEvent() {
      setOpen(true)
    }
    document.addEventListener('keydown', onKeyDown)
    window.addEventListener('flux:command-palette-open', onOpenEvent as EventListener)
    return () => {
      document.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('flux:command-palette-open', onOpenEvent as EventListener)
    }
  }, [])

  const rows = useMemo<CommandRow[]>(() => {
    const go = (path: string) => {
      navigate(path)
      setOpen(false)
    }

    const out: CommandRow[] = [
      { id: 'nav-dashboard', label: 'Dashboard', action: () => go('/dashboard'), meta: 'D', group: 'Navigation' },
      { id: 'nav-goals', label: 'Goals', action: () => go('/goals'), group: 'Navigation' },
      { id: 'nav-tasks', label: 'Tasks', action: () => go('/tasks'), meta: 'T', group: 'Navigation' },
      { id: 'nav-projects', label: 'Projects', action: () => go('/projects'), meta: 'P', group: 'Navigation' },
      { id: 'nav-prs', label: 'Pull Requests', action: () => go('/prs'), group: 'Navigation' },
      { id: 'nav-pods', label: 'Pods', action: () => go('/pods'), group: 'Navigation' },
      { id: 'nav-insights', label: 'Insights', action: () => go('/insights'), group: 'Navigation' },
      { id: 'nav-logs', label: 'Logs', action: () => go('/logs'), group: 'Navigation' },
      { id: 'nav-settings', label: 'Settings', action: () => go('/settings'), group: 'Navigation' },
    ]

    tasks.slice(0, 20).forEach((t) => {
      out.push({
        id: `task-${t.id}`,
        label: t.title,
        action: () => go(`/tasks/${t.id}`),
        meta: t.status,
        group: 'Tasks',
      })
    })

    projects.slice(0, 20).forEach((p) => {
      out.push({
        id: `project-${p.id}`,
        label: p.name,
        action: () => go(`/projects/${p.id}`),
        meta: p.status,
        group: 'Projects',
      })
    })

    goals.slice(0, 20).forEach((g) => {
      out.push({
        id: `goal-${g.id}`,
        label: g.title,
        action: () => go('/goals'),
        meta: g.status,
        group: 'Goals',
      })
    })

    return out
  }, [goals, navigate, projects, tasks])

  const grouped = useMemo(() => {
    const map = new Map<string, CommandRow[]>()
    rows.forEach((row) => {
      const list = map.get(row.group) || []
      list.push(row)
      map.set(row.group, list)
    })
    return Array.from(map.entries())
  }, [rows])

  if (!open) return null

  return (
    <div className="fixed inset-0 z-[100] flex items-start justify-center pt-[14vh]" onClick={() => setOpen(false)}>
      <div className="absolute inset-0 bg-black/35 backdrop-blur-sm" />
      <div
        className="go relative z-[1] w-[min(680px,92vw)] max-h-[70vh] overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <Command label="Command palette" shouldFilter>
          <div className="px-4 pt-3 pb-2 border-b border-line-subtle flex items-center justify-between gap-3">
            <div className="text-[11px] uppercase tracking-[0.1em] text-content-faint">Command Palette</div>
            <Shortcut keys="⌘K" />
          </div>

          <Command.Input
            placeholder="Jump to page, task, project..."
            className="w-full px-4 py-3 text-sm bg-transparent border-none outline-none text-content placeholder:text-content-faint"
          />

          <Command.List className="max-h-[56vh] overflow-y-auto p-2">
            <Command.Empty className="text-sm text-content-faint text-center py-8">No results found.</Command.Empty>

            {grouped.map(([name, items]) => (
              <Command.Group key={name} heading={name} className="mb-1">
                {items.map((item) => (
                  <Command.Item
                    key={item.id}
                    value={`${item.label} ${item.meta || ''} ${name}`}
                    onSelect={item.action}
                    className="gi my-1 px-3 py-2.5 rounded-xl cursor-pointer flex items-center justify-between gap-3 text-sm text-content-secondary data-[selected=true]:text-content data-[selected=true]:border-primary-300/70"
                  >
                    <span className="truncate">{item.label}</span>
                    {item.meta && <span className="badge-secondary text-[10px]">{item.meta}</span>}
                  </Command.Item>
                ))}
              </Command.Group>
            ))}
          </Command.List>
        </Command>
      </div>
    </div>
  )
}
