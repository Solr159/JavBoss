import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { createDefaultIdolProfileFilters } from '@/constants/jav'
import { useCallback } from 'react'

export default function useSectionNavigation({
  setTagModalOpen,
  setVideoSettingsOpen,
  setJavSettingsOpen,
  setGlobalSettingsOpen,
  isJavMode,
  setJavSearchInput,
  forceReloadJavByTab,
  setSearchInput,
  forceReloadVideos,
  saveScrollBeforeUrlStateChange,
  applyJavTagFilter,
}) {
  const { javTab, javRandomMode, javRandomSeed } = useStore(
    useShallow((state) => ({
      javTab: state.javTab,
      javRandomMode: state.javRandomMode,
      javRandomSeed: state.javRandomSeed,
    }))
  )
  const handleHomeClick = () => {
    setTagModalOpen(false)
    setVideoSettingsOpen(false)
    setJavSettingsOpen(false)
    setGlobalSettingsOpen(false)
    if (isJavMode) {
      const updates = {
        viewMode: 'jav',
        javTab,
        videoTempSort: '',
        javTempSort: '',
        idolTempSort: '',
        javRandomMode: false,
        javRandomSeed: null,
        javIdolIds: [],
        javTags: [],
        javStudioId: null,
        javStudioName: '',
        javSeriesId: null,
        javSeriesName: '',
        javPrefix: '',
        javSoloOnly: false,
        javFavoriteRatingEnabled: false,
        javFavoriteRatingMin: 0.5,
        javFavoriteRatingMax: 5,
        idolProfileFilters: createDefaultIdolProfileFilters(),
        javSearchTerm: '',
        javPage: 1,
        idolPage: 1,
        studioPage: 1,
        seriesPage: 1,
      }
      if (javTab === 'idol') updates.idolFavoriteGroupId = null
      else if (javTab === 'studio') updates.studioFavoriteGroupId = null
      else if (javTab === 'series') updates.seriesFavoriteGroupId = null
      else updates.javFavoriteGroupId = null
      useStore.setState(updates)
      setJavSearchInput('')
      forceReloadJavByTab(javTab)
    } else {
      useStore.setState({
        viewMode: 'video',
        videoTempSort: '',
        randomMode: false,
        randomSeed: null,
        selectedTags: [],
        searchTerm: '',
        page: 1,
      })
      setSearchInput('')
      forceReloadVideos()
    }
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  const handleSwitchJavTab = (tab) => {
    const nextTab =
      tab === 'idol'
        ? 'idol'
        : tab === 'studio'
          ? 'studio'
          : tab === 'series'
            ? 'series'
            : tab === 'download'
              ? 'download'
              : 'list'
    const shouldResetRandomList = nextTab === 'list' && javRandomMode
    const shouldClearSearch = nextTab === 'list' || nextTab !== javTab || shouldResetRandomList
    const nextRandomMode = nextTab === 'list' && !shouldResetRandomList ? javRandomMode : false
    const nextRandomSeed = nextTab === 'list' && !shouldResetRandomList ? javRandomSeed : null
    const updates = {
      viewMode: 'jav',
      videoTempSort: '',
      javTab: nextTab,
      javTempSort: '',
      idolTempSort: '',
      javIdolIds: [],
      javTags: [],
      javStudioId: null,
      javStudioName: '',
      javSeriesId: null,
      javSeriesName: '',
      javPrefix: '',
      javSoloOnly: false,
      javFavoriteRatingEnabled: false,
      javFavoriteRatingMin: 0.5,
      javFavoriteRatingMax: 5,
      idolFavoriteGroupId: null,
      idolProfileFilters: createDefaultIdolProfileFilters(),
      javRandomMode: nextRandomMode,
      javRandomSeed: nextRandomSeed,
      javPage: 1,
      idolPage: 1,
      studioPage: 1,
      seriesPage: 1,
    }
    if (shouldClearSearch) {
      updates.javSearchTerm = ''
      setJavSearchInput('')
    }
    saveScrollBeforeUrlStateChange()
    useStore.setState(updates)
    forceReloadJavByTab(nextTab)
  }

  const handleSelectSideTab = (tab) => {
    if (tab === 'video') {
      if (!isJavMode) return
      saveScrollBeforeUrlStateChange()
      useStore.setState({ viewMode: 'video', javTempSort: '', idolTempSort: '' })
      forceReloadVideos()
      return
    }
    const nextTab =
      tab === 'idol'
        ? 'idol'
        : tab === 'studio'
          ? 'studio'
          : tab === 'series'
            ? 'series'
            : tab === 'download'
              ? 'download'
              : 'list'
    if (isJavMode && nextTab === javTab) return
    handleSwitchJavTab(nextTab)
  }

  const handleSelectIdol = (idol) => {
    const id = Number(idol?.id)
    if (!Number.isFinite(id) || id <= 0) return
    saveScrollBeforeUrlStateChange()
    useStore.setState({
      viewMode: 'jav',
      videoTempSort: '',
      javTab: 'list',
      javTempSort: '',
      idolTempSort: '',
      javRandomMode: false,
      javRandomSeed: null,
      javIdolIds: [id],
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
      idolProfileFilters: createDefaultIdolProfileFilters(),
      javSearchTerm: '',
      javPage: 1,
      idolPage: 1,
      studioPage: 1,
      seriesPage: 1,
    })
  }

  const handleJavIdolClick = useCallback(
    (idol) => {
      const id = Number(idol?.id ?? idol)
      if (!Number.isFinite(id) || id <= 0) return
      saveScrollBeforeUrlStateChange()
      useStore.getState().selectJavIdol(id)
    },
    [saveScrollBeforeUrlStateChange]
  )

  const handleSelectStudio = (studio) => {
    const id = Number(studio?.id)
    if (!Number.isFinite(id) || id <= 0) return
    saveScrollBeforeUrlStateChange()
    useStore.setState({
      viewMode: 'jav',
      videoTempSort: '',
      javTab: 'list',
      javTempSort: '',
      idolTempSort: '',
      javRandomMode: false,
      javRandomSeed: null,
      javIdolIds: [],
      javTags: [],
      javStudioId: id,
      javStudioName: String(studio?.name || '').trim(),
      javSeriesId: null,
      javSeriesName: '',
      javPrefix: '',
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
    })
  }

  const handleSelectSeries = (series) => {
    const id = Number(series?.id)
    if (!Number.isFinite(id) || id <= 0) return
    saveScrollBeforeUrlStateChange()
    useStore.setState({
      viewMode: 'jav',
      videoTempSort: '',
      javTab: 'list',
      javTempSort: '',
      idolTempSort: '',
      javRandomMode: false,
      javRandomSeed: null,
      javIdolIds: [],
      javTags: [],
      javStudioId: null,
      javStudioName: '',
      javSeriesId: id,
      javSeriesName: String(series?.name || '').trim(),
      javPrefix: '',
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
    })
  }

  const handleSelectJavPrefix = (prefixItem) => {
    const prefix = String(prefixItem?.prefix || prefixItem || '')
      .trim()
      .toUpperCase()
    if (!prefix) return
    const shouldIncludeStudio =
      typeof prefixItem === 'object' && prefixItem !== null && prefixItem.include_studio_filter
    const studioId = shouldIncludeStudio ? Number(prefixItem?.studio_id) : null
    const hasStudio = Number.isFinite(studioId) && studioId > 0
    saveScrollBeforeUrlStateChange()
    useStore.setState({
      viewMode: 'jav',
      videoTempSort: '',
      javTab: 'list',
      javTempSort: '',
      idolTempSort: '',
      javRandomMode: false,
      javRandomSeed: null,
      javIdolIds: [],
      javTags: [],
      javStudioId: hasStudio ? studioId : shouldIncludeStudio ? 0 : null,
      javStudioName: shouldIncludeStudio ? String(prefixItem?.studio_name || '').trim() : '',
      javSeriesId: null,
      javSeriesName: '',
      javPrefix: prefix,
      javSoloOnly: false,
      javFavoriteRatingEnabled: false,
      javFavoriteRatingMin: 0.5,
      javFavoriteRatingMax: 5,
      javFavoriteGroupId: null,
      idolFavoriteGroupId: null,
      javSearchTerm: '',
      javPage: 1,
      idolPage: 1,
      studioPage: 1,
      seriesPage: 1,
    })
  }

  const handleJavTagClick = useCallback(
    (tag) => {
      const raw = typeof tag === 'object' ? tag?.id : tag
      const parsed = Number.parseInt(String(raw), 10)
      if (!Number.isFinite(parsed) || parsed <= 0) return
      applyJavTagFilter([parsed])
    },
    [applyJavTagFilter]
  )
  return {
    handleHomeClick,
    handleSelectSideTab,
    handleSelectIdol,
    handleJavIdolClick,
    handleSelectStudio,
    handleSelectSeries,
    handleSelectJavPrefix,
    handleJavTagClick,
  }
}
