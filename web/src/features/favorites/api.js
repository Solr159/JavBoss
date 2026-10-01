import { apiFetch, apiError, jsonHeaders } from '@/api/client'

export const JAV_FAVORITE_ENTITY_ROUTES = {
  jav: 'jav',
  idol: 'idol',
  studio: 'studio',
  series: 'series',
}

export function javFavoriteEntityRoute(entityType = 'idol') {
  return JAV_FAVORITE_ENTITY_ROUTES[String(entityType || '').trim()] || 'idol'
}

export async function fetchJavFavoriteGroups(entityType = 'idol') {
  const route = javFavoriteEntityRoute(entityType)
  const res = await apiFetch(`/jav/${route}-favorite-groups`)
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

export async function fetchJavFavoriteGroupItems(entityType = 'idol', id) {
  const route = javFavoriteEntityRoute(entityType)
  const res = await apiFetch(`/jav/${route}-favorite-groups/${encodeURIComponent(id)}/items`)
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

export async function addJavsToFavoriteGroups(javIds, groupIds) {
  const res = await apiFetch('/jav/items/favorite-groups/add', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ jav_ids: javIds, group_ids: groupIds }),
  })
  if (!res.ok) throw await apiError(res)
  return res.json()
}
