import { normalizeIdolProfileFilters, resolveJavSort } from '@/constants/jav'

export const directoryScopeKey = (state) =>
  (state.directories || [])
    .map(
      (directory) =>
        `${directory.id}:${directory.enabled !== false ? 1 : 0}:${directory.is_delete ? 1 : 0}`
    )
    .sort()
    .join(',')

export function videoQuery(state) {
  return {
    limit: state.pageSize,
    offset: state.randomMode ? 0 : (state.page - 1) * state.pageSize,
    tags: state.selectedTags || [],
    search: state.searchTerm || '',
    sort: state.randomMode ? 'random' : state.videoTempSort || state.sortOrder,
    seed: state.randomMode ? state.randomSeed : null,
    hideJav: state.videoHideJav,
  }
}

export function javQuery(state) {
  return {
    limit: state.javPageSize,
    offset: state.javRandomMode ? 0 : (state.javPage - 1) * state.javPageSize,
    search: state.javSearchTerm || '',
    idolIds: state.javIdolIds || [],
    tagIds: state.javTags || [],
    studioId: state.javStudioId,
    seriesId: state.javSeriesId,
    prefix: state.javPrefix,
    soloOnly: state.javSoloOnly,
    favoriteRatingEnabled: state.javFavoriteRatingEnabled,
    favoriteRatingMin: state.javFavoriteRatingMin,
    favoriteRatingMax: state.javFavoriteRatingMax,
    favoriteGroupId: state.javFavoriteGroupId,
    sort: resolveJavSort(state).sort,
    seed: state.javRandomMode ? state.javRandomSeed : null,
  }
}

export function idolQuery(state) {
  return {
    limit: state.idolPageSize,
    offset: (state.idolPage - 1) * state.idolPageSize,
    search: state.javSearchTerm || '',
    sort: state.idolTempSort || (state.idolFavoriteGroupId ? '' : state.idolSort),
    favoriteGroupId: state.idolFavoriteGroupId,
    profileFilters: normalizeIdolProfileFilters(state.idolProfileFilters),
  }
}

export const studioQuery = (state) => ({
  limit: state.studioPageSize,
  offset: (state.studioPage - 1) * state.studioPageSize,
  search: state.javSearchTerm || '',
  favoriteGroupId: state.studioFavoriteGroupId,
})
export const seriesQuery = (state) => ({
  limit: state.seriesPageSize,
  offset: (state.seriesPage - 1) * state.seriesPageSize,
  search: state.javSearchTerm || '',
  favoriteGroupId: state.seriesFavoriteGroupId,
})

export const listQueryKey = (query, state) =>
  JSON.stringify([directoryScopeKey(state), query(state)])
export const videoQueryKey = (state) => listQueryKey(videoQuery, state)
export const javQueryKey = (state) => listQueryKey(javQuery, state)
export const idolQueryKey = (state) => listQueryKey(idolQuery, state)
export const studioQueryKey = (state) => listQueryKey(studioQuery, state)
export const seriesQueryKey = (state) => listQueryKey(seriesQuery, state)
