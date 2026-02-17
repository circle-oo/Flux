import { Routes, Route, Navigate } from 'react-router-dom'
import { useEffect, lazy, Suspense } from 'react'
import { useAuthStore } from './stores/authStore'
import CommandPalette from './components/CommandPalette'
import LoadingState from './components/LoadingState'

const Login = lazy(() => import('./pages/Login'))
const Layout = lazy(() => import('./components/Layout'))
const Dashboard = lazy(() => import('./pages/Dashboard'))
const Goals = lazy(() => import('./pages/Goals'))
const Tasks = lazy(() => import('./pages/Tasks'))
const Projects = lazy(() => import('./pages/Projects'))
const PRs = lazy(() => import('./pages/PRs'))
const Pods = lazy(() => import('./pages/Pods'))
const Insights = lazy(() => import('./pages/Insights'))
const Knowledge = lazy(() => import('./pages/Knowledge'))
const Logs = lazy(() => import('./pages/Logs'))
const Settings = lazy(() => import('./pages/Settings'))
const TaskDetail = lazy(() => import('./pages/TaskDetail'))
const ProjectDetail = lazy(() => import('./pages/ProjectDetail'))

function App() {
  const { isAuthenticated, authEnabled, checkAuthConfig } = useAuthStore()

  useEffect(() => {
    checkAuthConfig()
  }, [checkAuthConfig])

  if (authEnabled === null) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <LoadingState message="Loading Flux workspace..." />
      </div>
    )
  }

  if (authEnabled && !isAuthenticated) {
    return (
      <Suspense fallback={<LoadingState message="Loading login..." />}>
        <Login />
      </Suspense>
    )
  }

  return (
    <>
      <CommandPalette />
      <Suspense
        fallback={(
          <div className="min-h-screen flex items-center justify-center">
            <LoadingState message="Loading workspace..." />
          </div>
        )}
      >
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
            <Route path="pods" element={<Pods />} />
            <Route path="insights" element={<Insights />} />
            <Route path="knowledge" element={<Knowledge />} />
            <Route path="knowledge/*" element={<Knowledge />} />
            <Route path="logs" element={<Logs />} />
            <Route path="settings" element={<Settings />} />
          </Route>
        </Routes>
      </Suspense>
    </>
  )
}

export default App
