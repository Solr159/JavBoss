import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import useScrollRestoration from '@/navigation/useScrollRestoration'

export default function useContentRestoration({
  isJavMode,
  hydrated,
  configLoaded,
  pendingScrollRestoreRef,
  schedulePendingScrollRestore,
  waterfallModes,
}) {
  const {
    randomMode,
    page,
    pageSize,
    videos,
    total,
    javRandomMode,
    javPage,
    javPageSize,
    javItems,
    javTotal,
    idolPage,
    idolPageSize,
    idolItems,
    idolTotal,
    studioPage,
    studioPageSize,
    studioItems,
    studioTotal,
    seriesPage,
    seriesPageSize,
    seriesItems,
    seriesTotal,
    javTab,
    idolError,
    studioError,
    seriesError,
    javError,
    error,
    loading,
    directories,
    idolLoading,
    studioLoading,
    seriesLoading,
    javLoading,
    idolLoadingMore,
    studioLoadingMore,
    seriesLoadingMore,
    javLoadingMore,
    videoLoadingMore,
    loadMoreJavIdols,
    loadMoreJavSeries,
    loadMoreJavStudios,
    loadMoreJavs,
    loadMoreVideos,
  } = useStore(
    useShallow((state) => ({
      randomMode: state.randomMode,
      page: state.page,
      pageSize: state.pageSize,
      videos: state.videos,
      total: state.total,
      javRandomMode: state.javRandomMode,
      javPage: state.javPage,
      javPageSize: state.javPageSize,
      javItems: state.javItems,
      javTotal: state.javTotal,
      idolPage: state.idolPage,
      idolPageSize: state.idolPageSize,
      idolItems: state.idolItems,
      idolTotal: state.idolTotal,
      studioPage: state.studioPage,
      studioPageSize: state.studioPageSize,
      studioItems: state.studioItems,
      studioTotal: state.studioTotal,
      seriesPage: state.seriesPage,
      seriesPageSize: state.seriesPageSize,
      seriesItems: state.seriesItems,
      seriesTotal: state.seriesTotal,
      javTab: state.javTab,
      idolError: state.idolError,
      studioError: state.studioError,
      seriesError: state.seriesError,
      javError: state.javError,
      error: state.error,
      loading: state.loading,
      directories: state.directories,
      idolLoading: state.idolLoading,
      studioLoading: state.studioLoading,
      seriesLoading: state.seriesLoading,
      javLoading: state.javLoading,
      idolLoadingMore: state.idolLoadingMore,
      studioLoadingMore: state.studioLoadingMore,
      seriesLoadingMore: state.seriesLoadingMore,
      javLoadingMore: state.javLoadingMore,
      videoLoadingMore: state.videoLoadingMore,
      loadMoreJavIdols: state.loadMoreJavIdols,
      loadMoreJavSeries: state.loadMoreJavSeries,
      loadMoreJavStudios: state.loadMoreJavStudios,
      loadMoreJavs: state.loadMoreJavs,
      loadMoreVideos: state.loadMoreVideos,
    }))
  )
  const videoWaterfallHasMore =
    !randomMode && (page - 1) * pageSize + (videos?.length || 0) < (total || 0)

  const javWaterfallHasMore =
    !javRandomMode && (javPage - 1) * javPageSize + (javItems?.length || 0) < (javTotal || 0)

  const idolWaterfallHasMore =
    (idolPage - 1) * idolPageSize + (idolItems?.length || 0) < (idolTotal || 0)

  const studioWaterfallHasMore =
    (studioPage - 1) * studioPageSize + (studioItems?.length || 0) < (studioTotal || 0)

  const seriesWaterfallHasMore =
    (seriesPage - 1) * seriesPageSize + (seriesItems?.length || 0) < (seriesTotal || 0)

  const activeError = isJavMode
    ? javTab === 'download'
      ? null
      : javTab === 'idol'
        ? idolError
        : javTab === 'studio'
          ? studioError
          : javTab === 'series'
            ? seriesError
            : javError
    : error

  const showDirectorySetupHint =
    hydrated &&
    configLoaded &&
    !loading &&
    !activeError &&
    Array.isArray(directories) &&
    directories.length === 0 &&
    Array.isArray(videos) &&
    videos.length === 0

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

  const activeLoadingMore = isJavMode
    ? javTab === 'download'
      ? false
      : javTab === 'idol'
        ? idolLoadingMore
        : javTab === 'studio'
          ? studioLoadingMore
          : javTab === 'series'
            ? seriesLoadingMore
            : javLoadingMore
    : videoLoadingMore

  useScrollRestoration({
    activeJavLoading,
    activeLoadingMore,
    configLoaded,
    hydrated,
    idolItems,
    idolWaterfallHasMore,
    isJavMode,
    javItems,
    javTab,
    javWaterfallHasMore,
    loadMoreJavIdols,
    loadMoreJavSeries,
    loadMoreJavStudios,
    loadMoreJavs,
    loadMoreVideos,
    loading,
    pendingScrollRestoreRef,
    schedulePendingScrollRestore,
    seriesItems,
    seriesWaterfallHasMore,
    studioItems,
    studioWaterfallHasMore,
    videoWaterfallHasMore,
    videos,
    waterfallModes,
  })
  return { activeError, showDirectorySetupHint }
}
