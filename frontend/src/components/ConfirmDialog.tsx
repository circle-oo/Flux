import { useEffect, useRef } from 'react'

export interface ConfirmDialogProps {
  open: boolean
  title: string
  description?: string
  confirmLabel?: string
  cancelLabel?: string
  variant?: 'default' | 'danger'
  onConfirm: () => void
  onCancel: () => void
}

export default function ConfirmDialog({ open, title, description, confirmLabel = 'Confirm', cancelLabel = 'Cancel', variant = 'default', onConfirm, onCancel }: ConfirmDialogProps) {
  const confirmRef = useRef<HTMLButtonElement>(null)
  const dialogRef = useRef<HTMLDivElement>(null)

  useEffect(() => { if (open) confirmRef.current?.focus() }, [open])
  useEffect(() => {
    if (!open) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { onCancel(); return }
      if (e.key === 'Tab') {
        const focusable = dialogRef.current?.querySelectorAll<HTMLElement>('button:not([disabled])')
        if (!focusable || focusable.length === 0) return
        const first = focusable[0]; const last = focusable[focusable.length - 1]
        if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus() }
        else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus() }
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [open, onCancel])

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" role="dialog" aria-modal="true" aria-labelledby="confirm-dialog-title">
      <div className="fixed inset-0 bg-black/40 backdrop-blur-sm" onClick={onCancel} />
      <div ref={dialogRef} className="relative bg-surface-alt border border-line rounded-xl shadow-2xl max-w-md w-full p-6 animate-slide-up">
        <h2 id="confirm-dialog-title" className="text-base font-semibold text-content mb-1.5">{title}</h2>
        {description && <p className="text-sm text-content-muted mb-6">{description}</p>}
        <div className="flex justify-end gap-2">
          <button onClick={onCancel} className="btn-sm btn-secondary">{cancelLabel}</button>
          <button ref={confirmRef} onClick={onConfirm} className={`btn-sm ${variant === 'danger' ? 'btn-danger' : 'btn-primary'}`}>{confirmLabel}</button>
        </div>
      </div>
    </div>
  )
}
