interface ErrorBannerProps {
  message: string
}

export default function ErrorBanner({ message }: ErrorBannerProps) {
  return (
    <div className="p-3 bg-red-900/30 border border-red-600 rounded-lg text-sm text-red-200" role="alert">
      {message}
    </div>
  )
}
