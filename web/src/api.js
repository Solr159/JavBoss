import { zh } from '@/utils/i18n'

const jsonHeaders = { 'Content-Type': 'application/json' }
const javIdolResolveInFlight = new Map()
export const authExpiredEvent = 'javboss:auth-expired'

async function apiError(res, fallback) {
  const payload = await res.json().catch(() => ({}))
  return new Error(payload.error || fallback)
}

async function apiFetch(input, init = {}) {
  const res = await fetch(input, init)
  if (res.status === 401 && typeof window !== 'undefined') {
    window.dispatchEvent(new Event(authExpiredEvent))
  }
  return res
}

export async function fetchAuthStatus() {
  const res = await fetch('/auth/status', { cache: 'no-store' })
  if (!res.ok) throw await apiError(res, zh('检查登录状态失败', 'Failed to check login status'))
  return res.json()
}

export async function loginWithPassword(password) {
  const res = await fetch('/auth/login', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ password }),
  })
  if (!res.ok) {
    if (res.status === 401) throw new Error(zh('密码错误', 'Incorrect password'))
    if (res.status === 400) throw new Error(zh('密码格式无效', 'Invalid password format'))
    if (res.status === 429) {
      throw new Error(zh('登录尝试过多，请稍后再试', 'Too many attempts. Try again later.'))
    }
    throw await apiError(res, zh('登录失败', 'Login failed'))
  }
  return res.json()
}

export async function logoutSession() {
  const res = await fetch('/auth/logout', { method: 'POST' })
  if (!res.ok && res.status !== 401) {
    throw await apiError(res, zh('退出登录失败', 'Failed to sign out'))
  }
}

export async function changePassword(currentPassword, newPassword) {
  const res = await apiFetch('/auth/password', {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
  })
  if (!res.ok) {
    if (res.status === 400) throw new Error(zh('当前密码错误', 'Current password is incorrect'))
    if (res.status === 422) {
      throw new Error(
        zh(
          '新密码需为 6-20 个字符，且首尾不能有空格',
          'New password must be 6-20 characters without surrounding spaces'
        )
      )
    }
    throw await apiError(res, zh('修改密码失败', 'Failed to change password'))
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
  if (!res.ok) throw new Error(zh('加载视频失败', 'Failed to load videos'))
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
  if (!res.ok) throw new Error(zh('加载标签失败', 'Failed to load tags'))
  return res.json()
}

export async function createTag(name) {
  const res = await apiFetch('/tags', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ name }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('创建标签失败', 'Failed to create tag'))
  }
  return res.json()
}

export async function fetchConfig() {
  const res = await apiFetch('/config')
  if (!res.ok) throw new Error(zh('加载配置失败', 'Failed to load config'))
  return res.json()
}

export async function updateConfig(payload) {
  const res = await apiFetch('/config', {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('更新配置失败', 'Failed to update config'))
  }
  return res.json()
}

export async function deleteTag(id) {
  const res = await apiFetch(`/tags/${id}`, { method: 'DELETE' })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('删除标签失败', 'Failed to delete tag'))
  }
}

export async function deleteTagsBatch(tagIds) {
  const res = await apiFetch('/tags/batch_delete', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ tag_ids: tagIds }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('批量删除标签失败', 'Failed to delete tags'))
  }
}

export async function renameTag(id, name) {
  const res = await apiFetch(`/tags/${id}`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify({ name }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('重命名标签失败', 'Failed to rename tag'))
  }
}

export async function addTagToVideos(tagId, videoIds) {
  const res = await apiFetch('/videos/tags/add', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ tag_id: tagId, video_ids: videoIds }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('添加标签到视频失败', 'Failed to add tag to videos'))
  }
}

export async function removeTagFromVideos(tagId, videoIds) {
  const res = await apiFetch('/videos/tags/remove', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ tag_id: tagId, video_ids: videoIds }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('从视频移除标签失败', 'Failed to remove tag from videos'))
  }
}

export async function replaceTagsForVideos(videoIds, tagIds) {
  const res = await apiFetch('/videos/tags/replace', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ video_ids: videoIds, tag_ids: tagIds }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('更新视频标签失败', 'Failed to update video tags'))
  }
}

export async function openVideoFile({ path, dirPath }) {
  const res = await apiFetch('/videos/open', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ path, dir_path: dirPath }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('打开文件失败', 'Failed to open file'))
  }
}

export async function playVideoFile({ id, path, dirPath, startTime }) {
  const res = await apiFetch('/videos/play', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ video_id: id, path, dir_path: dirPath, start_time: startTime }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('播放文件失败', 'Failed to play file'))
  }
}

export async function revealVideoLocation({ path, dirPath }) {
  const res = await apiFetch('/videos/reveal', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ path, dir_path: dirPath }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('打开所在位置失败', 'Failed to reveal file'))
  }
}

export async function incrementVideoPlayCount(id) {
  const res = await apiFetch(`/videos/${id}/play`, { method: 'POST' })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('增加播放次数失败', 'Failed to increment play count'))
  }
}

