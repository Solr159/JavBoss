// Video.js owns track rendering and selection. Stable source keys survive player
// recreation; startup, cleanup and offset reloads never overwrite a user choice.
export function startSubtitleLoading(
  player,
  {
    resolve,
    getSettings,
    autoDownload = false,
    saveSelection = async () => {},
    onStatus = () => {},
  }
) {
  const controller = new AbortController()
  const trackList = player.textTracks()
  const loadedTracks = new Map()
  let started = false
  let loading = false
  let settingsReady = false
  let applying = false
  let changeQueued = false
  let preferredTrack = ''
  let lastSelection = ''
  const tracks = () =>
    Array.from(player.textTracks()).filter(
      (item) => item.kind === 'subtitles' || item.kind === 'captions'
    )
  const inactive = () => controller.signal.aborted || player.isDisposed()
  const keyOf = (track) => {
    for (const [key, item] of loadedTracks) if (item.track === track) return key
    const native = tracks().filter(
      (item) => ![...loadedTracks.values()].some((entry) => entry.track === item)
    )
    return `embedded:${native.indexOf(track) + 1}`
  }
  const selection = () => {
    const showing = tracks().find((item) => item.mode === 'showing')
    return showing ? keyOf(showing) : 'off'
  }
  const snapshot = () => {
    lastSelection = selection()
  }
  const select = (track) => {
    applying = true
    tracks().forEach((item) => {
      item.mode = item === track ? 'showing' : 'disabled'
    })
    snapshot()
    applying = false
  }
  const restore = (fallback = false) => {
    if (preferredTrack === 'off') select(null)
    else {
      const available = tracks()
      const preferred = available.find((item) => keyOf(item) === preferredTrack)
      if (preferred) select(preferred)
      else if (
        (fallback || preferredTrack !== '') &&
        !available.some((item) => item.mode === 'showing')
      )
        select(available[0])
    }
    snapshot()
  }
  const remember = () => {
    preferredTrack = selection()
    const track = preferredTrack
    snapshot()
    void Promise.resolve()
      .then(() => saveSelection(track))
      .catch(() => {
        if (!inactive()) onStatus('settings_error')
      })
  }
  const changed = () => {
    if (inactive() || !settingsReady || applying || changeQueued) return
    // Video.js emits a change for each track mode update. Observe the final
    // selection after a menu action, not the intermediate all-disabled state.
    changeQueued = true
    queueMicrotask(() => {
      changeQueued = false
      if (!inactive() && tracks().length && selection() !== lastSelection) remember()
    })
  }
  const add = (result, manual) => {
    if (!result.blob) return
    const hadTracks = tracks().length > 0
    const key =
      result.key || `${result.source === 'local' ? 'local' : 'download'}:${result.language}`
    let item = loadedTracks.get(key)
    if (!item) {
      const objectURL = URL.createObjectURL(result.blob)
      applying = true
      try {
        const { track } = player.addRemoteTextTrack(
          {
            id: `javboss:${key}`,
            kind: 'subtitles',
            src: objectURL,
            srclang: result.language,
            label: `${result.source === 'local' ? '同目录字幕' : '已下载字幕'} · ${result.language}`,
          },
          false
        )
        item = { track, objectURL }
        loadedTracks.set(key, item)
        track.mode = 'disabled'
      } catch (error) {
        URL.revokeObjectURL(objectURL)
        throw error
      } finally {
        applying = false
      }
    }
    if (manual || result.downloaded === true) {
      select(item.track)
      remember()
    } else restore(!hadTracks)
  }
  const load = async (manual = false) => {
    if (loading || inactive()) return
    started = true
    loading = true
    try {
      if (!settingsReady) {
        const settings = getSettings ? await getSettings(controller.signal) : {}
        if (inactive()) return
        preferredTrack = settings.track || ''
        restore()
        settingsReady = true
        trackList.addEventListener?.('change', changed)
      }
      const downloaded = [...loadedTracks].find(([key]) => key.startsWith('download:'))?.[1]
      if (manual && downloaded) {
        select(downloaded.track)
        remember()
        onStatus('existing')
        return
      }
      const cacheOptions = { download_only: true, cache_only: true }
      const hadTracks = tracks().length > 0
      if (manual || (autoDownload && !hadTracks)) onStatus('searching')
      const result = await resolve(
        controller.signal,
        manual,
        !manual && hadTracks ? cacheOptions : undefined
      )
      if (inactive()) return
      add(result, manual)
      if (!manual && !hadTracks && (result.source === 'local' || result.status === 'existing')) {
        const cached = await resolve(controller.signal, false, cacheOptions)
        if (inactive()) return
        add(cached, false)
      }
      onStatus(tracks().length ? (result.blob ? 'ready' : 'existing') : result.status)
    } catch {
      if (!inactive()) onStatus('error')
    } finally {
      applying = false
      loading = false
    }
  }
  const loadAutomatically = () => {
    if (!started) void load()
  }
  const loadManually = () => void load(true)
  player.on('loadedmetadata', loadAutomatically)
  player.on('javbossloadsubtitles', loadManually)
  if (player.readyState() >= 1) loadAutomatically()
  return () => {
    if (controller.signal.aborted) return
    controller.abort()
    trackList.removeEventListener?.('change', changed)
    if (!player.isDisposed()) {
      player.off('loadedmetadata', loadAutomatically)
      player.off('javbossloadsubtitles', loadManually)
    }
    for (const { track, objectURL } of loadedTracks.values()) {
      if (!player.isDisposed()) player.removeRemoteTextTrack(track)
      URL.revokeObjectURL(objectURL)
    }
    loadedTracks.clear()
  }
}
