import { useEffect, useMemo, useRef, useState } from 'react'
import { IconButton, Popper, Tooltip } from '@mui/material'
import Fade from '@mui/material/Fade'
import LocalOfferOutlinedIcon from '@mui/icons-material/LocalOfferOutlined'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import FolderOpenIcon from '@mui/icons-material/FolderOpen'
import OpenInNewIcon from '@mui/icons-material/OpenInNew'
import PhotoLibraryOutlinedIcon from '@mui/icons-material/PhotoLibraryOutlined'
import SearchIcon from '@mui/icons-material/Search'
import VideocamOutlinedIcon from '@mui/icons-material/VideocamOutlined'

import { fetchJavIdolPreview } from '@/api'
import { IdolCard, getIdolCardLayoutProps } from '@/components/JavIdolGrid'
import { isUserJavTag } from '@/constants/jav'
import { getJavDisplayTitle } from '@/utils/jav'
import { directoryQueryIds, useStore } from '@/store'
import { zh } from '@/utils/i18n'

function DurationIcon() {
  return (
    <svg viewBox="0 0 20 20" aria-hidden="true" className="h-4 w-4 shrink-0">
      <circle cx="10" cy="10" r="7" fill="#F59E0B" />
      <circle cx="10" cy="10" r="5.4" fill="#FEF3C7" />
      <path
        d="M10 6.7v3.5l2.5 1.6"
        fill="none"
        stroke="#7C3AED"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path d="M7.4 2.8h5.2" fill="none" stroke="#EF4444" strokeWidth="1.4" strokeLinecap="round" />
    </svg>
  )
}

function ReleaseIcon() {
  return (
    <svg viewBox="0 0 20 20" aria-hidden="true" className="h-4 w-4 shrink-0">
      <rect x="3.1" y="4.1" width="13.8" height="12.8" rx="2.4" fill="#A78BFA" />
      <rect x="3.9" y="7" width="12.2" height="8.9" rx="1.7" fill="#FFF7ED" />
      <rect
        x="3.1"
        y="4.1"
        width="13.8"
        height="12.8"
        rx="2.4"
        fill="none"
        stroke="#7C3AED"
        strokeWidth="0.8"
      />
      <path
        d="M6.4 3.2v2.8M13.6 3.2v2.8"
        fill="none"
        stroke="#EC4899"
        strokeWidth="1.6"
        strokeLinecap="round"
      />
      <path d="M5.8 8.8h8.4" fill="none" stroke="#F97316" strokeWidth="1.4" strokeLinecap="round" />
      <rect x="6.7" y="10.2" width="2.5" height="2.3" rx="0.5" fill="#22C55E" />
      <rect x="10.7" y="10.2" width="2.5" height="2.3" rx="0.5" fill="#3B82F6" />
      <rect x="6.7" y="13.4" width="2.5" height="2.3" rx="0.5" fill="#F43F5E" />
      <rect x="10.7" y="13.4" width="2.5" height="2.3" rx="0.5" fill="#14B8A6" />
    </svg>
  )
}

