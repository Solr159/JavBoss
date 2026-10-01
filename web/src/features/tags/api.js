import { apiFetch, apiError, jsonHeaders } from '@/api/client'

export async function fetchTags({ hideJav = false } = {}) {
  const params = new URLSearchParams()
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

export async function fetchTagCategories() {
  const res = await apiFetch('/tags/categories')
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function createTagCategory(name) {
  const res = await apiFetch('/tags/categories', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ name }),
  })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function reorderTagCategories(categoryIds) {
  const res = await apiFetch('/tags/categories/order', {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify({ category_ids: categoryIds }),
  })
  if (!res.ok) throw await apiError(res)
}

export async function renameTagCategory(id, name) {
  const res = await apiFetch(`/tags/categories/${id}`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify({ name }),
  })
  if (!res.ok) throw await apiError(res)
}

export async function deleteTagCategory(id) {
  const res = await apiFetch(`/tags/categories/${id}`, { method: 'DELETE' })
  if (!res.ok) throw await apiError(res)
}

export async function assignTagsCategory(tagIds, categoryId) {
  const res = await apiFetch('/tags/category', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ tag_ids: tagIds, category_id: categoryId }),
  })
  if (!res.ok) throw await apiError(res)
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
