import { NavLink, Outlet, useLocation } from 'react-router-dom'
import { useEffect, useMemo, useState } from 'react'
import Sidebar from './Sidebar'
import { useSettingsStore } from '../stores/settingsStore'

function MeshBackground() {
  return (
    <>
      <div className="vitro-mesh" style={{ position: 'fixed', inset: 0, zIndex: 0 }} />
      <div className="vitro-noise" />
    </>
  )
}

interface PageMeta {
  section: string
  label: string
}

interface MobileTab {
  path: string
  label: string
  tag: string
}

const mobileTabs: MobileTab[] = [
  { path: '/dashboard', label: 'Dashboard', tag: 'DB' },
  { path: '/tasks', label: 'Tasks', tag: 'TS' },
  { path: '/projects', label: 'Projects', tag: 'PJ' },
  { path: '/insights', label: 'Insights', tag: 'IN' },
  { path: '/settings', label: 'Settings', tag: 'ST' },
]

const pageTable: Array<{ prefix: string; meta: PageMeta }> = [
  { prefix: '/dashboard', meta: { section: 'Build', label: 'Dashboard' } },
  { prefix: '/goals', meta: { section: 'Build', label: 'Goals' } },
  { prefix: '/tasks/', meta: { section: 'Build', label: 'Task Detail' } },
  { prefix: '/tasks', meta: { section: 'Build', label: 'Tasks' } },
  { prefix: '/projects/', meta: { section: 'Build', label: 'Project Detail' } },
  { prefix: '/projects', meta: { section: 'Build', label: 'Projects' } },
  { prefix: '/prs', meta: { section: 'Build', label: 'Pull Requests' } },
  { prefix: '/pods', meta: { section: 'Observe', label: 'Pods' } },
  { prefix: '/insights', meta: { section: 'Observe', label: 'Insights' } },
  { prefix: '/knowledge', meta: { section: 'Observe', label: 'Knowledge' } },
  { prefix: '/logs', meta: { section: 'Observe', label: 'Logs' } },
  { prefix: '/settings', meta: { section: 'System', label: 'Settings' } },
]

function resolvePageMeta(pathname: string): PageMeta {
  const found = pageTable.find((item) => pathname.startsWith(item.prefix))
  return found?.meta ?? { section: 'Workspace', label: 'Flux' }
}

export default function Layout() {
  const location = useLocation()
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const sidebarCollapsed = useSettingsStore((s) => s.sidebarCollapsed)
  const [isMobile, setIsMobile] = useState(() => window.innerWidth < 1024)
  const pageMeta = useMemo(() => resolvePageMeta(location.pathname), [location.pathname])

  useEffect(() => {
    const onResize = () => setIsMobile(window.innerWidth < 1024)
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  const openCommandPalette = () => {
    window.dispatchEvent(new Event('flux:command-palette-open'))
  }

  return (
    <div className="relative min-h-screen">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:top-4 focus:left-4 focus:z-[110] focus:px-4 focus:py-2 focus:bg-primary-600 focus:text-white focus:rounded-lg focus:outline-none"
      >
        Skip to main content
      </a>

      <MeshBackground />

      <Sidebar mobileMenuOpen={mobileMenuOpen} setMobileMenuOpen={setMobileMenuOpen} />

      <main
        style={{
          marginLeft: isMobile ? 0 : sidebarCollapsed ? '88px' : '286px',
          padding: isMobile ? '14px 14px 106px' : '24px 28px',
          minHeight: '100vh',
          transition: 'margin-left .25s cubic-bezier(.22, 1, .36, 1)',
          position: 'relative',
          zIndex: 1,
        }}
      >
        <div className="go sticky top-3 z-20 mb-4 px-3 py-2.5">
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-2.5 min-w-0">
              {isMobile && (
                <button
                  className="btn-sm btn-secondary"
                  onClick={() => setMobileMenuOpen(true)}
                  aria-label="Open menu"
                >
                  Menu
                </button>
              )}
              <div className="min-w-0">
                <div className="text-[10px] text-content-faint uppercase tracking-[0.11em] font-semibold">{pageMeta.section}</div>
                <div className="text-sm text-content-secondary font-medium truncate">{pageMeta.label}</div>
              </div>
            </div>

            <div className="flex items-center gap-2 shrink-0">
              <button
                type="button"
                className="btn-sm btn-secondary hidden sm:inline-flex"
                onClick={openCommandPalette}
                title="Open command palette"
              >
                Search
                <span className="text-[10px] text-content-faint">Ctrl/Cmd+K</span>
              </button>
            </div>
          </div>
        </div>

        <div id="main-content" role="main" className="relative z-[1]">
          <Outlet />
        </div>
      </main>

      {isMobile && (
        <nav className="fixed inset-x-0 bottom-0 z-30 px-3 pt-2 pb-[max(0.75rem,env(safe-area-inset-bottom))] lg:hidden">
          <div className="gs rounded-2xl px-1.5 py-1.5 flex items-center justify-between">
            {mobileTabs.map((tab) => (
              <NavLink
                key={tab.path}
                to={tab.path}
                className={({ isActive }) =>
                  `min-w-0 flex-1 mx-0.5 px-2 py-1.5 rounded-xl border transition-colors ${
                    isActive
                      ? 'border-primary-300/70 text-primary-700 bg-primary-50/70'
                      : 'border-transparent text-content-muted hover:text-content-secondary'
                  }`
                }
              >
                <div className="text-center leading-tight">
                  <div className="text-[10px] font-semibold uppercase tracking-wider">{tab.tag}</div>
                  <div className="text-[10px] truncate">{tab.label}</div>
                </div>
              </NavLink>
            ))}
          </div>
        </nav>
      )}
    </div>
  )
}