export default function JavGrid({
  items,
  columns = 0,
  buildJavUrl,
  onPlay,
  onIdolClick,
  onStudioClick,
  onTagClick,
  onEditTags,
  onOpenFile,
  openFileLabel,
  onRevealFile,
  onOpenScreenshots,
}) {
  const directoryIds = useStore(directoryQueryIds)
  const javMetadataLanguage = useStore((state) =>
    state.config?.jav_metadata_language === 'en' ? 'en' : 'zh'
  )
  const idolPreviewCacheRef = useRef(new Map())
  const idolPreviewInflightRef = useRef(new Map())
  const [coverPreview, setCoverPreview] = useState(null)
  const hasItems = Array.isArray(items) && items.length > 0
  const columnCount = Number.isFinite(Number(columns)) ? Math.floor(Number(columns)) : 0
  const fixedColumnCount = columnCount > 0 ? Math.min(columnCount, 12) : 0
  const gridClassName = fixedColumnCount
    ? 'grid gap-4'
    : 'grid gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-3 2xl:grid-cols-4'
  const gridStyle = fixedColumnCount
    ? { gridTemplateColumns: `repeat(${fixedColumnCount}, minmax(0, 1fr))` }
    : undefined

  const loadIdolPreview = async (idol) => {
    const idolId = Number(idol?.id)
    if (!Number.isFinite(idolId) || idolId <= 0) {
      return idol || null
    }

    const cacheKey = `${idolId}|${(directoryIds || []).join(',')}`
    const cached = idolPreviewCacheRef.current.get(cacheKey)
    if (cached) {
      return cached
    }

    const inflight = idolPreviewInflightRef.current.get(cacheKey)
    if (inflight) {
      return inflight
    }

    const request = fetchJavIdolPreview(idolId, { directoryIds })
      .then((preview) => {
        idolPreviewCacheRef.current.set(cacheKey, preview)
        return preview
      })
      .finally(() => {
        idolPreviewInflightRef.current.delete(cacheKey)
      })
    idolPreviewInflightRef.current.set(cacheKey, request)
    return request
  }

  if (!hasItems) {
    return (
      <div className="mt-4 flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
        {zh('暂无 JAV 数据', 'No JAV data')}
      </div>
    )
  }

  return (
    <div className={gridClassName} style={gridStyle}>
      {items.map((item) => (
        <JavCard
          key={item.id || item.code}
          item={item}
          onPlay={onPlay}
          buildJavUrl={buildJavUrl}
          onIdolClick={onIdolClick}
          onStudioClick={onStudioClick}
          onTagClick={onTagClick}
          onEditTags={onEditTags}
          onOpenFile={onOpenFile}
          openFileLabel={openFileLabel}
          onRevealFile={onRevealFile}
          onOpenScreenshots={onOpenScreenshots}
          loadIdolPreview={loadIdolPreview}
          onOpenCoverPreview={setCoverPreview}
          javMetadataLanguage={javMetadataLanguage}
        />
      ))}
      {coverPreview ? (
        <CoverPreviewModal preview={coverPreview} onClose={() => setCoverPreview(null)} />
      ) : null}
    </div>
  )
}

function CoverPreviewModal({ preview, onClose }) {
  const [scale, setScale] = useState(1)

  useEffect(() => {
    if (!preview?.src) return undefined

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
  }, [preview?.src])

  if (!preview?.src) return null

  return (
    <div
      className="fixed inset-0 z-[1500] flex items-center justify-center bg-black/80 p-4"
      role="dialog"
      aria-modal="true"
      aria-label={zh('封面预览', 'Cover preview')}
    >
      <button
        type="button"
        className="absolute inset-0 cursor-default"
        aria-label={zh('关闭封面预览', 'Close cover preview')}
        onClick={onClose}
      />
      <button
        type="button"
        onClick={onClose}
        className="absolute right-4 top-4 z-10 rounded bg-black/50 px-3 py-1 text-xl leading-none text-white hover:bg-black/70"
        aria-label={zh('关闭封面预览', 'Close cover preview')}
      >
        ×
      </button>
      <img
        src={preview.src}
        alt={preview.alt || zh('JAV 封面', 'JAV cover')}
        className="relative z-10 max-h-[92vh] max-w-[94vw] transform-gpu cursor-zoom-in object-contain shadow-2xl"
        style={{ transform: `scale(${scale})` }}
      />
    </div>
  )
}

