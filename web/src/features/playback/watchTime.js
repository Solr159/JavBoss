// Include time browsing with seeks, but exclude initial loading, pauses and
// waiting for media data. Never use media-position deltas to measure elapsed time.
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
    started = true
    active = !player.paused() && !suspended
  }
  const stop = () => {
    advance()
    active = false
    void flush()
  }
  const canBrowse = () => started && !suspended && !player.paused()
  const targetBuffered = () => {
    const position = player.currentTime()
    const ranges = player.buffered()
    for (let i = 0; i < ranges.length; i++) {
      if (position >= ranges.start(i) && position < ranges.end(i)) return true
    }
    return false
  }
  const seeking = () => {
    advance()
    active = canBrowse() && targetBuffered()
  }
  const waiting = () => {
    // Browsers also emit waiting while decoding a seek within buffered data.
    // Seeking to an unbuffered range remains a real cache wait.
    if (player.seeking() && canBrowse() && targetBuffered()) seeking()
    else stop()
  }
  const seeked = () => {
    advance()
    active = canBrowse() && player.readyState() >= 3
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
    seeked()
  }
  const events = {
    playing: play,
    pause: stop,
    waiting,
    seeking,
    seeked,
    ended: unload,
    emptied: unload,
    error: unload,
    abort: unload,
  }
  for (const [event, handler] of Object.entries(events)) player.on(event, handler)
  page?.addEventListener('pagehide', suspend)
  page?.addEventListener('pageshow', resume)
  document?.addEventListener('visibilitychange', visibility)
  document?.addEventListener('freeze', suspend)
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
    page?.removeEventListener('pagehide', suspend)
    page?.removeEventListener('pageshow', resume)
    document?.removeEventListener('visibilitychange', visibility)
    document?.removeEventListener('freeze', suspend)
    document?.removeEventListener('resume', resume)
    void flush()
  }
}
