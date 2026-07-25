import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import ChevronLeftIcon from '@mui/icons-material/ChevronLeft'
import ChevronRightIcon from '@mui/icons-material/ChevronRight'
import CloseIcon from '@mui/icons-material/Close'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import ImageOutlinedIcon from '@mui/icons-material/ImageOutlined'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import RestoreIcon from '@mui/icons-material/Restore'
import { IconButton, Tooltip } from '@mui/material'
import {
  deleteVideoScreenshot,
  fetchVideoScreenshots,
  resetVideoCover,
  updateVideoCover,
} from '@/api'
import { getVideoDisplayName } from '@/utils/display'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'
import {
  PLAYER_HOTKEY_ACTIONS,
  formatPlayerHotkeyKey,
  parsePlayerHotkeys,
} from '@/utils/playerHotkeys'

export default function VideoScreenshotsModal({
  video,
  playerHotkeys,
  onClose,
  onPlayAtTime,
  onCoverChanged,
  allowSetCover = true,
}) {
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [previewItem, setPreviewItem] = useState(null)
  const [deletingName, setDeletingName] = useState('')
  const [settingCoverName, setSettingCoverName] = useState('')
  const open = Boolean(video?.id)
  const title = useMemo(() => getVideoDisplayName(video), [video])
  const currentCoverName = useMemo(() => items.find((item) => item?.is_cover)?.name || '', [items])
  const defaultCoverPreviewSrc = useMemo(() => {
    if (!video?.id) return ''
    const params = new URLSearchParams({ default: '1' })
    const version = [currentCoverName, video?.updated_at || ''].join('|')
    if (version) params.set('v', version)
    return `/videos/${video.id}/thumbnail?${params.toString()}`
  }, [currentCoverName, video?.id, video?.updated_at])
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
    setSettingCoverName('')
    fetchVideoScreenshots(video.id)
      .then((nextItems) => {
        if (!cancelled) setItems(nextItems)
      })
      .catch((err) => {
        console.error(zh('加载截图失败', 'Failed to load screenshots'), err)
        if (!cancelled) setError(getErrorMessage(err))
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
    return `${match[1]}:${match[2]}:${match[3]}`
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
      if (item.is_cover) onCoverChanged?.()
    } catch (err) {
      console.error(zh('删除截图失败', 'Failed to delete screenshot'), err)
      setError(getErrorMessage(err))
    } finally {
      setDeletingName('')
    }
  }

  const handleSetCover = async (item) => {
    if (!video?.id || !item?.name || settingCoverName || item.is_cover) return
    setSettingCoverName(item.name)
    setError('')
    try {
      const updated = await updateVideoCover(video.id, item.name)
      setItems((current) =>
        current.map((candidate) => ({ ...candidate, is_cover: candidate.name === item.name }))
      )
      onCoverChanged?.(updated)
    } catch (err) {
      console.error(zh('保存视频封面失败', 'Failed to save video cover'), err)
      setError(getErrorMessage(err))
    } finally {
      setSettingCoverName('')
    }
  }

  const handleResetCover = async () => {
    if (!video?.id || settingCoverName || !currentCoverName) return
    setSettingCoverName(currentCoverName)
    setError('')
    try {
      const updated = await resetVideoCover(video.id)
      setItems((current) => current.map((candidate) => ({ ...candidate, is_cover: false })))
      onCoverChanged?.(updated)
    } catch (err) {
      console.error(zh('恢复默认封面失败', 'Failed to restore default cover'), err)
      setError(getErrorMessage(err))
    } finally {
      setSettingCoverName('')
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
          <div className="flex items-center gap-1">
            {allowSetCover && currentCoverName ? (
              <Tooltip
                arrow
                placement="left"
                title={zh('恢复默认封面', 'Restore default cover')}
                slotProps={{
                  popper: {
                    sx: {
                      pointerEvents: 'none',
                      zIndex: (theme) => theme.zIndex.modal + 1000,
                    },
                    modifiers: [
                      {
                        name: 'offset',
                        options: {
                          offset: [0, 4],
                        },
                      },
                    ],
                  },
                }}
              >
                <span className="group relative inline-flex">
                  <IconButton
                    size="small"
                    onClick={handleResetCover}
                    disabled={Boolean(settingCoverName)}
                    aria-label={zh('恢复默认封面', 'Restore default cover')}
                  >
                    <RestoreIcon fontSize="inherit" />
                  </IconButton>
                  <DefaultCoverPreview src={defaultCoverPreviewSrc} />
                </span>
              </Tooltip>
            ) : null}
            <IconButton
              size="small"
              onClick={onClose}
              aria-label={zh('关闭截图弹窗', 'Close screenshots modal')}
            >
              <CloseIcon fontSize="inherit" />
            </IconButton>
          </div>
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
                    className="group overflow-hidden rounded border border-gray-200 bg-white"
                  >
                    <div
                      className="relative aspect-video cursor-pointer bg-gray-100"
                      role="button"
                      tabIndex={0}
                      onClick={() => setPreviewItem(item)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault()
                          setPreviewItem(item)
                        }
                      }}
                    >
                      <img
                        src={item.url}
                        alt={item.name}
                        loading="lazy"
                        className="h-full w-full object-contain"
                      />
                      {item.is_cover ? (
                        <div className="absolute left-2 top-2 z-10 inline-flex max-w-[calc(100%-1rem)] items-center gap-1 rounded bg-emerald-600/90 px-2 py-1 text-xs font-medium text-white">
                          <CheckCircleOutlineIcon className="h-4 w-4" fontSize="inherit" />
                          <span className="truncate">{zh('当前封面', 'Current cover')}</span>
                        </div>
                      ) : null}
                      <Tooltip title={zh('删除截图', 'Delete screenshot')}>
                        <IconButton
                          size="small"
                          onClick={(event) => {
                            event.stopPropagation()
                            handleDeleteScreenshot(item)
                          }}
                          disabled={deletingName === item.name}
                          aria-label={zh('删除截图', 'Delete screenshot')}
                          className="!absolute !right-2 !top-2 !z-10 !bg-white/90 !text-red-600 !opacity-0 hover:!bg-white disabled:!opacity-50 group-hover:!opacity-100"
                        >
                          <DeleteOutlineIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                      <div className="absolute inset-0 flex items-center justify-center gap-5 bg-transparent opacity-0 transition-opacity group-hover:opacity-100">
                        <Tooltip title={zh('从此处播放', 'Play from here')}>
                          <span>
                            <IconButton
                              onClick={(event) => {
                                event.stopPropagation()
                                onPlayAtTime?.(video, startTime)
                              }}
                              disabled={startTime == null}
                              aria-label={zh('从此处播放', 'Play from here')}
                              className="!h-12 !w-12 !bg-white/90 !text-gray-900 hover:!bg-white disabled:!opacity-50"
                            >
                              <PlayArrowIcon fontSize="medium" />
                            </IconButton>
                          </span>
                        </Tooltip>
                        {allowSetCover ? (
                          <Tooltip
                            title={
                              item.is_cover
                                ? zh('当前封面', 'Current cover')
                                : zh('设为封面', 'Set as cover')
                            }
                          >
                            <span>
                              <IconButton
                                onClick={(event) => {
                                  event.stopPropagation()
                                  handleSetCover(item)
                                }}
                                disabled={Boolean(settingCoverName) || item.is_cover}
                                aria-label={
                                  item.is_cover
                                    ? zh('当前封面', 'Current cover')
                                    : zh('设为封面', 'Set as cover')
                                }
                                className="!h-12 !w-12 !bg-white/90 !text-gray-900 hover:!bg-white disabled:!opacity-50"
                              >
                                {item.is_cover ? (
                                  <CheckCircleOutlineIcon fontSize="medium" />
                                ) : (
                                  <ImageOutlinedIcon fontSize="medium" />
                                )}
                              </IconButton>
                            </span>
                          </Tooltip>
                        ) : null}
                      </div>
                    </div>
                    <div className="truncate px-2 py-1 text-xs text-gray-600">{displayName}</div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>
      {previewItem ? (
        <ScreenshotPreviewModal
          item={previewItem}
          items={items}
          onClose={() => setPreviewItem(null)}
          onSelect={setPreviewItem}
        />
      ) : null}
    </div>
  )
}

