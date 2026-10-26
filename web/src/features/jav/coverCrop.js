export const COVER_VISIBLE_RATIO = 0.47
export const PORTRAIT_COVER_ASPECT = (800 * COVER_VISIBLE_RATIO) / 538

export function normalizeCoverCropPosition(value) {
  if (value == null || value === '') return 1
  const position = Number(value)
  return Number.isFinite(position) ? Math.min(1, Math.max(0, position)) : 1
}

// Match object-fit: cover and object-position: <position>% center for any source size.
export function getCoverCropRect(size, position) {
  const width = Number(size?.width)
  const height = Number(size?.height)
  if (!(width > 0 && height > 0 && Number.isFinite(width) && Number.isFinite(height))) return null
  const aspect = width / height
  const visibleWidth = Math.min(1, PORTRAIT_COVER_ASPECT / aspect)
  const visibleHeight = Math.min(1, aspect / PORTRAIT_COVER_ASPECT)
  return {
    left: normalizeCoverCropPosition(position) * (1 - visibleWidth),
    top: (1 - visibleHeight) / 2,
    width: visibleWidth,
    height: visibleHeight,
  }
}
