interface LoadingStateProps {
  message?: string
}

export default function LoadingState({ message = 'Loading...' }: LoadingStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 gap-3 text-center">
      <div
        className="gi"
        style={{
          width: 46,
          height: 46,
          borderRadius: 14,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <span
          style={{
            width: 11,
            height: 11,
            borderRadius: '50%',
            background: 'linear-gradient(135deg, var(--p500), var(--p400))',
            animation: 'vitro-loading 1.2s ease-in-out infinite',
          }}
        />
      </div>
      <div className="text-xs text-content-faint">{message}</div>
      <style>{`@keyframes vitro-loading {0%,100%{transform:scale(1);opacity:.6}50%{transform:scale(1.65);opacity:1}}`}</style>
    </div>
  )
}
