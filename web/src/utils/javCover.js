// Portrait cover crop helpers shared by the compact JAV card. The values mirror
// the idol cover defaults so both pages crop the right portion of landscape covers.
export const JAV_COVER_VISIBLE_RATIO = 0.47

export const JAV_COVER_DEFAULT_CROP_LEFT = 1 - JAV_COVER_VISIBLE_RATIO

export const JAV_COVER_SOURCE_WIDTH = 800

export const JAV_COVER_SOURCE_HEIGHT = 538

export const JAV_COVER_FRAME_ASPECT =
  (JAV_COVER_SOURCE_WIDTH * JAV_COVER_VISIBLE_RATIO) / JAV_COVER_SOURCE_HEIGHT

export function normalizeJavCoverCropLeft(value) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return JAV_COVER_DEFAULT_CROP_LEFT
  return Math.min(Math.max(parsed, 0), 1)
}

export function javCoverVisibleRatio(imageAspect) {
  const aspect = Number(imageAspect)
  if (!Number.isFinite(aspect) || aspect <= 0) return JAV_COVER_VISIBLE_RATIO
  return Math.min(1, JAV_COVER_FRAME_ASPECT / aspect)
}

// Maps a crop-left fraction to the CSS `object-position` percentage used by an
// `object-fit: cover` image inside the portrait compact frame.
export function resolveJavCoverObjectPosition(cropLeft, imageAspect) {
  const visibleRatio = javCoverVisibleRatio(imageAspect)
  const maxCropLeft = Math.max(0, 1 - visibleRatio)
  if (maxCropLeft <= 0) return 0
  const clamped = Math.min(Math.max(normalizeJavCoverCropLeft(cropLeft), 0), maxCropLeft)
  return (clamped / maxCropLeft) * 100
}
