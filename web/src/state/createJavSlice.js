import { mergeJavItem, mergeJavIdol } from '@/features/jav/entityUpdates'
import {
  JAV_PAGE_SIZE,
  JAV_GRID_COLUMNS_AUTO,
  JAV_TITLE_MAX_ROWS_DEFAULT,
  JAV_IDOL_TAG_MAX_ROWS_DEFAULT,
  JAV_TAG_MAX_ROWS_DEFAULT,
  normalizeSeed,
  generateSeed,
} from '@/state/model'
import { normalizeJavSort } from '@/constants/jav'
import { updateConfig } from '@/features/settings/api'
import { getErrorMessage } from '@/utils/errors'

export function createJavSlice({ set, get, lists }) {
  return {
    patchJavItem: (updated) =>
      set((state) => ({ javItems: mergeJavItem(state.javItems, updated) })),
    patchJavIdol: (updated) =>
      set((state) => ({ javItems: mergeJavIdol(state.javItems, updated) })),
    setJavError: (message) => set({ javError: message }),
    videoHideJav: false,
    javSort: 'recent',
    javSortRules: [],
    javTempSort: '',
    javRandomMode: false,
    javRandomSeed: null,
    javPage: 1,
    javPageSize: JAV_PAGE_SIZE,
    javGridColumns: JAV_GRID_COLUMNS_AUTO,
    javTitleMaxRows: JAV_TITLE_MAX_ROWS_DEFAULT,
    javIdolTagMaxRows: JAV_IDOL_TAG_MAX_ROWS_DEFAULT,
    javTagMaxRows: JAV_TAG_MAX_ROWS_DEFAULT,
    setJavGridColumns: (columns) => {
      const n = Math.floor(Number(columns))
      const next = Number.isFinite(n) && n > 0 ? Math.min(n, 12) : JAV_GRID_COLUMNS_AUTO
      set({ javGridColumns: next })
    },
    javCompactMode: false,
    setJavCompactMode: (enabled) => {
      const next = Boolean(enabled)
      set({ javCompactMode: next })
      // Persist the user's choice so it survives reloads and stays in sync with
      // the "default compact mode" display setting.
      return updateConfig({ jav_compact_default: next })
        .then((config) => {
          if (get().javCompactMode === next) {
            set({ config })
          }
        })
        .catch((error) => {
          if (get().javCompactMode === next) {
            set({ javCompactMode: !next })
          }
          get().setJavError?.(getErrorMessage(error))
        })
    },
    setJavPageSize: (size) => {
      const next = Math.max(1, Math.floor(Number(size) || JAV_PAGE_SIZE))
      set({
        javPageSize: next,
        javTempSort: '',
        javRandomMode: false,
        javRandomSeed: null,
        javPage: 1,
      })
    },
    javSearchTerm: '',
    javIdolIds: [],
    javTags: [],
    javStudioId: null,
    javStudioName: '',
    javSeriesId: null,
    javSeriesName: '',
    javPrefix: '',
    javSoloOnly: false,
    javItems: [],
    javTotal: 0,
    javLoading: false,
    javLoadingMore: false,
    javError: null,
    selectJavIdol: (id) =>
      set({
        viewMode: 'jav',
        videoTempSort: '',
        javTab: 'list',
        javTempSort: '',
        idolTempSort: '',
        javRandomMode: false,
        javRandomSeed: null,
        javIdolIds: [id],
        javPrefix: '',
        javTags: [],
        javStudioId: null,
        javStudioName: '',
        javSeriesId: null,
        javSeriesName: '',
        javSoloOnly: false,
        javFavoriteRatingEnabled: false,
        javFavoriteRatingMin: 0.5,
        javFavoriteRatingMax: 5,
        idolFavoriteGroupId: null,
        javSearchTerm: '',
        javPage: 1,
        idolPage: 1,
        studioPage: 1,
        seriesPage: 1,
      }),
    setJavSort: (order) => {
      const normalized = normalizeJavSort(order)
      set({
        javSort: normalized,
        javTempSort: '',
        javRandomMode: false,
        javRandomSeed: null,
        javPage: 1,
      })
    },
    setJavTempSort: (order) => {
      const normalized = normalizeJavSort(order, '')
      set({ javTempSort: normalized, javRandomMode: false, javRandomSeed: null, javPage: 1 })
    },
    clearJavRandom: () => set({ javTempSort: '', javRandomMode: false, javRandomSeed: null }),
    setJavIdolIds: (idolIds) => {
      const clean = Array.from(
        new Set(
          (idolIds || [])
            .map((id) => Number.parseInt(String(id), 10))
            .filter((id) => Number.isFinite(id) && id > 0)
        )
      )
      set({
        javIdolIds: clean,
        javStudioId: null,
        javStudioName: '',
        javSeriesId: null,
        javSeriesName: '',
        javPrefix: '',
        javTempSort: '',
        javPage: 1,
      })
    },
    setJavTags: (tags) => {
      const clean = Array.from(
        new Set(
          (tags || [])
            .map((t) => Number.parseInt(String(t), 10))
            .filter((id) => Number.isFinite(id) && id > 0)
        )
      )
      set({
        javTags: clean,
        javStudioId: null,
        javStudioName: '',
        javSeriesId: null,
        javSeriesName: '',
        javPrefix: '',
        javTempSort: '',
        javPage: 1,
      })
    },
    setJavStudio: (studio) => {
      const id = Number(studio?.id)
      if (!Number.isFinite(id) || id <= 0) {
        set({ javStudioId: null, javStudioName: '', javPage: 1 })
        return
      }
      set({
        javStudioId: id,
        javStudioName: String(studio?.name || '').trim(),
        javSeriesId: null,
        javSeriesName: '',
        javPrefix: '',
        javSoloOnly: false,
        javIdolIds: [],
        javTags: [],
        javTempSort: '',
        javRandomMode: false,
        javRandomSeed: null,
        javPage: 1,
      })
    },
    setJavSeries: (series) => {
      const id = Number(series?.id)
      if (!Number.isFinite(id) || id <= 0) {
        set({ javSeriesId: null, javSeriesName: '', javPage: 1 })
        return
      }
      set({
        javSeriesId: id,
        javSeriesName: String(series?.name || '').trim(),
        javSoloOnly: false,
        javStudioId: null,
        javStudioName: '',
        javPrefix: '',
        javIdolIds: [],
        javTags: [],
        javTempSort: '',
        javRandomMode: false,
        javRandomSeed: null,
        javPage: 1,
      })
    },
    setJavPage: (p) => {
      const state = get()
      set({ javPage: state.javRandomMode ? 1 : p })
    },
    setJavSearchTerm: (value, options = {}) => {
      const { resetPage = true } = options
      const trimmed = (value || '').trim()
      const state = get()
      if (trimmed === state.javSearchTerm) {
        if (resetPage && state.javPage !== 1) {
          set({
            javTempSort: '',
            idolTempSort: '',
            javPage: 1,
            idolPage: 1,
            studioPage: 1,
            seriesPage: 1,
          })
        }
        return
      }
      const next = { javSearchTerm: trimmed, javTempSort: '', idolTempSort: '' }
      if (resetPage) {
        next.javPage = 1
        next.idolPage = 1
        next.studioPage = 1
        next.seriesPage = 1
      }
      set(next)
    },
    loadJavs: lists.jav.load,
    loadMoreJavs: lists.jav.loadMore,
    loadJavRandom: async (seed) => {
      const nextSeed = normalizeSeed(seed) ?? generateSeed()
      set({ javTempSort: '', javRandomMode: true, javRandomSeed: nextSeed, javPage: 1 })
    },
  }
}
