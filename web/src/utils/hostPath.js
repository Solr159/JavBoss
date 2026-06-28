export function displayHostPath(value, enabled = false) {
  const raw = String(value || '').trim()
  if (!enabled) return raw
  if (raw === '/host') return '/'
  if (raw.startsWith('/host/')) return raw.slice('/host'.length)
  return raw
}

export function apiHostPath(value, enabled = false) {
  const raw = String(value || '').trim()
  if (!enabled || !raw || raw === '/host' || raw.startsWith('/host/')) return raw
  if (raw === '/') return '/host'
  if (raw.startsWith('/')) return `/host${raw}`
  return raw
}
