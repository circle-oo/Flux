interface LoadingStateProps {
  message?: string
}

export default function LoadingState({ message = 'Loading...' }: LoadingStateProps) {
  return (
    <div className="flex items-center justify-center py-12">
      <div className="text-slate-400">{message}</div>
    </div>
  )
}