export async function fetchPlaybackInfo(id, { locationId } = {}) {
  const params = new URLSearchParams()
  if (locationId) params.set('location_id', String(locationId))
  const query = params.toString()
  const res = await apiFetch(`/videos/${id}/streams${query ? `?${query}` : ''}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载播放信息失败', 'Failed to load playback info'))
  }
  return res.json()
}

export async function fetchVideoScreenshots(id) {
  const res = await apiFetch(`/videos/${id}/screenshots`, { cache: 'no-store' })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载截图失败', 'Failed to load screenshots'))
  }
  const data = await res.json()
  return Array.isArray(data?.items) ? data.items : []
}

export async function fetchVideoScreenshotsByIds(videoIds) {
  const params = new URLSearchParams()
  params.set('video_id_list', (videoIds || []).join(','))
  const res = await apiFetch(`/videos/screenshots?${params.toString()}`, { cache: 'no-store' })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载视频截图失败', 'Failed to load video screenshots'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('截图失败', 'Failed to capture screenshot'))
  }
  return res.json()
}

export async function deleteVideoScreenshot(videoId, name) {
  const res = await apiFetch(`/videos/${videoId}/screenshots/${encodeURIComponent(name)}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('删除截图失败', 'Failed to delete screenshot'))
  }
}

export async function updateVideoCover(videoId, screenshotName) {
  const res = await apiFetch(`/videos/${videoId}/cover`, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify({ screenshot_name: screenshotName }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('保存视频封面失败', 'Failed to save video cover'))
  }
  return res.json()
}

export async function resetVideoCover(videoId) {
  const res = await apiFetch(`/videos/${videoId}/cover`, { method: 'DELETE' })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('恢复默认封面失败', 'Failed to restore default cover'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('重命名视频失败', 'Failed to rename video'))
  }
  return res.json()
}

export async function deleteVideoLocation(videoId, locationId) {
  const res = await apiFetch(`/videos/${videoId}/locations/${locationId}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('删除视频失败', 'Failed to delete video'))
  }
}

export async function updateVideoJavScrapeSettings(videoId, { mode = 'auto', code = '' } = {}) {
  const res = await apiFetch(`/videos/${videoId}/jav-scrape`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify({ mode, code }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('保存刮削设置失败', 'Failed to save scrape settings'))
  }
  return res.json()
}

export async function lookupVideoJavScrapeJavDB(videoId, code) {
  const params = new URLSearchParams()
  params.set('code', String(code || '').trim())
  const res = await apiFetch(`/videos/${videoId}/jav-scrape/javdb?${params.toString()}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('从 JavDB 获取信息失败', 'Failed to fetch metadata from JavDB'))
  }
  return res.json()
}

export async function fetchVideoJavScrapePossibleCodes(videoId) {
  const res = await apiFetch(`/videos/${videoId}/jav-scrape/possible-codes`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('提取番号失败', 'Failed to extract codes'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('手动刮削失败', 'Manual scrape failed'))
  }
  return res.json()
}

// Directories
export async function fetchDirectories() {
  const res = await apiFetch('/directories')
  if (!res.ok) throw new Error(zh('加载目录失败', 'Failed to load directories'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('创建目录失败', 'Failed to create directory'))
  }
  return res.json()
}

export async function pickDirectory() {
  const res = await apiFetch('/directories/pick', {
    method: 'POST',
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('选择目录失败', 'Failed to choose directory'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('更新目录失败', 'Failed to update directory'))
  }
  return res.json()
}

export async function deleteDirectory(id) {
  return updateDirectory(id, { is_delete: true })
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载 JAV 失败', 'Failed to load JAV'))
  }
  return res.json()
}

export async function fetchJavPrefixes({ directoryIds = [] } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/prefixes${query ? `?${query}` : ''}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载番号失败', 'Failed to load JAV codes'))
  }
  return res.json()
}

export async function fetchJavTags({ directoryIds = [] } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/tags${query ? `?${query}` : ''}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载 JAV 标签失败', 'Failed to load JAV tags'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('保存 JAV 封面失败', 'Failed to save JAV cover'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('保存 JAV 信息失败', 'Failed to save JAV info'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('创建 JAV 标签失败', 'Failed to create JAV tag'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('重命名 JAV 标签失败', 'Failed to rename JAV tag'))
  }
}

export async function deleteJavTag(id) {
  const res = await apiFetch(`/jav/tags/${id}`, { method: 'DELETE' })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('删除 JAV 标签失败', 'Failed to delete JAV tag'))
  }
}

export async function deleteJavTagsBatch(tagIds) {
  const res = await apiFetch('/jav/tags/batch_delete', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ tag_ids: tagIds }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('批量删除 JAV 标签失败', 'Failed to delete JAV tags'))
  }
}

export async function replaceJavTagsForItems(javIds, tagIds) {
  const res = await apiFetch('/jav/tags/replace', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ jav_ids: javIds, tag_ids: tagIds }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('更新 JAV 标签失败', 'Failed to update JAV tags'))
  }
}

