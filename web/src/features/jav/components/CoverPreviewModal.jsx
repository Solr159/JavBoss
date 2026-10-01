import { useState, useEffect } from 'react'
import AppModal from '@/components/AppModal'
import { zh } from '@/utils/i18n'

export function CoverPreviewModal({ preview, onClose }) {
  const [scale, setScale] = useState(1)

  useEffect(() => {
    if (!preview?.src) return undefined

    const handleWheel = (event) => {
      event.preventDefault()
      event.stopPropagation()
      const direction = event.deltaY < 0 ? 1 : -1
      setScale((current) => Math.min(5, Math.max(0.5, current + direction * 0.2)))
    }

    window.addEventListener('wheel', handleWheel, { passive: false, capture: true })

    return () => {
      window.removeEventListener('wheel', handleWheel, true)
    }
  }, [preview?.src])

  if (!preview?.src) return null

  return (
    <AppModal
      ariaLabel={zh('封面预览', 'Cover preview')}
      className="p-4"
      contentClassName="relative flex max-h-[92vh] max-w-[94vw] items-center justify-center"
      onClose={onClose}
      zIndex={1500}
    >
      <button
        type="button"
        onClick={onClose}
        className="fixed right-4 top-4 z-10 rounded bg-black/50 px-3 py-1 text-xl leading-none text-white hover:bg-black/70"
        aria-label={zh('关闭封面预览', 'Close cover preview')}
      >
        ×
      </button>
      <img
        src={preview.src}
        alt={preview.alt || zh('JAV 封面', 'JAV cover')}
        className="max-h-[92vh] max-w-[94vw] transform-gpu cursor-zoom-in object-contain shadow-2xl"
        style={{ transform: `scale(${scale})` }}
      />
    </AppModal>
  )
}
