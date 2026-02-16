import { Command } from 'cmdk'
import { useEffect, useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTaskStore } from '../stores/taskStore'
import { useProjectStore } from '../stores/projectStore'
import { useGoalStore } from '../stores/goalStore'

export default function CommandPalette() {
  const [open, setOpen] = useState(false)
  const navigate = useNavigate()
  const tasks = useTaskStore((s) => s.tasks)
  const projects = useProjectStore((s) => s.projects)
  const goals = useGoalStore((s) => s.goals)

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        setOpen((prev) => !prev)
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [])

  const taskResults = useMemo(
    () => tasks.slice(0, 20).map((t) => ({ id: t.id, label: t.title, status: t.status })),
    [tasks]
  )

  const projectResults = useMemo(
    () => projects.map((p) => ({ id: p.id, label: p.name, status: p.status })),
    [projects]
  )

  const goalResults = useMemo(
    () => goals.map((g) => ({ id: g.id, label: g.title, status: g.status })),
    [goals]
  )

  function go(path: string) {
    navigate(path)
    setOpen(false)
  }

  if (!open) return null

  return (
    <div className="command-palette-overlay" onClick={() => setOpen(false)}>
      <div className="command-palette" onClick={(e) => e.stopPropagation()}>
        <Command label="Command palette" shouldFilter={true}>
          <Command.Input
            placeholder="Type a command or search..."
            className="command-palette-input"
          />
          <Command.List className="command-palette-list">
            <Command.Empty className="command-palette-empty">No results found.</Command.Empty>

            <Command.Group heading="Navigation" className="command-palette-group">
              <Command.Item onSelect={() => go('/dashboard')} className="command-palette-item">
                <NavIcon />
                <span>Dashboard</span>
                <Kbd>D</Kbd>
              </Command.Item>
              <Command.Item onSelect={() => go('/goals')} className="command-palette-item">
                <NavIcon />
                <span>Goals</span>
              </Command.Item>
              <Command.Item onSelect={() => go('/tasks')} className="command-palette-item">
                <NavIcon />
                <span>Tasks</span>
                <Kbd>T</Kbd>
              </Command.Item>
              <Command.Item onSelect={() => go('/projects')} className="command-palette-item">
                <NavIcon />
                <span>Projects</span>
                <Kbd>P</Kbd>
              </Command.Item>
              <Command.Item onSelect={() => go('/prs')} className="command-palette-item">
                <NavIcon />
                <span>Pull Requests</span>
              </Command.Item>
              <Command.Item onSelect={() => go('/logs')} className="command-palette-item">
                <NavIcon />
                <span>Logs</span>
              </Command.Item>
              <Command.Item onSelect={() => go('/settings')} className="command-palette-item">
                <NavIcon />
                <span>Settings</span>
              </Command.Item>
            </Command.Group>

            {taskResults.length > 0 && (
              <Command.Group heading="Tasks" className="command-palette-group">
                {taskResults.map((t) => (
                  <Command.Item key={t.id} onSelect={() => go(`/tasks/${t.id}`)} className="command-palette-item">
                    <TaskIcon />
                    <span className="truncate">{t.label}</span>
                    <span className="command-palette-badge">{t.status}</span>
                  </Command.Item>
                ))}
              </Command.Group>
            )}

            {projectResults.length > 0 && (
              <Command.Group heading="Projects" className="command-palette-group">
                {projectResults.map((p) => (
                  <Command.Item key={p.id} onSelect={() => go(`/projects/${p.id}`)} className="command-palette-item">
                    <ProjectIcon />
                    <span className="truncate">{p.label}</span>
                    <span className="command-palette-badge">{p.status}</span>
                  </Command.Item>
                ))}
              </Command.Group>
            )}

            {goalResults.length > 0 && (
              <Command.Group heading="Goals" className="command-palette-group">
                {goalResults.map((g) => (
                  <Command.Item key={g.id} onSelect={() => go('/goals')} className="command-palette-item">
                    <GoalIcon />
                    <span className="truncate">{g.label}</span>
                    <span className="command-palette-badge">{g.status}</span>
                  </Command.Item>
                ))}
              </Command.Group>
            )}
          </Command.List>
        </Command>
      </div>
    </div>
  )
}

function Kbd({ children }: { children: string }) {
  return <kbd className="command-palette-kbd">{children}</kbd>
}

function NavIcon() {
  return (
    <svg className="w-4 h-4 shrink-0 text-content-faint" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3" />
    </svg>
  )
}

function TaskIcon() {
  return (
    <svg className="w-4 h-4 shrink-0 text-content-faint" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 002.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 00-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 00.75-.75 2.25 2.25 0 00-.1-.664m-5.8 0A2.251 2.251 0 0113.5 2.25H15a2.25 2.25 0 012.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25z" />
    </svg>
  )
}

function ProjectIcon() {
  return (
    <svg className="w-4 h-4 shrink-0 text-content-faint" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44l-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z" />
    </svg>
  )
}

function GoalIcon() {
  return (
    <svg className="w-4 h-4 shrink-0 text-content-faint" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M3 3v1.5M3 21v-6m0 0l2.77-.693a9 9 0 016.208.682l.108.054a9 9 0 006.086.71l3.114-.732a48.524 48.524 0 01-.005-10.499l-3.11.732a9 9 0 01-6.085-.711l-.108-.054a9 9 0 00-6.208-.682L3 4.5M3 15V4.5" />
    </svg>
  )
}
