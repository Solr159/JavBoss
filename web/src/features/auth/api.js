import { apiError, jsonHeaders, apiFetch } from '@/api/client'

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

export async function fetchExtensionTokens() {
  const res = await apiFetch('/auth/extension-tokens', { cache: 'no-store' })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function createExtensionToken(name, expiresInDays) {
  const res = await apiFetch('/auth/extension-tokens', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ name, expires_in_days: expiresInDays }),
  })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function rotateExtensionToken(id, expiresInDays) {
  const res = await apiFetch(`/auth/extension-tokens/${id}/rotate`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ expires_in_days: expiresInDays }),
  })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function deleteExtensionToken(id) {
  const res = await apiFetch(`/auth/extension-tokens/${id}`, { method: 'DELETE' })
  if (!res.ok) throw await apiError(res)
}
