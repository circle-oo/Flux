import { useEffect, useState, useCallback, createContext, useContext } from 'react'

type ToastVariant = 'success' | 'error' | 'info'

interface Toast {
  id: number
  message: string
  variant: ToastVariant
}

interface ToastContextValue {
  toast: (message: string, variant?: ToastVariant) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

let nextId = 0

const variantStyles: Record<ToastVariant, string> = {
  success: 'bg-emerald-500/90 border-emerald-400/30 text-white backdrop-blur-sm',
  error: 'bg-rose-500/90 border-rose-400/30 text-white backdrop-blur-sm',
  info: 'bg-primary-500/90 border-primary-400/30 text-white backdrop-blur-sm',
}

function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: (id: number) => void }) {
  useEffect(() => { const timer = setTimeout(() => onDismiss(toast.id), 4000); return () => clearTimeout(timer) }, [toast.id, onDismiss])
  return (
    <div className={`px-4 py-3 rounded-lg border shadow-lg text-sm font-medium animate-slide-in ${variantStyles[toast.variant]}`} role="alert">
      {toast.message}
    </div>
  )
}

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const addToast = useCallback((message: string, variant: ToastVariant = 'info') => { const id = nextId++; setToasts((prev) => [...prev, { id, message, variant }]) }, [])
  const dismissToast = useCallback((id: number) => { setToasts((prev) => prev.filter((t) => t.id !== id)) }, [])
  return (
    <ToastContext.Provider value={{ toast: addToast }}>
      {children}
      <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
        {toasts.map((t) => <ToastItem key={t.id} toast={t} onDismiss={dismissToast} />)}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within a ToastProvider')
  return ctx
}
