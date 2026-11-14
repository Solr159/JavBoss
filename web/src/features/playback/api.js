import { apiFetch, apiError, jsonHeaders } from '@/api/client'

const subtitleSelectionSaves = new Map()

export async function fetchVideoPreview(videoId, locationId, second, signal) {
  const params = new URLSearchParams({ second: String(second) })
  if (locationId) params.set('location_id', String(locationId))
  const res = await apiFetch(`/videos/${videoId}/preview?${params}`, { signal, cache: 'no-store' })
  if (!res.ok) throw await apiError(res)
  return res.blob()
}

export async function fetchVideoSubtitleSettings(videoId, signal) {
  await subtitleSelectionSaves.get(videoId)?.catch(() => {})
  const res = await apiFetch(`/videos/${videoId}/subtitle-settings`, { signal, cache: 'no-store' })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export function updateVideoSubtitleSelection(videoId, track) {
  const previous = subtitleSelectionSaves.get(videoId) || Promise.resolve()
  const pending = previous
    .catch(() => {})
    .then(async () => {
      const res = await apiFetch(`/videos/${videoId}/subtitle-settings`, {
        method: 'PATCH',
        headers: jsonHeaders,
        body: JSON.stringify({ track }),
        keepalive: true,
        signal: AbortSignal.timeout(10000),
      })
      if (!res.ok) throw await apiError(res)
      return res.json()
    })
  subtitleSelectionSaves.set(videoId, pending)
  const clear = () => {
    if (subtitleSelectionSaves.get(videoId) === pending) subtitleSelectionSaves.delete(videoId)
  }
  pending.then(clear, clear)
  return pending
}

export async function updateVideoSubtitleSettings(videoId, offsetMS) {
  const res = await apiFetch(`/videos/${videoId}/subtitle-settings`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify({ offset_ms: offsetMS }),
  })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function resolveVideoSubtitle(
  videoId,
  locationId,
  signal,
  manual = false,
  options = {}
) {
  const res = await apiFetch(`/videos/${videoId}/subtitles/resolve`, {
    method: 'POST',
    headers: jsonHeaders,
    signal,
    body: JSON.stringify({ location_id: locationId || 0, format: 'vtt', manual, ...options }),
  })
  if (res.status === 204) return { status: res.headers.get('X-Subtitle-Status') }
  if (!res.ok) throw await apiError(res)
  if (!res.headers.get('Content-Type')?.startsWith('text/vtt'))
    throw new Error('Invalid subtitle response')
  return {
    blob: await res.blob(),
    status: 'ready',
    language: res.headers.get('X-Subtitle-Language'),
    source: res.headers.get('X-Subtitle-Source'),
    key: decodeURIComponent(res.headers.get('X-Subtitle-Key') || ''),
    downloaded: res.headers.get('X-Subtitle-Downloaded') === 'true',
  }
}

export async function fetchWatchedTimes({ videoIds, javIds }, signal) {
  const query = new URLSearchParams()
  if (videoIds.length) query.set('video_ids', videoIds.join(','))
  if (javIds.length) query.set('jav_ids', javIds.join(','))
  const res = await apiFetch(`/videos/watched-time?${query}`, { signal, cache: 'no-store' })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function createPlaybackSession(videoId, locationId) {
  const res = await apiFetch(`/videos/${videoId}/playback-sessions`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ location_id: locationId || 0 }),
    keepalive: true,
    signal: AbortSignal.timeout(5000),
  })
  if (!res.ok) throw await apiError(res)
  const data = await res.json()
  if (!data.session_id) throw new Error('Missing playback session')
  return data.session_id
}

export async function reportPlaybackSession(videoId, sessionId, watchedMS) {
  const res = await apiFetch(`/videos/${videoId}/playback-sessions/${sessionId}`, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify({ watched_ms: watchedMS }),
    keepalive: true,
    signal: AbortSignal.timeout(5000),
  })
  if (res.status === 410) return false
  if (!res.ok) throw await apiError(res)
  return true
}
