import { JAV_PAGE_SIZE, JAV_STUDIO_PAGE_SIZE, JAV_SERIES_PAGE_SIZE } from '@/state/model'
import {
  createDefaultIdolProfileFilters,
  normalizeIdolSort,
  normalizeIdolProfileFilters,
} from '@/constants/jav'

export function createCatalogSlice({ set, lists }) {
  return {
    idolPage: 1,
    idolPageSize: JAV_PAGE_SIZE,
    idolSort: 'work',
    idolTempSort: '',
    idolProfileFilters: createDefaultIdolProfileFilters(),
    idolItems: [],
    idolTotal: 0,
    idolLoading: false,
    idolLoadingMore: false,
    idolError: null,
    studioPage: 1,
    studioPageSize: JAV_STUDIO_PAGE_SIZE,
    studioItems: [],
    studioTotal: 0,
    studioLoading: false,
    studioLoadingMore: false,
    studioError: null,
    seriesPage: 1,
    seriesPageSize: JAV_SERIES_PAGE_SIZE,
    seriesItems: [],
    seriesTotal: 0,
    seriesLoading: false,
    seriesLoadingMore: false,
    seriesError: null,
    setIdolPageSize: (size) => {
      const next = Math.max(1, Math.floor(Number(size) || JAV_PAGE_SIZE))
      set({ idolPageSize: next, idolPage: 1, studioPage: 1, seriesPage: 1 })
    },
    setStudioPageSize: (size) => {
      const next = Math.max(1, Math.floor(Number(size) || JAV_STUDIO_PAGE_SIZE))
      set({ studioPageSize: next, studioPage: 1 })
    },
    setSeriesPageSize: (size) => {
      const next = Math.max(1, Math.floor(Number(size) || JAV_SERIES_PAGE_SIZE))
      set({ seriesPageSize: next, seriesPage: 1 })
    },
    setIdolSort: (sort) => {
      const normalized = normalizeIdolSort(sort)
      set({ idolSort: normalized, idolTempSort: '', idolPage: 1 })
    },
    setIdolTempSort: (sort) => {
      const normalized = normalizeIdolSort(sort, '')
      set({ idolTempSort: normalized })
    },
    setIdolProfileFilters: (value) => {
      set({ idolProfileFilters: normalizeIdolProfileFilters(value), idolPage: 1 })
    },
    setIdolPage: (p) => set({ idolPage: p }),
    setStudioPage: (p) => set({ studioPage: p }),
    setSeriesPage: (p) => set({ seriesPage: p }),
    loadJavIdols: lists.idol.load,
    loadMoreJavIdols: lists.idol.loadMore,
    loadJavStudios: lists.studio.load,
    loadMoreJavStudios: lists.studio.loadMore,
    loadJavSeries: lists.series.load,
    loadMoreJavSeries: lists.series.loadMore,
  }
}
