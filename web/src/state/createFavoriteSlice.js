import { fetchJavFavoriteGroups } from '@/features/favorites/api'
import { getErrorMessage } from '@/utils/errors'

export function createFavoriteSlice({ set, get }) {
  const lastFavoriteGroupFetchKeys = {}
  return {
    javFavoriteRatingEnabled: false,
    javFavoriteRatingMin: 0.5,
    javFavoriteRatingMax: 5,
    javFavoriteGroupId: null,
    idolFavoriteGroupId: null,
    favoriteGroupsByType: {
      jav: [],
      idol: [],
      studio: [],
      series: [],
    },
    favoriteGroupsLoadingByType: {},
    favoriteGroupsErrorByType: {},
    studioFavoriteGroupId: null,
    seriesFavoriteGroupId: null,
    patchJavFavoriteCount: (type, entityID, groupIds, activeGroupId) => {
      const id = Number(entityID)
      if (!Number.isFinite(id) || id <= 0) return

      const nextGroupIds = Array.from(
        new Set((groupIds || []).map((value) => Number(value)).filter((value) => value > 0))
      )
      const nextGroupSet = new Set(nextGroupIds)
      const activeGroupID = Number(activeGroupId)
      const removeFromCurrentList =
        Number.isFinite(activeGroupID) && activeGroupID > 0
          ? !nextGroupSet.has(activeGroupID)
          : false
      const listKey =
        type === 'jav'
          ? 'javItems'
          : type === 'studio'
            ? 'studioItems'
            : type === 'series'
              ? 'seriesItems'
              : 'idolItems'
      const totalKey =
        type === 'jav'
          ? 'javTotal'
          : type === 'studio'
            ? 'studioTotal'
            : type === 'series'
              ? 'seriesTotal'
              : 'idolTotal'

      set((state) => {
        // Details and previews need updates even when absent from the current list.
        const sharedCounts = {
          favoriteCountsByType: {
            ...state.favoriteCountsByType,
            [type]: { ...state.favoriteCountsByType[type], [id]: nextGroupIds.length },
          },
        }
        const items = Array.isArray(state[listKey]) ? state[listKey] : []
        let changed = false
        const nextItems = removeFromCurrentList
          ? items.filter((item) => {
              const keep = Number(item?.id) !== id
              if (!keep) changed = true
              return keep
            })
          : items.map((item) => {
              if (Number(item?.id) !== id) return item
              changed = true
              return { ...item, favorite_count: nextGroupIds.length }
            })
        if (!changed) return sharedCounts
        return {
          ...sharedCounts,
          [listKey]: nextItems,
          ...(removeFromCurrentList
            ? { [totalKey]: Math.max(0, Number(state[totalKey] || 0) - 1) }
            : {}),
        }
      })
    },
    favoriteCountsByType: {},
    favoriteCountsRevision: {},
    invalidateJavFavoriteCounts: (type) =>
      set((state) => ({
        favoriteCountsByType: { ...state.favoriteCountsByType, [type]: {} },
        favoriteCountsRevision: {
          ...state.favoriteCountsRevision,
          [type]: (state.favoriteCountsRevision[type] || 0) + 1,
        },
      })),
    setIdolFavoriteGroupId: (id) => {
      const parsed = Number(id)
      const next = Number.isFinite(parsed) && parsed > 0 ? parsed : null
      set({ idolFavoriteGroupId: next, idolTempSort: '', idolPage: 1 })
    },
    setJavFavoriteGroupId: (id) => {
      const parsed = Number(id)
      const next = Number.isFinite(parsed) && parsed > 0 ? parsed : null
      set({ javFavoriteGroupId: next, javPage: 1, javRandomMode: false, javRandomSeed: null })
    },
    setStudioFavoriteGroupId: (id) => {
      const parsed = Number(id)
      const next = Number.isFinite(parsed) && parsed > 0 ? parsed : null
      set({ studioFavoriteGroupId: next, studioPage: 1 })
    },
    setSeriesFavoriteGroupId: (id) => {
      const parsed = Number(id)
      const next = Number.isFinite(parsed) && parsed > 0 ? parsed : null
      set({ seriesFavoriteGroupId: next, seriesPage: 1 })
    },
    loadJavFavoriteGroups: async (entityType = 'idol', options = {}) => {
      const type = ['jav', 'idol', 'studio', 'series'].includes(entityType) ? entityType : 'idol'
      const key = `${type}-favorite-groups`
      if (!options.force && key === lastFavoriteGroupFetchKeys[type]) {
        return get().favoriteGroupsByType?.[type] || []
      }
      lastFavoriteGroupFetchKeys[type] = key
      set((state) => ({
        favoriteGroupsLoadingByType: { ...(state.favoriteGroupsLoadingByType || {}), [type]: true },
        favoriteGroupsErrorByType: { ...(state.favoriteGroupsErrorByType || {}), [type]: null },
      }))
      try {
        const groups = await fetchJavFavoriteGroups(type)
        set((state) => ({
          favoriteGroupsByType: { ...(state.favoriteGroupsByType || {}), [type]: groups || [] },
        }))
        return groups || []
      } catch (e) {
        delete lastFavoriteGroupFetchKeys[type]
        const message = getErrorMessage(e)
        set((state) => ({
          favoriteGroupsErrorByType: {
            ...(state.favoriteGroupsErrorByType || {}),
            [type]: message,
          },
        }))
        return get().favoriteGroupsByType?.[type] || []
      } finally {
        set((state) => ({
          favoriteGroupsLoadingByType: {
            ...(state.favoriteGroupsLoadingByType || {}),
            [type]: false,
          },
        }))
      }
    },
    invalidateFavoriteRequests: () => {
      for (const key of Object.keys(lastFavoriteGroupFetchKeys))
        delete lastFavoriteGroupFetchKeys[key]
    },
  }
}
