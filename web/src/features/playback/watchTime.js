// Measure time in the playing state, independent of media position and speed.
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
  const flush = async () => {
    advance()
    if (busy) {
      pending = true
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
  const play = () => {
    advance()
    active = !player.paused() && !player.seeking()
  }
  const stop = () => {
    advance()
    active = false
    void flush()
  }
  const seeked = () => {
    if (player.readyState() >= 3) play()
  }
  const visibility = () => void flush()
  const resume = () => {
    previous = now()
    if (player.readyState() >= 3) play()
  }
  const events = {
    playing: play,
    pause: stop,
    waiting: stop,
    seeking: stop,
    seeked,
    ended: stop,
    emptied: stop,
    error: stop,
    abort: stop,
  }
  for (const [event, handler] of Object.entries(events)) player.on(event, handler)
  page?.addEventListener('pagehide', stop)
  page?.addEventListener('pageshow', resume)
  document?.addEventListener('visibilitychange', visibility)
  document?.addEventListener('freeze', stop)
  document?.addEventListener('resume', resume)
  const timer = schedule(() => void flush(), 10000)
  void flush()

  return () => {
    if (closed) return
    advance()
    active = false
    closed = true
    closedAt = now()
    for (const [event, handler] of Object.entries(events)) player.off(event, handler)
    page?.removeEventListener('pagehide', stop)
    page?.removeEventListener('pageshow', resume)
    document?.removeEventListener('visibilitychange', visibility)
    document?.removeEventListener('freeze', stop)
    document?.removeEventListener('resume', resume)
    void flush()
  }
}
