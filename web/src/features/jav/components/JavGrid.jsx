import useJavPresentation from '@/features/jav/hooks/useJavPresentation'
import useJavPreviews from '@/features/jav/hooks/useJavPreviews'
import { useState, useMemo } from 'react'
import { zh } from '@/utils/i18n'
import JavCard from '@/features/jav/components/JavCard'
import { CoverPreviewModal } from '@/features/jav/components/CoverPreviewModal'
import { JavVideoManagerModal } from '@/features/jav/components/JavVideoManagerModal'

export default function JavGrid({
  items,
  selectedIds,
  onToggleSelect,
  selectionDisabled = false,
  columns = 0,
  titleMaxRows = 2,
  idolTagMaxRows = 2,
  tagMaxRows = 2,
  buildJavUrl,
  onPlay,
  onIdolClick,
  onOpenFavorites,
  onOpenJavFavorites,
  onOpenStudioFavorites,
  onOpenSeriesFavorites,
  onPrefixClick,
  onStudioClick,
  onSeriesClick,
  onTagClick,
  onOpenFile,
  openFileLabel,
  onOpenScreenshots,
  onManageVideoPlay,
  onManageVideoPlayAtTime,
  onManageVideoCoverChanged,
  onManageVideoOpenFile,
  onManageVideoRevealFile,
  onManageVideoOpenTagPicker,
  onManageVideoOpenScreenshots,
  onManageVideoOpenScrapeSettings,
  onManageVideoRename,
  onManageVideoDelete,
  onManageVideoTagClick,
}) {
  const {
    preferChineseName,
    portraitMode,
    hideSeries,
    hideIdols,
    hideTags,
    hideActions,
    showFullFavoriteRating,
    displayItems,
  } = useJavPresentation(items)
  const { loadIdolPreview, loadStudioPreview, loadSeriesPreview, handleIdolPreviewUpdated } =
    useJavPreviews()
  const [coverPreview, setCoverPreview] = useState(null)
  const [videoManagerItem, setVideoManagerItem] = useState(null)
  const activeVideoManagerItem = useMemo(() => {
    if (!videoManagerItem) return null
    const managerID = Number(videoManagerItem?.id)
    const managerCode = String(videoManagerItem?.code || '').trim()
    return (
      (items || []).find((item) => {
        if (Number.isFinite(managerID) && managerID > 0 && Number(item?.id) === managerID) {
          return true
        }
        return managerCode && String(item?.code || '').trim() === managerCode
      }) || videoManagerItem
    )
  }, [items, videoManagerItem])
  const hasItems = Array.isArray(displayItems) && displayItems.length > 0
  const columnCount = Number.isFinite(Number(columns)) ? Math.floor(Number(columns)) : 0
  const fixedColumnCount = columnCount > 0 ? Math.min(columnCount, 12) : 0
  const gridClassName = 'grid gap-4'
  const gridStyle = fixedColumnCount
    ? { gridTemplateColumns: `repeat(${fixedColumnCount}, minmax(0, 1fr))` }
    : { gridTemplateColumns: 'repeat(auto-fill, minmax(21rem, 1fr))' }

  if (!hasItems) {
    return (
      <div className="mt-4 flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
        {zh('暂无 JAV 数据', 'No JAV data')}
      </div>
    )
  }

  return (
    <>
      <div className={gridClassName} style={gridStyle}>
        {displayItems.map((item) => (
          <JavCard
            key={item.id || item.code}
            item={item}
            checked={selectedIds?.has(Number(item.id)) || false}
            onToggleSelect={onToggleSelect}
            selectionDisabled={selectionDisabled}
            onPlay={onPlay}
            buildJavUrl={buildJavUrl}
            onIdolClick={onIdolClick}
            onOpenFavorites={onOpenFavorites}
            onOpenJavFavorites={onOpenJavFavorites}
            onOpenStudioFavorites={onOpenStudioFavorites}
            onOpenSeriesFavorites={onOpenSeriesFavorites}
            onPrefixClick={onPrefixClick}
            onStudioClick={onStudioClick}
            onSeriesClick={onSeriesClick}
            onTagClick={onTagClick}
            onOpenFile={onOpenFile}
            openFileLabel={openFileLabel}
            onOpenScreenshots={onOpenScreenshots}
            onOpenVideoManager={setVideoManagerItem}
            onManageVideoPlay={onManageVideoPlay}
            onManageVideoPlayAtTime={onManageVideoPlayAtTime}
            onManageVideoCoverChanged={onManageVideoCoverChanged}
            onManageVideoOpenFile={onManageVideoOpenFile}
            onManageVideoRevealFile={onManageVideoRevealFile}
            onManageVideoOpenTagPicker={onManageVideoOpenTagPicker}
            onManageVideoOpenScreenshots={onManageVideoOpenScreenshots}
            onManageVideoOpenScrapeSettings={onManageVideoOpenScrapeSettings}
            onManageVideoRename={onManageVideoRename}
            onManageVideoDelete={onManageVideoDelete}
            onManageVideoTagClick={onManageVideoTagClick}
            loadIdolPreview={loadIdolPreview}
            loadStudioPreview={loadStudioPreview}
            loadSeriesPreview={loadSeriesPreview}
            onIdolPreviewUpdated={handleIdolPreviewUpdated}
            onOpenCoverPreview={setCoverPreview}
            preferChineseName={preferChineseName}
            portraitMode={portraitMode}
            titleMaxRows={titleMaxRows}
            idolTagMaxRows={idolTagMaxRows}
            tagMaxRows={tagMaxRows}
            hideSeries={hideSeries}
            hideIdols={hideIdols}
            hideTags={hideTags}
            hideActions={hideActions}
            showFullFavoriteRating={showFullFavoriteRating}
          />
        ))}
      </div>
      {coverPreview ? (
        <CoverPreviewModal preview={coverPreview} onClose={() => setCoverPreview(null)} />
      ) : null}
      <JavVideoManagerModal
        open={Boolean(videoManagerItem)}
        item={activeVideoManagerItem}
        openFileLabel={openFileLabel}
        onClose={() => setVideoManagerItem(null)}
        onPlay={onManageVideoPlay}
        onOpenFile={onManageVideoOpenFile}
        onRevealFile={onManageVideoRevealFile}
        onOpenTagPicker={onManageVideoOpenTagPicker}
        onOpenScreenshots={onManageVideoOpenScreenshots}
        onOpenScrapeSettings={onManageVideoOpenScrapeSettings}
        onRenameVideo={onManageVideoRename}
        onDeleteVideo={onManageVideoDelete}
        onTagClick={onManageVideoTagClick}
      />
    </>
  )
}
