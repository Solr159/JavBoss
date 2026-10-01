import { apiFetch, apiError, jsonHeaders } from '@/api/client'
import { zh } from '@/utils/i18n'

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

export async function browseDirectories(path = '', { showHidden = false, signal } = {}) {
  const params = new URLSearchParams({ path, show_hidden: String(showHidden) })
  const res = await apiFetch(`/directories/browse?${params}`, {
    cache: 'no-store',
    signal,
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
