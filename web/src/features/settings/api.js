import { apiFetch, apiError, parseJSONResponse, jsonHeaders } from '@/api/client'

export async function fetchAvailabilityProviders({ signal } = {}) {
  const res = await apiFetch('/jav/providers', { signal, cache: 'no-store' })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function checkProviderAvailability(provider, { signal } = {}) {
  const res = await apiFetch(`/jav/providers/${encodeURIComponent(provider)}/availability`, {
    method: 'POST',
    signal,
  })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
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
