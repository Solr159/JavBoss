import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'

const jsonHeaders = { 'Content-Type': 'application/json' }
const javIdolResolveInFlight = new Map()
const javSampleImagesResolveInFlight = new Map()
const javSampleImagesResolved = new Map()
export const authExpiredEvent = 'javboss:auth-expired'

async function apiError(res) {
  const payload = await res.json().catch(() => ({}))
  return new Error(
    getErrorMessage(zh(String(payload.error_zh || ''), String(payload.error_en || '')))
  )
}

async function apiFetch(input, init = {}) {
  const res = await fetch(input, init)
  if (res.status === 401 && typeof window !== 'undefined') {
    window.dispatchEvent(new Event(authExpiredEvent))
  }
  return res
}

async function parseJSONResponse(res) {
  const contentType = String(res.headers.get('content-type') || '').toLowerCase()
  if (!contentType.includes('json')) {
    throw new Error(
      zh(
        '服务端接口返回了网页，请确认后端已更新并重启',
        'The server returned a web page. Make sure the backend is updated and restarted.'
      )
    )
  }
  return res.json()
}

export async function fetchAuthStatus() {
  const res = await fetch('/auth/status', { cache: 'no-store' })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function loginWithPassword(password) {
  const res = await fetch('/auth/login', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ password }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function logoutSession() {
  const res = await fetch('/auth/logout', { method: 'POST' })
  if (!res.ok && res.status !== 401) {
    throw await apiError(res)
  }
}

export async function changePassword(currentPassword, newPassword) {
  const res = await apiFetch('/auth/password', {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function fetchVideos({
  limit = 25,
  offset = 0,
  tags = [],
  search = '',
  sort = '',
  seed = null,
  directoryIds = [],
  hideJav = false,
} = {}) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  if (tags.length) params.set('tags', tags.join(','))
  if (search) params.set('search', search)
  if (sort) params.set('sort', sort)
  if (seed != null) params.set('seed', String(seed))
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  params.set('hide_jav', hideJav ? '1' : '0')
  const res = await apiFetch(`/videos?${params.toString()}`)
  if (!res.ok) throw await apiError(res)
  const data = await res.json()
  // Support both new shape {items,total} and legacy array for backward compatibility
  if (Array.isArray(data)) {
    return { items: data, total: data.length }
  }
  return data
}

export async function fetchTags({ directoryIds = [], hideJav = false } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  params.set('hide_jav', hideJav ? '1' : '0')
  const query = params.toString()
  const res = await apiFetch(`/tags${query ? `?${query}` : ''}`)
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function createTag(name) {
  const res = await apiFetch('/tags', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ name }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchConfig() {
  const res = await apiFetch('/config')
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function updateConfig(payload) {
  const res = await apiFetch('/config', {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchTools() {
  const res = await apiFetch('/tools', { cache: 'no-store' })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function downloadFFmpeg() {
  const res = await apiFetch('/tools/ffmpeg/download', { method: 'POST' })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function deleteTag(id) {
  const res = await apiFetch(`/tags/${id}`, { method: 'DELETE' })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function deleteTagsBatch(tagIds) {
  const res = await apiFetch('/tags/batch_delete', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ tag_ids: tagIds }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function renameTag(id, name) {
  const res = await apiFetch(`/tags/${id}`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify({ name }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function addTagToVideos(tagId, videoIds) {
  const res = await apiFetch('/videos/tags/add', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ tag_id: tagId, video_ids: videoIds }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function removeTagFromVideos(tagId, videoIds) {
  const res = await apiFetch('/videos/tags/remove', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ tag_id: tagId, video_ids: videoIds }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function replaceTagsForVideos(videoIds, tagIds) {
  const res = await apiFetch('/videos/tags/replace', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ video_ids: videoIds, tag_ids: tagIds }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
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

export async function playVideoFile({ id, path, dirPath, startTime }) {
  const res = await apiFetch('/videos/play', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ video_id: id, path, dir_path: dirPath, start_time: startTime }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
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

export async function updateVideoJavScrapeSettings(videoId, { mode = 'auto', code = '' } = {}) {
  const res = await apiFetch(`/videos/${videoId}/jav-scrape`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify({ mode, code }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function lookupVideoJavScrapeJavDB(videoId, code) {
  const params = new URLSearchParams()
  params.set('code', String(code || '').trim())
  const res = await apiFetch(`/videos/${videoId}/jav-scrape/javdb?${params.toString()}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchVideoJavScrapePossibleCodes(videoId) {
  const res = await apiFetch(`/videos/${videoId}/jav-scrape/possible-codes`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function manualVideoJavScrape(videoId, locationId, info) {
  const res = await apiFetch(`/videos/${videoId}/jav-scrape/manual`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ ...(info || {}), location_id: locationId }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

// Directories
export async function fetchDirectories() {
  const res = await apiFetch('/directories', { cache: 'no-store' })
  if (!res.ok) throw await apiError(res)
  const ct = res.headers.get('content-type') || ''
  if (!ct.includes('application/json')) {
    console.warn(
      zh('目录接口返回非 JSON，响应类型:', 'Directory API returned non-JSON content type:'),
      ct
    )
    return []
  }
  return res.json()
}

export async function createDirectory({ path }) {
  const res = await apiFetch('/directories', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ path }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function pickDirectory() {
  const res = await apiFetch('/directories/pick', {
    method: 'POST',
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function updateDirectory(id, payload) {
  const res = await apiFetch(`/directories/${id}`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function deleteDirectory(id) {
  return updateDirectory(id, { is_delete: true })
}

export async function processDirectory(id, mode, layout = 'prefix') {
  const res = await apiFetch(`/directories/${id}/process`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ mode, layout }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function scanDirectory(id) {
  const res = await apiFetch(`/directories/${id}/scan`, {
    method: 'POST',
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavs({
  limit = 25,
  offset = 0,
  search = '',
  idolIds = [],
  tagIds = [],
  studioId = null,
  seriesId = null,
  prefix = '',
  soloOnly = false,
  sort = '',
  seed = null,
  directoryIds = [],
  favoriteGroupId = null,
} = {}) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  if (search) params.set('search', search)
  if (idolIds.length) params.set('idol_ids', idolIds.join(','))
  if (tagIds.length) params.set('tag_ids', tagIds.join(','))
  if (studioId !== null && studioId !== undefined) params.set('studio_id', String(studioId))
  if (seriesId) params.set('series_id', String(seriesId))
  if (prefix) params.set('prefix', prefix)
  if (soloOnly) params.set('solo', '1')
  if (sort) params.set('sort', sort)
  if (seed != null) params.set('seed', String(seed))
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  if (favoriteGroupId) params.set('favorite_group_id', String(favoriteGroupId))
  const res = await apiFetch(`/jav?${params.toString()}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

function javSampleImagesRequest(id, directoryIds) {
  const javId = Number(id)
  const normalizedDirectoryIds = directoryIds
    .map((directoryId) => Number(directoryId))
    .filter((directoryId) => Number.isFinite(directoryId) && directoryId > 0)
  return {
    javId,
    normalizedDirectoryIds,
    requestKey: `${javId}:${normalizedDirectoryIds.join(',')}`,
  }
}

export function getResolvedJavSampleImages(id, { directoryIds = [] } = {}) {
  const { javId, requestKey } = javSampleImagesRequest(id, directoryIds)
  if (!Number.isFinite(javId) || javId <= 0) return null
  return javSampleImagesResolved.get(requestKey) || null
}

export function resolveJavSampleImages(id, { directoryIds = [] } = {}) {
  const { javId, normalizedDirectoryIds, requestKey } = javSampleImagesRequest(id, directoryIds)
  if (!Number.isFinite(javId) || javId <= 0) return Promise.resolve([])
  const resolved = javSampleImagesResolved.get(requestKey)
  if (resolved) return Promise.resolve(resolved)

  const params = new URLSearchParams()
  if (normalizedDirectoryIds.length) {
    params.set('directory_ids', normalizedDirectoryIds.join(','))
  }
  const query = params.toString()
  const existing = javSampleImagesResolveInFlight.get(requestKey)
  if (existing) return existing

  const request = apiFetch(
    `/jav/items/${encodeURIComponent(javId)}/sample-images${query ? `?${query}` : ''}`,
    { method: 'POST', cache: 'no-store' }
  )
    .then(async (res) => {
      if (!res.ok) throw await apiError(res)
      const payload = await res.json()
      const images = Array.isArray(payload?.sample_images) ? payload.sample_images : []
      javSampleImagesResolved.set(requestKey, images)
      return images
    })
    .finally(() => {
      javSampleImagesResolveInFlight.delete(requestKey)
    })
  javSampleImagesResolveInFlight.set(requestKey, request)
  return request
}

export async function fetchJavPrefixes({ directoryIds = [] } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/prefixes${query ? `?${query}` : ''}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavTags({ directoryIds = [] } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/tags${query ? `?${query}` : ''}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function updateJavCover(code, url) {
  const res = await apiFetch(`/jav/${encodeURIComponent(code)}/cover`, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify({ url }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function updateJavItem(id, payload, { directoryIds = [] } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/items/${encodeURIComponent(id)}${query ? `?${query}` : ''}`, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify(payload || {}),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function createJavTag(name) {
  const res = await apiFetch('/jav/tags', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ name }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function renameJavTag(id, name) {
  const res = await apiFetch(`/jav/tags/${id}`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify({ name }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function deleteJavTag(id) {
  const res = await apiFetch(`/jav/tags/${id}`, { method: 'DELETE' })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function deleteJavTagsBatch(tagIds) {
  const res = await apiFetch('/jav/tags/batch_delete', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ tag_ids: tagIds }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function replaceJavTagsForItems(javIds, tagIds) {
  const res = await apiFetch('/jav/tags/replace', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ jav_ids: javIds, tag_ids: tagIds }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function addJavTagToJavs(tagId, javIds) {
  const res = await apiFetch('/jav/tags/add', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ tag_id: tagId, jav_ids: javIds }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function removeJavTagFromJavs(tagId, javIds) {
  const res = await apiFetch('/jav/tags/remove', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ tag_id: tagId, jav_ids: javIds }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function fetchJavIdols({
  limit = 25,
  offset = 0,
  search = '',
  sort = '',
  directoryIds = [],
  favoriteGroupId = null,
} = {}) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  if (search) params.set('search', search)
  if (sort) params.set('sort', sort)
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  if (favoriteGroupId) params.set('favorite_group_id', String(favoriteGroupId))
  const res = await apiFetch(`/jav/idols?${params.toString()}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavIdolOptions({ limit = 25, offset = 0, search = '' } = {}) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  if (search) params.set('search', search)
  const res = await apiFetch(`/jav/idols/options?${params.toString()}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function mergeJavIdols({ canonicalId, mergeIds = [], directoryIds = [] } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/idols/merge${query ? `?${query}` : ''}`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({
      canonical_id: canonicalId,
      merge_ids: mergeIds,
    }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function updateJavIdol(id, payload, { directoryIds = [] } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/idols/${encodeURIComponent(id)}${query ? `?${query}` : ''}`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

const JAV_FAVORITE_ENTITY_ROUTES = {
  jav: 'jav',
  idol: 'idol',
  studio: 'studio',
  series: 'series',
}

function javFavoriteEntityRoute(entityType = 'idol') {
  return JAV_FAVORITE_ENTITY_ROUTES[String(entityType || '').trim()] || 'idol'
}

export async function fetchJavFavoriteGroups(entityType = 'idol', { directoryIds = [] } = {}) {
  const route = javFavoriteEntityRoute(entityType)
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/${route}-favorite-groups${query ? `?${query}` : ''}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  const data = await res.json()
  return Array.isArray(data?.items) ? data.items : []
}

export async function createJavFavoriteGroup(entityType = 'idol', name) {
  const route = javFavoriteEntityRoute(entityType)
  const res = await apiFetch(`/jav/${route}-favorite-groups`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ name }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function renameJavFavoriteGroup(entityType = 'idol', id, name) {
  const route = javFavoriteEntityRoute(entityType)
  const res = await apiFetch(`/jav/${route}-favorite-groups/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify({ name }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function deleteJavFavoriteGroup(entityType = 'idol', id) {
  const route = javFavoriteEntityRoute(entityType)
  const res = await apiFetch(`/jav/${route}-favorite-groups/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function reorderJavFavoriteGroups(entityType = 'idol', groupIds = []) {
  const route = javFavoriteEntityRoute(entityType)
  const res = await apiFetch(`/jav/${route}-favorite-groups/order`, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify({ group_ids: groupIds }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function fetchJavFavoriteGroupItems(
  entityType = 'idol',
  id,
  { directoryIds = [] } = {}
) {
  const route = javFavoriteEntityRoute(entityType)
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(
    `/jav/${route}-favorite-groups/${encodeURIComponent(id)}/items${query ? `?${query}` : ''}`
  )
  if (!res.ok) {
    throw await apiError(res)
  }
  const data = await res.json()
  return Array.isArray(data?.items) ? data.items : []
}

export async function reorderJavFavoriteGroupItems(entityType = 'idol', id, entityIds = []) {
  const route = javFavoriteEntityRoute(entityType)
  const res = await apiFetch(`/jav/${route}-favorite-groups/${encodeURIComponent(id)}/item-order`, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify({ entity_ids: entityIds }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function removeJavFavoriteGroupItems(entityType = 'idol', id, entityIds = []) {
  const route = javFavoriteEntityRoute(entityType)
  const res = await apiFetch(
    `/jav/${route}-favorite-groups/${encodeURIComponent(id)}/items/remove`,
    {
      method: 'POST',
      headers: jsonHeaders,
      body: JSON.stringify({ entity_ids: entityIds }),
    }
  )
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function fetchJavFavoriteSelection(entityType = 'idol', id) {
  const route = javFavoriteEntityRoute(entityType)
  const itemPath = route === 'jav' ? 'items' : route === 'series' ? 'series' : `${route}s`
  const res = await apiFetch(`/jav/${itemPath}/${encodeURIComponent(id)}/favorite-groups`)
  if (!res.ok) {
    throw await apiError(res)
  }
  const data = await res.json()
  return Array.isArray(data?.selected_group_ids) ? data.selected_group_ids : []
}

export async function replaceJavFavoriteGroups(entityType = 'idol', id, groupIds = []) {
  const route = javFavoriteEntityRoute(entityType)
  const itemPath = route === 'jav' ? 'items' : route === 'series' ? 'series' : `${route}s`
  const res = await apiFetch(`/jav/${itemPath}/${encodeURIComponent(id)}/favorite-groups`, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify({ group_ids: groupIds }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function fetchJavIdolCoverOptions(id, { directoryIds = [] } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(
    `/jav/idols/${encodeURIComponent(id)}/cover-options${query ? `?${query}` : ''}`
  )
  if (!res.ok) {
    throw await apiError(res)
  }
  const data = await res.json()
  return Array.isArray(data?.items) ? data.items : []
}

export async function updateJavIdolCover(
  id,
  { javId = 0, cropLeft = 0.53, directoryIds = [] } = {}
) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(
    `/jav/idols/${encodeURIComponent(id)}/cover${query ? `?${query}` : ''}`,
    {
      method: 'PUT',
      headers: jsonHeaders,
      body: JSON.stringify({ jav_id: javId, crop_left: cropLeft }),
    }
  )
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavStudios({
  limit = 25,
  offset = 0,
  search = '',
  directoryIds = [],
  favoriteGroupId = null,
} = {}) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  if (search) params.set('search', search)
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  if (favoriteGroupId) params.set('favorite_group_id', String(favoriteGroupId))
  const res = await apiFetch(`/jav/studios?${params.toString()}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavStudioOptions({ limit = 25, offset = 0, search = '' } = {}) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  if (search) params.set('search', search)
  const res = await apiFetch(`/jav/studios/options?${params.toString()}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function mergeJavStudios({ canonicalId, mergeIds = [], directoryIds = [] } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/studios/merge${query ? `?${query}` : ''}`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({
      canonical_id: canonicalId,
      merge_ids: mergeIds,
    }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function updateJavStudio(id, payload, { directoryIds = [] } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/studios/${encodeURIComponent(id)}${query ? `?${query}` : ''}`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavStudioJavDBURL({ studioId = null } = {}) {
  const params = new URLSearchParams()
  params.set('studio_id', String(studioId || ''))
  const res = await apiFetch(`/jav/studios/javdb-url?${params.toString()}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  const data = await res.json()
  return data?.url || ''
}

export async function fetchJavStudioPreview(id, { directoryIds = [] } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/studios/${encodeURIComponent(id)}${query ? `?${query}` : ''}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavSeries({
  limit = 25,
  offset = 0,
  search = '',
  directoryIds = [],
  favoriteGroupId = null,
} = {}) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  if (search) params.set('search', search)
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  if (favoriteGroupId) params.set('favorite_group_id', String(favoriteGroupId))
  const res = await apiFetch(`/jav/series?${params.toString()}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavSeriesJavDBURL({ seriesId = null } = {}) {
  const params = new URLSearchParams()
  params.set('series_id', String(seriesId || ''))
  const res = await apiFetch(`/jav/series/javdb-url?${params.toString()}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  const data = await res.json()
  return data?.url || ''
}

export async function fetchJavSeriesPreview(id, { directoryIds = [] } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/series/${encodeURIComponent(id)}${query ? `?${query}` : ''}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavIdolPreview(id, { directoryIds = [] } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/idols/${encodeURIComponent(id)}${query ? `?${query}` : ''}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavIdolJavDBURL({ code = '', name = '' } = {}) {
  const params = new URLSearchParams()
  params.set('code', code)
  params.set('name', name)
  const res = await apiFetch(`/jav/idols/javdb-url?${params.toString()}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  const data = await res.json()
  return data?.url || ''
}

export async function fetchJavJavDBURL({ code = '' } = {}) {
  const params = new URLSearchParams()
  params.set('code', code)
  const res = await apiFetch(`/jav/javdb-url?${params.toString()}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  const data = await res.json()
  return data?.url || ''
}

export async function resolveJavIdols(ids = []) {
  const clean = Array.from(
    new Set(
      (ids || [])
        .map((id) => Number.parseInt(String(id), 10))
        .filter((id) => Number.isFinite(id) && id > 0)
    )
  ).sort((a, b) => a - b)
  if (!clean.length) return []
  const key = clean.join(',')
  if (javIdolResolveInFlight.has(key)) {
    return javIdolResolveInFlight.get(key)
  }
  const params = new URLSearchParams()
  params.set('ids', clean.join(','))
  const request = apiFetch(`/jav/idols/resolve?${params.toString()}`)
    .then(async (res) => {
      if (!res.ok) {
        throw await apiError(res)
      }
      const data = await res.json()
      return Array.isArray(data?.items) ? data.items : []
    })
    .finally(() => {
      javIdolResolveInFlight.delete(key)
    })
  javIdolResolveInFlight.set(key, request)
  return request
}
