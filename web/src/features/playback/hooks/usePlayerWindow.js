import { useCallback, useLayoutEffect, useRef, useState } from 'react'
import {
  changePlayerWindow,
  defaultPlayerWindow,
  fitPlayerWindow,
  isWindowGeometry,
} from '@/features/playback/windowGeometry'

const STORAGE_KEY = 'javboss.player.window'
const getViewport = () => ({ width: window.innerWidth, height: window.innerHeight })

function readGeometry() {
  try {
    const saved = JSON.parse(localStorage.getItem(STORAGE_KEY))
    if (isWindowGeometry(saved)) return saved
  } catch {
    // Storage may be unavailable or contain an older, invalid value.
  }
  return defaultPlayerWindow(getViewport())
}

export default function usePlayerWindow(open) {
  const [preferredRect, setPreferredRect] = useState(readGeometry)
  const [viewport, setViewport] = useState(getViewport)
  const interactionRef = useRef(null)
  const latestRectRef = useRef(preferredRect)
  const rect = fitPlayerWindow(preferredRect, viewport)

  const finishInteraction = useCallback(() => {
    const interaction = interactionRef.current
    if (!interaction) return
    interactionRef.current = null
    if (interaction.element.hasPointerCapture(interaction.pointerId)) {
      interaction.element.releasePointerCapture(interaction.pointerId)
    }
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(latestRectRef.current))
    } catch {
      // Keep the in-memory position when the browser disables storage.
    }
  }, [])

  useLayoutEffect(() => {
    if (!open) return
    const updateViewport = () => {
      finishInteraction()
      setViewport(getViewport())
    }
    updateViewport()
    window.addEventListener('resize', updateViewport)
    window.addEventListener('blur', finishInteraction)
    return () => {
      finishInteraction()
      window.removeEventListener('resize', updateViewport)
      window.removeEventListener('blur', finishInteraction)
    }
  }, [open, finishInteraction])

  const startInteraction = (event, direction) => {
    if (event.button !== 0 || !event.isPrimary || interactionRef.current) return
    if (direction === 'move' && event.target.closest('button, a, input, select, textarea')) return
    event.preventDefault()
    latestRectRef.current = rect
    interactionRef.current = {
      direction,
      rect,
      x: event.clientX,
      y: event.clientY,
      pointerId: event.pointerId,
      element: event.currentTarget,
    }
    event.currentTarget.setPointerCapture(event.pointerId)
  }

  const move = useCallback((event) => {
    const interaction = interactionRef.current
    if (!interaction || event.pointerId !== interaction.pointerId) return
    const next = changePlayerWindow(
      interaction.rect,
      interaction.direction,
      event.clientX - interaction.x,
      event.clientY - interaction.y,
      getViewport()
    )
    latestRectRef.current = next
    setPreferredRect(next)
  }, [])

  const finish = useCallback(
    (event) => {
      if (event.pointerId === interactionRef.current?.pointerId) finishInteraction()
    },
    [finishInteraction]
  )

  useLayoutEffect(() => {
    if (!open) return
    // Track the whole viewport so releasing outside the dialog always ends the gesture.
    window.addEventListener('pointermove', move, true)
    window.addEventListener('pointerup', finish, true)
    window.addEventListener('pointercancel', finish, true)
    return () => {
      window.removeEventListener('pointermove', move, true)
      window.removeEventListener('pointerup', finish, true)
      window.removeEventListener('pointercancel', finish, true)
    }
  }, [open, move, finish])

  return {
    style: {
      position: 'absolute',
      left: rect.x,
      top: rect.y,
      width: rect.width,
      height: rect.height,
    },
    onMoveStart: (event) => startInteraction(event, 'move'),
    onResizeStart: startInteraction,
    pointerProps: { onLostPointerCapture: finish },
  }
}
