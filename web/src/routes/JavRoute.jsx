import JavIdolView from '@/components/JavIdolView'
import JavSeriesView from '@/components/JavSeriesView'
import JavStudioView from '@/components/JavStudioView'
import JavView from '@/components/JavView'

function JavIdolRoute({
  buildJavUrl,
  config,
  directoryIds,
  hasMore,
  hasNext,
  hasPrev,
  items,
  lastPage,
  loading,
  loadingMore,
  onFirst,
  onGoToPage,
  onLast,
  onLoadMore,
  onNext,
  onOpenFavorites,
  onPrev,
  onSelectIdol,
  onWaterfallModeChange,
  page,
  totalItems,
  waterfallMode,
}) {
  return (
    <JavIdolView
      page={page}
      lastPage={lastPage}
      totalItems={totalItems}
      hasPrev={hasPrev}
      hasNext={hasNext}
      loading={loading}
      buildPageUrl={({ page: targetPage }) => buildJavUrl({ page: targetPage, tab: 'idol' })}
      buildIdolUrl={(idol) =>
        buildJavUrl({
          page: 1,
          search: '',
          tab: 'list',
          idolIds: [idol.id],
          tagIds: [],
          tempSort: '',
        })
      }
      onFirst={onFirst}
      onPrev={onPrev}
      onGoToPage={onGoToPage}
      onNext={onNext}
      onLast={onLast}
      items={items}
      directoryIds={directoryIds}
      javMetadataLanguage={config?.jav_metadata_language === 'en' ? 'en' : 'zh'}
      onSelectIdol={onSelectIdol}
      onOpenFavorites={onOpenFavorites}
      waterfallMode={waterfallMode}
      onWaterfallModeChange={onWaterfallModeChange}
      onLoadMore={onLoadMore}
      loadingMore={loadingMore}
      hasMore={hasMore}
    />
  )
}

function JavStudioRoute({
  buildJavUrl,
  hasMore,
  hasNext,
  hasPrev,
  items,
  lastPage,
  loading,
  loadingMore,
  onFirst,
  onGoToPage,
  onLast,
  onLoadMore,
  onNext,
  onOpenFavorites,
  onPrev,
  onSelectStudio,
  onWaterfallModeChange,
  page,
  totalItems,
  waterfallMode,
}) {
  return (
    <JavStudioView
      page={page}
      lastPage={lastPage}
      totalItems={totalItems}
      hasPrev={hasPrev}
      hasNext={hasNext}
      loading={loading}
      buildPageUrl={({ page: targetPage }) => buildJavUrl({ page: targetPage, tab: 'studio' })}
      buildStudioUrl={(studio) =>
        buildJavUrl({
          page: 1,
          search: '',
          tab: 'list',
          idolIds: [],
          tagIds: [],
          studioId: studio.id,
          studioName: studio.name,
          tempSort: '',
        })
      }
      onFirst={onFirst}
      onPrev={onPrev}
      onGoToPage={onGoToPage}
      onNext={onNext}
      onLast={onLast}
      items={items}
      onSelectStudio={onSelectStudio}
      onOpenFavorites={onOpenFavorites}
      waterfallMode={waterfallMode}
      onWaterfallModeChange={onWaterfallModeChange}
      onLoadMore={onLoadMore}
      loadingMore={loadingMore}
      hasMore={hasMore}
    />
  )
}

function JavSeriesRoute({
  buildJavUrl,
  hasMore,
  hasNext,
  hasPrev,
  items,
  lastPage,
  loading,
  loadingMore,
  onFirst,
  onGoToPage,
  onLast,
  onLoadMore,
  onNext,
  onOpenFavorites,
  onPrev,
  onSelectSeries,
  onSelectStudio,
  onWaterfallModeChange,
  page,
  totalItems,
  waterfallMode,
}) {
  return (
    <JavSeriesView
      page={page}
      lastPage={lastPage}
      totalItems={totalItems}
      hasPrev={hasPrev}
      hasNext={hasNext}
      loading={loading}
      buildPageUrl={({ page: targetPage }) => buildJavUrl({ page: targetPage, tab: 'series' })}
      buildSeriesUrl={(series) =>
        buildJavUrl({
          page: 1,
          search: '',
          tab: 'list',
          idolIds: [],
          tagIds: [],
          studioId: null,
          seriesId: series.id,
          seriesName: series.name,
          tempSort: '',
        })
      }
      onFirst={onFirst}
      onPrev={onPrev}
      onGoToPage={onGoToPage}
      onNext={onNext}
      onLast={onLast}
      items={items}
      onSelectSeries={onSelectSeries}
      onSelectStudio={onSelectStudio}
      onOpenFavorites={onOpenFavorites}
      waterfallMode={waterfallMode}
      onWaterfallModeChange={onWaterfallModeChange}
      onLoadMore={onLoadMore}
      loadingMore={loadingMore}
      hasMore={hasMore}
    />
  )
}

