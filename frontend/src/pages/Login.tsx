import { useState, FormEvent } from 'react'
import { useAuthStore } from '../stores/authStore'

export default function Login() {
  const [password, setPassword] = useState('')
  const { login, isLoading, error, clearError } = useAuthStore()

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    clearError()
    try {
      await login(password)
    } catch {
      // Error is handled by store
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-page px-4 sm:px-6 lg:px-8 relative overflow-hidden">
      {/* Background gradient orbs */}
      <div className="absolute top-1/4 left-1/4 w-96 h-96 bg-primary-600/10 rounded-full blur-[128px] pointer-events-none" />
      <div className="absolute bottom-1/4 right-1/4 w-96 h-96 bg-primary-400/10 rounded-full blur-[128px] pointer-events-none" />

      <div className="max-w-sm w-full relative z-10 animate-fade-in">
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-14 h-14 rounded-2xl bg-gradient-to-br from-primary-500 to-primary-300 mb-5 shadow-xl shadow-primary-600/30">
            <svg className="w-7 h-7 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z" />
            </svg>
          </div>
          <h1 className="text-2xl font-bold text-content mb-1">Flux</h1>
          <p className="text-sm text-content-faint">Autonomous Engineering System</p>
        </div>

        <div className="card p-6 sm:p-8">
          <form onSubmit={handleSubmit} className="space-y-5">
            <div>
              <label htmlFor="password" className="label">
                Password
              </label>
              <input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="input"
                placeholder="Enter password"
                autoFocus
                required
              />
            </div>

            {error && (
              <div className="bg-rose-500/10 border border-rose-500/20 text-rose-600 px-4 py-3 rounded-lg text-sm" role="alert">
                {error}
              </div>
            )}

            <button
              type="submit"
              disabled={isLoading || !password}
              className="w-full py-3 rounded-lg font-medium text-sm text-white bg-gradient-to-r from-primary-600 to-primary-400 hover:from-primary-500 hover:to-primary-300 transition-all duration-200 shadow-lg shadow-primary-600/25 disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 focus:ring-offset-page"
            >
              {isLoading ? 'Signing in...' : 'Sign in'}
            </button>
          </form>
        </div>
      </div>
    </div>
  )
}