function DefaultCoverPreview({ src }) {
  const [imageFailed, setImageFailed] = useState(false)

  useEffect(() => {
    setImageFailed(false)
  }, [src])

  return (
    <span className="pointer-events-none invisible absolute left-1/2 top-full z-[2000] mt-2 w-72 -translate-x-1/2 overflow-hidden rounded border border-gray-200 bg-white text-gray-900 opacity-0 shadow-lg transition group-focus-within:visible group-focus-within:opacity-100 group-hover:visible group-hover:opacity-100">
      <div className="border-b border-gray-100 px-3 py-2 text-xs font-medium">
        {zh('默认封面预览', 'Default cover preview')}
      </div>
      <div className="flex aspect-video items-center justify-center bg-gray-100">
        {src && !imageFailed ? (
          <img
            src={src}
            alt={zh('默认封面预览', 'Default cover preview')}
            className="h-full w-full object-cover"
            onError={() => setImageFailed(true)}
          />
        ) : (
          <div className="px-3 text-center text-xs text-gray-500">
            {zh('默认封面待生成', 'Default cover pending')}
          </div>
        )}
      </div>
    </span>
  )
}

function screenshotPreviewIdentity(item) {
  return `${item?.video_id || item?.video?.id || ''}:${item?.name || item?.url || ''}`
}

