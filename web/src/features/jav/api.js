import { apiFetch, jsonHeaders, apiError } from '@/api/client'

export async function fetchTitleTranslationModels(apiKey, { signal } = {}) {
  const res = await apiFetch('/jav/title-translation/models', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ api_key: apiKey }),
    signal,
  })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function translateJavTitle(id, { refresh = false, title, thinking } = {}) {
  const res = await apiFetch(`/jav/items/${encodeURIComponent(id)}/title-translation`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ refresh, title, thinking }),
  })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function startTitleTranslationBatch() {
  const res = await apiFetch('/jav/title-translation/batch', { method: 'POST' })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function fetchTitleTranslationBatch({ signal } = {}) {
  const res = await apiFetch('/jav/title-translation/batch', { signal, cache: 'no-store' })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export const javIdolResolveInFlight = new Map()

export const javSampleImagesResolveInFlight = new Map()

export const javSampleImagesResolved = new Map()

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

export async function linkVideoToExistingJav(videoId, locationId, code) {
  const res = await apiFetch(`/videos/${videoId}/jav-scrape/link`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ location_id: locationId, code }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavItem(id) {
  const res = await apiFetch(`/jav/items/${encodeURIComponent(id)}`)
  if (!res.ok) throw await apiError(res)
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
  favoriteRatingEnabled = false,
  favoriteRatingMin = 0.5,
  favoriteRatingMax = 5,
  sort = '',
  seed = null,
  favoriteGroupId = null,
  signal,
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
  if (favoriteRatingEnabled) {
    params.set('favorite_rating_min', String(favoriteRatingMin))
    params.set('favorite_rating_max', String(favoriteRatingMax))
  }
  if (sort) params.set('sort', sort)
  if (seed != null) params.set('seed', String(seed))
  if (favoriteGroupId) params.set('favorite_group_id', String(favoriteGroupId))
  const res = await apiFetch(`/jav?${params.toString()}`, { signal })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavFilterOptions({
  search = '',
  idolIds = [],
  tagIds = [],
  studioId = null,
  seriesId = null,
  prefix = '',
  soloOnly = false,
  favoriteRatingEnabled = false,
  favoriteRatingMin = 0.5,
  favoriteRatingMax = 5,
  favoriteGroupId = null,
  prefixSearch = '',
  idolSearch = '',
  tagSearch = '',
  studioSearch = '',
  seriesSearch = '',
  optionLimit = 120,
  signal,
} = {}) {
  const params = new URLSearchParams()
  if (search) params.set('search', search)
  if (idolIds.length) params.set('idol_ids', idolIds.join(','))
  if (tagIds.length) params.set('tag_ids', tagIds.join(','))
  if (studioId !== null && studioId !== undefined) params.set('studio_id', String(studioId))
  if (seriesId) params.set('series_id', String(seriesId))
  if (prefix) params.set('prefix', prefix)
  if (soloOnly) params.set('solo', '1')
  if (favoriteRatingEnabled) {
    params.set('favorite_rating_min', String(favoriteRatingMin))
    params.set('favorite_rating_max', String(favoriteRatingMax))
  }
  if (favoriteGroupId) params.set('favorite_group_id', String(favoriteGroupId))
  if (prefixSearch) params.set('prefix_search', prefixSearch)
  if (idolSearch) params.set('idol_search', idolSearch)
  if (tagSearch) params.set('tag_search', tagSearch)
  if (studioSearch) params.set('studio_search', studioSearch)
  if (seriesSearch) params.set('series_search', seriesSearch)
  params.set('option_limit', String(optionLimit))
  const res = await apiFetch(`/jav/filter-options?${params.toString()}`, { signal })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export function javSampleImagesRequest(id) {
  const javId = Number(id)
  return {
    javId,
    requestKey: String(javId),
  }
}

export function getResolvedJavSampleImages(id) {
  const { javId, requestKey } = javSampleImagesRequest(id)
  if (!Number.isFinite(javId) || javId <= 0) return null
  return javSampleImagesResolved.get(requestKey) || null
}

export function resolveJavSampleImages(id) {
  const { javId, requestKey } = javSampleImagesRequest(id)
  if (!Number.isFinite(javId) || javId <= 0) return Promise.resolve([])
  const resolved = javSampleImagesResolved.get(requestKey)
  if (resolved) return Promise.resolve(resolved)

  const existing = javSampleImagesResolveInFlight.get(requestKey)
  if (existing) return existing

  const request = apiFetch(`/jav/items/${encodeURIComponent(javId)}/sample-images`, {
    method: 'POST',
    cache: 'no-store',
  })
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

export async function fetchJavPrefixes() {
  const res = await apiFetch('/jav/prefixes')
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavTags() {
  const res = await apiFetch('/jav/tags')
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavTagCategories() {
  const res = await apiFetch('/jav/tag-categories')
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

export async function deleteJavVideos(id) {
  const res = await apiFetch(`/jav/items/${encodeURIComponent(id)}/videos`, { method: 'DELETE' })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function updateJavItem(id, payload) {
  const res = await apiFetch(`/jav/items/${encodeURIComponent(id)}`, {
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

export async function createJavScrapedTag(name) {
  const res = await apiFetch('/jav/tags/scraped', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ name }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function organizeJavTags() {
  const res = await apiFetch('/jav/tags/organize', { method: 'POST' })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function createJavTagCategory(name) {
  const res = await apiFetch('/jav/tag-categories', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ name }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function reorderJavTagCategories(categoryIds) {
  const res = await apiFetch('/jav/tag-categories/order', {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify({ category_ids: categoryIds }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function renameJavTagCategory(id, name) {
  const res = await apiFetch(`/jav/tag-categories/${id}`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify({ name }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function deleteJavTagCategory(id) {
  const res = await apiFetch(`/jav/tag-categories/${id}`, { method: 'DELETE' })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function assignJavTagsCategory(tagIds, categoryId) {
  const res = await apiFetch('/jav/tags/category', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ tag_ids: tagIds, category_id: categoryId }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
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
  favoriteGroupId = null,
  profileFilters = {},
  signal,
} = {}) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  if (search) params.set('search', search)
  if (sort) params.set('sort', sort)
  if (favoriteGroupId) params.set('favorite_group_id', String(favoriteGroupId))
  for (const key of ['height', 'age', 'cup', 'bust', 'waist', 'hips']) {
    const value = profileFilters?.[key]
    if (!value?.enabled) continue
    params.set(`idol_${key}_min`, String(value.min))
    params.set(`idol_${key}_max`, String(value.max))
  }
  const res = await apiFetch(`/jav/idols?${params.toString()}`, { signal })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function createJavIdol(name) {
  const res = await apiFetch('/jav/idols', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ name }),
  })
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

export async function mergeJavIdols({ canonicalId, mergeIds = [] } = {}) {
  const res = await apiFetch('/jav/idols/merge', {
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

export async function updateJavIdol(id, payload) {
  const res = await apiFetch(`/jav/idols/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavIdolCoverOptions(id) {
  const res = await apiFetch(`/jav/idols/${encodeURIComponent(id)}/cover-options`)
  if (!res.ok) {
    throw await apiError(res)
  }
  const data = await res.json()
  return Array.isArray(data?.items) ? data.items : []
}

export async function updateJavIdolCover(id, { javId = 0, cropLeft = 0.53 } = {}) {
  const res = await apiFetch(`/jav/idols/${encodeURIComponent(id)}/cover`, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify({ jav_id: javId, crop_left: cropLeft }),
  })
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavStudios({
  limit = 25,
  offset = 0,
  search = '',
  favoriteGroupId = null,
  signal,
} = {}) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  if (search) params.set('search', search)
  if (favoriteGroupId) params.set('favorite_group_id', String(favoriteGroupId))
  const res = await apiFetch(`/jav/studios?${params.toString()}`, { signal })
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

export async function mergeJavStudios({ canonicalId, mergeIds = [] } = {}) {
  const res = await apiFetch('/jav/studios/merge', {
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

export async function updateJavStudio(id, payload) {
  const res = await apiFetch(`/jav/studios/${encodeURIComponent(id)}`, {
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

export async function fetchJavStudioPreview(id) {
  const res = await apiFetch(`/jav/studios/${encodeURIComponent(id)}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavSeries({
  limit = 25,
  offset = 0,
  search = '',
  favoriteGroupId = null,
  signal,
} = {}) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  if (search) params.set('search', search)
  if (favoriteGroupId) params.set('favorite_group_id', String(favoriteGroupId))
  const res = await apiFetch(`/jav/series?${params.toString()}`, { signal })
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

export async function fetchJavSeriesPreview(id) {
  const res = await apiFetch(`/jav/series/${encodeURIComponent(id)}`)
  if (!res.ok) {
    throw await apiError(res)
  }
  return res.json()
}

export async function fetchJavIdolPreview(id) {
  const res = await apiFetch(`/jav/idols/${encodeURIComponent(id)}`)
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
