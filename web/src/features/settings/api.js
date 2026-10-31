import { apiFetch, apiError, parseJSONResponse, jsonHeaders } from '@/api/client'

export async function fetchResources({ signal } = {}) {
  const res = await apiFetch('/system/resources', { signal, cache: 'no-store' })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

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

export async function fetchAppUpdate({ local = false, signal } = {}) {
  const res = await apiFetch(local ? '/__client/update' : '/system/update', {
    signal: updateRequestSignal(signal),
    cache: 'no-store',
  })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

function updateRequestSignal(signal) {
  const timeout = AbortSignal.timeout(5000)
  return signal ? AbortSignal.any([signal, timeout]) : timeout
}

// A restarted service may be reachable before its update endpoint can be read
// by the old page. Probe the public endpoint for the selected target instead.
// In client mode /auth/status is proxied to the server; /healthz is local.
export async function isAppUpdateTargetReady({ local = false, signal } = {}) {
  const res = await fetch(local ? '/healthz' : '/auth/status', {
    signal: updateRequestSignal(signal),
    cache: 'no-store',
  })
  if (!res.ok) return false
  const status = await parseJSONResponse(res)
  return local ? status?.status === 'ok' : typeof status?.authenticated === 'boolean'
}

export async function requestAppUpdate(action, { local = false } = {}) {
  const prefix = local ? '/__client/update' : '/system/update'
  const res = await apiFetch(`${prefix}/${action}`, { method: 'POST' })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}
