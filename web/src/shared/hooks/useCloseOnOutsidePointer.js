import { useEffect } from 'react'

export function useCloseOnOutsidePointer(open, rootRef, onOpenChange) {
  useEffect(() => {
    if (!open) return undefined

    const handlePointerDown = (event) => {
      if (!rootRef.current?.contains(event.target)) {
        onOpenChange?.(false)
      }
    }
    document.addEventListener('pointerdown', handlePointerDown, true)
    return () => document.removeEventListener('pointerdown', handlePointerDown, true)
  }, [onOpenChange, open, rootRef])
}
