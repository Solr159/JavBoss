// After playback first starts, count elapsed time until pause or end, including
// buffering and seeks. Never use media-position deltas to measure elapsed time.
export function startWatchTracking(
  player,
  {
    create,
    report,
    now = () => performance.now(),
    schedule = setInterval,
    unschedule = clearInterval,
    page = globalThis.window,
    document = globalThis.document,
  }
) {
  let active = false
  let started = false
  let suspended = false
  let previous = now()
  let total = 0
  let session = null
  let creating = null
  let closed = false
  let closedAt = 0

  const advance = () => {
    const time = now()
    if (active) total += Math.max(0, time - previous)
    previous = time
  }
  const createSession = () => {
    if (!creating) {
      const baseline = total
      creating = create()
        .then((id) => {
          session = { id, baseline, acknowledged: 0 }
          return session
        })
        .finally(() => {
          creating = null
        })
    }
    return creating
  }
  const flush = async () => {
    advance()
    try {
      // An existing session must reach report() synchronously during pagehide.
      const current = session || (await createSession())
      if (current !== session) return
      advance()
      const checkpoint = Math.floor(total - current.baseline)
      if (checkpoint > current.acknowledged) {
        const accepted = await report(current.id, checkpoint)
        // Late responses must never change a replacement session's state.
        if (current !== session) return
        if (accepted) {
          current.acknowledged = Math.max(current.acknowledged, checkpoint)
        } else {
          // Never replay an expired session's total into a new session.
          session = null
          if (closed) unschedule(timer)
          else void flush()
        }
      }
    } catch {
      // Retain the checkpoint for retry, even when the response was lost after commit.
    } finally {
      if (
        closed &&
        ((session && session.acknowledged >= Math.floor(total - session.baseline)) ||
          now() - closedAt >= 30000)
      )
        unschedule(timer)
    }
  }
  const syncPlayback = () => {
    advance()
    active = started && !player.paused() && !suspended
  }
  const playing = () => {
    started = true
    syncPlayback()
  }
  const stop = () => {
    advance()
    active = false
    void flush()
  }
  const unload = () => {
    started = false
    stop()
  }
  const suspend = () => {
    suspended = true
    stop()
  }
  const visibility = () => void flush()
  const resume = () => {
    previous = now()
    suspended = false
    syncPlayback()
  }
  const events = {
    playing,
    play: syncPlayback,
    pause: stop,
    ended: unload,
    emptied: unload,
    error: unload,
    abort: unload,
  }
  for (const [event, handler] of Object.entries(events)) player.on(event, handler)
  page?.addEventListener('pagehide', suspend)
  page?.addEventListener('pageshow', resume)
  document?.addEventListener('visibilitychange', visibility)
  const timer = schedule(() => void flush(), 10000)
  void flush()

  return () => {
    if (closed) return
    advance()
    active = false
    closed = true
    closedAt = now()
    for (const [event, handler] of Object.entries(events)) player.off(event, handler)
    page?.removeEventListener('pagehide', suspend)
    page?.removeEventListener('pageshow', resume)
    document?.removeEventListener('visibilitychange', visibility)
    void flush()
  }
}

export const DEFAULT_WATCH_TIME_ICON_MINUTES = 30

export function normalizeWatchTimeIconMinutes(value) {
  const minutes = Number(value)
  return Number.isSafeInteger(minutes) && minutes > 0 ? minutes : DEFAULT_WATCH_TIME_ICON_MINUTES
}
