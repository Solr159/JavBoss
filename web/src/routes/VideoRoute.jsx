import { videoQueryKey } from '@/query/listQueries'
import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { useCallback, useEffect } from 'react'
import VideoView from '@/features/video/components/VideoView'

export default function VideoRoute({
  hydrated,
  configLoaded,

  buildVideoUrl,
  handleSelectAllVideos,
  handleSelectVideoPage,
  handlePlayVideoPage,
  handlePlayAllVideos,
  videoBulkActionBusy,
  selectionPlaying,
  bulkPlaybackEnabled,
  handleOpenPlayer,
  containerMode,
  alternatePlayer,
  handleOpenAlternatePlayer,
  desktopIntegrationEnabled,
  handleRevealVideoFile,
  alternatePlayerLabel,
  openTagEditor,
  openVideoScreenshots,
  handleOpenScrapeSettings,
  handleRenameVideo,
  handleDeleteVideo,
  handleVideoTagClick,
  waterfallModes,
  setWaterfallMode,
}) {
  const {
    page,
    hasNext,
    total,
    pageSize,
    setPage,
    randomMode,
    videos,
    loadVideos,

    sortOrder,
    videoTempSort,

    loading,
    setVideoTempSort,
    selectedVideoIds,
    toggleSelectVideo,
    loadMoreVideos,
    videoLoadingMore,
  } = useStore(
    useShallow((state) => ({
      page: state.page,
      hasNext: state.hasNext,
      total: state.total,
      pageSize: state.pageSize,
      setPage: state.setPage,
      randomMode: state.randomMode,
      videos: state.videos,
      loadVideos: state.loadVideos,

      sortOrder: state.sortOrder,
      videoTempSort: state.videoTempSort,

      loading: state.loading,
      setVideoTempSort: state.setVideoTempSort,
      selectedVideoIds: state.selectedVideoIds,
      toggleSelectVideo: state.toggleSelectVideo,
      loadMoreVideos: state.loadMoreVideos,
      videoLoadingMore: state.videoLoadingMore,
    }))
  )
  const requestKey = useStore(videoQueryKey)
  const canPrev = page > 1
  const canNext = hasNext
  const lastPage = Math.max(1, Math.ceil((total || 0) / pageSize))
  const navigateVideoPage = useCallback(
    (targetPage) => {
      if (!targetPage || targetPage === page) return
      setPage(targetPage)
    },
    [page, setPage]
  )
  const videoWaterfallHasMore =
    !randomMode && (page - 1) * pageSize + (videos?.length || 0) < (total || 0)
  useEffect(() => {
    if (hydrated && configLoaded) loadVideos()
  }, [hydrated, configLoaded, requestKey, loadVideos])
  return (
    <VideoView
      page={page}
      lastPage={lastPage}
      totalItems={total}
      canPrev={canPrev}
      canNext={canNext}
      loading={loading}
      randomMode={randomMode}
      videoTempSort={videoTempSort}
      videoGlobalSort={sortOrder}
      buildVideoUrl={buildVideoUrl}
      setPage={navigateVideoPage}
      setVideoTempSort={setVideoTempSort}
      goToLastPage={() => navigateVideoPage(lastPage)}
      videos={videos}
      selectedVideoIds={selectedVideoIds}
      toggleSelectVideo={toggleSelectVideo}
      onSelectAll={handleSelectAllVideos}
      onSelectPage={handleSelectVideoPage}
      onPlayPage={handlePlayVideoPage}
      onPlayAll={handlePlayAllVideos}
      bulkActionBusy={videoBulkActionBusy || selectionPlaying}
      bulkPlaybackEnabled={bulkPlaybackEnabled}
      openPlayer={handleOpenPlayer}
      openAlternatePlayer={containerMode || alternatePlayer ? handleOpenAlternatePlayer : null}
      revealFile={containerMode || desktopIntegrationEnabled ? handleRevealVideoFile : null}
      alternatePlayerLabel={alternatePlayerLabel}
      setTagPickerFor={openTagEditor}
      onOpenScreenshots={openVideoScreenshots}
      onOpenScrapeSettings={handleOpenScrapeSettings}
      onRenameVideo={handleRenameVideo}
      onDeleteVideo={handleDeleteVideo}
      onTagClick={handleVideoTagClick}
      waterfallMode={waterfallModes.video}
      onWaterfallModeChange={(enabled) => setWaterfallMode('video', enabled)}
      onLoadMore={loadMoreVideos}
      loadingMore={videoLoadingMore}
      hasMore={videoWaterfallHasMore}
    />
  )
}
