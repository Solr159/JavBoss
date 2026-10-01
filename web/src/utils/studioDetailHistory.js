export const STUDIO_DETAIL_HISTORY_KEY = '__javbossStudioDetail'

export function getStudioDetailId(search) {
  const raw = new URLSearchParams(search).get('studio_detail') || ''
  const id = Number(raw)
  return /^\d+$/.test(raw) && Number.isSafeInteger(id) && id > 0 ? id : null
}

export function withStudioDetail(route, id) {
  const [pathname, search = ''] = route.split('?')
  const params = new URLSearchParams(search)
  if (id) params.set('studio_detail', String(id))
  else params.delete('studio_detail')
  const query = params.toString()
  return `${pathname}${query ? `?${query}` : ''}`
}

export function normalizeStudioDetailState(value) {
  const scrollTop = Number(value?.scrollTop)
  return {
    prefixSort: value?.prefixSort === 'work_count_desc' ? 'work_count_desc' : 'name_asc',
    seriesSort: value?.seriesSort === 'name_asc' ? 'name_asc' : 'work_count_desc',
    scrollTop: Number.isFinite(scrollTop) && scrollTop > 0 ? scrollTop : 0,
  }
}

export function canReturnToStudioBackground(detailState, id, pageRoute) {
  return detailState?.id === id && detailState?.backgroundRoute === pageRoute
}