export async function addJavTagToJavs(tagId, javIds) {
  const res = await apiFetch('/jav/tags/add', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ tag_id: tagId, jav_ids: javIds }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('添加 JAV 标签失败', 'Failed to add JAV tag'))
  }
}

export async function removeJavTagFromJavs(tagId, javIds) {
  const res = await apiFetch('/jav/tags/remove', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ tag_id: tagId, jav_ids: javIds }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('移除 JAV 标签失败', 'Failed to remove JAV tag'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载女优失败', 'Failed to load idols'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载女优失败', 'Failed to load idols'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('合并女优失败', 'Failed to merge idols'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('保存女优信息失败', 'Failed to save idol info'))
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

function javFavoriteEntityLabel(entityType = 'idol') {
  switch (javFavoriteEntityRoute(entityType)) {
    case 'jav':
      return zh('作品', 'JAV')
    case 'studio':
      return zh('片商', 'studio')
    case 'series':
      return zh('系列', 'series')
    case 'idol':
    default:
      return zh('女优', 'idol')
  }
}

export async function fetchJavFavoriteGroups(entityType = 'idol', { directoryIds = [] } = {}) {
  const route = javFavoriteEntityRoute(entityType)
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/${route}-favorite-groups${query ? `?${query}` : ''}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    const label = javFavoriteEntityLabel(route)
    throw new Error(
      err.error || zh(`加载${label}收藏夹失败`, `Failed to load ${label} favorite groups`)
    )
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('创建收藏夹失败', 'Failed to create favorite group'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('重命名收藏夹失败', 'Failed to rename favorite group'))
  }
}

export async function deleteJavFavoriteGroup(entityType = 'idol', id) {
  const route = javFavoriteEntityRoute(entityType)
  const res = await apiFetch(`/jav/${route}-favorite-groups/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('删除收藏夹失败', 'Failed to delete favorite group'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(
      err.error || zh('保存女优收藏夹顺序失败', 'Failed to save favorite group order')
    )
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载收藏夹内容失败', 'Failed to load favorite group items'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('保存收藏夹顺序失败', 'Failed to save favorite item order'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('移除收藏夹内容失败', 'Failed to remove favorite items'))
  }
}

export async function fetchJavFavoriteSelection(entityType = 'idol', id) {
  const route = javFavoriteEntityRoute(entityType)
  const itemPath = route === 'jav' ? 'items' : route === 'series' ? 'series' : `${route}s`
  const res = await apiFetch(`/jav/${itemPath}/${encodeURIComponent(id)}/favorite-groups`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载收藏夹选择失败', 'Failed to load favorites'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('保存收藏夹失败', 'Failed to save favorites'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载封面作品失败', 'Failed to load cover works'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('保存女优封面失败', 'Failed to save idol cover'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载片商失败', 'Failed to load studios'))
  }
  return res.json()
}

export async function fetchJavStudioJavDBURL({ studioId = null } = {}) {
  const params = new URLSearchParams()
  params.set('studio_id', String(studioId || ''))
  const res = await apiFetch(`/jav/studios/javdb-url?${params.toString()}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载 JavDB 片商链接失败', 'Failed to load JavDB studio URL'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载片商预览失败', 'Failed to load studio preview'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载系列失败', 'Failed to load series'))
  }
  return res.json()
}

export async function fetchJavSeriesJavDBURL({ seriesId = null } = {}) {
  const params = new URLSearchParams()
  params.set('series_id', String(seriesId || ''))
  const res = await apiFetch(`/jav/series/javdb-url?${params.toString()}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载 JavDB 系列链接失败', 'Failed to load JavDB series URL'))
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
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载系列预览失败', 'Failed to load series preview'))
  }
  return res.json()
}

export async function fetchJavIdolPreview(id, { directoryIds = [] } = {}) {
  const params = new URLSearchParams()
  if (directoryIds.length) params.set('directory_ids', directoryIds.join(','))
  const query = params.toString()
  const res = await apiFetch(`/jav/idols/${encodeURIComponent(id)}${query ? `?${query}` : ''}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载女优预览失败', 'Failed to load idol preview'))
  }
  return res.json()
}

export async function fetchJavIdolJavDBURL({ code = '', name = '' } = {}) {
  const params = new URLSearchParams()
  params.set('code', code)
  params.set('name', name)
  const res = await apiFetch(`/jav/idols/javdb-url?${params.toString()}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载 JavDB 女优链接失败', 'Failed to load JavDB idol URL'))
  }
  const data = await res.json()
  return data?.url || ''
}

export async function fetchJavJavDBURL({ code = '' } = {}) {
  const params = new URLSearchParams()
  params.set('code', code)
  const res = await apiFetch(`/jav/javdb-url?${params.toString()}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || zh('加载 JavDB 影片链接失败', 'Failed to load JavDB movie URL'))
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
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || zh('加载女优名称失败', 'Failed to load idol names'))
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
