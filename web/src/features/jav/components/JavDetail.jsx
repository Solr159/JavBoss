import useJavItemActions from '@/features/jav/hooks/useJavItemActions'
import JavDetailModal from '@/components/JavDetailModal'
import { JavItemEditors } from '@/features/jav/components/JavItemEditors'

export default function JavDetail(props) {
  const model = useJavItemActions(props)
  const {
    item,
    cover,
    titleText,
    releaseText,
    durationText,
    preferredSeries,
    tags,
    externalLinks,
    preferChineseName,
    canPlay,
    detailView,
    handlePlay,
    onOpenJavFavorites,
    setEditorOpen,
    favoriteRating,
    favoriteRatingSaving,
    favoriteRatingError,
    handleFavoriteRatingChange,
    onStudioClick,
    onSeriesClick,
    onIdolClick,
    onPrefixClick,
    onTagClick,
    loadIdolPreview,
    loadStudioPreview,
    loadSeriesPreview,
    buildIdolFilterHref,
    buildStudioFilterHref,
    buildSeriesFilterHref,
    buildTagFilterHref,
    onOpenFavorites,
    onOpenStudioFavorites,
    onOpenSeriesFavorites,
    handleOpenIdolCoverEditor,
    handleOpenIdolEditor,
    onManageVideoPlay,
    onManageVideoPlayAtTime,
    onManageVideoCoverChanged,
    onManageVideoOpenFile,
    onManageVideoRevealFile,
    openFileLabel,
    onManageVideoOpenTagPicker,
    onManageVideoOpenScreenshots,
    onManageVideoOpenScrapeSettings,
    onManageVideoRename,
    onManageVideoDelete,
    onManageVideoTagClick,
  } = model
  return (
    <>
      <JavDetailModal
        item={item}
        cover={cover}
        title={titleText}
        releaseText={releaseText}
        durationText={durationText}
        studio={item?.studio}
        series={preferredSeries}
        tags={tags}
        externalLinks={externalLinks}
        preferChineseName={preferChineseName}
        canPlay={canPlay}
        onClose={detailView.onClose}
        initialScrollTop={detailView.scrollTop}
        onScrollChange={detailView.onScrollChange}
        onPlay={handlePlay}
        onOpenFavorites={() => onOpenJavFavorites?.(item)}
        onEdit={() => setEditorOpen(true)}
        favoriteRating={favoriteRating}
        favoriteRatingSaving={favoriteRatingSaving}
        favoriteRatingError={favoriteRatingError}
        onFavoriteRatingChange={handleFavoriteRatingChange}
        onSelectStudio={onStudioClick}
        onSelectSeries={onSeriesClick}
        onSelectIdol={onIdolClick}
        onSelectPrefix={onPrefixClick}
        onSelectTag={onTagClick}
        loadIdolPreview={loadIdolPreview}
        loadStudioPreview={loadStudioPreview}
        loadSeriesPreview={loadSeriesPreview}
        buildIdolUrl={buildIdolFilterHref}
        buildStudioUrl={buildStudioFilterHref}
        buildSeriesUrl={buildSeriesFilterHref}
        buildTagUrl={buildTagFilterHref}
        onOpenIdolFavorites={onOpenFavorites}
        onOpenStudioFavorites={onOpenStudioFavorites}
        onOpenSeriesFavorites={onOpenSeriesFavorites}
        onOpenIdolCoverEditor={handleOpenIdolCoverEditor}
        onOpenIdolEditor={handleOpenIdolEditor}
        onVideoPlay={onManageVideoPlay}
        onVideoPlayAtTime={onManageVideoPlayAtTime}
        onVideoCoverChanged={onManageVideoCoverChanged}
        onVideoOpenFile={onManageVideoOpenFile}
        onVideoRevealFile={onManageVideoRevealFile}
        openFileLabel={openFileLabel}
        onVideoOpenTagPicker={onManageVideoOpenTagPicker}
        onVideoOpenScreenshots={onManageVideoOpenScreenshots}
        onVideoOpenScrapeSettings={onManageVideoOpenScrapeSettings}
        onVideoRename={onManageVideoRename}
        onVideoDelete={onManageVideoDelete}
        onVideoTagClick={onManageVideoTagClick}
      />
      <JavItemEditors {...model} />
    </>
  )
}
