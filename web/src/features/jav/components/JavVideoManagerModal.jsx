import { useState, useEffect } from 'react'
import { getJavDisplayTitle } from '@/utils/jav'
import { useStore, videoSelectionKey } from '@/store'
import { configFlag } from '@/utils/config'
import AppModal from '@/shared/ui/AppModal'
import { zh } from '@/utils/i18n'
import VideoGrid from '@/features/video/components/VideoGrid'

export function JavVideoManagerModal({
  open,
  item,
  openFileLabel,
  onClose,
  onPlay,
  onOpenFile,
  onRevealFile,
  onOpenTagPicker,
  onOpenScreenshots,
  onOpenScrapeSettings,
  onRenameVideo,
  onDeleteVideo,
  onTagClick,
}) {
  const [selectedIds, setSelectedIds] = useState(() => new Set())
  const translateTitle = useStore((state) =>
    configFlag(state.config?.jav_title_translation_enabled)
  )

  useEffect(() => {
    if (open) setSelectedIds(new Set())
  }, [item?.id, open])

  if (!open) return null

  const videos = Array.isArray(item?.videos) ? item.videos : []
  const title = getJavDisplayTitle(item, translateTitle)
  const toggleSelectVideo = (video) => {
    const key = videoSelectionKey(video)
    if (!key) return
    setSelectedIds((current) => {
      const next = new Set(current)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  return (
    <AppModal
      ariaLabel={zh('视频管理', 'Manage videos')}
      className="px-4"
      contentClassName="flex max-h-[90vh] w-full max-w-6xl flex-col rounded-lg bg-white p-4 shadow-xl"
      onClose={onClose}
    >
      <div className="mb-3 flex items-center justify-between gap-3">
        <div className="min-w-0">
          <h2 className="truncate text-base font-semibold">{zh('视频管理', 'Manage videos')}</h2>
          <div className="mt-1 truncate text-xs text-gray-500">
            {item?.code || zh('未知番号', 'Unknown code')}
            {title && title !== item?.code ? ` · ${title}` : ''}
          </div>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="rounded px-2 py-1 text-gray-500 hover:bg-gray-100"
          aria-label={zh('关闭视频管理', 'Close video manager')}
        >
          ✕
        </button>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto pr-1">
        {videos.length > 0 ? (
          <VideoGrid
            videos={videos}
            selectedIds={selectedIds}
            onToggleSelect={toggleSelectVideo}
            showSelection={false}
            onPlay={onPlay}
            onOpenFile={onOpenFile}
            onRevealFile={onRevealFile}
            openFileLabel={openFileLabel}
            onOpenTagPicker={onOpenTagPicker}
            showTagEditor={false}
            onOpenScreenshots={onOpenScreenshots}
            onOpenScrapeSettings={onOpenScrapeSettings}
            onRenameVideo={onRenameVideo}
            onDeleteVideo={onDeleteVideo}
            onTagClick={onTagClick}
          />
        ) : (
          <div className="flex min-h-[160px] items-center justify-center rounded border border-dashed border-gray-200 text-sm text-gray-500">
            {zh('暂无关联视频', 'No linked videos')}
          </div>
        )}
      </div>
      <div className="mt-3 flex justify-end">
        <button
          type="button"
          onClick={onClose}
          className="rounded border px-3 py-1.5 text-sm hover:bg-gray-50"
        >
          {zh('关闭', 'Close')}
        </button>
      </div>
    </AppModal>
  )
}
