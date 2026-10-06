const JAV_PORTRAIT_VISIBLE_RATIO = 0.47
export const JAV_PORTRAIT_ASPECT_RATIO = (800 * JAV_PORTRAIT_VISIBLE_RATIO) / 538

export function getJavPortraitImageStyle(width, height) {
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return null

  // Scale to the frame height, then position the image like an idol cover.
  const renderedWidthRatio = width / height / JAV_PORTRAIT_ASPECT_RATIO
  const leftRatio =
    renderedWidthRatio <= 1
      ? (1 - renderedWidthRatio) / 2
      : -Math.min((1 - JAV_PORTRAIT_VISIBLE_RATIO) * renderedWidthRatio, renderedWidthRatio - 1)

  return { left: `${leftRatio * 100}%` }
}