export function ScreenshotPreviewModal({ item, items, onClose, onSelect }) {
  const lastWheelAtRef = useRef(0)
  const itemIdentity = screenshotPreviewIdentity(item)
  const currentIndex = useMemo(
    () => items.findIndex((candidate) => screenshotPreviewIdentity(candidate) === itemIdentity),
    [itemIdentity, items]
  )
  const canNavigate = items.length > 1 && currentIndex >= 0
  const counterText =
    currentIndex >= 0 ? `${currentIndex + 1}/${items.length}` : `1/${Math.max(1, items.length)}`
  const goBy = useCallback(
    (direction) => {
      if (!canNavigate) return
      const nextIndex = (currentIndex + direction + items.length) % items.length
      onSelect?.(items[nextIndex])
    },
    [canNavigate, currentIndex, items, onSelect]
  )

  useEffect(() => {
    if (!item?.url) return undefined

    const previousOverflow = document.body.style.overflow
    const previousHtmlOverflow = document.documentElement.style.overflow
    const handleWheel = (event) => {
      event.preventDefault()
      event.stopPropagation()
      const now = Date.now()
      if (now - lastWheelAtRef.current < 180) return
      lastWheelAtRef.current = now
      goBy(event.deltaY > 0 ? 1 : -1)
    }

    document.body.style.overflow = 'hidden'
    document.documentElement.style.overflow = 'hidden'
    window.addEventListener('wheel', handleWheel, { passive: false, capture: true })

    return () => {
      document.body.style.overflow = previousOverflow
      document.documentElement.style.overflow = previousHtmlOverflow
      window.removeEventListener('wheel', handleWheel, true)
    }
  }, [goBy, item?.url])

  if (!item?.url) return null

  return createPortal(
    <div
      className="fixed inset-0 z-[1900] flex flex-col items-center justify-center bg-black/80 p-4"
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
      <div className="relative z-10 flex max-w-[82vw] items-center justify-center">
        <div className="flex max-h-[78vh] max-w-full items-center justify-center">
          <img
            src={item.url}
            alt={item.name || zh('MPV 截图', 'MPV screenshot')}
            className="max-h-[78vh] max-w-full object-contain shadow-2xl"
          />
        </div>
      </div>
      <div className="fixed bottom-10 left-1/2 z-20 flex -translate-x-1/2 items-center justify-center gap-2.5">
        {canNavigate ? (
          <IconButton
            onClick={(event) => {
              event.stopPropagation()
              goBy(-1)
            }}
            aria-label={zh('上一张截图', 'Previous screenshot')}
            className="!h-11 !w-11 !bg-black/55 !text-white !shadow-lg hover:!bg-black/75"
          >
            <ChevronLeftIcon className="!text-[36px]" />
          </IconButton>
        ) : null}
        <div className="flex h-11 min-w-14 items-center justify-center rounded bg-black/50 px-3 text-center text-base font-semibold text-white shadow-lg">
          {counterText}
        </div>
        {canNavigate ? (
          <IconButton
            onClick={(event) => {
              event.stopPropagation()
              goBy(1)
            }}
            aria-label={zh('下一张截图', 'Next screenshot')}
            className="!h-11 !w-11 !bg-black/55 !text-white !shadow-lg hover:!bg-black/75"
          >
            <ChevronRightIcon className="!text-[36px]" />
          </IconButton>
        ) : null}
      </div>
    </div>,
    document.body
  )
}
