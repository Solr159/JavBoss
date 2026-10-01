export function normalizeIdolTagMaxRows(value) {
  const rows = Math.floor(Number(value))
  return Number.isFinite(rows) && rows > 0 ? Math.min(rows, 12) : 0
}

export function normalizeJavTagMaxRows(value) {
  const rows = Math.floor(Number(value))
  return Number.isFinite(rows) && rows > 0 ? Math.min(rows, 12) : 0
}

export function normalizeJavTitleMaxRows(value) {
  const rows = Math.floor(Number(value))
  return Number.isFinite(rows) && rows >= 0 ? Math.min(rows, 12) : 2
}

export function countFlexRows(itemWidths, trailingWidth, containerWidth, gap) {
  const widths = trailingWidth > 0 ? [...itemWidths, trailingWidth] : itemWidths
  if (widths.length === 0) return 0

  let rows = 1
  let rowWidth = 0
  for (const width of widths) {
    const nextWidth = rowWidth === 0 ? width : rowWidth + gap + width
    if (rowWidth > 0 && nextWidth > containerWidth) {
      rows += 1
      rowWidth = width
    } else {
      rowWidth = nextWidth
    }
  }
  return rows
}
