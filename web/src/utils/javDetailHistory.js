export const JAV_DETAIL_HISTORY_KEY = '__javbossJavDetail'

export function getJavDetailId(search) {
  const raw = new URLSearchParams(search).get('jav_detail') || ''
  const id = Number(raw)
  return /^\d+$/.test(raw) && Number.isSafeInteger(id) && id > 0 ? id : null
}

export function withJavDetail(route, id) {
  const [pathname, search = ''] = route.split('?')
  const params = new URLSearchParams(search)
  if (id) params.set('jav_detail', String(id))
  else params.delete('jav_detail')
  const query = params.toString()
  return `${pathname}${query ? `?${query}` : ''}`
}

export function normalizeJavDetailState(value) {
  const scrollTop = Number(value?.scrollTop)
  return { scrollTop: Number.isFinite(scrollTop) && scrollTop > 0 ? scrollTop : 0 }
}
