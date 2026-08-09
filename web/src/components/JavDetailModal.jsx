import { useEffect, useMemo, useRef, useState } from 'react'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import FavoriteBorderRoundedIcon from '@mui/icons-material/FavoriteBorderRounded'
import FavoriteRoundedIcon from '@mui/icons-material/FavoriteRounded'
import { MovieEdit } from '@mui/icons-material'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import RemoveCircleOutlineRoundedIcon from '@mui/icons-material/RemoveCircleOutlineRounded'
import StarBorderRoundedIcon from '@mui/icons-material/StarBorderRounded'
import StarRoundedIcon from '@mui/icons-material/StarRounded'
import { IconButton, Popper, Rating, Tooltip } from '@mui/material'

import {
  deleteVideoScreenshot,
  fetchVideoScreenshotsByIds,
  getResolvedJavSampleImages,
  resolveJavSampleImages,
} from '@/api'
import AppModal from '@/components/AppModal'
import { IdolCard, getIdolCardLayoutProps } from '@/components/JavIdolGrid'
import { SeriesCard } from '@/components/JavSeriesView'
import { StudioCard } from '@/components/JavStudioView'
import VideoGrid from '@/components/VideoGrid'
import { ScreenshotPreviewModal } from '@/components/VideoScreenshotsModal'
import { isUserJavTag } from '@/constants/jav'
import { getVideoDisplayName } from '@/utils/display'
import { getIdolDisplayName } from '@/utils/javIdol'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'

function formatScreenshotTime(name) {
  const stem = String(name || '')
    .replace(/\.[^.]+$/, '')
    .replace(/^mpv_/, '')
  const match = stem.match(/^(\d{2})-(\d{2})-(\d{2})(\.\d+)?$/)
  if (!match) return stem || name
  return `${match[1]}:${match[2]}:${match[3]}`
}

