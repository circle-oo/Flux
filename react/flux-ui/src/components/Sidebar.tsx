import { NavLink } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import { useWSStore } from '../stores/wsStore'
import { useState } from 'react'
import { api } from '../lib/api'

export default function Sidebar() {
  const { logout } = useAuthStore()
  const wsConnected = useWSStore((s) => s.connected)
  const [isCollapsed, setIsCollapsed] = useState(false)
  const [isRestarting, setIsRestarting] = useState(false)

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
  ]

  return (
    <aside
      className={`bg-slate-800 border-r border-slate-700 flex flex-col transition-all duration-300 ${
        isCollapsed ? 'w-16' : 'w-64'
      }`}
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
        <button
          onClick={() => setIsCollapsed(!isCollapsed)}
          className="text-slate-400 hover:text-slate-200 transition-colors"
          title={isCollapsed ? 'Expand' : 'Collapse'}
        >
          {isCollapsed ? '→' : '←'}
        </button>
      </div>

      {/* Navigation */}
      <nav className="flex-1 p-4 space-y-2">
        {navItems.map((item) => (
          <NavLink
            key={item.path}
            to={item.path}
            className={({ isActive }) =>
              `flex items-center gap-3 px-3 py-2 rounded-lg transition-colors ${
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
          className="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-slate-300 hover:bg-slate-700 hover:text-white transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          title={isCollapsed ? 'Restart' : undefined}
        >
          <span className="text-xl">{isRestarting ? '⏳' : '🔄'}</span>
          {!isCollapsed && <span>{isRestarting ? 'Restarting...' : 'Restart'}</span>}
        </button>
        <button
          onClick={logout}
          className="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-slate-300 hover:bg-slate-700 hover:text-white transition-colors"
          title={isCollapsed ? 'Logout' : undefined}
        >
          <span className="text-xl">🚪</span>
          {!isCollapsed && <span>Logout</span>}
        </button>
      </div>
    </aside>
  )
}
