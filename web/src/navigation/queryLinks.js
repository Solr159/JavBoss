import { buildUrlFromState, normalizeUrlStateFromStore } from '@/utils/urlState'
import { normalizeIdolSort, normalizeJavSort, normalizeIdolProfileFilters } from '@/constants/jav'
import { normalizeVideoSort } from '@/constants/video'

const has = (object, key) => Object.prototype.hasOwnProperty.call(object, key)

// Link overrides and browser synchronization share the same serializer.
export function buildVideoQueryLink(state, options = {}, pathname = '/') {
  const tagsByName = new Map((state.tags || []).map((tag) => [tag.name, tag.id]))
  const current = normalizeUrlStateFromStore(state, tagsByName)
  const video = { ...current.video }
  for (const key of ['page', 'search', 'random', 'seed', 'tagIds']) {
    if (options[key] != null) video[key] = options[key]
  }
  video.search = String(video.search || '').trim()
  if (has(options, 'tempSort')) video.tempSort = normalizeVideoSort(options.tempSort, '')
  return buildUrlFromState({ ...current, view: 'video', video }, pathname)
}

export function buildJavQueryLink(state, options = {}, pathname = '/') {
  const tab = options.tab ?? state.javTab
  const current = normalizeUrlStateFromStore({ ...state, javTab: tab }, new Map())
  const jav = { ...current.jav }
  // Preserve explicit nulls for clearing relations and favorites.
  for (const key of ['studioId', 'seriesId', 'favoriteGroupId']) {
    if (has(options, key)) jav[key] = options[key]
  }
  for (const key of [
    'page',
    'search',
    'idolIds',
    'tagIds',
    'studioName',
    'seriesName',
    'favoriteRatingMin',
    'favoriteRatingMax',
    'seed',
  ]) {
    if (options[key] != null) jav[key] = options[key]
  }
  if (has(options, 'studioId') && options.studioName == null) jav.studioName = ''
  if (has(options, 'seriesId') && options.seriesName == null) jav.seriesName = ''
  jav.search = String(jav.search || '').trim()
  jav.prefix = String(has(options, 'prefix') ? options.prefix || '' : state.javPrefix || '')
    .trim()
    .toUpperCase()
  for (const key of ['soloOnly', 'favoriteRatingEnabled']) {
    if (has(options, key)) jav[key] = Boolean(options[key])
  }
  jav.idolProfileFilters = normalizeIdolProfileFilters(
    options.idolProfileFilters ?? state.idolProfileFilters
  )
  jav.random = tab === 'list' && (options.random ?? state.javRandomMode)
  jav.seed = jav.random ? (options.seed ?? state.javRandomSeed) : null
  jav.page =
    options.page ??
    ({ idol: state.idolPage, studio: state.studioPage, series: state.seriesPage }[tab] ||
      state.javPage)
  const normalizeSort = tab === 'idol' ? normalizeIdolSort : normalizeJavSort
  jav.tempSort = has(options, 'tempSort')
    ? normalizeSort(options.tempSort, '')
    : tab === 'idol'
      ? state.idolTempSort
      : state.javTempSort
  return buildUrlFromState({ ...current, view: 'jav', jav }, pathname)
}
