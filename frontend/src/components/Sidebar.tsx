import { NavLink } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import { useWSStore } from '../stores/wsStore'
import { useState, useEffect } from 'react'
import { api } from '../lib/api'

interface SidebarProps {
  mobileMenuOpen: boolean
  setMobileMenuOpen: (open: boolean) => void
}

export default function Sidebar({ mobileMenuOpen, setMobileMenuOpen }: SidebarProps) {
  const { logout } = useAuthStore()
  const wsConnected = useWSStore((s) => s.connected)
  const [isCollapsed, setIsCollapsed] = useState(false)
  const [isRestarting, setIsRestarting] = useState(false)

  // Close mobile menu on desktop resize
  useEffect(() => {
    const handleResize = () => {
      if (window.innerWidth >= 1024) {
        setMobileMenuOpen(false)
      }
    }
    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [setMobileMenuOpen])

  const handleRestart = async () => {
    if (!confirm('Are you sure you want to restart Flux? This will update and restart the service.')) {
      return
    }

    try {
      setIsRestarting(true)
      await api.restart()
      // Show a message that restart is in progress
      alert('Flux is restarting... The page will reload automatically.')
      // Wait a bit and reload the page
      setTimeout(() => {
        window.location.reload()
      }, 5000)
    } catch (error) {
      setIsRestarting(false)
      alert(`Failed to restart: ${error}`)
    }
  }

  const navItems = [
    { path: '/dashboard', label: 'Dashboard', icon: '📊' },
    { path: '/goals', label: 'Goals', icon: '🎯' },
    { path: '/tasks', label: 'Tasks', icon: '📋' },
    { path: '/projects', label: 'Projects', icon: '🚀' },
    { path: '/prs', label: 'Pull Requests', icon: '🔀' },
    { path: '/logs', label: 'Logs', icon: '📜' },
    { path: '/settings', label: 'Settings', icon: '⚙️' },
  ]

  const closeMobileMenu = () => setMobileMenuOpen(false)

  return (
    <>
      {/* Mobile overlay */}
      {mobileMenuOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-40 lg:hidden"
          onClick={closeMobileMenu}
        />
      )}

      {/* Sidebar */}
      <aside
        className={`
          bg-slate-800 border-r border-slate-700 flex flex-col transition-all duration-300
          ${isCollapsed ? 'w-16' : 'w-64'}
          lg:relative lg:translate-x-0
          fixed inset-y-0 left-0 z-50
          ${mobileMenuOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
        `}
      >
      {/* Header */}
      <div className="p-4 border-b border-slate-700 flex items-center justify-between">
        {!isCollapsed && (
          <div className="flex items-center gap-2">
            <h1 className="text-xl font-bold text-blue-400">Flux</h1>
            <div
              className={`w-2 h-2 rounded-full ${wsConnected ? 'bg-green-500' : 'bg-red-500'}`}
              title={wsConnected ? 'Connected' : 'Disconnected'}
            />
          </div>
        )}
        <div className="flex items-center gap-2">
          {/* Close button for mobile */}
          <button
            onClick={closeMobileMenu}
            className="lg:hidden p-2 -mr-2 text-slate-400 hover:text-slate-200 transition-colors touch-manipulation"
            aria-label="Close menu"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
          {/* Collapse button for desktop */}
          <button
            onClick={() => setIsCollapsed(!isCollapsed)}
            className="hidden lg:block text-slate-400 hover:text-slate-200 transition-colors touch-manipulation"
            title={isCollapsed ? 'Expand' : 'Collapse'}
          >
            {isCollapsed ? '→' : '←'}
          </button>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 p-4 space-y-2">
        {navItems.map((item) => (
          <NavLink
            key={item.path}
            to={item.path}
            onClick={closeMobileMenu}
            className={({ isActive }) =>
              `flex items-center gap-3 px-3 py-3 rounded-lg transition-colors touch-manipulation min-h-[44px] ${
                isActive
                  ? 'bg-blue-600 text-white'
                  : 'text-slate-300 hover:bg-slate-700 hover:text-white'
              }`
            }
            title={isCollapsed ? item.label : undefined}
          >
            <span className="text-xl">{item.icon}</span>
            {!isCollapsed && <span>{item.label}</span>}
          </NavLink>
        ))}
      </nav>

      {/* Footer */}
      <div className="p-4 border-t border-slate-700 space-y-2">
        <button
          onClick={handleRestart}
          disabled={isRestarting}
          className="w-full flex items-center gap-3 px-3 py-3 rounded-lg text-slate-300 hover:bg-slate-700 hover:text-white transition-colors disabled:opacity-50 disabled:cursor-not-allowed touch-manipulation min-h-[44px]"
          title={isCollapsed ? 'Restart' : undefined}
        >
          <span className="text-xl">{isRestarting ? '⏳' : '🔄'}</span>
          {!isCollapsed && <span>{isRestarting ? 'Restarting...' : 'Restart'}</span>}
        </button>
        <button
          onClick={logout}
          className="w-full flex items-center gap-3 px-3 py-3 rounded-lg text-slate-300 hover:bg-slate-700 hover:text-white transition-colors touch-manipulation min-h-[44px]"
          title={isCollapsed ? 'Logout' : undefined}
        >
          <span className="text-xl">🚪</span>
          {!isCollapsed && <span>Logout</span>}
        </button>
      </div>
    </aside>
    </>
  )
}
