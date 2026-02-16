import { Routes, Route, Navigate } from 'react-router-dom'
import { useEffect } from 'react'
import { useAuthStore } from './stores/authStore'
import Login from './pages/Login'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import Goals from './pages/Goals'
import Tasks from './pages/Tasks'
import Projects from './pages/Projects'
import PRs from './pages/PRs'
import Logs from './pages/Logs'
import Settings from './pages/Settings'
import TaskDetail from './pages/TaskDetail'
import ProjectDetail from './pages/ProjectDetail'

function App() {
  const { isAuthenticated, authEnabled, checkAuthConfig } = useAuthStore()

  // Check auth config on mount
  useEffect(() => {
    checkAuthConfig()
  }, [checkAuthConfig])

  // Show loading while checking auth config
  if (authEnabled === null) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-slate-900">
        <div className="text-slate-400">Loading...</div>
      </div>
    )
  }

  // Show login page only if auth is enabled and not authenticated
  if (authEnabled && !isAuthenticated) {
    return <Login />
  }

  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="dashboard" element={<Dashboard />} />
        <Route path="goals" element={<Goals />} />
        <Route path="tasks" element={<Tasks />} />
        <Route path="tasks/:id" element={<TaskDetail />} />
        <Route path="projects" element={<Projects />} />
        <Route path="projects/:id" element={<ProjectDetail />} />
        <Route path="prs" element={<PRs />} />
        <Route path="logs" element={<Logs />} />
        <Route path="settings" element={<Settings />} />
      </Route>
    </Routes>
  )
}

export default App
