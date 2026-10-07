// Playback positions stay in this browser and are separate from watch-time totals.
export function createBrowserResume({
  videoId,
  locationId,
  enabled = true,
  storage,
  now = Date.now,
}) {
  const key = `javboss.player.position.${videoId}.${locationId || 0}`
  const getStorage = () => storage ?? globalThis.localStorage
  let position = 0
  if (enabled) {
    try {
      const saved = Number(getStorage().getItem(key))
      if (Number.isFinite(saved) && saved > 0) position = saved
    } catch {
      // Playback still works when browser storage is unavailable.
    }
  }
  let pending = null
  let lastWrite = 0
  const flush = () => {
    if (!enabled || pending === null) return
    try {
      if (pending > 0) getStorage().setItem(key, String(pending))
      else getStorage().removeItem(key)
      lastWrite = now()
    } catch {
      // Storage failure must not interrupt playback.
    }
  }
  return {
    position,
    record: (seconds, duration) => {
      if (
        !enabled ||
        !Number.isFinite(seconds) ||
        seconds < 0 ||
        !Number.isFinite(duration) ||
        duration <= 0
      )
        return
      pending = seconds < duration ? seconds : 0
      if (now() - lastWrite >= 5000) flush()
    },
    complete: () => {
      pending = 0
      flush()
    },
    flush,
  }
}