function JavListRoute({
  activeJavLoading,
  alternatePlayerLabel,
  buildJavUrl,
  hasMore,
  javGlobalSort,
  javGridColumns,
  javHasNext,
  javHasPrev,
  javIdolTagMaxRows,
  javItems,
  javLastPage,
  javPage,
  javRandomMode,
  javTagMaxRows,
  javTempSort,
  javTitleMaxRows,
  javTotal,
  loadingMore,
  onIdolClick,
  onLoadMore,
  onOpenFavorites,
  onOpenJavFavorites,
  onOpenFile,
  onOpenScreenshots,
  onManageVideoPlay,
  onManageVideoOpenFile,
  onManageVideoRevealFile,
  onManageVideoOpenTagPicker,
  onManageVideoOpenScreenshots,
  onManageVideoOpenScrapeSettings,
  onManageVideoRename,
  onManageVideoDelete,
  onManageVideoTagClick,
  onPlay,
  onRevealFile,
  onSeriesClick,
  onStudioClick,
  onTagClick,
  onWaterfallModeChange,
  setJavPage,
  setJavTempSort,
  waterfallMode,
}) {
  return (
    <JavView
      javPage={javPage}
      javLastPage={javLastPage}
      javTotal={javTotal}
      javHasPrev={javHasPrev}
      javHasNext={javHasNext}
      javLoading={activeJavLoading}
      javRandomMode={javRandomMode}
      javTempSort={javTempSort}
      javGlobalSort={javGlobalSort}
      buildJavUrl={buildJavUrl}
      setJavPage={setJavPage}
      setJavTempSort={setJavTempSort}
      javItems={javItems}
      javGridColumns={javGridColumns}
      javTitleMaxRows={javTitleMaxRows}
      javIdolTagMaxRows={javIdolTagMaxRows}
      javTagMaxRows={javTagMaxRows}
      onPlay={onPlay}
      onOpenFile={onOpenFile}
      openFileLabel={alternatePlayerLabel}
      onRevealFile={onRevealFile}
      onOpenScreenshots={onOpenScreenshots}
      onManageVideoPlay={onManageVideoPlay}
      onManageVideoOpenFile={onManageVideoOpenFile}
      onManageVideoRevealFile={onManageVideoRevealFile}
      onManageVideoOpenTagPicker={onManageVideoOpenTagPicker}
      onManageVideoOpenScreenshots={onManageVideoOpenScreenshots}
      onManageVideoOpenScrapeSettings={onManageVideoOpenScrapeSettings}
      onManageVideoRename={onManageVideoRename}
      onManageVideoDelete={onManageVideoDelete}
      onManageVideoTagClick={onManageVideoTagClick}
      onIdolClick={onIdolClick}
      onOpenFavorites={onOpenFavorites}
      onOpenJavFavorites={onOpenJavFavorites}
      onStudioClick={onStudioClick}
      onSeriesClick={onSeriesClick}
      onTagClick={onTagClick}
      waterfallMode={waterfallMode}
      onWaterfallModeChange={onWaterfallModeChange}
      onLoadMore={onLoadMore}
      loadingMore={loadingMore}
      hasMore={hasMore}
    />
  )
}

export default function JavRoute({ tab, ...props }) {
  if (tab === 'idol') return <JavIdolRoute {...props.idol} buildJavUrl={props.buildJavUrl} />
  if (tab === 'studio') return <JavStudioRoute {...props.studio} buildJavUrl={props.buildJavUrl} />
  if (tab === 'series') {
    return (
      <JavSeriesRoute
        {...props.series}
        buildJavUrl={props.buildJavUrl}
        onSelectStudio={props.onSelectStudio}
      />
    )
  }
  return <JavListRoute {...props.list} buildJavUrl={props.buildJavUrl} />
}
