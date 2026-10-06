import { collectWatchedTimeIDs } from '@/features/playback/watchedTimeState'

// One stream for the application, with bounded batches when a snapshot is needed.
export function startWatchedTimeSync({
  store,
  fetchSnapshot,
  createSource = () => new EventSource('/videos/watched-time/events'),
  page = window,
  document = page.document,
}) {
  let stopped = false
  let timer = null
  let pending = false
  let dirty = false
  let retryDelay = 1000
  let idsKey = JSON.stringify(collectWatchedTimeIDs(store.getState()))
  const controller = new AbortController()
  const source = createSource()
  const schedule = (delay = 50) => {
    if (stopped) return
    dirty = true
    if (pending) return
    clearTimeout(timer)
    timer = setTimeout(refresh, delay)
  }
  const refresh = async () => {
    if (stopped || pending) return
    pending = true
    dirty = false
    try {
      const { videoIds, javIds } = collectWatchedTimeIDs(store.getState())
      for (let offset = 0; offset < Math.max(videoIds.length, javIds.length); offset += 200) {
        const snapshot = await fetchSnapshot(
          {
            videoIds: videoIds.slice(offset, offset + 200),
            javIds: javIds.slice(offset, offset + 200),
          },
          controller.signal
        )
        if (stopped) return
        store.getState().updateWatchedTimes(snapshot)
      }
      retryDelay = 1000
    } catch {
      if (!stopped) {
        // Retry a failed reconnect/focus snapshot even if no further playback occurs.
        dirty = true
        retryDelay = Math.min(retryDelay * 2, 30000)
      }
    } finally {
      pending = false
      if (dirty && !stopped) schedule(retryDelay === 1000 ? 50 : retryDelay)
    }
  }
  source.addEventListener('watched-time', (event) => {
    if (stopped) return
    try {
      store.getState().updateWatchedTimes(JSON.parse(event.data))
    } catch {
      schedule()
    }
  })
  source.addEventListener('open', () => schedule())
  // Also check authentication through apiFetch on errors (EventSource hides HTTP status).
  source.addEventListener('error', () => schedule())
  const foreground = () => {
    if (document.visibilityState !== 'hidden') schedule()
  }
  page.addEventListener('focus', foreground)
  document.addEventListener('visibilitychange', foreground)
  const unsubscribe = store.subscribe((state, previous) => {
    if (state.videos === previous.videos && state.javItems === previous.javItems) return
    const nextKey = JSON.stringify(collectWatchedTimeIDs(state))
    if (nextKey === idsKey) return
    idsKey = nextKey
    schedule()
  })
  schedule()
  return () => {
    stopped = true
    controller.abort()
    clearTimeout(timer)
    source.close()
    unsubscribe()
    page.removeEventListener('focus', foreground)
    document.removeEventListener('visibilitychange', foreground)
  }
}
