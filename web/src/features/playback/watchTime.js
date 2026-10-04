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
  let baseline = 0
  let acknowledged = 0
  let session = ''
  let busy = false
  let pending = false
  let closed = false
  let closedAt = 0

  const advance = () => {
    const time = now()
    if (active) total += Math.max(0, time - previous)
    previous = time
  }
  const flush = async ({ immediate = false } = {}) => {
    advance()
    if (busy) {
      pending = true
      const checkpoint = Math.floor(total - baseline)
      if (immediate && session && checkpoint > acknowledged) {
        // Teardown cannot wait for the in-flight request's continuation. The
        // server accepts duplicate/out-of-order totals; leave session state to
        // the regular loop so late responses cannot affect a replacement session.
        try {
          await report(session, checkpoint)
        } catch {
          // The regular loop can retry if the page survives or is restored.
        }
      }
      return
    }
    busy = true
    try {
      do {
        pending = false
        if (!session) {
          baseline = total
          session = await create()
          acknowledged = 0
        }
        advance()
        const checkpoint = Math.floor(total - baseline)
        if (checkpoint > acknowledged) {
          const accepted = await report(session, checkpoint)
          if (accepted) {
            acknowledged = checkpoint
          } else {
            // Never replay an expired session's total into a new session.
            session = ''
            baseline = total
            pending = !closed
          }
        }
      } while (pending)
      if (closed) unschedule(timer)
    } catch {
      // Retain the checkpoint for retry, even when the response was lost after commit.
    } finally {
      busy = false
      if (closed && now() - closedAt >= 30000) unschedule(timer)
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
    advance()
    active = false
    void flush({ immediate: true })
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
    void flush({ immediate: true })
  }
}
