import { create } from 'zustand'
import {
  fetchTags,
  fetchVideos,
  createTag,
  deleteTag,
  renameTag,
  addTagToVideos,
  removeTagFromVideos,
  fetchDirectories,
  createDirectory,
  updateDirectory,
  deleteDirectory as deleteDirectoryApi,
  fetchJavs,
  fetchJavIdols,
  fetchJavStudios,
  fetchJavSeries,
  fetchJavTags,
  fetchConfig,
} from '@/api'
import { normalizeIdolSort, normalizeJavSort } from '@/constants/jav'
import { normalizeVideoSort } from '@/constants/video'
import { zh } from '@/utils/i18n'

const VIDEO_PAGE_SIZE = 25
const JAV_PAGE_SIZE = 24
const JAV_STUDIO_PAGE_SIZE = 24
const JAV_GRID_COLUMNS_AUTO = 0
const JAV_TITLE_MAX_ROWS_DEFAULT = 2
const JAV_IDOL_TAG_MAX_ROWS_DEFAULT = 2
const JAV_TAG_MAX_ROWS_DEFAULT = 2
let videoLoadSeq = 0
let lastVideoFetchKey = null
let lastJavFetchKey = null
let lastIdolFetchKey = null
let lastStudioFetchKey = null
let lastSeriesFetchKey = null
let lastTagFetchKey = null
let lastJavTagFetchKey = null
let tagFetchInFlight = null
let tagFetchInFlightKey = null
let javTagFetchInFlight = null
let javTagFetchInFlightKey = null
const RANDOM_SEED_MAX = 2147483646
const DIRECTORY_FILTER_ALL = 'all'
const DIRECTORY_FILTER_CUSTOM = 'custom'

const normalizeSeed = (seed) => {
  const num = Math.floor(Number(seed))
  if (!Number.isFinite(num) || num <= 0) return null
  return Math.min(num, RANDOM_SEED_MAX)
}

const generateSeed = () => Math.floor(Math.random() * RANDOM_SEED_MAX) + 1

export const videoSelectionKey = (video) => {
  if (video?.location_id) return `loc:${video.location_id}`
  if (video?.id) return `vid:${video.id}`
  return ''
}

const selectedVideoContentIds = (state) => {
  const ids = new Set()
  for (const key of state.selectedVideoIds || []) {
    const meta = state.selectedVideoMeta?.[key]
    const raw = meta && typeof meta === 'object' ? meta.video_id : key
    const parsed = Number(raw)
    if (Number.isFinite(parsed) && parsed > 0) ids.add(parsed)
  }
  return Array.from(ids)
}

const cleanDirectoryIds = (ids) =>
  Array.from(
    new Set((ids || []).map((id) => Number(id)).filter((id) => Number.isFinite(id) && id > 0))
  ).sort((a, b) => a - b)

const sameIds = (a, b) => {
  if (a === b) return true
  if (!Array.isArray(a) || !Array.isArray(b) || a.length !== b.length) return false
  return a.every((id, index) => id === b[index])
}

export const directoryQueryIds = (state) => {
  if (state?.directoryFilterMode !== DIRECTORY_FILTER_CUSTOM) {
    return []
  }
  const enabled = cleanDirectoryIds(state.enabledDirectoryIds)
  if (enabled.length === 0) {
    return [0]
  }
  const active = cleanDirectoryIds((state.directories || []).map((directory) => directory?.id))
  if (active.length === 0) {
    return enabled
  }
  const activeSet = new Set(active)
  const scoped = enabled.filter((id) => activeSet.has(id))
  if (scoped.length === 0) {
    return [0]
  }
  if (scoped.length === active.length) {
    return []
  }
  return scoped
}

