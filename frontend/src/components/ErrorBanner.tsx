interface ErrorBannerProps {
  message: string
}

export default function ErrorBanner({ message }: ErrorBannerProps) {
  return (
    <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-lg text-sm text-rose-600" role="alert">
      {message}
    </div>
  )
}
