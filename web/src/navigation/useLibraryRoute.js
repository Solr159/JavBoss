import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { useRef, useState, useMemo, useCallback, useEffect } from 'react'
import { normalizeInitialViewMode } from '@/utils/config'
import { normalizeIdolProfileFilters, createDefaultIdolProfileFilters } from '@/constants/jav'
import { normalizeUrlStateFromStore } from '@/utils/urlState'
import useUrlStateSync from '@/hooks/useUrlStateSync'
import { buildVideoQueryLink, buildJavQueryLink } from '@/navigation/queryLinks'

export default function useLibraryRoute({ setJavSearchInput, setSearchInput, configLoaded }) {
  const {
    tags,
    config,
    viewMode,
    page,
    searchTerm,
    videoTempSort,
    selectedTags,
    randomMode,
    randomSeed,
    javTab,
    javPage,
    javSearchTerm,
    javIdolIds,
    javTags,
    javStudioId,
    javStudioName,
    javSeriesId,
    javSeriesName,
    javPrefix,
    javSoloOnly,
    javFavoriteRatingEnabled,
    javFavoriteRatingMin,
    javFavoriteRatingMax,
    javFavoriteGroupId,
    javTempSort,
    idolTempSort,
    javRandomMode,
    javRandomSeed,
    idolPage,
    idolFavoriteGroupId,
    idolProfileFilters,
    studioFavoriteGroupId,
    seriesFavoriteGroupId,
    studioPage,
    seriesPage,
    setSelectedTags,
  } = useStore(
    useShallow((state) => ({
      tags: state.tags,
      config: state.config,
      viewMode: state.viewMode,
      page: state.page,
      searchTerm: state.searchTerm,
      videoTempSort: state.videoTempSort,
      selectedTags: state.selectedTags,
      randomMode: state.randomMode,
      randomSeed: state.randomSeed,
      javTab: state.javTab,
      javPage: state.javPage,
      javSearchTerm: state.javSearchTerm,
      javIdolIds: state.javIdolIds,
      javTags: state.javTags,
      javStudioId: state.javStudioId,
      javStudioName: state.javStudioName,
      javSeriesId: state.javSeriesId,
      javSeriesName: state.javSeriesName,
      javPrefix: state.javPrefix,
      javSoloOnly: state.javSoloOnly,
      javFavoriteRatingEnabled: state.javFavoriteRatingEnabled,
      javFavoriteRatingMin: state.javFavoriteRatingMin,
      javFavoriteRatingMax: state.javFavoriteRatingMax,
      javFavoriteGroupId: state.javFavoriteGroupId,
      javTempSort: state.javTempSort,
      idolTempSort: state.idolTempSort,
      javRandomMode: state.javRandomMode,
      javRandomSeed: state.javRandomSeed,
      idolPage: state.idolPage,
      idolFavoriteGroupId: state.idolFavoriteGroupId,
      idolProfileFilters: state.idolProfileFilters,
      studioFavoriteGroupId: state.studioFavoriteGroupId,
      seriesFavoriteGroupId: state.seriesFavoriteGroupId,
      studioPage: state.studioPage,
      seriesPage: state.seriesPage,
      setSelectedTags: state.setSelectedTags,
    }))
  )
  const pendingVideoTagIdsRef = useRef(null)

  const [hydrated, setHydrated] = useState(false)

  const tagsByName = useMemo(() => new Map(tags.map((t) => [t.name, t.id])), [tags])

  const initialViewMode = normalizeInitialViewMode(config?.initial_view_mode)

  const mapTagIdsToNames = useCallback(
    (ids) => {
      if (!Array.isArray(ids) || ids.length === 0) return []
      const idSet = new Set(ids)
      return tags.filter((t) => idSet.has(t.id)).map((t) => t.name)
    },
    [tags]
  )

  const applyUrlState = useCallback(
    (parsed) => {
      const mapTagIdsToNamesFromStore = (ids) => {
        if (!Array.isArray(ids) || ids.length === 0) return []
        const { tags: storeTags } = useStore.getState()
        const idSet = new Set(ids)
        return (storeTags || []).filter((t) => idSet.has(t.id)).map((t) => t.name)
      }
      if (parsed.view === 'jav') {
        const { jav } = parsed
        const current = useStore.getState()
        const sameIdolFavoriteGroup =
          jav.tab === 'idol' &&
          Number(jav.favoriteGroupId || 0) > 0 &&
          current.javTab === 'idol' &&
          Number(current.idolFavoriteGroupId || 0) === Number(jav.favoriteGroupId || 0)
        useStore.setState({
          viewMode: 'jav',
          videoTempSort: '',
          javTab: jav.tab,
          javRandomMode: jav.tab === 'list' ? jav.random : false,
          javRandomSeed: jav.tab === 'list' && jav.random ? jav.seed : null,
          javSearchTerm: jav.search,
          javIdolIds: jav.tab === 'list' ? jav.idolIds : [],
          javTags: jav.tab === 'list' ? jav.tagIds : [],
          javStudioId: jav.tab === 'list' ? jav.studioId : null,
          javStudioName:
            jav.tab === 'list' && jav.studioId !== null && jav.studioId !== undefined
              ? jav.studioName
              : '',
          javSeriesId: jav.tab === 'list' ? jav.seriesId : null,
          javSeriesName: jav.tab === 'list' && jav.seriesId ? jav.seriesName : '',
          javPrefix: jav.tab === 'list' ? jav.prefix : '',
          javSoloOnly: jav.tab === 'list' ? jav.soloOnly : false,
          javFavoriteRatingEnabled: jav.tab === 'list' ? jav.favoriteRatingEnabled : false,
          javFavoriteRatingMin: jav.tab === 'list' ? jav.favoriteRatingMin : 0.5,
          javFavoriteRatingMax: jav.tab === 'list' ? jav.favoriteRatingMax : 5,
          javFavoriteGroupId: jav.tab === 'list' ? jav.favoriteGroupId : null,
          javPage: jav.random ? 1 : jav.page,
          idolPage: jav.tab === 'idol' ? jav.page : 1,
          idolFavoriteGroupId: jav.tab === 'idol' ? jav.favoriteGroupId : null,
          idolProfileFilters:
            jav.tab === 'idol'
              ? normalizeIdolProfileFilters(jav.idolProfileFilters)
              : createDefaultIdolProfileFilters(),
          studioPage: jav.tab === 'studio' ? jav.page : 1,
          studioFavoriteGroupId: jav.tab === 'studio' ? jav.favoriteGroupId : null,
          seriesPage: jav.tab === 'series' ? jav.page : 1,
          seriesFavoriteGroupId: jav.tab === 'series' ? jav.favoriteGroupId : null,
          javTempSort: jav.tab !== 'list' || jav.random ? '' : jav.tempSort,
          idolTempSort:
            jav.tab === 'idol' && (!jav.favoriteGroupId || sameIdolFavoriteGroup || jav.tempSort)
              ? jav.tempSort
              : '',
        })
        setJavSearchInput(jav.search)
        if (jav.tab === 'list' && jav.random) {
          useStore.getState().loadJavRandom(jav.seed ?? undefined)
        }
        setHydrated(true)
        return
      }

      const { video } = parsed
      useStore.setState({
        viewMode: 'video',
        javTempSort: '',
        idolTempSort: '',
        videoTempSort: video.random ? '' : video.tempSort,
        randomMode: video.random,
        randomSeed: video.random ? video.seed : null,
        searchTerm: video.search,
        page: video.random ? 1 : video.page,
      })
      setSearchInput(video.search)
      const names = mapTagIdsToNamesFromStore(video.tagIds)
      if (names.length || video.tagIds.length === 0) {
        useStore.getState().setSelectedTags(names, { resetPage: false, preserveTempSort: true })
      } else {
        pendingVideoTagIdsRef.current = video.tagIds
      }
      if (video.random) {
        useStore.getState().loadRandom(video.seed ?? undefined)
      }
      setHydrated(true)
    },
    [setJavSearchInput, setSearchInput]
  )

  const currentUrlState = useMemo(
    () =>
      normalizeUrlStateFromStore(
        {
          viewMode,
          page,
          searchTerm,
          videoTempSort,
          selectedTags,
          randomMode,
          randomSeed,
          javTab,
          javPage,
          javSearchTerm,
          javIdolIds,
          javTags,
          javStudioId,
          javStudioName,
          javSeriesId,
          javSeriesName,
          javPrefix,
          javSoloOnly,
          javFavoriteRatingEnabled,
          javFavoriteRatingMin,
          javFavoriteRatingMax,
          javFavoriteGroupId,
          javTempSort,
          idolTempSort,
          javRandomMode,
          javRandomSeed,
          idolPage,
          idolFavoriteGroupId,
          idolProfileFilters,
          studioFavoriteGroupId,
          seriesFavoriteGroupId,
          studioPage,
          seriesPage,
        },
        tagsByName
      ),
    [
      idolFavoriteGroupId,
      idolProfileFilters,
      idolTempSort,
      javFavoriteGroupId,
      idolPage,
      studioPage,
      seriesPage,
      javIdolIds,
      javStudioId,
      javSeriesId,
      javPrefix,
      javSoloOnly,
      javFavoriteRatingEnabled,
      javFavoriteRatingMin,
      javFavoriteRatingMax,
      studioFavoriteGroupId,
      seriesFavoriteGroupId,
      javPage,
      javRandomMode,
      javRandomSeed,
      javSearchTerm,
      javTempSort,
      javTab,
      javTags,
      page,
      randomMode,
      randomSeed,
      searchTerm,
      selectedTags,
      videoTempSort,
      tagsByName,
      viewMode,
      javStudioName,
      javSeriesName,
    ]
  )

  const handleParsedUrlView = useCallback((parsedView) => {
    useStore.setState({ viewMode: parsedView === 'jav' ? 'jav' : 'video' })
  }, [])

  const {
    javDetailId,
    javDetailItem,
    javDetailState,
    cacheJavDetail,
    openJavDetail,
    closeJavDetail,
    saveJavDetailState,
    navigateFromJavDetail,
    studioDetailId,
    studioDetailKey,
    studioDetailState,
    studioDetailItem,
    cacheStudioDetail,
    openStudioDetail,
    closeStudioDetail,
    saveStudioDetailState,
    navigateFromStudioDetail,
    browserNavigation,
    handleBrowserBack,
    handleBrowserForward,
    pathname,
    pendingScrollRestoreRef,
    saveScrollBeforeUrlStateChange,
    schedulePendingScrollRestore,
  } = useUrlStateSync({
    applyUrlState,
    configLoaded,
    currentUrlState,
    hydrated,
    initialViewMode,
    onParsedView: handleParsedUrlView,
  })

  const buildVideoUrl = useCallback(
    (options = {}) => buildVideoQueryLink(useStore.getState(), options, pathname),
    [pathname]
  )

  const buildJavUrl = useCallback(
    (options = {}) => buildJavQueryLink(useStore.getState(), options, pathname),
    [pathname]
  )

  useEffect(() => {
    if (!pendingVideoTagIdsRef.current || !tags.length) return
    const names = mapTagIdsToNames(pendingVideoTagIdsRef.current)
    setSelectedTags(names, { resetPage: false, preserveTempSort: true })
    pendingVideoTagIdsRef.current = null
  }, [mapTagIdsToNames, setSelectedTags, tags])
  return {
    hydrated,
    javDetailId,
    javDetailItem,
    javDetailState,
    cacheJavDetail,
    openJavDetail,
    closeJavDetail,
    saveJavDetailState,
    navigateFromJavDetail,
    studioDetailId,
    studioDetailKey,
    studioDetailState,
    studioDetailItem,
    cacheStudioDetail,
    openStudioDetail,
    closeStudioDetail,
    saveStudioDetailState,
    navigateFromStudioDetail,
    browserNavigation,
    handleBrowserBack,
    handleBrowserForward,
    pendingScrollRestoreRef,
    saveScrollBeforeUrlStateChange,
    schedulePendingScrollRestore,
    buildVideoUrl,
    buildJavUrl,
  }
}