function screenshotStartTime(name) {
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

function screenshotActionKey(video, screenshot) {
  return `${video?.id || 'video'}:${screenshot?.name || ''}`
}

function screenshotItemsMatch(current, next) {
  if (current.length !== next.length) return false
  return current.every((item, index) => {
    const candidate = next[index]
    return (
      Number(item?.video?.id) === Number(candidate?.video?.id) &&
      item?.name === candidate?.name &&
      item?.url === candidate?.url &&
      Boolean(item?.is_cover) === Boolean(candidate?.is_cover)
    )
  })
}

function normalizeSampleImages(images) {
  if (!Array.isArray(images)) return []
  const seen = new Set()
  return images.flatMap((image) => {
    const thumbnailURL = String(image?.thumbnail_url || image?.detail_url || '').trim()
    const detailURL = String(image?.detail_url || image?.thumbnail_url || '').trim()
    if (thumbnailURL === ':not_found' || detailURL === ':not_found') return []
    if (!thumbnailURL || !detailURL) return []
    const key = `${thumbnailURL}\u0000${detailURL}`
    if (seen.has(key)) return []
    seen.add(key)
    return [{ thumbnail_url: thumbnailURL, detail_url: detailURL }]
  })
}

function sampleImagesNotFound(images) {
  return (
    Array.isArray(images) &&
    images.length === 1 &&
    images[0]?.thumbnail_url === ':not_found' &&
    images[0]?.detail_url === ':not_found'
  )
}

function JavFavoriteRatingEditor({ value, saving, error, onChange }) {
  const rating = Number(value) || 0
  const [editing, setEditing] = useState(false)
  const [preview, setPreview] = useState(null)
  const tooltipValue = preview ?? rating
  const hasTooltipValue = preview !== null || rating > 0
  const displayCount = Math.ceil(rating)
  const ratingWidth = !editing ? Math.max(displayCount, 1) * 21 : 5 * 21
  const tooltipTitle = error
    ? error
    : preview === 0
      ? zh('清空喜爱度', 'Clear favorite rating')
      : hasTooltipValue
        ? zh(`喜爱度：${tooltipValue.toFixed(1)} 分`, `Favorite rating: ${tooltipValue.toFixed(1)}`)
        : zh('设置喜爱度评分', 'Set favorite rating')

  return (
    <Tooltip title={tooltipTitle} placement="top" arrow>
      <span
        role="group"
        aria-label={zh('喜爱度评分', 'Favorite rating')}
        className={`inline-flex items-center rounded-full bg-gray-100 px-1.5 py-0.5 transition-opacity ${
          saving ? 'opacity-60' : 'opacity-100'
        }`}
        onMouseLeave={() => {
          setEditing(false)
          setPreview(null)
        }}
        onBlur={(event) => {
          if (event.currentTarget.contains(event.relatedTarget)) return
          setEditing(false)
          setPreview(null)
        }}
      >
        <span
          className="flex overflow-hidden transition-[width] duration-150"
          style={{ width: ratingWidth }}
        >
          <Rating
            name="jav-detail-favorite-rating"
            value={rating}
            precision={0.5}
            size="small"
            icon={<FavoriteRoundedIcon fontSize="inherit" />}
            emptyIcon={<FavoriteBorderRoundedIcon fontSize="inherit" />}
            disabled={saving}
            onChange={onChange}
            onMouseEnter={() => setEditing(true)}
            onFocus={() => setEditing(true)}
            onChangeActive={(_, nextValue) => setPreview(nextValue >= 0.5 ? nextValue : null)}
            sx={{
              flexShrink: 0,
              color: '#fbbf24',
              fontSize: 21,
              '& .MuiRating-iconEmpty': { color: '#9ca3af' },
            }}
          />
        </span>
        {editing && rating > 0 ? (
          <button
            type="button"
            className="ml-1 flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-gray-500 transition hover:bg-gray-200 hover:text-gray-800"
            disabled={saving}
            aria-label={zh('清空喜爱度', 'Clear favorite rating')}
            onMouseEnter={() => setPreview(0)}
            onMouseLeave={() => setPreview(null)}
            onClick={(event) => onChange?.(event, 0)}
          >
            <RemoveCircleOutlineRoundedIcon sx={{ fontSize: 16 }} />
          </button>
        ) : null}
        {rating > 0 && !editing ? (
          <span className="ml-1 shrink-0 text-xs font-semibold tabular-nums leading-none text-gray-700">
            {rating.toFixed(1)}
          </span>
        ) : null}
      </span>
    </Tooltip>
  )
}

/* eslint-disable jsx-a11y/no-noninteractive-element-to-interactive-role */
function JavSampleImageGrid({ images }) {
  const [previewItem, setPreviewItem] = useState(null)
  const previewItems = useMemo(
    () =>
      images.map((image, index) => ({
        name: zh(`样品图像 ${index + 1}`, `Sample image ${index + 1}`),
        url: image.detail_url,
      })),
    [images]
  )

  return (
    <>
      <div className="flex flex-wrap gap-2">
        {images.map((image, index) => (
          <img
            key={`${image.detail_url}-${index}`}
            src={image.thumbnail_url}
            alt={zh(`样品图像 ${index + 1}`, `Sample image ${index + 1}`)}
            onClick={() => setPreviewItem(previewItems[index])}
            aria-label={zh(
              `放大查看第 ${index + 1} 张样品图像`,
              `Enlarge sample image ${index + 1}`
            )}
            onKeyDown={(event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault()
                setPreviewItem(previewItems[index])
              }
            }}
            className="h-24 w-auto cursor-pointer object-contain"
            loading="lazy"
            referrerPolicy="no-referrer"
            role="button"
            tabIndex={0}
          />
        ))}
      </div>
      {previewItem ? (
        <ScreenshotPreviewModal
          item={previewItem}
          items={previewItems}
          onClose={() => setPreviewItem(null)}
          onSelect={setPreviewItem}
        />
      ) : null}
    </>
  )
}
/* eslint-enable jsx-a11y/no-noninteractive-element-to-interactive-role */

