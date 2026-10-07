import { useCallback, useLayoutEffect, useRef, useState } from 'react'

const STORAGE_KEY = 'javboss.player.playlistWidth'
const HANDLE_WIDTH = 8

function readWidth() {
  try {
    const value = Number(localStorage.getItem(STORAGE_KEY))
    if (Number.isFinite(value) && value > 0) return value
  } catch {
    // Resizing still works when browser storage is unavailable.
  }
  return null
}

export default function usePlaylistResize() {
  const panelRef = useRef(null)
  const dragRef = useRef(null)
  const [preferredWidth, setPreferredWidth] = useState(readWidth)
  const latestWidthRef = useRef(preferredWidth)
  const [containerWidth, setContainerWidth] = useState(0)
  const [dragging, setDragging] = useState(false)
  const available = Math.max(0, containerWidth - HANDLE_WIDTH)
  const max = Math.floor(available * 0.6)
  const min = Math.min(128, Math.floor(available * 0.38))
  const clampWidth = (value) => Math.min(max, Math.max(min, value))
  const width = clampWidth(preferredWidth ?? Math.min(256, available * 0.38))

  const persist = useCallback(() => {
    if (!(latestWidthRef.current > 0)) return
    try {
      localStorage.setItem(STORAGE_KEY, String(latestWidthRef.current))
    } catch {
      // Keep the chosen width for this session if storage is blocked.
    }
  }, [])

  const finish = useCallback(() => {
    const drag = dragRef.current
    if (!drag) return
    dragRef.current = null
    setDragging(false)
    if (drag.element.hasPointerCapture(drag.pointerId)) {
      drag.element.releasePointerCapture(drag.pointerId)
    }
    persist()
  }, [persist])

  useLayoutEffect(() => {
    const container = panelRef.current.parentElement
    const update = () => {
      finish()
      setContainerWidth(container.clientWidth)
    }
    update()
    const observer = new ResizeObserver(update)
    observer.observe(container)
    window.addEventListener('blur', finish)
    return () => {
      finish()
      observer.disconnect()
      window.removeEventListener('blur', finish)
    }
  }, [finish])

  const move = useCallback(
    (event) => {
      const drag = dragRef.current
      if (!drag || event.pointerId !== drag.pointerId) return
      const next = Math.min(max, Math.max(min, drag.width + drag.x - event.clientX))
      latestWidthRef.current = next
      setPreferredWidth(next)
    },
    [min, max]
  )

  const finishPointer = useCallback(
    (event) => {
      if (event.pointerId === dragRef.current?.pointerId) finish()
    },
    [finish]
  )

  useLayoutEffect(() => {
    // Keep tracking when the pointer crosses the native video element or leaves the panel.
    window.addEventListener('pointermove', move, true)
    window.addEventListener('pointerup', finishPointer, true)
    window.addEventListener('pointercancel', finishPointer, true)
    return () => {
      window.removeEventListener('pointermove', move, true)
      window.removeEventListener('pointerup', finishPointer, true)
      window.removeEventListener('pointercancel', finishPointer, true)
    }
  }, [move, finishPointer])

  const changeWidth = (value) => {
    const next = clampWidth(value)
    latestWidthRef.current = next
    setPreferredWidth(next)
  }

  return {
    panelRef,
    width,
    dragging,
    separatorProps: {
      'aria-valuemin': min,
      'aria-valuemax': max,
      'aria-valuenow': Math.round(width),
      onPointerDown: (event) => {
        if (event.button !== 0 || !event.isPrimary || dragRef.current) return
        event.preventDefault()
        event.stopPropagation()
        event.currentTarget.focus()
        dragRef.current = {
          x: event.clientX,
          width,
          pointerId: event.pointerId,
          element: event.currentTarget,
        }
        event.currentTarget.setPointerCapture(event.pointerId)
        setDragging(true)
      },
      onLostPointerCapture: finishPointer,
      onKeyDown: (event) => {
        const next = { ArrowLeft: width + 16, ArrowRight: width - 16, Home: min, End: max }[
          event.key
        ]
        if (next == null) return
        event.preventDefault()
        event.stopPropagation()
        changeWidth(next)
        persist()
      },
    },
  }
}
