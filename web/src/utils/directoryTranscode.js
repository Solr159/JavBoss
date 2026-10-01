export const transcodeIsActive = (progress) =>
  Boolean(progress) && !['completed', 'failed', 'cancelled'].includes(progress.phase)

export function transcodeOverallPercent(progress) {
  const total = Number(progress?.total)
  if (!Number.isFinite(total) || total <= 0 || progress?.phase === 'discovering') return 0
  const processed = Math.max(0, Number(progress.processed) || 0)
  const current = transcodeIsActive(progress)
    ? Math.min(99, Math.max(0, Number(progress.current_percent) || 0)) / 100
    : 0
  return Math.min(100, (100 * (processed + current)) / total)
}

export function transcodeResultsChanged(previous, current) {
  if (!current || !(current.converted > 0)) return false
  return (
    current.started_at_unix_ms !== previous?.started_at_unix_ms ||
    current.converted !== previous?.converted
  )
}