function JavScreenshotGrid({ videos, onPlayAtTime, onCoverChanged }) {
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(false)
  const [failedCount, setFailedCount] = useState(0)
  const [error, setError] = useState('')
  const [deletingKey, setDeletingKey] = useState('')
  const [previewItem, setPreviewItem] = useState(null)
  const videoIdentity = (videos || [])
    .map((video) => `${video?.id || ''}:${video?.updated_at || ''}`)
    .join('|')

  useEffect(() => {
    let cancelled = false
    let refreshInFlight = false
    let initialLoad = true
    const videoById = new Map()
    for (const video of videos || []) {
      const videoId = Number(video?.id)
      if (videoId > 0 && !videoById.has(videoId)) videoById.set(videoId, video)
    }
    setItems([])
    setFailedCount(0)
    setError('')
    setDeletingKey('')
    if (videoById.size === 0) {
      setLoading(false)
      return undefined
    }

    setLoading(true)
    const refreshScreenshots = async () => {
      if (refreshInFlight) return
      refreshInFlight = true
      try {
        const screenshots = await fetchVideoScreenshotsByIds(Array.from(videoById.keys()))
        if (cancelled) return
        setItems((current) => {
          const nextItems = screenshots.flatMap((screenshot) => {
            const video = videoById.get(Number(screenshot?.video_id))
            return video ? [{ ...screenshot, video }] : []
          })
          return screenshotItemsMatch(current, nextItems) ? current : nextItems
        })
        setFailedCount(0)
      } catch {
        if (!cancelled) setFailedCount(1)
      } finally {
        refreshInFlight = false
        if (!cancelled && initialLoad) {
          initialLoad = false
          setLoading(false)
        }
      }
    }

    void refreshScreenshots()
    const refreshTimer = window.setInterval(() => {
      void refreshScreenshots()
    }, 1000)

    return () => {
      cancelled = true
      window.clearInterval(refreshTimer)
    }
  }, [videoIdentity, videos])

  const handleDeleteScreenshot = async (video, screenshot) => {
    if (!video?.id || !screenshot?.name || deletingKey) return
    const actionKey = screenshotActionKey(video, screenshot)
    setDeletingKey(actionKey)
    setError('')
    try {
      await deleteVideoScreenshot(video.id, screenshot.name)
      setItems((current) =>
        current.filter(
          (candidate) =>
            !(
              Number(candidate?.video?.id) === Number(video.id) &&
              candidate?.name === screenshot.name
            )
        )
      )
      if (screenshot.is_cover) {
        onCoverChanged?.({
          id: video.id,
          cover_screenshot_name: '',
          updated_at: new Date().toISOString(),
        })
      }
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setDeletingKey('')
    }
  }

  if (loading) {
    return (
      <div className="flex min-h-28 items-center justify-center rounded-lg border border-dashed border-gray-200 text-xs text-gray-500">
        {zh('正在加载视频截图…', 'Loading video screenshots...')}
      </div>
    )
  }

  if (items.length === 0) {
    return (
      <div className="flex min-h-28 items-center justify-center rounded-lg border border-dashed border-gray-200 px-4 text-center text-xs text-gray-500">
        {failedCount > 0 ? getErrorMessage() : zh('暂无视频截图', 'No video screenshots')}
      </div>
    )
  }

  return (
    <>
      {failedCount > 0 ? (
        <div className="mb-2 rounded border border-amber-200 bg-amber-50 px-2.5 py-1.5 text-xs text-amber-700">
          {zh(
            '截图实时刷新失败，正在保留现有结果并继续重试。',
            'Live screenshot refresh failed. Existing results are preserved while retrying.'
          )}
        </div>
      ) : null}
      {error ? (
        <div className="mb-2 rounded border border-red-200 bg-red-50 px-2.5 py-1.5 text-xs text-red-700">
          {error}
        </div>
      ) : null}
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
        {items.map((screenshot) => {
          const video = screenshot.video
          const videoName = getVideoDisplayName(video)
          const startTime = screenshotStartTime(screenshot.name)
          const actionKey = screenshotActionKey(video, screenshot)
          return (
            <div
              key={`${video?.id || 'video'}-${screenshot?.name || screenshot?.url}`}
              className="group overflow-hidden rounded-md border border-gray-200 bg-white text-left"
            >
              <div
                className="relative aspect-video cursor-pointer overflow-hidden bg-gray-100"
                role="button"
                tabIndex={0}
                onClick={() => setPreviewItem(screenshot)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault()
                    setPreviewItem(screenshot)
                  }
                }}
                aria-label={zh(
                  `放大查看 ${videoName} 的截图`,
                  `Enlarge screenshot for ${videoName}`
                )}
              >
                <img
                  src={screenshot.url}
                  alt={screenshot.name}
                  className="h-full w-full object-contain"
                  loading="lazy"
                />
                {screenshot.is_cover ? (
                  <span className="absolute left-1.5 top-1.5 rounded bg-emerald-600/90 px-1.5 py-0.5 text-[10px] font-medium text-white">
                    {zh('当前封面', 'Current cover')}
                  </span>
                ) : null}
                <Tooltip title={zh('删除截图', 'Delete screenshot')}>
                  <IconButton
                    size="small"
                    onClick={(event) => {
                      event.stopPropagation()
                      void handleDeleteScreenshot(video, screenshot)
                    }}
                    disabled={Boolean(deletingKey)}
                    aria-label={zh('删除截图', 'Delete screenshot')}
                    className="!absolute !right-1.5 !top-1.5 !z-10 !bg-white/90 !text-red-600 !opacity-0 hover:!bg-white disabled:!opacity-50 group-hover:!opacity-100"
                  >
                    <DeleteOutlineIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
                <div className="absolute inset-0 flex items-center justify-center bg-transparent opacity-0 transition-opacity group-hover:opacity-100">
                  <Tooltip title={zh('从此处播放', 'Play from here')}>
                    <span>
                      <IconButton
                        onClick={(event) => {
                          event.stopPropagation()
                          onPlayAtTime?.(video, startTime)
                        }}
                        disabled={startTime == null}
                        aria-label={zh('从此处播放', 'Play from here')}
                        className="!h-10 !w-10 !bg-white/90 !text-gray-900 hover:!bg-white disabled:!opacity-50"
                      >
                        <PlayArrowIcon fontSize="small" />
                      </IconButton>
                    </span>
                  </Tooltip>
                </div>
              </div>
              <div className="px-2 py-1.5">
                <div className="truncate text-[11px] font-medium text-gray-800" title={videoName}>
                  {videoName}
                </div>
                <div className="text-[10px] text-gray-500">
                  {formatScreenshotTime(screenshot.name)}
                </div>
              </div>
              {deletingKey === actionKey ? (
                <div className="h-0.5 animate-pulse bg-blue-500" />
              ) : null}
            </div>
          )
        })}
      </div>
      {previewItem ? (
        <ScreenshotPreviewModal
          item={previewItem}
          items={items}
          onClose={() => setPreviewItem(null)}
          onSelect={setPreviewItem}
        />
      ) : null}
    </>
  )
}

