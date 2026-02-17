import { useEffect, useState } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import { useWSStore } from '../stores/wsStore'
import { useSettingsStore } from '../stores/settingsStore'
import { api } from '../lib/api'
import { useConfirm } from '../hooks/useConfirm'
import { useToast } from './Toast'

interface SidebarProps {
  mobileMenuOpen: boolean
  setMobileMenuOpen: (open: boolean) => void
}

interface NavItem {
  path: string
  label: string
  glyph: string
  group: 'build' | 'observe' | 'system'
}

const navItems: NavItem[] = [
  { path: '/dashboard', label: 'Dashboard', glyph: 'DB', group: 'build' },
  { path: '/goals', label: 'Goals', glyph: 'GL', group: 'build' },
  { path: '/tasks', label: 'Tasks', glyph: 'TS', group: 'build' },
  { path: '/projects', label: 'Projects', glyph: 'PJ', group: 'build' },
  { path: '/prs', label: 'Pull Requests', glyph: 'PR', group: 'build' },
  { path: '/pods', label: 'Pods', glyph: 'PD', group: 'observe' },
  { path: '/insights', label: 'Insights', glyph: 'IN', group: 'observe' },
  { path: '/knowledge', label: 'Knowledge', glyph: 'KN', group: 'observe' },
  { path: '/logs', label: 'Logs', glyph: 'LG', group: 'observe' },
  { path: '/settings', label: 'Settings', glyph: 'ST', group: 'system' },
]

const groups: Array<{ id: NavItem['group']; label: string }> = [
  { id: 'build', label: 'Build' },
  { id: 'observe', label: 'Observe' },
  { id: 'system', label: 'System' },
]

function RailGlyph({ glyph }: { glyph: string }) {
  return (
    <span
      style={{
        width: 20,
        height: 20,
        borderRadius: 8,
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontSize: 10,
        fontWeight: 700,
        color: 'var(--t3)',
        background: 'color-mix(in srgb, var(--gi-bg) 74%, transparent)',
        border: '1px solid color-mix(in srgb, var(--div) 65%, transparent)',
      }}
    >
      {glyph}
    </span>
  )
}

function TinyIconButton({
  label,
  onClick,
  children,
  className,
}: {
  label: string
  onClick: () => void
  children: React.ReactNode
  className?: string
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className={`gi ${className ?? ''}`}
      style={{
        width: 34,
        height: 34,
        border: 'none',
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        color: 'var(--t2)',
        fontSize: 14,
        fontWeight: 700,
        cursor: 'pointer',
      }}
    >
      {children}
    </button>
  )
}