function JavCard({
  item,
  onPlay,
  buildJavUrl,
  onIdolClick,
  onStudioClick,
  onTagClick,
  onEditTags,
  onOpenFile,
  openFileLabel,
  onRevealFile,
  onOpenScreenshots,
  loadIdolPreview,
  onOpenCoverPreview,
  javMetadataLanguage,
}) {
  const primaryVideo = useMemo(() => (item?.videos || [])[0], [item])
  const { bgWidthPercent, coverAspectPercent } = useMemo(() => getIdolCardLayoutProps(), [])
  const cover = item?.code ? `/jav/${encodeURIComponent(item.code)}/cover` : null

  const release =
    item?.release_unix && Number.isFinite(item.release_unix)
      ? new Date(item.release_unix * 1000)
      : null
  const releaseText = release ? release.toISOString().slice(0, 10) : zh('未知', 'Unknown')
  const durationText = item?.duration_min
    ? zh(`${item.duration_min} 分钟`, `${item.duration_min} min`)
    : ''
  const studioText = String(item?.studio?.name || '').trim()
  const canFilterStudio = studioText && typeof onStudioClick === 'function'
  const codeText = item?.code?.trim()
  const mainTitle = getJavDisplayTitle(item, javMetadataLanguage)
  const titleText = [codeText, mainTitle].filter(Boolean).join(' ')
  const videos = item?.videos || []
  const openableVideos = videos.filter((video) =>
    Boolean(video?.path && (video?.directory?.path || video?.directory_path))
  )
  const canOpen = openableVideos.length > 0
  const code = item?.code?.trim()
  const encodedCode = code ? encodeURIComponent(code) : ''
  const externalLinks = encodedCode
    ? [
        {
          key: 'javlibrary',
          name: 'JavLibrary',
          href: `https://www.javlibrary.com/cn/vl_searchbyid.php?keyword=${encodedCode}`,
          icon: '/ico/javlibrary.ico',
        },
        {
          key: 'javbus',
          name: 'JavBus',
          href: `https://www.javbus.com/${encodedCode}`,
          icon: '/ico/javbus.ico',
        },
        {
          key: 'missav',
          name: 'MissAV',
          href: `https://missav.ws/ja/${encodedCode}`,
          icon: '/ico/missav.ico',
        },
        {
          key: 'javmost',
          name: 'JavMost',
          href: `https://www.javmost.ws/search/${encodedCode}/`,
          icon: '/ico/javmost.ico',
        },
      ]
    : []

  const handleOpenFile = (event) => {
    event.stopPropagation()
    if (!canOpen) return
    onOpenFile?.(openableVideos[0] || primaryVideo, item)
  }

  const handleRevealFile = (event) => {
    event.stopPropagation()
    if (!canOpen) return
    onRevealFile?.(openableVideos[0] || primaryVideo, item)
  }

  const handleOpenScreenshots = (event) => {
    event.stopPropagation()
    if (!canOpen) return
    onOpenScreenshots?.(openableVideos[0] || primaryVideo, item)
  }

  const handleOpenCoverPreview = (event) => {
    event.stopPropagation()
    if (!cover) return
    onOpenCoverPreview?.({ src: cover, alt: titleText })
  }

  const handleToggleExternalMenu = (event) => {
    event.stopPropagation()
    setExternalAnchorEl((current) => (current ? null : event.currentTarget))
  }

  const closeExternalMenu = () => {
    setExternalAnchorEl(null)
  }

  const canPlay = Boolean(primaryVideo && primaryVideo.id)
  const handlePlay = (event) => {
    event?.stopPropagation()
    if (!canPlay) return
    onPlay?.(primaryVideo, item)
  }
  const handleEditTags = (event) => {
    event?.stopPropagation()
    onEditTags?.(item)
  }
  const tags = Array.isArray(item?.tags) ? item.tags : []
  const showEditTags = typeof onEditTags === 'function'
  const [previewIdol, setPreviewIdol] = useState(null)
  const [hoverAnchorEl, setHoverAnchorEl] = useState(null)
  const [externalAnchorEl, setExternalAnchorEl] = useState(null)
  const closeTimerRef = useRef(null)
  const activeHoverIdRef = useRef(null)
  const externalMenuRef = useRef(null)
  const externalMenuOpen = Boolean(externalAnchorEl)

  const isModifiedClick = (event) =>
    event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0

  const handleFilterLinkClick = (event, action) => {
    event.stopPropagation()
    if (isModifiedClick(event)) return
    event.preventDefault()
    action?.()
  }

  const buildIdolFilterHref = (idol) => {
    const id = Number(idol?.id)
    if (!Number.isFinite(id) || id <= 0) return '#'
    return (
      buildJavUrl?.({
        tab: 'list',
        page: 1,
        search: '',
        idolIds: [id],
        tagIds: [],
        studioId: null,
        studioName: '',
        random: false,
        tempSort: '',
      }) || '#'
    )
  }

  const buildTagFilterHref = (tag) => {
    const id = Number(tag?.id)
    if (!Number.isFinite(id) || id <= 0) return '#'
    return (
      buildJavUrl?.({
        tab: 'list',
        page: 1,
        search: '',
        idolIds: [],
        tagIds: [id],
        studioId: null,
        studioName: '',
        random: false,
        tempSort: '',
      }) || '#'
    )
  }

  useEffect(() => {
    return () => {
      if (closeTimerRef.current) {
        window.clearTimeout(closeTimerRef.current)
      }
    }
  }, [])

  useEffect(() => {
    if (!externalMenuOpen) return undefined

    const handleOutsideClick = (event) => {
      const target = event.target
      if (externalAnchorEl?.contains(target) || externalMenuRef.current?.contains(target)) {
        return
      }
      setExternalAnchorEl(null)
    }

    document.addEventListener('mousedown', handleOutsideClick)
    document.addEventListener('touchstart', handleOutsideClick)
    return () => {
      document.removeEventListener('mousedown', handleOutsideClick)
      document.removeEventListener('touchstart', handleOutsideClick)
    }
  }, [externalAnchorEl, externalMenuOpen])

  const clearHoverCloseTimer = () => {
    if (closeTimerRef.current) {
      window.clearTimeout(closeTimerRef.current)
      closeTimerRef.current = null
    }
  }

  const scheduleHoverClose = () => {
    clearHoverCloseTimer()
    closeTimerRef.current = window.setTimeout(() => {
      activeHoverIdRef.current = null
      setPreviewIdol(null)
      setHoverAnchorEl(null)
      closeTimerRef.current = null
    }, 120)
  }

  const handleIdolHoverStart = (idol, event) => {
    clearHoverCloseTimer()
    const idolId = Number(idol?.id)
    activeHoverIdRef.current = Number.isFinite(idolId) ? idolId : null
    setPreviewIdol(idol || null)
    setHoverAnchorEl(event.currentTarget)

    void loadIdolPreview?.(idol)
      .then((loadedIdol) => {
        if (!loadedIdol) return
        if (activeHoverIdRef.current !== Number(loadedIdol.id)) return
        setPreviewIdol((current) =>
          current && current.id === loadedIdol.id ? { ...current, ...loadedIdol } : current
        )
      })
      .catch((error) => {
        console.warn('load idol preview failed', error)
      })
  }

  const showIdolWorkCount =
    typeof previewIdol?.work_count === 'number' && previewIdol.work_count > 0

  return (
    <div className="flex flex-col overflow-hidden rounded-lg border bg-white shadow-sm transition hover:shadow-lg">
      <div className="group relative aspect-[800/538] overflow-hidden bg-gray-100">
        {cover ? (
          <img src={cover} alt={item?.code} className="h-full w-full object-cover" loading="lazy" />
        ) : (
          <div className="flex h-full w-full items-center justify-center bg-gradient-to-br from-gray-100 to-gray-200 text-lg font-semibold text-gray-600">
            {item?.code || zh('未知番号', 'Unknown code')}
          </div>
        )}
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center bg-black/0 text-white opacity-0 transition-opacity group-hover:opacity-100">
          <button
            onClick={handlePlay}
            disabled={!canPlay}
            className={`pointer-events-auto rounded-full p-3 ${
              canPlay ? 'bg-black/60 hover:bg-black/80' : 'cursor-not-allowed bg-black/30'
            }`}
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
        {cover || canOpen ? (
          <div className="absolute bottom-2 left-2 z-10 flex items-center gap-2 opacity-0 transition-opacity group-hover:opacity-100">
            {cover ? (
              <button
                type="button"
                onClick={handleOpenCoverPreview}
                title={zh('查看封面', 'View cover')}
                aria-label={zh('查看封面', 'View cover')}
                className="flex h-8 w-8 items-center justify-center rounded-full bg-black/70 text-white shadow-lg shadow-black/60 hover:bg-black/85"
              >
                <SearchIcon className="h-5 w-5 text-white" fontSize="inherit" />
              </button>
            ) : null}
            <button
              type="button"
              onClick={handleOpenScreenshots}
              disabled={!canOpen}
              title={zh('查看截图', 'View screenshots')}
              aria-label={zh('查看截图', 'View screenshots')}
              className={`flex h-8 w-8 items-center justify-center rounded-full text-white shadow-lg shadow-black/60 ${
                canOpen ? 'bg-black/70 hover:bg-black/85' : 'cursor-not-allowed bg-black/30'
              }`}
            >
              <PhotoLibraryOutlinedIcon className="h-5 w-5 text-white" fontSize="inherit" />
            </button>
          </div>
        ) : null}
      </div>
      <div className="flex flex-1 flex-col gap-2 p-3">
        <div className="line-clamp-2 text-sm leading-tight" title={titleText}>
          {codeText ? <span className="font-semibold text-gray-800">{codeText}</span> : null}
          {codeText ? ' ' : null}
          <span className="font-medium text-gray-800">{mainTitle}</span>
        </div>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-600">
          <span className="inline-flex items-center gap-1">
            <Tooltip title={zh('时长', 'Duration')} arrow>
              <span className="inline-flex">
                <DurationIcon />
              </span>
            </Tooltip>
            <span>{durationText || zh('时长未知', 'Unknown duration')}</span>
          </span>
          <span className="inline-flex items-center gap-1">
            <Tooltip title={zh('发行日期', 'Release date')} arrow>
              <span className="inline-flex">
                <ReleaseIcon />
              </span>
            </Tooltip>
            <span>{releaseText}</span>
          </span>
          {studioText ? (
            <span className="inline-flex min-w-0 items-center gap-1">
              <Tooltip title={zh('片商', 'Studio')} arrow>
                <span className="inline-flex">
                  <VideocamOutlinedIcon sx={{ fontSize: 16 }} className="shrink-0 text-sky-600" />
                </span>
              </Tooltip>
              <button
                type="button"
                className={`min-w-0 truncate text-left ${
                  canFilterStudio ? 'cursor-pointer hover:text-blue-700 hover:underline' : ''
                }`}
                onClick={() => {
                  if (canFilterStudio) onStudioClick(item.studio)
                }}
                disabled={!canFilterStudio}
              >
                {studioText}
              </button>
            </span>
          ) : null}
        </div>
        {Array.isArray(item?.idols) && item.idols.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {item.idols.map((idol) => (
              <a
                key={idol.id || idol.name}
                href={buildIdolFilterHref(idol)}
                className="rounded-full bg-purple-100 px-2 py-1 text-xs font-medium text-purple-700 transition hover:bg-purple-200"
                onMouseEnter={(event) => handleIdolHoverStart(idol, event)}
                onMouseLeave={scheduleHoverClose}
                onFocus={(event) => handleIdolHoverStart(idol, event)}
                onBlur={scheduleHoverClose}
                onClick={(event) => handleFilterLinkClick(event, () => onIdolClick?.(idol))}
              >
                {idol.name}
              </a>
            ))}
            <Popper
              open={Boolean(previewIdol && hoverAnchorEl)}
              anchorEl={hoverAnchorEl}
              placement="right-start"
              className="z-[1400]"
              modifiers={[
                {
                  name: 'offset',
                  options: {
                    offset: [10, 0],
                  },
                },
              ]}
            >
              <div
                className="w-[220px]"
                onMouseEnter={clearHoverCloseTimer}
                onMouseLeave={scheduleHoverClose}
              >
                {previewIdol ? (
                  <IdolCard
                    item={previewIdol}
                    onSelectIdol={(idol) => onIdolClick?.(idol)}
                    href={buildIdolFilterHref(previewIdol)}
                    bgWidthPercent={bgWidthPercent}
                    coverAspectPercent={coverAspectPercent}
                    showWorkCount={showIdolWorkCount}
                    javMetadataLanguage={javMetadataLanguage}
                  />
                ) : null}
              </div>
            </Popper>
          </div>
        )}
        {tags.length > 0 && (
          <div className="flex flex-wrap items-center gap-1">
            {tags.map((tag) => {
              const isUser = isUserJavTag(tag)
              const tagClass = isUser
                ? 'bg-emerald-500 hover:bg-emerald-600'
                : 'bg-orange-500 hover:bg-orange-600'
              return (
                <a
                  key={tag.id || tag.name}
                  href={buildTagFilterHref(tag)}
                  className={`rounded-full px-2 py-1 text-xs font-medium text-white transition ${tagClass}`}
                  onClick={(event) => handleFilterLinkClick(event, () => onTagClick?.(tag))}
                >
                  {tag.name}
                </a>
              )
            })}
          </div>
        )}
        <div className="flex flex-wrap items-center gap-2">
          {Array.isArray(item?.videos) && item.videos.length > 1 && (
            <span className="text-xs text-gray-500">
              {zh(`共 ${item.videos.length} 个视频`, `${item.videos.length} video files`)}
            </span>
          )}
          {externalLinks.length > 0 && (
            <div className="relative flex items-center">
              <IconButton
                size="small"
                aria-label={zh('外部站点', 'External links')}
                aria-haspopup="menu"
                aria-expanded={externalMenuOpen}
                className="h-6 w-6"
                onClick={handleToggleExternalMenu}
              >
                <OpenInNewIcon fontSize="inherit" />
              </IconButton>
              <Popper
                open={externalMenuOpen}
                anchorEl={externalAnchorEl}
                placement="top-start"
                className="z-[1300]"
                modifiers={[
                  {
                    name: 'offset',
                    options: {
                      offset: [0, 8],
                    },
                  },
                ]}
              >
                <div
                  ref={externalMenuRef}
                  role="menu"
                  className="flex items-center gap-2 rounded-full border border-gray-200 bg-white/95 px-2 py-1 text-xs text-gray-700 shadow-lg backdrop-blur"
                >
                  {externalLinks.map((site) => (
                    <Tooltip
                      key={site.key}
                      title={zh(`在 ${site.name} 中打开`, `Open in ${site.name}`)}
                      placement="top"
                      arrow
                      TransitionComponent={Fade}
                      TransitionProps={{ timeout: 0 }}
                    >
                      <a
                        href={site.href}
                        target="_blank"
                        rel="noopener noreferrer"
                        role="menuitem"
                        className="flex h-7 w-7 items-center justify-center rounded-full bg-gray-100 transition hover:bg-gray-200"
                        aria-label={zh(`在 ${site.name} 中打开`, `Open in ${site.name}`)}
                        onClick={(event) => {
                          event.stopPropagation()
                          closeExternalMenu()
                        }}
                      >
                        <img src={site.icon} alt={site.name} className="h-4 w-4" loading="lazy" />
                      </a>
                    </Tooltip>
                  ))}
                </div>
              </Popper>
            </div>
          )}
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
          {showEditTags && (
            <Tooltip title={zh('编辑标签', 'Edit tags')}>
              <IconButton
                size="small"
                onClick={handleEditTags}
                aria-label={zh('编辑标签', 'Edit tags')}
                className="h-6 w-6"
              >
                <LocalOfferOutlinedIcon fontSize="inherit" />
              </IconButton>
            </Tooltip>
          )}
        </div>
      </div>
    </div>
  )
}
