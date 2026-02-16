import { useNavigate } from 'react-router-dom'

interface BackButtonProps {
  to: string
  label: string
}

export default function BackButton({ to, label }: BackButtonProps) {
  const navigate = useNavigate()
  return (
    <button
      onClick={() => navigate(to)}
      className="text-xs text-content-faint hover:text-content-secondary mb-2 inline-flex items-center gap-1 transition-colors"
    >
      <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
      </svg>
      {label}
    </button>
  )
}