export default function JavDetailModal({
  item,
  cover,
  title,
  releaseText,
  durationText,
  studio,
  series,
  tags,
  externalLinks,
  preferChineseName,
  canPlay,
  onClose,
  onPlay,
  onOpenFavorites,
  onEdit,
  favoriteRating,
  favoriteRatingSaving,
  favoriteRatingError,
  onFavoriteRatingChange,
  onSelectStudio,
  onSelectSeries,
  onSelectIdol,
  onSelectPrefix,
  loadIdolPreview,
  loadStudioPreview,
  loadSeriesPreview,
  buildIdolUrl,
  buildStudioUrl,
  buildSeriesUrl,
  buildTagUrl,
  directoryIds,
  onOpenIdolFavorites,
  onOpenStudioFavorites,
  onOpenSeriesFavorites,
  onOpenIdolCoverEditor,
  onOpenIdolEditor,
  onVideoPlay,
  onVideoPlayAtTime,
  onVideoCoverChanged,
  onVideoOpenFile,
  onVideoRevealFile,
  openFileLabel,
  onVideoOpenTagPicker,
  onVideoOpenScreenshots,
  onVideoOpenScrapeSettings,
  onVideoRename,
  onVideoDelete,
  onVideoTagClick,
}) {
  const titleId = `jav-detail-title-${item?.id || 'item'}`
  const itemId = item?.id
  const itemSampleImages = item?.sample_images
  const code = String(item?.code || '').trim()
  const idols = useMemo(() => (Array.isArray(item?.idols) ? item.idols : []), [item?.idols])
  const videos = useMemo(() => (Array.isArray(item?.videos) ? item.videos : []), [item?.videos])
  const studioName = String(studio?.name || '').trim()
  const seriesName = String(series?.name || '').trim()
  const favoriteCount = Number(item?.favorite_count) || 0
  const emptyVideoSelection = useMemo(() => new Set(), [])
  const { coverAspectPercent } = useMemo(() => getIdolCardLayoutProps(), [])
  const [hoverPreview, setHoverPreview] = useState(null)
  const [sampleImages, setSampleImages] = useState(() => normalizeSampleImages(itemSampleImages))
  const [sampleImagesLoading, setSampleImagesLoading] = useState(false)
  const [sampleImagesError, setSampleImagesError] = useState('')
  const hoverCloseTimerRef = useRef(null)
  const activeHoverKeyRef = useRef('')
  const hoverPreviewLockedRef = useRef(false)
  const directoryIdentity = (directoryIds || []).join(',')

  useEffect(() => {
    return () => {
      if (hoverCloseTimerRef.current) window.clearTimeout(hoverCloseTimerRef.current)
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    const initialImages = normalizeSampleImages(itemSampleImages)
    setSampleImages(initialImages)
    setSampleImagesError('')
    if (!itemId) {
      setSampleImagesLoading(false)
      return undefined
    }
    if (initialImages.length > 0) {
      setSampleImagesLoading(false)
      return undefined
    }
    if (sampleImagesNotFound(itemSampleImages)) {
      setSampleImagesLoading(false)
      return undefined
    }

    const requestOptions = {
      directoryIds: directoryIdentity ? directoryIdentity.split(',') : [],
    }
    const resolvedImages = getResolvedJavSampleImages(itemId, requestOptions)
    if (resolvedImages) {
      setSampleImages(normalizeSampleImages(resolvedImages))
      setSampleImagesLoading(false)
      return undefined
    }

    setSampleImagesLoading(true)
    void resolveJavSampleImages(itemId, requestOptions)
      .then((images) => {
        if (!cancelled) setSampleImages(normalizeSampleImages(images))
      })
      .catch((error) => {
        if (!cancelled) setSampleImagesError(getErrorMessage(error))
      })
      .finally(() => {
        if (!cancelled) setSampleImagesLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [directoryIdentity, itemId, itemSampleImages])

  const clearHoverCloseTimer = () => {
    if (!hoverCloseTimerRef.current) return
    window.clearTimeout(hoverCloseTimerRef.current)
    hoverCloseTimerRef.current = null
  }

  const closeHoverPreview = () => {
    activeHoverKeyRef.current = ''
    hoverPreviewLockedRef.current = false
    setHoverPreview(null)
  }

  const scheduleHoverClose = () => {
    clearHoverCloseTimer()
    if (hoverPreviewLockedRef.current) return
    hoverCloseTimerRef.current = window.setTimeout(() => {
      closeHoverPreview()
      hoverCloseTimerRef.current = null
    }, 120)
  }

  const handleHoverStart = (type, previewItem, event) => {
    clearHoverCloseTimer()
    const identity = previewItem?.id || previewItem?.name || ''
    const previewKey = `${type}:${identity}`
    activeHoverKeyRef.current = previewKey
    setHoverPreview({ type, item: previewItem, anchorEl: event.currentTarget })

    const loader =
      type === 'idol' ? loadIdolPreview : type === 'studio' ? loadStudioPreview : loadSeriesPreview
    if (!loader) return
    void loader(previewItem)
      .then((loadedItem) => {
        if (!loadedItem || activeHoverKeyRef.current !== previewKey) return
        setHoverPreview((current) =>
          current?.type === type
            ? { ...current, item: { ...current.item, ...loadedItem } }
            : current
        )
      })
      .catch((error) => {
        console.warn(`load ${type} preview failed`, error)
      })
  }

  const handleStudioSeriesListOpenChange = (open) => {
    clearHoverCloseTimer()
    hoverPreviewLockedRef.current = Boolean(open)
    if (!open) scheduleHoverClose()
  }

  const detailRows = [
    {
      label: zh('识别码', 'Code'),
      content: (
        <div className="flex flex-wrap items-center gap-2">
          <span>{code || zh('未知', 'Unknown')}</span>
          {typeof item?.is_uncensored === 'boolean' ? (
            <span
              className={`rounded px-2 py-0.5 text-xs font-medium ${
                item.is_uncensored ? 'bg-rose-100 text-rose-700' : 'bg-sky-100 text-sky-700'
              }`}
            >
              {item.is_uncensored ? zh('无码', 'Uncensored') : zh('有码', 'Censored')}
            </span>
          ) : null}
        </div>
      ),
    },
    { label: zh('发行日期', 'Release date'), content: releaseText },
    { label: zh('时长', 'Runtime'), content: durationText || zh('未知', 'Unknown') },
    {
      label: zh('片商', 'Studio'),
      content: studioName ? (
        <a
          href={buildStudioUrl?.(studio) || '#'}
          className="text-left font-medium text-blue-700 hover:underline"
          onMouseEnter={(event) => handleHoverStart('studio', studio, event)}
          onMouseLeave={scheduleHoverClose}
        >
          {studioName}
        </a>
      ) : (
        zh('未知', 'Unknown')
      ),
    },
    {
      label: zh('系列', 'Series'),
      content: seriesName ? (
        <a
          href={buildSeriesUrl?.(series) || '#'}
          className="text-left font-medium text-blue-700 hover:underline"
          onMouseEnter={(event) => handleHoverStart('series', series, event)}
          onMouseLeave={scheduleHoverClose}
        >
          {seriesName}
        </a>
      ) : (
        zh('未知', 'Unknown')
      ),
    },
  ]

  return (
    <AppModal
      ariaLabelledby={titleId}
      backdropBlur="2px"
      backdropColor="rgba(2, 6, 23, 0.7)"
      className="p-3 sm:p-6"
      contentClassName="flex max-h-[92vh] w-full max-w-[90rem] flex-col overflow-hidden rounded-xl bg-white shadow-2xl"
      onClose={onClose}
      zIndex={1300}
    >
      <div className="flex items-center justify-between gap-3 border-b border-gray-200 px-4 py-1.5 sm:px-5">
        <div className="min-w-0">
          <h2 id={titleId} className="truncate text-sm font-semibold text-gray-900 sm:text-base">
            {title}
          </h2>
        </div>
        <button
          type="button"
          className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-gray-500 transition hover:bg-gray-100 hover:text-gray-900"
          onClick={onClose}
          aria-label={zh('关闭', 'Close')}
        >
          <CloseOutlinedIcon sx={{ fontSize: 16 }} />
        </button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-4 sm:p-6">
        <div className="grid items-stretch gap-6 lg:grid-cols-[minmax(0,3fr)_minmax(19rem,2fr)]">
          <div className="group relative aspect-[800/538] w-full overflow-hidden rounded-lg border border-gray-200 bg-gray-50 shadow-sm">
            {cover ? (
              <img
                src={cover}
                alt={code || zh('JAV 封面', 'JAV cover')}
                className="h-full w-full object-contain object-top"
              />
            ) : (
              <span className="flex h-full items-center justify-center bg-gradient-to-br from-gray-100 to-gray-200 text-lg font-semibold text-gray-500">
                {code || zh('暂无封面', 'No cover')}
              </span>
            )}
            {canPlay ? (
              <button
                type="button"
                className="absolute left-1/2 top-1/2 z-[1] flex h-20 w-20 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full bg-black/65 text-white opacity-0 shadow-lg transition hover:bg-black/80 focus-visible:opacity-100 group-hover:opacity-100"
                onClick={onPlay}
                aria-label={zh('播放', 'Play')}
              >
                <PlayArrowIcon sx={{ fontSize: 54 }} />
              </button>
            ) : null}
          </div>

          <div className="flex min-w-0 flex-col gap-5">
            <dl className="overflow-hidden rounded-lg border border-gray-200 bg-white">
              {detailRows.map((row, index) => (
                <div
                  key={row.label}
                  className={`grid grid-cols-[5.5rem_minmax(0,1fr)] gap-3 px-4 py-2.5 text-sm ${
                    index > 0 ? 'border-t border-gray-100' : ''
                  }`}
                >
                  <dt className="font-medium text-gray-500">{row.label}</dt>
                  <dd className="min-w-0 break-words text-gray-800">{row.content}</dd>
                </div>
              ))}
            </dl>

            {idols.length > 0 ? (
              <section aria-label={zh('女优', 'Actresses')}>
                <h3 className="mb-2 text-sm font-semibold text-gray-800">
                  {zh('女优', 'Actresses')}
                </h3>
                <div className="flex flex-wrap gap-2">
                  {idols.map((idol) => (
                    <a
                      key={idol?.id || idol?.name}
                      href={buildIdolUrl?.(idol) || '#'}
                      className="rounded-full border border-purple-200 bg-purple-50 px-3 py-1 text-xs font-medium text-purple-700 transition hover:border-purple-300 hover:bg-purple-100"
                      onMouseEnter={(event) => handleHoverStart('idol', idol, event)}
                      onMouseLeave={scheduleHoverClose}
                    >
                      {getIdolDisplayName(idol, preferChineseName)}
                    </a>
                  ))}
                </div>
              </section>
            ) : null}

            {tags.length > 0 ? (
              <section aria-label={zh('标签', 'Tags')}>
                <h3 className="mb-2 text-sm font-semibold text-gray-800">{zh('标签', 'Tags')}</h3>
                <div className="flex flex-wrap gap-2">
                  {tags.map((tag) => {
                    const isUser = isUserJavTag(tag)
                    return (
                      <a
                        key={`${tag?.id || tag?.name}-${tag?.provider || 0}`}
                        href={buildTagUrl?.(tag) || '#'}
                        className={`rounded px-2.5 py-1 text-xs font-medium transition ${
                          isUser
                            ? 'bg-emerald-100 text-emerald-700 hover:bg-emerald-200'
                            : 'bg-orange-100 text-orange-700 hover:bg-orange-200'
                        }`}
                      >
                        {tag?.name}
                      </a>
                    )
                  })}
                </div>
              </section>
            ) : null}

            {externalLinks.length > 0 ? (
              <section aria-label={zh('外部链接', 'External links')}>
                <h3 className="mb-2 text-sm font-semibold text-gray-800">
                  {zh('外部链接', 'External links')}
                </h3>
                <div className="flex flex-wrap gap-2">
                  {externalLinks.map((site) => (
                    <a
                      key={site.key}
                      href={site.href}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1.5 rounded border border-gray-200 bg-white px-2.5 py-1.5 text-xs font-medium text-gray-700 transition hover:border-gray-300 hover:bg-gray-50"
                      onClick={site.onClick}
                    >
                      <img
                        src={site.icon}
                        alt=""
                        className={`h-4 w-4 ${site.loading ? 'animate-pulse' : ''}`}
                        loading="lazy"
                      />
                      <span>{site.name}</span>
                    </a>
                  ))}
                </div>
              </section>
            ) : null}

            <section className="mt-auto pt-1" aria-label={zh('操作', 'Actions')}>
              <h3 className="mb-2 text-sm font-semibold text-gray-800">
                {zh('操作栏', 'Actions')}
              </h3>
              <div className="group flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  className="inline-flex items-center gap-1.5 rounded-md border border-gray-300 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 transition hover:border-gray-400 hover:bg-gray-50"
                  onClick={onOpenFavorites}
                >
                  {favoriteCount > 0 ? (
                    <StarRoundedIcon className="text-amber-500" sx={{ fontSize: 16 }} />
                  ) : (
                    <StarBorderRoundedIcon sx={{ fontSize: 16 }} />
                  )}
                  {zh('收藏', 'Favorite')}
                </button>
                <button
                  type="button"
                  className="inline-flex items-center gap-1.5 rounded-md border border-gray-300 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 transition hover:border-gray-400 hover:bg-gray-50"
                  onClick={onEdit}
                >
                  <MovieEdit sx={{ fontSize: 16 }} />
                  {zh('编辑', 'Edit')}
                </button>
                <JavFavoriteRatingEditor
                  value={favoriteRating}
                  saving={favoriteRatingSaving}
                  error={favoriteRatingError}
                  onChange={onFavoriteRatingChange}
                />
              </div>
            </section>
          </div>
        </div>

        <div className="mt-7 space-y-7">
          <section className="border-t border-gray-200 pt-5" aria-labelledby={`${titleId}-videos`}>
            <div className="mb-3">
              <h3 id={`${titleId}-videos`} className="text-base font-semibold text-gray-900">
                {zh('关联视频', 'Related videos')}
              </h3>
            </div>
            {videos.length > 0 ? (
              <VideoGrid
                videos={videos}
                selectedIds={emptyVideoSelection}
                onToggleSelect={() => {}}
                showSelection={false}
                onPlay={onVideoPlay}
                onOpenFile={onVideoOpenFile}
                onRevealFile={onVideoRevealFile}
                openFileLabel={openFileLabel}
                onOpenTagPicker={onVideoOpenTagPicker}
                showTagEditor
                onOpenScreenshots={onVideoOpenScreenshots}
                onOpenScrapeSettings={onVideoOpenScrapeSettings}
                onRenameVideo={onVideoRename}
                onDeleteVideo={onVideoDelete}
                onTagClick={onVideoTagClick}
              />
            ) : (
              <div className="flex min-h-28 items-center justify-center rounded-lg border border-dashed border-gray-200 text-xs text-gray-500">
                {zh('暂无关联视频', 'No related videos')}
              </div>
            )}
          </section>

          {sampleImagesLoading || sampleImagesError || sampleImages.length > 0 ? (
            <section
              className="border-t border-gray-200 pt-5"
              aria-labelledby={`${titleId}-sample-images`}
            >
              <h3
                id={`${titleId}-sample-images`}
                className="mb-3 text-base font-semibold text-gray-900"
              >
                {zh('样品图像', 'Sample images')}
              </h3>
              {sampleImagesLoading ? (
                <div className="flex min-h-28 items-center justify-center rounded-lg border border-dashed border-gray-200 text-xs text-gray-500">
                  {zh('正在加载样品图像…', 'Loading sample images...')}
                </div>
              ) : sampleImages.length > 0 ? (
                <JavSampleImageGrid images={sampleImages} />
              ) : (
                <div className="rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700">
                  {sampleImagesError}
                </div>
              )}
            </section>
          ) : null}

          <section
            className="border-t border-gray-200 pt-5"
            aria-labelledby={`${titleId}-screenshots`}
          >
            <h3
              id={`${titleId}-screenshots`}
              className="mb-3 text-base font-semibold text-gray-900"
            >
              {zh('视频截图', 'Video screenshots')}
            </h3>
            <JavScreenshotGrid
              videos={videos}
              onPlayAtTime={onVideoPlayAtTime}
              onCoverChanged={onVideoCoverChanged}
            />
          </section>
        </div>
      </div>

      <Popper
        open={Boolean(hoverPreview?.item && hoverPreview?.anchorEl)}
        anchorEl={hoverPreview?.anchorEl || null}
        placement="right-start"
        className="z-[1550]"
        modifiers={[{ name: 'offset', options: { offset: [10, 0] } }]}
      >
        <div
          className={
            hoverPreview?.type === 'studio'
              ? 'w-[320px]'
              : hoverPreview?.type === 'series'
                ? 'w-[260px]'
                : 'w-[220px]'
          }
          onMouseEnter={clearHoverCloseTimer}
          onMouseLeave={scheduleHoverClose}
        >
          {hoverPreview?.type === 'studio' ? (
            <StudioCard
              item={hoverPreview.item}
              href={buildStudioUrl?.(hoverPreview.item)}
              onSelectStudio={onSelectStudio}
              onSelectSeries={onSelectSeries}
              onSelectPrefix={onSelectPrefix}
              onOpenFavorites={onOpenStudioFavorites}
              buildSeriesUrl={buildSeriesUrl}
              onOpenSeriesFavorites={onOpenSeriesFavorites}
              onSeriesListOpenChange={handleStudioSeriesListOpenChange}
              directoryIds={directoryIds}
            />
          ) : null}
          {hoverPreview?.type === 'series' ? (
            <SeriesCard
              item={hoverPreview.item}
              href={buildSeriesUrl?.(hoverPreview.item)}
              onSelectSeries={onSelectSeries}
              onSelectStudio={onSelectStudio}
              onOpenFavorites={onOpenSeriesFavorites}
            />
          ) : null}
          {hoverPreview?.type === 'idol' ? (
            <IdolCard
              item={hoverPreview.item}
              onSelectIdol={onSelectIdol}
              onOpenFavorites={onOpenIdolFavorites}
              onOpenCoverEditor={onOpenIdolCoverEditor}
              onOpenEditor={onOpenIdolEditor}
              href={buildIdolUrl?.(hoverPreview.item)}
              coverAspectPercent={coverAspectPercent}
              showWorkCount={
                typeof hoverPreview.item?.work_count === 'number' &&
                hoverPreview.item.work_count > 0
              }
              preferChineseName={preferChineseName}
            />
          ) : null}
        </div>
      </Popper>
    </AppModal>
  )
}
