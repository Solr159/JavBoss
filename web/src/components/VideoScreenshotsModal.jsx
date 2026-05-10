import { useEffect, useMemo, useState } from 'react'
import CloseIcon from '@mui/icons-material/Close'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import ZoomInIcon from '@mui/icons-material/ZoomIn'
import { IconButton, Tooltip } from '@mui/material'
import { deleteVideoScreenshot, fetchVideoScreenshots } from '@/api'
import { getVideoDisplayName } from '@/utils/display'
import { zh } from '@/utils/i18n'
import {
  PLAYER_HOTKEY_ACTIONS,
  formatPlayerHotkeyKey,
  parsePlayerHotkeys,
} from '@/utils/playerHotkeys'

export default function VideoScreenshotsModal({ video, playerHotkeys, onClose, onPlayAtTime }) {
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [previewItem, setPreviewItem] = useState(null)
  const [deletingName, setDeletingName] = useState('')
  const open = Boolean(video?.id)
  const title = useMemo(() => getVideoDisplayName(video), [video])
  const screenshotKey = useMemo(() => {
    const hotkeys = parsePlayerHotkeys(playerHotkeys)
    const screenshotHotkey = hotkeys.find(
      (item) => item.action === PLAYER_HOTKEY_ACTIONS.SCREENSHOT
    )
    return formatPlayerHotkeyKey(screenshotHotkey?.key || 'e')
  }, [playerHotkeys])

  useEffect(() => {
    let cancelled = false
    if (!open) return undefined

    setLoading(true)
    setError('')
    setItems([])
    setPreviewItem(null)
    setDeletingName('')
    fetchVideoScreenshots(video.id)
      .then((nextItems) => {
        if (!cancelled) setItems(nextItems)
      })
      .catch((err) => {
        console.error(zh('加载截图失败', 'Failed to load screenshots'), err)
        if (!cancelled) setError(err?.message || zh('加载截图失败', 'Failed to load screenshots'))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [open, video?.id])

  if (!open) return null

  const formatScreenshotName = (name) => {
    const stem = String(name || '')
      .replace(/\.[^.]+$/, '')
      .replace(/^mpv_/, '')
    const match = stem.match(/^(\d{2})-(\d{2})-(\d{2})(\.\d+)?$/)
    if (!match) return stem || name
    return `${match[1]}:${match[2]}:${match[3]}${match[4] || ''}`
  }

  const screenshotStartTime = (name) => {
    const stem = String(name || '')
      .replace(/\.[^.]+$/, '')
      .replace(/^mpv_/, '')
    const match = stem.match(/^(\d{2})-(\d{2})-(\d{2})(\.\d+)?$/)
    if (!match) return null
    return (
      Number.parseInt(match[1], 10) * 3600 +
      Number.parseInt(match[2], 10) * 60 +
      Number.parseInt(match[3], 10) +
      Number.parseFloat(match[4] || '0')
    )
  }

  const handleDeleteScreenshot = async (item) => {
    if (!video?.id || !item?.name || deletingName) return
    setDeletingName(item.name)
    setError('')
    try {
      await deleteVideoScreenshot(video.id, item.name)
      setItems((current) => current.filter((candidate) => candidate.name !== item.name))
      setPreviewItem((current) => (current?.name === item.name ? null : current))
    } catch (err) {
      console.error(zh('删除截图失败', 'Failed to delete screenshot'), err)
      setError(err?.message || zh('删除截图失败', 'Failed to delete screenshot'))
    } finally {
      setDeletingName('')
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4 py-6">
      <div className="flex max-h-full w-full max-w-5xl flex-col rounded-lg bg-white shadow-xl">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <div className="min-w-0">
            <h2 className="truncate text-base font-semibold text-gray-900">
              {zh('视频截图', 'Video Screenshots')}
            </h2>
            <div className="truncate text-xs text-gray-500" title={title}>
              {title}
            </div>
          </div>
          <IconButton
            size="small"
            onClick={onClose}
            aria-label={zh('关闭截图弹窗', 'Close screenshots modal')}
          >
            <CloseIcon fontSize="inherit" />
          </IconButton>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          {loading ? (
            <div className="flex min-h-48 items-center justify-center rounded border border-dashed border-gray-200 text-sm text-gray-500">
              {zh('加载中...', 'Loading...')}
            </div>
          ) : error ? (
            <div className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
              {error}
            </div>
          ) : items.length === 0 ? (
            <div className="flex min-h-48 items-center justify-center rounded border border-dashed border-gray-200 px-4 text-center text-sm text-gray-500">
              {zh(
                `暂无截图。使用 MPV播放器 播放时按 ${screenshotKey} 键截图，会显示在此处。`,
                `No screenshots yet. Press ${screenshotKey} while playing with the MPV player to capture one, and it will appear here.`
              )}
            </div>
          ) : (
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {items.map((item) => {
                const displayName = formatScreenshotName(item.name)
                const startTime = screenshotStartTime(item.name)
                return (
                  <div
                    key={item.name}
                    className="group overflow-hidden rounded border border-gray-200 bg-white hover:border-gray-300"
                  >
                    <div className="relative aspect-video bg-gray-100">
                      <img
                        src={item.url}
                        alt={item.name}
                        loading="lazy"
                        className="h-full w-full object-contain"
                      />
                      <Tooltip title={zh('删除截图', 'Delete screenshot')}>
                        <IconButton
                          size="small"
                          onClick={() => handleDeleteScreenshot(item)}
                          disabled={deletingName === item.name}
                          aria-label={zh('删除截图', 'Delete screenshot')}
                          className="!absolute !right-2 !top-2 !z-10 !bg-white/90 !text-red-600 !opacity-0 hover:!bg-white disabled:!opacity-50 group-hover:!opacity-100"
                        >
                          <DeleteOutlineIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                      <div className="absolute inset-0 flex items-center justify-center gap-5 bg-black/0 opacity-0 transition group-hover:bg-black/35 group-hover:opacity-100">
                        <Tooltip title={zh('放大图片', 'Enlarge image')}>
                          <IconButton
                            onClick={() => setPreviewItem(item)}
                            aria-label={zh('放大图片', 'Enlarge image')}
                            className="!h-12 !w-12 !bg-white/90 !text-gray-900 hover:!bg-white"
                          >
                            <ZoomInIcon fontSize="medium" />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title={zh('从此处播放', 'Play from here')}>
                          <span>
                            <IconButton
                              onClick={() => onPlayAtTime?.(video, startTime)}
                              disabled={startTime == null}
                              aria-label={zh('从此处播放', 'Play from here')}
                              className="!h-12 !w-12 !bg-white/90 !text-gray-900 hover:!bg-white disabled:!opacity-50"
                            >
                              <PlayArrowIcon fontSize="medium" />
                            </IconButton>
                          </span>
                        </Tooltip>
                      </div>
                    </div>
                    <div className="truncate px-2 py-1 text-xs text-gray-600 group-hover:text-gray-900">
                      {displayName}
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>
      {previewItem ? (
        <ScreenshotPreviewModal item={previewItem} onClose={() => setPreviewItem(null)} />
      ) : null}
    </div>
  )
}

function ScreenshotPreviewModal({ item, onClose }) {
  const [scale, setScale] = useState(1)

  useEffect(() => {
    if (!item?.url) return undefined

    setScale(1)
    const previousOverflow = document.body.style.overflow
    const previousHtmlOverflow = document.documentElement.style.overflow
    const handleWheel = (event) => {
      event.preventDefault()
      event.stopPropagation()
      const direction = event.deltaY < 0 ? 1 : -1
      setScale((current) => Math.min(5, Math.max(0.5, current + direction * 0.2)))
    }

    document.body.style.overflow = 'hidden'
    document.documentElement.style.overflow = 'hidden'
    window.addEventListener('wheel', handleWheel, { passive: false, capture: true })

    return () => {
      document.body.style.overflow = previousOverflow
      document.documentElement.style.overflow = previousHtmlOverflow
      window.removeEventListener('wheel', handleWheel, true)
    }
  }, [item?.url])

  if (!item?.url) return null

  return (
    <div
      className="fixed inset-0 z-[1500] flex items-center justify-center bg-black/80 p-4"
      role="dialog"
      aria-modal="true"
      aria-label={zh('截图预览', 'Screenshot preview')}
    >
      <button
        type="button"
        className="absolute inset-0 cursor-default"
        aria-label={zh('关闭截图预览', 'Close screenshot preview')}
        onClick={onClose}
      />
      <button
        type="button"
        onClick={onClose}
        className="absolute right-4 top-4 z-10 rounded bg-black/50 px-3 py-1 text-xl leading-none text-white hover:bg-black/70"
        aria-label={zh('关闭截图预览', 'Close screenshot preview')}
      >
        ×
      </button>
      <img
        src={item.url}
        alt={item.name || zh('MPV 截图', 'MPV screenshot')}
        className="relative z-10 max-h-[92vh] max-w-[94vw] transform-gpu cursor-zoom-in object-contain shadow-2xl"
        style={{ transform: `scale(${scale})` }}
      />
    </div>
  )
}
