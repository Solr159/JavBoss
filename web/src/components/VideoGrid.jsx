import VideoCard from '@/components/VideoCard'
import { videoSelectionKey } from '@/store'

export default function VideoGrid({
  videos,
  selectedIds,
  onToggleSelect,
  onPlay,
  onOpenFile,
  onRevealFile,
  openFileLabel,
  onOpenTagPicker,
  onOpenScreenshots,
  onTagClick,
}) {
  return (
    <div className="grid grid-cols-1 gap-8 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
      {videos.map((v) => (
        <VideoCard
          key={videoSelectionKey(v)}
          video={v}
          checked={selectedIds.has(videoSelectionKey(v))}
          onToggle={() => onToggleSelect(v)}
          onPlay={onPlay}
          onOpenFile={onOpenFile}
          onRevealFile={onRevealFile}
          openFileLabel={openFileLabel}
          onOpenTagPicker={() => onOpenTagPicker(v.id)}
          onOpenScreenshots={onOpenScreenshots}
          onTagClick={onTagClick}
        />
      ))}
    </div>
  )
}
