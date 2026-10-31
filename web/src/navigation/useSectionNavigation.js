import { getSeriesDisplayName, preferChineseSeries } from '@/features/jav/titleTranslation'
import { useStore } from '@/store'
import { createDefaultIdolProfileFilters } from '@/constants/jav'
import { useCallback } from 'react'

export default function useSectionNavigation({
  setTagModalOpen,
  setDisplaySettingsTab,
  setGlobalSettingsOpen,
  isJavMode,
  setJavSearchInput,
  forceReloadJavByTab,
  setSearchInput,
  forceReloadVideos,
  saveScrollBeforeUrlStateChange,
  applyJavTagFilter,
  switchLibraryTab,
}) {
  const javTab = useStore((state) => state.javTab)
  const handleHomeClick = () => {
    setTagModalOpen(false)
    setDisplaySettingsTab(null)
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
        videoDirectoryId: null,
        videoSubdirectory: '',
        page: 1,
      })
      setSearchInput('')
      forceReloadVideos()
    }
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  const handleSelectSideTab = (tab) => {
    if (!switchLibraryTab(tab)) return
    // Enter the existing loading view before mounting a potentially large cached grid.
    if (tab === 'video') forceReloadVideos()
    else forceReloadJavByTab(tab)
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
      javSeriesName: getSeriesDisplayName(series, preferChineseSeries(useStore.getState().config)),
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
