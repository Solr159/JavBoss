import { VIDEO_PAGE_SIZE, videoSelectionKey, normalizeSeed, generateSeed } from '@/state/model'
import { normalizeVideoSort } from '@/constants/video'
import { videoQueryKey } from '@/query/listQueries'

export function createVideoSlice({ set, get, lists }) {
  return {
    removeVideoLocations: (keys) => {
      const deletedKeys = new Set(keys.filter(Boolean))
      if (deletedKeys.size === 0) return
      // Cancel requests started before deletion and revalidate lists on their next visit.
      Object.values(lists).forEach((list) => list.invalidate())
      get().invalidateFavoriteRequests()
      set((state) => {
        const selectedVideoIds = new Set(state.selectedVideoIds)
        const selectedVideoMeta = { ...state.selectedVideoMeta }
        for (const key of deletedKeys) {
          selectedVideoIds.delete(key)
          delete selectedVideoMeta[key]
        }
        return {
          videos: state.videos.filter((video) => !deletedKeys.has(videoSelectionKey(video))),
          selectedVideoIds,
          selectedVideoMeta,
          total: Math.max(0, state.total - deletedKeys.size),
        }
      })
      void get().loadTags({ force: true })
      void get().loadJavTags({ force: true })
      void get().loadJavFavoriteGroups('jav', { force: true })
    },
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
    randomMode: false,
    randomSeed: null,
    videos: [],
    loading: false,
    videoLoadingMore: false,
    error: null,
    total: 0,
    hasNext: false,
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
        meta[key] = {
          label,
          video_id: video.id,
          location_id: video.location_id || null,
          jav_id: video.jav_id || null,
          jav_code: video.jav?.code || video.locations?.[0]?.jav?.code || '',
        }
      }
      set({ selectedVideoIds: setIds, selectedVideoMeta: meta })
    },
    clearSelection: () => set({ selectedVideoIds: new Set(), selectedVideoMeta: {} }),
    setSortOrder: (order) => {
      const normalized = normalizeVideoSort(order)
      set({
        sortOrder: normalized,
        videoTempSort: '',
        randomMode: false,
        randomSeed: null,
        page: 1,
      })
    },
    setVideoTempSort: (order) => {
      const normalized = normalizeVideoSort(order, '')
      set({ videoTempSort: normalized, randomMode: false, randomSeed: null })
    },
    clearRandomMode: () => set({ randomMode: false, randomSeed: null }),
    loadVideos: lists.video.load,
    loadMoreVideos: lists.video.loadMore,
    goToLastPage: async () => {
      const queryKey = videoQueryKey(get())
      if (!get().total) await lists.video.load({ force: true })
      if (queryKey !== videoQueryKey(get())) return
      const state = get()
      set({ page: Math.max(1, Math.ceil((state.total || 0) / state.pageSize)) })
      return lists.video.load()
    },
    loadRandom: async (seed) => {
      const nextSeed = normalizeSeed(seed) ?? generateSeed()
      const nextPage = 1
      set({ videoTempSort: '', randomMode: true, randomSeed: nextSeed, page: nextPage })
    },
  }
}
