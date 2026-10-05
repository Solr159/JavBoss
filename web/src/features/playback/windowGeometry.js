const MARGIN = 8
const MIN_WIDTH = 360
const MIN_HEIGHT = 240
const clamp = (value, min, max) => Math.min(max, Math.max(min, value))

function limits(viewport) {
  const width = Math.max(1, viewport.width - MARGIN * 2)
  const height = Math.max(1, viewport.height - MARGIN * 2)
  return {
    width,
    height,
    minWidth: Math.min(MIN_WIDTH, width),
    minHeight: Math.min(MIN_HEIGHT, height),
  }
}

export function isWindowGeometry(value) {
  return (
    value &&
    ['x', 'y', 'width', 'height'].every((key) => Number.isFinite(value[key])) &&
    value.width > 0 &&
    value.height > 0
  )
}

export function fitPlayerWindow(rect, viewport) {
  const bounds = limits(viewport)
  const width = clamp(rect.width, bounds.minWidth, bounds.width)
  const height = clamp(rect.height, bounds.minHeight, bounds.height)
  return {
    x: clamp(rect.x, MARGIN, MARGIN + bounds.width - width),
    y: clamp(rect.y, MARGIN, MARGIN + bounds.height - height),
    width,
    height,
  }
}

export function defaultPlayerWindow(viewport) {
  const width = Math.min(1152, viewport.width - 32)
  const height = viewport.height * 0.75 + 46
  return fitPlayerWindow(
    { x: (viewport.width - width) / 2, y: (viewport.height - height) / 2, width, height },
    viewport
  )
}

export function changePlayerWindow(rect, direction, dx, dy, viewport) {
  if (direction === 'move')
    return fitPlayerWindow({ ...rect, x: rect.x + dx, y: rect.y + dy }, viewport)
  const bounds = limits(viewport)
  let left = rect.x
  let top = rect.y
  let right = rect.x + rect.width
  let bottom = rect.y + rect.height
  if (direction.includes('w')) left = clamp(left + dx, MARGIN, right - bounds.minWidth)
  if (direction.includes('e'))
    right = clamp(right + dx, left + bounds.minWidth, MARGIN + bounds.width)
  if (direction.includes('n')) top = clamp(top + dy, MARGIN, bottom - bounds.minHeight)
  if (direction.includes('s'))
    bottom = clamp(bottom + dy, top + bounds.minHeight, MARGIN + bounds.height)
  return fitPlayerWindow({ x: left, y: top, width: right - left, height: bottom - top }, viewport)
}
