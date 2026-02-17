interface ErrorBannerProps {
  message: string
}

export default function ErrorBanner({ message }: ErrorBannerProps) {
  return (
    <div
      className="go"
      role="alert"
      style={{
        padding: '12px 16px',
        fontSize: '13px',
        color: 'var(--err)',
        borderColor: 'color-mix(in srgb, var(--err) 30%, transparent)',
      }}
    >
      {message}
    </div>
  )
}
