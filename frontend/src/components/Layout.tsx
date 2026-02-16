import { Outlet } from 'react-router-dom'
import { useState } from 'react'
import Sidebar from './Sidebar'

export default function Layout() {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)

  return (
    <div className="flex h-screen bg-page overflow-hidden">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:top-4 focus:left-4 focus:z-[100] focus:px-4 focus:py-2 focus:bg-primary-600 focus:text-white focus:rounded-lg focus:outline-none"
      >
        Skip to main content
      </a>
      <Sidebar
        mobileMenuOpen={mobileMenuOpen}
        setMobileMenuOpen={setMobileMenuOpen}
      />
      <div className="flex flex-col flex-1 min-w-0">
        {/* Mobile header */}
        <header className="lg:hidden bg-sidebar/80 backdrop-blur-xl border-b border-line px-4 py-3 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-primary-500 to-primary-300 flex items-center justify-center">
              <svg className="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z" />
              </svg>
            </div>
            <span className="text-base font-semibold text-content">Flux</span>
          </div>
          <button
            onClick={() => setMobileMenuOpen(true)}
            className="p-2 text-content-muted hover:text-content transition-colors touch-manipulation rounded-lg hover:bg-surface-active"
            aria-label="Open navigation menu"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>
        </header>
        <main id="main-content" className="flex-1 overflow-y-auto" role="main">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