export default function Sidebar({ mobileMenuOpen, setMobileMenuOpen }: SidebarProps) {
  const location = useLocation()
  const { logout, authEnabled } = useAuthStore()
  const wsConnected = useWSStore((s) => s.connected)
  const {
    sidebarCollapsed,
    setSidebarCollapsed,
  } = useSettingsStore()

  const [isRestarting, setIsRestarting] = useState(false)
  const { confirm, dialog } = useConfirm()
  const { toast } = useToast()

  useEffect(() => {
    const handleResize = () => {
      if (window.innerWidth >= 1024) setMobileMenuOpen(false)
    }
    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [setMobileMenuOpen])

  useEffect(() => {
    setMobileMenuOpen(false)
  }, [location.pathname, setMobileMenuOpen])

  const handleRestart = async () => {
    const confirmed = await confirm({
      title: 'Restart Flux?',
      description: 'This will update and restart the service. The page will reload automatically.',
      confirmLabel: 'Restart',
      variant: 'danger',
    })
    if (!confirmed) return
    try {
      setIsRestarting(true)
      await api.restart()
      toast('Flux is restarting... The page will reload automatically.', 'info')
      setTimeout(() => window.location.reload(), 5000)
    } catch (error) {
      setIsRestarting(false)
      toast(`Failed to restart: ${error}`, 'error')
    }
  }

  const closeMobileMenu = () => setMobileMenuOpen(false)

  return (
    <>
      {dialog}

      {mobileMenuOpen && (
        <div
          className="fixed inset-0 z-30 lg:hidden"
          style={{
            background: 'rgba(0, 0, 0, 0.35)',
            backdropFilter: 'blur(4px)',
            WebkitBackdropFilter: 'blur(4px)',
          }}
          onClick={closeMobileMenu}
        />
      )}

      <aside
        className={`gs sidebar-shell fixed inset-y-0 left-0 z-40 m-0 lg:m-3 rounded-r-[22px] lg:rounded-[22px] transition-all duration-300 ${sidebarCollapsed ? 'w-[84px]' : 'w-[276px]'} ${mobileMenuOpen ? 'translate-x-0 w-[88vw] max-w-[320px]' : '-translate-x-full lg:translate-x-0'}`}
      >
        <div className="h-full flex flex-col">
          <div className={`px-3 pt-3 pb-4 ${sidebarCollapsed ? 'flex justify-center' : 'flex items-center justify-between'}`}>
            <div className="flex items-center gap-3 min-w-0">
              <div
                style={{
                  width: 34,
                  height: 34,
                  borderRadius: 12,
                  background: 'linear-gradient(135deg, var(--p500), var(--p400))',
                  boxShadow: '0 8px 20px rgba(var(--gl), .28)',
                  color: 'white',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontWeight: 800,
                  letterSpacing: '-.02em',
                }}
              >
                F
              </div>

              {!sidebarCollapsed && (
                <div className="min-w-0">
                  <div className="text-sm font-semibold text-content truncate">Flux</div>
                  <div className="text-[11px] text-content-faint truncate">Autonomous Engineering</div>
                </div>
              )}
            </div>

            <div className="flex items-center gap-1">
              <button
                onClick={closeMobileMenu}
                className="lg:hidden gi"
                style={{
                  width: 34,
                  height: 34,
                  border: 'none',
                  display: 'inline-flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  color: 'var(--t2)',
                  fontSize: 18,
                  cursor: 'pointer',
                }}
                aria-label="Close menu"
              >
                ×
              </button>
              <TinyIconButton
                label={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
                onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
                className="hidden lg:inline-flex"
              >
                {sidebarCollapsed ? '>>' : '<<'}
              </TinyIconButton>
            </div>
          </div>

          <div className="sidebar-divider" />

          <nav className="px-2 py-2 overflow-y-auto flex-1 space-y-2">
            {groups.map((group) => {
              const items = navItems.filter((item) => item.group === group.id)
              return (
                <div key={group.id} className="space-y-1">
                  {!sidebarCollapsed && (
                    <div className="text-[10px] px-2.5 uppercase tracking-[0.12em] text-content-faint font-semibold mt-1">
                      {group.label}
                    </div>
                  )}
                  {items.map((item) => (
                    <NavLink
                      key={item.path}
                      to={item.path}
                      onClick={closeMobileMenu}
                      title={sidebarCollapsed ? item.label : undefined}
                      className={({ isActive }) =>
                        `gi w-full min-h-[42px] rounded-xl px-2.5 py-2.5 border text-left transition-all duration-150 flex items-center ${sidebarCollapsed ? 'justify-center' : 'gap-2.5'} ${isActive ? 'text-primary-700 border-primary-300/60 shadow-[0_0_0_1px_rgba(var(--gl),.14)]' : 'text-content-secondary border-transparent hover:text-content hover:border-line-subtle'}`
                      }
                    >
                      <RailGlyph glyph={item.glyph} />
                      {!sidebarCollapsed && <span className="text-[13px] font-medium truncate">{item.label}</span>}
                    </NavLink>
                  ))}
                </div>
              )
            })}
          </nav>

          <div className="sidebar-divider" />

          <div className="px-2 py-2 space-y-2">
            <div className={`flex items-center px-1 ${sidebarCollapsed ? 'justify-center' : 'justify-start'}`}>
              <div className="flex items-center gap-2 text-[11px] text-content-faint uppercase tracking-wider font-medium">
                <span className={`inline-block w-2 h-2 rounded-full ${wsConnected ? 'bg-emerald-400' : 'bg-rose-400'}`} />
                {!sidebarCollapsed && <span>{wsConnected ? 'Live' : 'Offline'}</span>}
              </div>
            </div>

            <div className="grid gap-1">
              <button
                onClick={handleRestart}
                disabled={isRestarting}
                className={`btn-sm btn-secondary w-full ${sidebarCollapsed ? 'justify-center' : 'justify-start'}`}
                title={sidebarCollapsed ? 'Restart' : undefined}
              >
                {isRestarting ? 'Restarting...' : 'Restart'}
              </button>

              {authEnabled && (
                <button
                  onClick={logout}
                  className={`btn-sm btn-danger w-full ${sidebarCollapsed ? 'justify-center' : 'justify-start'}`}
                  title={sidebarCollapsed ? 'Logout' : undefined}
                >
                  Logout
                </button>
              )}
            </div>
          </div>
        </div>
      </aside>
    </>
  )
}