export const useStore = create((set, get) => ({
  // UI state
  page: 1,
  pageSize: VIDEO_PAGE_SIZE,
  setPageSize: (size) => {
    const next = Math.max(1, Math.floor(Number(size) || VIDEO_PAGE_SIZE))
    set({ pageSize: next, videoTempSort: '', page: 1, randomMode: false, randomSeed: null })
  },
  selectedTags: [],
  selectedVideoIds: new Set(),
  selectedVideoMeta: {},
  searchTerm: '',
  sortOrder: 'recent',
  videoTempSort: '',
  videoHideJav: true,
  javSort: 'recent',
  javTempSort: '',
  randomMode: false,
  randomSeed: null,
  javRandomMode: false,
  javRandomSeed: null,
  viewMode: 'video', // video | jav
  javTab: 'list', // list | idol | studio | series
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
  javItems: [],
  javTotal: 0,
  javLoading: false,
  javError: null,
  idolPage: 1,
  idolPageSize: JAV_PAGE_SIZE,
  idolSort: 'work',
  idolItems: [],
  idolTotal: 0,
  idolLoading: false,
  idolError: null,
  studioPage: 1,
  studioItems: [],
  studioTotal: 0,
  studioLoading: false,
  studioError: null,
  seriesPage: 1,
  seriesItems: [],
  seriesTotal: 0,
  seriesLoading: false,
  seriesError: null,
  setIdolPageSize: (size) => {
    const next = Math.max(1, Math.floor(Number(size) || JAV_PAGE_SIZE))
    set({ idolPageSize: next, idolPage: 1, studioPage: 1, seriesPage: 1 })
  },
  setIdolSort: (sort) => {
    const normalized = normalizeIdolSort(sort)
    set({ idolSort: normalized, idolPage: 1 })
  },

  // data
  config: {},
  videos: [],
  tags: [],
  javTagOptions: [],
  directories: [],
  enabledDirectoryIds: [],
  directoryFilterMode: DIRECTORY_FILTER_ALL,
  loading: false,
  error: null,
  total: 0,
  hasNext: false,

  // actions
  setPage: (p) => set({ page: p }),
  setSelectedTags: (names, options = {}) => {
    const { resetPage = true, preserveTempSort = false } = options
    const clean = Array.from(new Set((names || []).map((n) => (n || '').trim()).filter(Boolean)))
    const updates = { selectedTags: clean }
    if (!preserveTempSort) {
      updates.videoTempSort = ''
    }
    if (resetPage) {
      updates.page = 1
    }
    set(updates)
  },
  setSearchTerm: (value, options = {}) => {
    const { resetPage = true } = options
    const trimmed = (value || '').trim()
    const state = get()
    const baseUpdate = { videoTempSort: '', randomMode: false, randomSeed: null }
    if (trimmed === state.searchTerm) {
      // 仅重置分页/随机模式
      const updates = { ...baseUpdate }
      if (resetPage && state.page !== 1) {
        updates.page = 1
      }
      set(updates)
      return
    }
    const next = { searchTerm: trimmed, ...baseUpdate }
    if (resetPage) {
      next.page = 1
    }
    set(next)
  },
  toggleTagFilter: (tagName) => {
    const { selectedTags } = get()
    const exists = selectedTags.includes(tagName)
    const next = exists ? selectedTags.filter((t) => t !== tagName) : [...selectedTags, tagName]
    set({ selectedTags: next, videoTempSort: '', page: 1 })
  },
  clearFilters: () => set({ selectedTags: [], videoTempSort: '', page: 1 }),
  toggleSelectVideo: (video) => {
    const key = videoSelectionKey(video)
    if (!video || !video.id || !key) return
    const label = video.filename || video.path || `#${video.id}`
    const setIds = new Set(get().selectedVideoIds)
    const meta = { ...get().selectedVideoMeta }
    if (setIds.has(key)) {
      setIds.delete(key)
      delete meta[key]
    } else {
      setIds.add(key)
      meta[key] = { label, video_id: video.id, location_id: video.location_id || null }
    }
    set({ selectedVideoIds: setIds, selectedVideoMeta: meta })
  },
  clearSelection: () => set({ selectedVideoIds: new Set(), selectedVideoMeta: {} }),
  setSortOrder: (order) => {
    const normalized = normalizeVideoSort(order)
    set({ sortOrder: normalized, videoTempSort: '', randomMode: false, randomSeed: null, page: 1 })
  },
  setVideoTempSort: (order) => {
    const normalized = normalizeVideoSort(order, '')
    set({ videoTempSort: normalized, randomMode: false, randomSeed: null })
  },
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
    set({ javTempSort: normalized, javRandomMode: false, javRandomSeed: null })
  },
  clearRandomMode: () => set({ randomMode: false, randomSeed: null }),
  clearJavRandom: () => set({ javTempSort: '', javRandomMode: false, javRandomSeed: null }),
  setViewMode: (mode) => {
    if (mode !== 'video' && mode !== 'jav') return
    set({ viewMode: mode, ...(mode === 'jav' ? { videoTempSort: '' } : { javTempSort: '' }) })
  },
  setJavTab: (tab) => {
    if (tab !== 'list' && tab !== 'idol' && tab !== 'studio' && tab !== 'series') return
    set({ javTab: tab, javTempSort: '' })
  },
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
      javStudioId: null,
      javStudioName: '',
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
  setIdolPage: (p) => set({ idolPage: p }),
  setStudioPage: (p) => set({ studioPage: p }),
  setSeriesPage: (p) => set({ seriesPage: p }),
  setJavSearchTerm: (value, options = {}) => {
    const { resetPage = true } = options
    const trimmed = (value || '').trim()
    const state = get()
    if (trimmed === state.javSearchTerm) {
      if (resetPage && state.javPage !== 1) {
        set({ javTempSort: '', javPage: 1, idolPage: 1, studioPage: 1, seriesPage: 1 })
      }
      return
    }
    const next = { javSearchTerm: trimmed, javTempSort: '' }
    if (resetPage) {
      next.javPage = 1
      next.idolPage = 1
      next.studioPage = 1
      next.seriesPage = 1
    }
    set(next)
  },

  loadTags: async (options = {}) => {
    const { videoHideJav } = get()
    const directoryIds = directoryQueryIds(get())
    const key = `tags|${directoryIds.join(',')}|${videoHideJav ? 'hide-jav' : 'show-jav'}`
    if (tagFetchInFlight && tagFetchInFlightKey === key) {
      return tagFetchInFlight
    }
    if (!options.force && options.skipUnchanged && key === lastTagFetchKey) {
      return null
    }
    tagFetchInFlightKey = key
    tagFetchInFlight = (async () => {
      try {
        const tags = await fetchTags({ directoryIds, hideJav: videoHideJav })
        set({ tags })
        lastTagFetchKey = key
        return tags
      } catch (e) {
        set({ error: e.message })
        return null
      } finally {
        if (tagFetchInFlightKey === key) {
          tagFetchInFlight = null
          tagFetchInFlightKey = null
        }
      }
    })()
    return tagFetchInFlight
  },
  loadJavTags: async (options = {}) => {
    const directoryIds = directoryQueryIds(get())
    const key = `jav-tags|${directoryIds.join(',')}`
    if (javTagFetchInFlight && javTagFetchInFlightKey === key) {
      return javTagFetchInFlight
    }
    if (!options.force && options.skipUnchanged && key === lastJavTagFetchKey) {
      return null
    }
    javTagFetchInFlightKey = key
    javTagFetchInFlight = (async () => {
      try {
        const tags = await fetchJavTags({ directoryIds })
        set({ javTagOptions: tags })
        lastJavTagFetchKey = key
        return tags
      } catch (e) {
        set({ javError: e.message || zh('加载 JAV 标签失败', 'Failed to load JAV tags') })
        return null
      } finally {
        if (javTagFetchInFlightKey === key) {
          javTagFetchInFlight = null
          javTagFetchInFlightKey = null
        }
      }
    })()
    return javTagFetchInFlight
  },
  loadConfig: async () => {
    try {
      const cfg = await fetchConfig()
      const state = get()
      const clamp = (raw) => {
        const n = parseInt(raw, 10)
        if (!Number.isFinite(n) || n <= 0) return null
        return Math.min(n, 500)
      }
      const updates = { config: cfg }
      const videoSize = clamp(cfg?.video_page_size)
      const videoSort = normalizeVideoSort((cfg?.video_sort || '').toLowerCase(), '')
      const videoHideJav = String(cfg?.video_hide_jav || '').toLowerCase() !== 'false'
      const javSize = clamp(cfg?.jav_page_size)
      const javGridColumnsRaw = parseInt(cfg?.jav_grid_columns, 10)
      const javGridColumns =
        Number.isFinite(javGridColumnsRaw) && javGridColumnsRaw > 0
          ? Math.min(javGridColumnsRaw, 12)
          : JAV_GRID_COLUMNS_AUTO
      const javTitleMaxRowsRaw = parseInt(cfg?.jav_title_max_rows, 10)
      const javTitleMaxRows =
        Number.isFinite(javTitleMaxRowsRaw) && javTitleMaxRowsRaw >= 0
          ? Math.min(javTitleMaxRowsRaw, 12)
          : JAV_TITLE_MAX_ROWS_DEFAULT
      const javIdolTagMaxRowsRaw = parseInt(cfg?.jav_idol_tag_max_rows, 10)
      const javIdolTagMaxRows =
        Number.isFinite(javIdolTagMaxRowsRaw) && javIdolTagMaxRowsRaw >= 0
          ? Math.min(javIdolTagMaxRowsRaw, 12)
          : JAV_IDOL_TAG_MAX_ROWS_DEFAULT
      const javTagMaxRowsRaw = parseInt(cfg?.jav_tag_max_rows, 10)
      const javTagMaxRows =
        Number.isFinite(javTagMaxRowsRaw) && javTagMaxRowsRaw >= 0
          ? Math.min(javTagMaxRowsRaw, 12)
          : JAV_TAG_MAX_ROWS_DEFAULT
      const idolSize = clamp(cfg?.idol_page_size)
      const javSort = normalizeJavSort((cfg?.jav_sort || '').toLowerCase(), '')
      const idolSort = normalizeIdolSort((cfg?.idol_sort || '').toLowerCase(), '')
      if (videoSize && videoSize !== state.pageSize) {
        updates.pageSize = videoSize
      }
      if (videoSort) {
        updates.sortOrder = videoSort
      }
      if (videoHideJav !== state.videoHideJav) {
        updates.videoHideJav = videoHideJav
      }
      if (javSort) {
        updates.javSort = javSort
      }
      if (idolSort) {
        updates.idolSort = idolSort
      }
      if (javSize && javSize !== state.javPageSize) {
        updates.javPageSize = javSize
      }
      if (javGridColumns !== state.javGridColumns) {
        updates.javGridColumns = javGridColumns
      }
      if (javTitleMaxRows !== state.javTitleMaxRows) {
        updates.javTitleMaxRows = javTitleMaxRows
      }
      if (javIdolTagMaxRows !== state.javIdolTagMaxRows) {
        updates.javIdolTagMaxRows = javIdolTagMaxRows
      }
      if (javTagMaxRows !== state.javTagMaxRows) {
        updates.javTagMaxRows = javTagMaxRows
      }
      if (idolSize && idolSize !== state.idolPageSize) {
        updates.idolPageSize = idolSize
      }
      set(updates)
      return cfg
    } catch (e) {
      console.error('load config failed', e)
      return null
    }
  },
  loadDirectories: async () => {
    try {
      const directories = await fetchDirectories()
      const active = directories.filter((d) => !d.is_delete)
      const activeIDs = cleanDirectoryIds(active.map((d) => d.id))
      const activeSet = new Set(activeIDs)
      const state = get()
      const enabled =
        state.directoryFilterMode === DIRECTORY_FILTER_ALL
          ? activeIDs
          : cleanDirectoryIds(state.enabledDirectoryIds).filter((id) => activeSet.has(id))
      const nextMode =
        state.directoryFilterMode === DIRECTORY_FILTER_CUSTOM && enabled.length === activeIDs.length
          ? DIRECTORY_FILTER_ALL
          : state.directoryFilterMode
      set({
        directories: active,
        enabledDirectoryIds: nextMode === DIRECTORY_FILTER_ALL ? activeIDs : enabled,
        directoryFilterMode: nextMode,
      })
    } catch (e) {
      console.error(zh('加载目录失败', 'Failed to load directories'), e)
    }
  },
  loadVideos: async (options = {}) => {
    const {
      page: p0,
      pageSize,
      selectedTags,
      searchTerm,
      sortOrder,
      videoTempSort,
      videoHideJav,
      randomMode,
      randomSeed,
    } = get()
    const directoryIds = directoryQueryIds(get())
    const search = searchTerm ? searchTerm : ''
    const effectiveSort = videoTempSort || sortOrder
    const key = [
      randomMode ? 'r' : 'p',
      randomMode ? 1 : p0,
      pageSize,
      search,
      effectiveSort,
      randomMode ? randomSeed || '' : '',
      (selectedTags || []).join(','),
      directoryIds.join(','),
      videoHideJav ? 'hide-jav' : 'show-jav',
    ].join('|')
    if (!options.force && key === lastVideoFetchKey) {
      return
    }
    lastVideoFetchKey = key
    const reqId = (videoLoadSeq += 1)
    set({ loading: true, error: null })
    try {
      const resp = await fetchVideos({
        limit: pageSize,
        offset: randomMode ? 0 : (p0 - 1) * pageSize,
        tags: selectedTags,
        search,
        sort: randomMode ? 'random' : effectiveSort,
        seed: randomMode ? randomSeed : null,
        directoryIds,
        hideJav: videoHideJav,
      })
      if (reqId !== videoLoadSeq) return
      const total = resp.total ?? 0
      const items = resp.items ?? []
      const lastPage = Math.max(1, Math.ceil(total / pageSize))
      const hasNext = randomMode ? false : p0 < lastPage
      set({ videos: items, total, hasNext })
    } catch (e) {
      if (reqId !== videoLoadSeq) return
      set({ error: e.message })
    } finally {
      if (reqId === videoLoadSeq) {
        set({ loading: false })
      }
    }
  },
  loadJavs: async (options = {}) => {
    const {
      javPage,
      javPageSize,
      javSearchTerm,
      javIdolIds,
      javTags,
      javStudioId,
      javSeriesId,
      javSort,
      javTempSort,
      javRandomMode,
      javRandomSeed,
    } = get()
    const directoryIds = directoryQueryIds(get())
    const search = javSearchTerm || ''
    const effectiveSort = javTempSort || javSort
    const key = [
      javRandomMode ? 'r' : 'p',
      javRandomMode ? 1 : javPage,
      javPageSize,
      search,
      (javIdolIds || []).join(','),
      (javTags || []).join(','),
      javStudioId || '',
      javSeriesId || '',
      effectiveSort,
      javRandomMode ? javRandomSeed || '' : '',
      directoryIds.join(','),
    ].join('|')
    if (!options.force && key === lastJavFetchKey) {
      return
    }
    lastJavFetchKey = key
    set({ javLoading: true, javError: null })
    try {
      const resp = await fetchJavs({
        limit: javPageSize,
        offset: javRandomMode ? 0 : (javPage - 1) * javPageSize,
        search,
        idolIds: javIdolIds,
        tagIds: javTags,
        studioId: javStudioId,
        seriesId: javSeriesId,
        sort: javRandomMode ? 'random' : effectiveSort,
        seed: javRandomMode ? javRandomSeed : null,
        directoryIds,
      })
      const items = resp.items || []
      set({
        javItems: items,
        javTotal: javRandomMode ? items.length : resp.total || 0,
      })
    } catch (e) {
      set({ javError: e.message || zh('加载 JAV 失败', 'Failed to load JAV') })
    } finally {
      set({ javLoading: false })
    }
  },
  loadJavIdols: async (options = {}) => {
    const { idolPage, idolPageSize, javSearchTerm, idolSort } = get()
    const directoryIds = directoryQueryIds(get())
    const search = javSearchTerm || ''
    const key = ['idol', idolPage, idolPageSize, search, idolSort, directoryIds.join(',')].join('|')
    if (!options.force && key === lastIdolFetchKey) {
      return
    }
    lastIdolFetchKey = key
    set({ idolLoading: true, idolError: null })
    try {
      const resp = await fetchJavIdols({
        limit: idolPageSize,
        offset: (idolPage - 1) * idolPageSize,
        search,
        sort: idolSort,
        directoryIds,
      })
      set({
        idolItems: resp.items || [],
        idolTotal: resp.total || 0,
      })
    } catch (e) {
      set({ idolError: e.message || zh('加载女优失败', 'Failed to load idols') })
    } finally {
      set({ idolLoading: false })
    }
  },
  loadJavStudios: async (options = {}) => {
    const { studioPage, javSearchTerm } = get()
    const directoryIds = directoryQueryIds(get())
    const search = javSearchTerm || ''
    const key = ['studio', studioPage, JAV_STUDIO_PAGE_SIZE, search, directoryIds.join(',')].join(
      '|'
    )
    if (!options.force && key === lastStudioFetchKey) {
      return
    }
    lastStudioFetchKey = key
    set({ studioLoading: true, studioError: null })
    try {
      const resp = await fetchJavStudios({
        limit: JAV_STUDIO_PAGE_SIZE,
        offset: (studioPage - 1) * JAV_STUDIO_PAGE_SIZE,
        search,
        directoryIds,
      })
      set({
        studioItems: resp.items || [],
        studioTotal: resp.total || 0,
      })
    } catch (e) {
      set({ studioError: e.message || zh('加载片商失败', 'Failed to load studios') })
    } finally {
      set({ studioLoading: false })
    }
  },
  loadJavSeries: async (options = {}) => {
    const { seriesPage, javSearchTerm } = get()
    const directoryIds = directoryQueryIds(get())
    const search = javSearchTerm || ''
    const key = ['series', seriesPage, JAV_STUDIO_PAGE_SIZE, search, directoryIds.join(',')].join(
      '|'
    )
    if (!options.force && key === lastSeriesFetchKey) {
      return
    }
    lastSeriesFetchKey = key
    set({ seriesLoading: true, seriesError: null })
    try {
      const resp = await fetchJavSeries({
        limit: JAV_STUDIO_PAGE_SIZE,
        offset: (seriesPage - 1) * JAV_STUDIO_PAGE_SIZE,
        search,
        directoryIds,
      })
      set({
        seriesItems: resp.items || [],
        seriesTotal: resp.total || 0,
      })
    } catch (e) {
      set({ seriesError: e.message || zh('加载系列失败', 'Failed to load series') })
    } finally {
      set({ seriesLoading: false })
    }
  },

  createTag: async (name) => {
    const tag = await createTag(name)
    set({ tags: [...get().tags, tag] })
  },
  deleteTag: async (id) => {
    await deleteTag(id)
    set({ tags: get().tags.filter((t) => t.id !== id) })
  },
  renameTag: async (id, name) => {
    await renameTag(id, name)
    set({ tags: get().tags.map((t) => (t.id === id ? { ...t, name } : t)) })
  },
  addTagToSelection: async (tagId) => {
    const ids = selectedVideoContentIds(get())
    if (ids.length === 0) return
    await addTagToVideos(tagId, ids)
    await get().loadVideos()
  },
  removeTagFromSelection: async (tagId) => {
    const ids = selectedVideoContentIds(get())
    if (ids.length === 0) return
    await removeTagFromVideos(tagId, ids)
    await get().loadVideos()
  },
  goToLastPage: async () => {
    set({ loading: true, error: null })
    try {
      const {
        pageSize,
        selectedTags,
        searchTerm,
        sortOrder,
        videoTempSort,
        randomMode,
        randomSeed,
      } = get()
      const directoryIds = directoryQueryIds(get())
      const effectiveSort = videoTempSort || sortOrder
      // Get total via a cheap fetch (limit=1) or use existing total
      let { total } = get()
      const search = searchTerm ? searchTerm : ''
      if (!total) {
        const res = await fetchVideos({
          limit: 1,
          offset: 0,
          tags: selectedTags,
          search,
          sort: randomMode ? 'random' : effectiveSort,
          seed: randomMode ? randomSeed : null,
          directoryIds,
        })
        total = res.total ?? 0
        set({ total })
      }
      const lastPage = Math.max(1, Math.ceil(total / pageSize))
      const res2 = await fetchVideos({
        limit: pageSize,
        offset: (lastPage - 1) * pageSize,
        tags: selectedTags,
        search,
        sort: randomMode ? 'random' : effectiveSort,
        seed: randomMode ? randomSeed : null,
        directoryIds,
      })
      const items = res2.items ?? []
      set({ page: lastPage, videos: items, hasNext: false })
    } catch (e) {
      set({ error: e.message })
    } finally {
      set({ loading: false })
    }
  },
  loadRandom: async (seed) => {
    const nextSeed = normalizeSeed(seed) ?? generateSeed()
    const nextPage = 1
    set({ videoTempSort: '', randomMode: true, randomSeed: nextSeed, page: nextPage })
  },
  loadJavRandom: async (seed) => {
    const nextSeed = normalizeSeed(seed) ?? generateSeed()
    set({ javTempSort: '', javRandomMode: true, javRandomSeed: nextSeed, javPage: 1 })
  },

  setEnabledDirectoryIds: (ids) => {
    const clean = cleanDirectoryIds(ids)
    const active = cleanDirectoryIds(get().directories.map((directory) => directory?.id))
    const mode =
      active.length > 0 && clean.length === active.length
        ? DIRECTORY_FILTER_ALL
        : DIRECTORY_FILTER_CUSTOM
    set({
      enabledDirectoryIds: mode === DIRECTORY_FILTER_ALL ? active : clean,
      directoryFilterMode: mode,
      page: 1,
      javPage: 1,
      idolPage: 1,
      studioPage: 1,
      seriesPage: 1,
      videoTempSort: '',
      javTempSort: '',
      randomMode: false,
      randomSeed: null,
      javRandomMode: false,
      javRandomSeed: null,
    })
    lastVideoFetchKey = null
    lastJavFetchKey = null
    lastIdolFetchKey = null
    lastStudioFetchKey = null
    lastSeriesFetchKey = null
    lastTagFetchKey = null
    lastJavTagFetchKey = null
  },
  setDirectoryFilterFromUrl: (ids) => {
    if (ids == null) {
      const active = cleanDirectoryIds(get().directories.map((directory) => directory?.id))
      const state = get()
      if (
        state.directoryFilterMode === DIRECTORY_FILTER_ALL &&
        sameIds(state.enabledDirectoryIds, active)
      ) {
        return
      }
      set({ directoryFilterMode: DIRECTORY_FILTER_ALL, enabledDirectoryIds: active })
      return
    }
    const clean = cleanDirectoryIds(ids)
    const state = get()
    if (
      state.directoryFilterMode === DIRECTORY_FILTER_CUSTOM &&
      sameIds(state.enabledDirectoryIds, clean)
    ) {
      return
    }
    set({
      directoryFilterMode: DIRECTORY_FILTER_CUSTOM,
      enabledDirectoryIds: clean,
      page: 1,
      javPage: 1,
      idolPage: 1,
      studioPage: 1,
      seriesPage: 1,
    })
    lastVideoFetchKey = null
    lastJavFetchKey = null
    lastIdolFetchKey = null
    lastStudioFetchKey = null
    lastSeriesFetchKey = null
    lastTagFetchKey = null
    lastJavTagFetchKey = null
  },

  createDirectory: async ({ path }) => {
    const dir = await createDirectory({ path })
    const next = dir && !dir.is_delete ? [...get().directories, dir] : get().directories
    const state = get()
    set({
      directories: next,
      enabledDirectoryIds:
        state.directoryFilterMode === DIRECTORY_FILTER_ALL
          ? cleanDirectoryIds(next.map((directory) => directory?.id))
          : state.enabledDirectoryIds,
    })
    return dir
  },
  updateDirectory: async (id, payload) => {
    const dir = await updateDirectory(id, payload)
    const state = get()
    const next = state.directories
      .map((d) => (d.id === id ? dir : d))
      .filter((d) => d && !d.is_delete)
    const active = cleanDirectoryIds(next.map((directory) => directory?.id))
    const activeSet = new Set(active)
    set({
      directories: next,
      enabledDirectoryIds:
        state.directoryFilterMode === DIRECTORY_FILTER_ALL
          ? active
          : cleanDirectoryIds(state.enabledDirectoryIds).filter((enabledID) =>
              activeSet.has(enabledID)
            ),
    })
    return dir
  },
  deleteDirectory: async (id) => {
    const dir = await deleteDirectoryApi(id)
    const state = get()
    const next = state.directories
      .map((d) => (d.id === id ? dir : d))
      .filter((d) => d && !d.is_delete)
    const active = cleanDirectoryIds(next.map((directory) => directory?.id))
    const activeSet = new Set(active)
    set({
      directories: next,
      enabledDirectoryIds:
        state.directoryFilterMode === DIRECTORY_FILTER_ALL
          ? active
          : cleanDirectoryIds(state.enabledDirectoryIds).filter((enabledID) =>
              activeSet.has(enabledID)
            ),
    })
    return dir
  },
}))
