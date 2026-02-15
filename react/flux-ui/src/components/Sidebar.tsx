import { NavLink } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import { useState } from 'react'

export default function Sidebar() {
  const { logout } = useAuthStore()
  const [isCollapsed, setIsCollapsed] = useState(false)

  const navItems = [
    { path: '/dashboard', label: 'Dashboard', icon: '📊' },
    { path: '/goals', label: 'Goals', icon: '🎯' },
    { path: '/tasks', label: 'Tasks', icon: '📋' },
    { path: '/projects', label: 'Projects', icon: '🚀' },
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
          <h1 className="text-xl font-bold text-blue-400">Flux</h1>
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
      <div className="p-4 border-t border-slate-700">
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
