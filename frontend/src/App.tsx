import { Routes, Route, Navigate } from 'react-router-dom'
import { useEffect } from 'react'
import { useAuthStore } from './stores/authStore'
import Login from './pages/Login'
import CommandPalette from './components/CommandPalette'
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

  useEffect(() => {
    checkAuthConfig()
  }, [checkAuthConfig])

  if (authEnabled === null) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-page">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary-500 to-primary-300 flex items-center justify-center animate-pulse">
            <svg className="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z" />
            </svg>
          </div>
          <span className="text-content-faint text-sm">Loading...</span>
        </div>
      </div>
    )
  }

  if (authEnabled && !isAuthenticated) {
    return <Login />
  }

  return (
    <>
    <CommandPalette />
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
    </>
  )
}

export default App
