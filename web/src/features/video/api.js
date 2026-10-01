import { apiFetch, apiError, jsonHeaders } from '@/api/client'

export async function fetchVideos({
  limit = 25,
  offset = 0,
  tags = [],
  search = '',
  sort = '',
  seed = null,
  hideJav = false,
  signal,
} = {}) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  if (tags.length) params.set('tags', tags.join(','))
  if (search) params.set('search', search)
  if (sort) params.set('sort', sort)
  if (seed != null) params.set('seed', String(seed))
  params.set('hide_jav', hideJav ? '1' : '0')
  const res = await apiFetch(`/videos?${params.toString()}`, { signal })
  if (!res.ok) throw await apiError(res)
  const data = await res.json()
  // Support both new shape {items,total} and legacy array for backward compatibility
  if (Array.isArray(data)) {
    return { items: data, total: data.length }
  }
  return data
}

export async function openVideoFile({ path, dirPath }) {
  const res = await apiFetch('/videos/open', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ path, dir_path: dirPath }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function playVideoFile({ id, locationId, path, dirPath, startTime }) {
  const res = await apiFetch('/videos/play', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({
      video_id: id,
      location_id: locationId,
      path,
      dir_path: dirPath,
      start_time: startTime,
    }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function playVideoPlaylist(items) {
  const res = await apiFetch('/videos/playlist', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ items }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function revealVideoLocation({ path, dirPath }) {
  const res = await apiFetch('/videos/reveal', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ path, dir_path: dirPath }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function incrementVideoPlayCount(id) {
  const res = await apiFetch(`/videos/${id}/play`, { method: 'POST' })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function fetchPlaybackInfo(id, { locationId } = {}) {
  const params = new URLSearchParams()
  if (locationId) params.set('location_id', String(locationId))
  const query = params.toString()
  const res = await apiFetch(`/videos/${id}/streams${query ? `?${query}` : ''}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchVideoScreenshots(id) {
  const res = await apiFetch(`/videos/${id}/screenshots`, { cache: 'no-store' })
  if (!res.ok) {
    throw await apiError(res)
  }
  const data = await res.json()
  return Array.isArray(data?.items) ? data.items : []
}

export async function fetchVideoScreenshotsByIds(videoIds) {
  const params = new URLSearchParams()
  params.set('video_id_list', (videoIds || []).join(','))
  const res = await apiFetch(`/videos/screenshots?${params.toString()}`, { cache: 'no-store' })
  if (!res.ok) {
    throw await apiError(res)
  }
  const data = await res.json()
  return Array.isArray(data?.items) ? data.items : []
}

export async function createVideoScreenshot(id, { second = 0, locationId } = {}) {
  const params = new URLSearchParams()
  if (locationId) params.set('location_id', String(locationId))
  const query = params.toString()
  const res = await apiFetch(`/videos/${id}/screenshots${query ? `?${query}` : ''}`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ second }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function deleteVideoScreenshot(videoId, name) {
  const res = await apiFetch(`/videos/${videoId}/screenshots/${encodeURIComponent(name)}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function updateVideoCover(videoId, screenshotName) {
  const res = await apiFetch(`/videos/${videoId}/cover`, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify({ screenshot_name: screenshotName }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function resetVideoCover(videoId) {
  const res = await apiFetch(`/videos/${videoId}/cover`, { method: 'DELETE' })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function renameVideoLocation(videoId, locationId, filename) {
  const res = await apiFetch(`/videos/${videoId}/locations/${locationId}`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify({ filename }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function deleteVideoLocation(videoId, locationId) {
  const res = await apiFetch(`/videos/${videoId}/locations/${locationId}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}
