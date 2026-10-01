import { javQueryKey, idolQueryKey, studioQueryKey, seriesQueryKey } from '@/query/listQueries'
import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { resolveJavSort, IDOL_FAVORITE_ORDER_SORT } from '@/constants/jav'
import { useEffect } from 'react'
import JavTabs from '@/features/jav/components/JavTabs'

export default function JavRoute({
  hydrated,
  configLoaded,

  buildJavUrl,
  handleSelectStudio,
  handleSelectIdol,
  handleOpenIdolFavoriteModal,
  waterfallModes,
  setWaterfallMode,
  handleSelectSeries,
  handleSelectJavPrefix,
  handleOpenFavoriteModal,
  javSelection,
  mpvEnabled,
  javCardActions,
}) {
  const {
    javSearchTerm,
    javIdolIds,
    javTags,
    javStudioId,
    javSeriesId,
    javPrefix,
    javSoloOnly,
    javFavoriteRatingEnabled,
    javFavoriteGroupId,
    javSort,
    javSortRules,
    javTempSort,
    javRandomMode,
    javTotal,
    javPageSize,
    javPage,
    idolTotal,
    idolPageSize,
    idolPage,
    studioTotal,
    studioPageSize,
    studioPage,
    seriesTotal,
    seriesPageSize,
    seriesPage,
    javItems,
    idolItems,
    studioItems,
    seriesItems,
    javTab,
    idolLoading,
    studioLoading,
    seriesLoading,
    javLoading,
    loadJavIdols,
    loadJavFavoriteGroups,
    loadJavStudios,
    loadJavSeries,
    loadJavs,

    idolSort,
    idolTempSort,
    idolFavoriteGroupId,

    setIdolTempSort,
    setIdolPage,
    config,
    loadMoreJavIdols,
    idolLoadingMore,
    setStudioPage,
    loadMoreJavStudios,
    studioLoadingMore,
    setSeriesPage,
    loadMoreJavSeries,
    seriesLoadingMore,
    setJavPage,
    setJavTempSort,
    javGridColumns,
    javTitleMaxRows,
    javIdolTagMaxRows,
    javTagMaxRows,
    loadMoreJavs,
    javLoadingMore,
  } = useStore(
    useShallow((state) => ({
      javSearchTerm: state.javSearchTerm,
      javIdolIds: state.javIdolIds,
      javTags: state.javTags,
      javStudioId: state.javStudioId,
      javSeriesId: state.javSeriesId,
      javPrefix: state.javPrefix,
      javSoloOnly: state.javSoloOnly,
      javFavoriteRatingEnabled: state.javFavoriteRatingEnabled,
      javFavoriteGroupId: state.javFavoriteGroupId,
      javSort: state.javSort,
      javSortRules: state.javSortRules,
      javTempSort: state.javTempSort,
      javRandomMode: state.javRandomMode,
      javTotal: state.javTotal,
      javPageSize: state.javPageSize,
      javPage: state.javPage,
      idolTotal: state.idolTotal,
      idolPageSize: state.idolPageSize,
      idolPage: state.idolPage,
      studioTotal: state.studioTotal,
      studioPageSize: state.studioPageSize,
      studioPage: state.studioPage,
      seriesTotal: state.seriesTotal,
      seriesPageSize: state.seriesPageSize,
      seriesPage: state.seriesPage,
      javItems: state.javItems,
      idolItems: state.idolItems,
      studioItems: state.studioItems,
      seriesItems: state.seriesItems,
      javTab: state.javTab,
      idolLoading: state.idolLoading,
      studioLoading: state.studioLoading,
      seriesLoading: state.seriesLoading,
      javLoading: state.javLoading,
      loadJavIdols: state.loadJavIdols,
      loadJavFavoriteGroups: state.loadJavFavoriteGroups,
      loadJavStudios: state.loadJavStudios,
      loadJavSeries: state.loadJavSeries,
      loadJavs: state.loadJavs,

      idolSort: state.idolSort,
      idolTempSort: state.idolTempSort,
      idolFavoriteGroupId: state.idolFavoriteGroupId,

      setIdolTempSort: state.setIdolTempSort,
      setIdolPage: state.setIdolPage,
      config: state.config,
      loadMoreJavIdols: state.loadMoreJavIdols,
      idolLoadingMore: state.idolLoadingMore,
      setStudioPage: state.setStudioPage,
      loadMoreJavStudios: state.loadMoreJavStudios,
      studioLoadingMore: state.studioLoadingMore,
      setSeriesPage: state.setSeriesPage,
      loadMoreJavSeries: state.loadMoreJavSeries,
      seriesLoadingMore: state.seriesLoadingMore,
      setJavPage: state.setJavPage,
      setJavTempSort: state.setJavTempSort,
      javGridColumns: state.javGridColumns,
      javTitleMaxRows: state.javTitleMaxRows,
      javIdolTagMaxRows: state.javIdolTagMaxRows,
      javTagMaxRows: state.javTagMaxRows,
      loadMoreJavs: state.loadMoreJavs,
      javLoadingMore: state.javLoadingMore,
    }))
  )
  const requestKey = useStore((state) =>
    (
      ({ idol: idolQueryKey, studio: studioQueryKey, series: seriesQueryKey })[state.javTab] ||
      javQueryKey
    )(state)
  )
  const javSortResolution = resolveJavSort({
    javSearchTerm,
    javIdolIds,
    javTags,
    javStudioId,
    javSeriesId,
    javPrefix,
    javSoloOnly,
    javFavoriteRatingEnabled,
    javFavoriteGroupId,
    javSort,
    javSortRules,
    javTempSort,
    javRandomMode,
  })
  const javLastPage = Math.max(1, Math.ceil((javTotal || 0) / javPageSize))
  const javHasPrev = javPage > 1
  const javHasNext = javPage < javLastPage
  const idolLastPage = Math.max(1, Math.ceil((idolTotal || 0) / idolPageSize))
  const idolHasPrev = idolPage > 1
  const idolHasNext = idolPage < idolLastPage
  const studioLastPage = Math.max(1, Math.ceil((studioTotal || 0) / studioPageSize))
  const studioHasPrev = studioPage > 1
  const studioHasNext = studioPage < studioLastPage
  const seriesLastPage = Math.max(1, Math.ceil((seriesTotal || 0) / seriesPageSize))
  const seriesHasPrev = seriesPage > 1
  const seriesHasNext = seriesPage < seriesLastPage
  const javWaterfallHasMore =
    !javRandomMode && (javPage - 1) * javPageSize + (javItems?.length || 0) < (javTotal || 0)
  const idolWaterfallHasMore =
    (idolPage - 1) * idolPageSize + (idolItems?.length || 0) < (idolTotal || 0)
  const studioWaterfallHasMore =
    (studioPage - 1) * studioPageSize + (studioItems?.length || 0) < (studioTotal || 0)
  const seriesWaterfallHasMore =
    (seriesPage - 1) * seriesPageSize + (seriesItems?.length || 0) < (seriesTotal || 0)
  const activeJavLoading =
    javTab === 'download'
      ? false
      : javTab === 'idol'
        ? idolLoading
        : javTab === 'studio'
          ? studioLoading
          : javTab === 'series'
            ? seriesLoading
            : javLoading
  useEffect(() => {
    if (!hydrated || !configLoaded) return
    if (javTab === 'idol') {
      loadJavIdols()
      loadJavFavoriteGroups('idol')
    } else if (javTab === 'studio') {
      loadJavStudios()
      loadJavFavoriteGroups('studio')
    } else if (javTab === 'series') {
      loadJavSeries()
      loadJavFavoriteGroups('series')
    } else if (javTab !== 'download') {
      loadJavs()
      loadJavFavoriteGroups('jav')
    }
  }, [
    hydrated,
    configLoaded,
    javTab,
    requestKey,
    loadJavs,
    loadJavIdols,
    loadJavStudios,
    loadJavSeries,
    loadJavFavoriteGroups,
  ])
  return (
    <JavTabs
      tab={javTab}
      buildJavUrl={buildJavUrl}
      onSelectStudio={handleSelectStudio}
      idol={{
        page: idolPage,
        lastPage: idolLastPage,
        totalItems: idolTotal,
        hasPrev: idolHasPrev,
        hasNext: idolHasNext,
        loading: idolLoading,
        idolTempSort,
        idolGlobalSort: idolFavoriteGroupId ? IDOL_FAVORITE_ORDER_SORT : idolSort,
        setIdolTempSort,
        onFirst: () => setIdolPage(1),
        onPrev: () => idolHasPrev && setIdolPage(idolPage - 1),
        onGoToPage: (p) => setIdolPage(p),
        onNext: () => idolHasNext && setIdolPage(idolPage + 1),
        onLast: () => setIdolPage(idolLastPage),
        items: idolItems,
        config,
        onSelectIdol: handleSelectIdol,
        onOpenFavorites: handleOpenIdolFavoriteModal,
        onMerged: () => {
          loadJavIdols({ force: true })
          loadJavFavoriteGroups('idol', { force: true })
        },
        waterfallMode: waterfallModes.idol,
        onWaterfallModeChange: (enabled) => setWaterfallMode('idol', enabled),
        onLoadMore: loadMoreJavIdols,
        loadingMore: idolLoadingMore,
        hasMore: idolWaterfallHasMore,
      }}
      studio={{
        page: studioPage,
        lastPage: studioLastPage,
        totalItems: studioTotal,
        hasPrev: studioHasPrev,
        hasNext: studioHasNext,
        loading: studioLoading,
        onFirst: () => setStudioPage(1),
        onPrev: () => studioHasPrev && setStudioPage(studioPage - 1),
        onGoToPage: (p) => setStudioPage(p),
        onNext: () => studioHasNext && setStudioPage(studioPage + 1),
        onLast: () => setStudioPage(studioLastPage),
        items: studioItems,
        onSelectStudio: handleSelectStudio,
        onSelectSeries: handleSelectSeries,
        onSelectPrefix: handleSelectJavPrefix,
        onOpenFavorites: (studio) => handleOpenFavoriteModal('studio', studio),
        onOpenSeriesFavorites: (series) => handleOpenFavoriteModal('series', series),
        onMerged: () => {
          loadJavStudios({ force: true })
          loadJavSeries({ force: true })
          loadJavFavoriteGroups('studio', { force: true })
        },
        waterfallMode: waterfallModes.studio,
        onWaterfallModeChange: (enabled) => setWaterfallMode('studio', enabled),
        onLoadMore: loadMoreJavStudios,
        loadingMore: studioLoadingMore,
        hasMore: studioWaterfallHasMore,
      }}
      series={{
        page: seriesPage,
        lastPage: seriesLastPage,
        totalItems: seriesTotal,
        hasPrev: seriesHasPrev,
        hasNext: seriesHasNext,
        loading: seriesLoading,
        onFirst: () => setSeriesPage(1),
        onPrev: () => seriesHasPrev && setSeriesPage(seriesPage - 1),
        onGoToPage: (p) => setSeriesPage(p),
        onNext: () => seriesHasNext && setSeriesPage(seriesPage + 1),
        onLast: () => setSeriesPage(seriesLastPage),
        items: seriesItems,
        onSelectSeries: handleSelectSeries,
        onOpenFavorites: (series) => handleOpenFavoriteModal('series', series),
        waterfallMode: waterfallModes.series,
        onWaterfallModeChange: (enabled) => setWaterfallMode('series', enabled),
        onLoadMore: loadMoreJavSeries,
        loadingMore: seriesLoadingMore,
        hasMore: seriesWaterfallHasMore,
      }}
      list={{
        javPage,
        javLastPage,
        javHasPrev,
        javHasNext,
        activeJavLoading,
        javRandomMode,
        javResolvedSort: javSortResolution.sort,
        javSortSource: javSortResolution.source,
        javPrefix,
        setJavPage,
        setJavTempSort,
        javItems,
        javTotal,
        javGridColumns,
        javTitleMaxRows,
        javIdolTagMaxRows,
        javTagMaxRows,
        selectedJavIds: javSelection.selectedIds,
        onToggleSelect: javSelection.toggle,
        onSelectAll: javSelection.selectAll,
        onSelectPage: javSelection.selectPage,
        onPlayPage: javSelection.playPage,
        onPlayAll: javSelection.playAll,
        bulkActionBusy: javSelection.busy,
        mpvEnabled,
        ...javCardActions,
        waterfallMode: waterfallModes.jav,
        onWaterfallModeChange: (enabled) => setWaterfallMode('jav', enabled),
        onLoadMore: loadMoreJavs,
        loadingMore: javLoadingMore,
        hasMore: javWaterfallHasMore,
      }}
    />
  )
}
