import { useEffect } from 'react'

export default function CenterToast({ open, message, onClose, duration = 1800 }) {
  useEffect(() => {
    if (!open || !message) return
    const timer = window.setTimeout(() => {
      onClose?.()
    }, duration)
    return () => window.clearTimeout(timer)
  }, [duration, message, onClose, open])

  if (!open || !message) return null

  return (
    <div className="pointer-events-none fixed inset-0 z-[80] flex items-center justify-center px-4">
      <div
        role="alert"
        aria-live="assertive"
        className="max-w-md rounded-lg bg-zinc-800/95 px-5 py-3 text-center text-sm leading-6 text-white shadow-xl backdrop-blur"
      >
        {message}
      </div>
    </div>
  )
}
