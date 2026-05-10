import { IconButton, Tooltip } from '@mui/material'
import LocalOfferOutlinedIcon from '@mui/icons-material/LocalOfferOutlined'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import FolderOpenIcon from '@mui/icons-material/FolderOpen'
import { revealVideoLocation } from '@/api'
import { formatBytes, getVideoDisplayName, parseVideoFingerprint } from '@/utils/display'
import { zh } from '@/utils/i18n'
import PhotoLibraryOutlinedIcon from '@mui/icons-material/PhotoLibraryOutlined'

export default function VideoCard({
  video,
  checked,
  onToggle,
  onPlay,
  onOpenFile,
  onRevealFile,
  openFileLabel,
  onOpenTagPicker,
  onOpenScreenshots,
  onTagClick,
}) {
  const displayName = getVideoDisplayName(video)
  const durationSec = Number(video?.duration_sec)
  const durationMinutes =
    Number.isFinite(durationSec) && durationSec > 0
      ? Math.max(1, Math.round(durationSec / 60))
      : null
  const meta = parseVideoFingerprint(video?.fingerprint)
  const resolution =
    meta.width && meta.height && meta.width > 0 && meta.height > 0
      ? `${meta.width}x${meta.height}`
      : ''
  const sizeText = formatBytes(meta.size || video?.size)
  const directoryPath = video?.directory?.path || video?.directory_path || ''
  const videoPath = video?.path || ''
  const canOpen = Boolean(directoryPath && videoPath)
  const inputId = `check-${video?.location_id || video.id}`

  const handleOpenFile = async (event) => {
    event.stopPropagation()
    if (!canOpen) return
    try {
      await onOpenFile?.(video)
    } catch (err) {
      console.error(zh('打开文件失败', 'Open file failed'), err)
    }
  }

  const handleRevealFile = async (event) => {
    event.stopPropagation()
    if (!canOpen) return
    try {
      if (onRevealFile) {
        await onRevealFile(video)
      } else {
        await revealVideoLocation({ path: videoPath, dirPath: directoryPath })
      }
    } catch (err) {
      console.error(zh('打开所在位置失败', 'Reveal file failed'), err)
    }
  }

  const handleOpenScreenshots = (event) => {
    event.stopPropagation()
    onOpenScreenshots?.(video)
  }

  return (
    <div
      className={`video-card group relative overflow-hidden rounded-xl border bg-white shadow transition-all ${
        checked ? 'border-sky-400 ring-2 ring-sky-200' : 'border-gray-200 hover:border-gray-300'
      }`}
    >
      <div className={`video-card-select ${checked ? 'is-visible' : ''}`}>
        <input
          id={inputId}
          type="checkbox"
          checked={checked}
          onChange={onToggle}
          className="video-select-check"
          onClick={(e) => e.stopPropagation()}
          onPointerUp={(e) => {
            e.currentTarget.blur()
          }}
          aria-label={zh(`选择 ${displayName}`, `Select ${displayName}`)}
        />
      </div>
      <div className="relative aspect-video w-full overflow-hidden bg-gray-200">
        <img
          src={`/videos/${video.id}/thumbnail`}
          alt={displayName}
          className="h-full w-full object-cover"
          loading="lazy"
          onError={(e) => {
            e.currentTarget.style.display = 'none'
          }}
        />
        <div className="absolute bottom-2 left-2 z-10 opacity-0 transition-opacity group-hover:opacity-100">
          <button
            type="button"
            onClick={handleOpenScreenshots}
            title={zh('查看截图', 'View screenshots')}
            aria-label={zh('查看截图', 'View screenshots')}
            className="flex h-8 w-8 items-center justify-center rounded-full bg-black/70 text-white shadow-lg shadow-black/60 hover:bg-black/85"
          >
            <PhotoLibraryOutlinedIcon className="h-5 w-5 text-white" fontSize="inherit" />
          </button>
        </div>
      </div>

      <div className="p-3">
        <div className="flex items-center gap-2">
          <div
            className="line-clamp-2 text-[11px] font-medium leading-snug sm:text-xs"
            title={displayName}
          >
            {displayName}
          </div>
        </div>
        <div className="mt-2 flex flex-wrap items-center gap-1">
          <span className="inline-flex h-4 items-center rounded bg-gray-100 px-1 text-[10px] font-medium text-gray-700">
            {durationMinutes
              ? zh(`${durationMinutes} 分钟`, `${durationMinutes} min`)
              : zh('时长未知', 'Unknown duration')}
          </span>
          {resolution ? (
            <span className="inline-flex h-4 items-center rounded bg-gray-100 px-1 text-[10px] font-medium text-gray-700">
              {resolution}
            </span>
          ) : null}
          {sizeText ? (
            <span className="inline-flex h-4 items-center rounded bg-gray-100 px-1 text-[10px] font-medium text-gray-700">
              {sizeText}
            </span>
          ) : null}
        </div>
        <div className="mt-2 flex flex-wrap items-center gap-1">
          {video.tags?.length
            ? video.tags.map((t) => (
                <button
                  key={t.id}
                  type="button"
                  className="inline-flex h-4 items-center justify-center rounded bg-orange-300 px-1 text-[10px] font-semibold text-black"
                  onClick={(e) => {
                    e.stopPropagation()
                    onTagClick?.(t.name)
                  }}
                >
                  {t.name}
                </button>
              ))
            : null}
          <Tooltip title={zh('修改标签', 'Edit tags')}>
            <IconButton
              size="small"
              onClick={(e) => {
                e.stopPropagation()
                onOpenTagPicker()
              }}
              aria-label={zh('修改标签', 'Edit tags')}
              className="h-6 w-6"
            >
              <LocalOfferOutlinedIcon fontSize="inherit" />
            </IconButton>
          </Tooltip>
          <Tooltip title={openFileLabel || zh('用默认程序打开', 'Open with default app')}>
            <IconButton
              size="small"
              onClick={handleOpenFile}
              disabled={!canOpen}
              aria-label={openFileLabel || zh('打开文件', 'Open file')}
              className="h-6 w-6"
            >
              <PlayArrowIcon fontSize="inherit" />
            </IconButton>
          </Tooltip>
          <Tooltip title={zh('打开所在位置', 'Reveal in folder')}>
            <IconButton
              size="small"
              onClick={handleRevealFile}
              disabled={!canOpen}
              aria-label={zh('打开所在位置', 'Reveal in folder')}
              className="h-6 w-6"
            >
              <FolderOpenIcon fontSize="inherit" />
            </IconButton>
          </Tooltip>
        </div>
      </div>

      <div className="pointer-events-none absolute inset-0 flex items-center justify-center bg-black/0 text-white opacity-0 transition-opacity group-hover:opacity-100">
        <button
          onClick={(e) => {
            e.stopPropagation()
            onPlay(video)
          }}
          className="pointer-events-auto rounded-full bg-black/60 p-3 hover:bg-black/80"
          aria-label={zh('播放', 'Play')}
          title={zh('播放', 'Play')}
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="currentColor"
            className="h-10 w-10"
          >
            <path d="M8 5v14l11-7z" />
          </svg>
        </button>
      </div>
    </div>
  )
}
