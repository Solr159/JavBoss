import { useEffect } from 'react'
import { zh } from '@/utils/i18n'

export default function Toast({ open, message, onClose, duration = 4000 }) {
  useEffect(() => {
    if (!open || !message) return
    const timer = window.setTimeout(() => {
      onClose?.()
    }, duration)
    return () => window.clearTimeout(timer)
  }, [duration, message, onClose, open])

  if (!open || !message) return null

  return (
    <div className="pointer-events-none fixed bottom-4 right-4 z-[80] w-[calc(100vw-2rem)] max-w-md">
      <div className="pointer-events-auto rounded-lg border border-emerald-200 bg-white/95 px-4 py-3 shadow-xl backdrop-blur">
        <div className="flex items-start gap-3">
          <div className="mt-0.5 h-2.5 w-2.5 shrink-0 rounded-full bg-emerald-500" />
          <div className="min-w-0 flex-1 text-sm leading-6 text-gray-800">{message}</div>
          <button
            type="button"
            onClick={() => onClose?.()}
            className="shrink-0 rounded px-1.5 py-0.5 text-xs text-gray-500 hover:bg-gray-100"
          >
            {zh('关闭', 'Close')}
          </button>
        </div>
      </div>
    </div>
  )
}
