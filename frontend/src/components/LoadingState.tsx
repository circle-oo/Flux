interface LoadingStateProps {
  message?: string
}

export default function LoadingState({ message = 'Loading...' }: LoadingStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 gap-3">
      <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary-500/20 to-primary-300/20 flex items-center justify-center animate-pulse">
        <div className="w-3 h-3 rounded-full bg-primary-400/60" />
      </div>
      <div className="text-xs text-content-faint">{message}</div>
    </div>
  )
}
