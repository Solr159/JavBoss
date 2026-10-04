import { apiFetch, apiError, jsonHeaders } from '@/api/client'

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
