import { useMemo, useState, useContext, useEffect, useRef } from 'react'
import { getIdolCardLayoutProps } from '@/features/jav/components/JavIdolGrid'
import { JavDetailNavigationContext } from '@/navigation/javDetailNavigation'
import { zh } from '@/utils/i18n'
import { getJavDisplayTitle } from '@/utils/jav'
import { normalizeJavTitleMaxRows } from '@/features/jav/presentation'
import { openJavDBWithAssist } from '@/utils/javdb'
import { updateJavItem } from '@/features/jav/api'
import { useStore } from '@/store'
import { getErrorMessage } from '@/utils/errors'
import { isUserJavTag } from '@/constants/jav'

export default function useJavItemActions({
  item,
  detailView,
  checked = false,
  onToggleSelect,
  selectionDisabled = false,
  onPlay,
  buildJavUrl,
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
  onOpenVideoManager,
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
  loadIdolPreview,
  loadStudioPreview,
  loadSeriesPreview,
  onIdolPreviewUpdated,
  onOpenCoverPreview,
  preferChineseName = false,
  titleMaxRows,
  idolTagMaxRows,
  tagMaxRows,
  hideSeries = false,
  hideIdols = false,
  hideTags = false,
  hideActions = false,
  showFullFavoriteRating = false,
}) {
  const primaryVideo = useMemo(() => (item?.videos || [])[0], [item])
  const { coverAspectPercent } = useMemo(() => getIdolCardLayoutProps(), [])
  const code = item?.code?.trim()
  const [coverVersion, setCoverVersion] = useState(0)
  const [editorOpen, setEditorOpen] = useState(false)
  const [customTagEditorOpen, setCustomTagEditorOpen] = useState(false)
  const openJavDetail = useContext(JavDetailNavigationContext)
  const coverBase = code ? `/jav/${encodeURIComponent(code)}/cover` : null
  const cover = coverBase ? `${coverBase}${coverVersion ? `?v=${coverVersion}` : ''}` : null

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
  const preferredSeries = item?.series
  const seriesText = String(preferredSeries?.name || '').trim()
  const canFilterSeries = seriesText && typeof onSeriesClick === 'function'
  const codeText = code
  const mainTitle = getJavDisplayTitle(item)
  const titleText = [codeText, mainTitle].filter(Boolean).join(' ')
  const normalizedTitleMaxRows = normalizeJavTitleMaxRows(titleMaxRows)
  const titleClampStyle =
    normalizedTitleMaxRows > 0
      ? {
          display: '-webkit-box',
          WebkitBoxOrient: 'vertical',
          WebkitLineClamp: normalizedTitleMaxRows,
          overflow: 'hidden',
        }
      : undefined
  const videos = item?.videos || []
  const openableVideos = videos.filter((video) =>
    Boolean(video?.path && (video?.directory?.path || video?.directory_path))
  )
  const canOpen = openableVideos.length > 0
  const encodedCode = code ? encodeURIComponent(code) : ''
  const javdbSearchURL = encodedCode ? `https://javdb.com/search?q=${encodedCode}&f=all` : ''
  const favoriteCount = Number(item?.favorite_count) || 0
  const itemFavoriteRating = Number(item?.favorite_rating) || 0
  const [favoriteRating, setFavoriteRating] = useState(itemFavoriteRating)
  const [favoriteRatingSaving, setFavoriteRatingSaving] = useState(false)
  const [favoriteRatingError, setFavoriteRatingError] = useState('')
  const [favoriteRatingEditing, setFavoriteRatingEditing] = useState(false)
  const [favoriteRatingPreview, setFavoriteRatingPreview] = useState(null)
  const favoriteRatingTooltipValue = favoriteRatingPreview ?? favoriteRating
  const hasFavoriteRatingTooltipValue = favoriteRatingPreview !== null || favoriteRating > 0
  const favoriteRatingDisplayCount = showFullFavoriteRating
    ? Math.ceil(favoriteRating)
    : favoriteRating > 0
      ? 1
      : 0
  const favoriteRatingWidth = !favoriteRatingEditing
    ? Math.max(favoriteRatingDisplayCount, 1) * 21
    : 5 * 21

  useEffect(() => {
    setFavoriteRating(itemFavoriteRating)
  }, [item?.id, itemFavoriteRating])

  const handleExternalLinkClick = (event, site) => {
    if (site.onClick) {
      site.onClick(event)
      return
    }
    event.stopPropagation()
  }

  const handleOpenJavDB = (event) => {
    event.preventDefault()
    event.stopPropagation()
    openJavDBWithAssist(javdbSearchURL, { target: 'movie', code })
  }

  const externalLinks = encodedCode
    ? item?.is_uncensored === true
      ? [
          {
            key: 'javbus',
            name: 'JavBus',
            href: `https://www.javbus.com/${encodedCode}`,
            icon: '/ico/javbus.ico',
          },
          {
            key: 'javdb',
            name: 'JavDB',
            href: javdbSearchURL,
            icon: '/ico/javdb.png',
            onClick: handleOpenJavDB,
          },
          {
            key: 'avsox',
            name: 'AVSOX',
            href: `/jav/avsox-redirect?code=${encodedCode}`,
            icon: '/ico/avsox.ico',
          },
        ]
      : [
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
            key: 'javdb',
            name: 'JavDB',
            href: javdbSearchURL,
            icon: '/ico/javdb.png',
            onClick: handleOpenJavDB,
          },
          {
            key: 'javmenu',
            name: 'JavMenu',
            href: `https://javmenu.com/${encodedCode}`,
            icon: '/ico/javmenu.png',
          },
          {
            key: 'missav',
            name: 'MissAV',
            href: `https://missav.ws/${encodedCode}`,
            icon: '/ico/missav.ico',
          },
        ]
    : []

  const handleOpenFile = (event) => {
    event.stopPropagation()
    if (!canOpen) return
    onOpenFile?.(openableVideos[0] || primaryVideo, item)
  }

  const handleOpenScreenshots = (event) => {
    event.stopPropagation()
    if (!canOpen) return
    onOpenScreenshots?.(openableVideos[0] || primaryVideo, item)
  }

  const handleOpenVideoManager = (event) => {
    event.stopPropagation()
    onOpenVideoManager?.(item)
  }

  const handleOpenCoverPreview = (event) => {
    event.stopPropagation()
    if (!cover) return
    onOpenCoverPreview?.({ src: cover, alt: titleText })
  }

  const handleOpenDetail = () => {
    clearHoverPreview()
    openJavDetail?.(item)
  }

  const handleOpenEditor = (event) => {
    event.stopPropagation()
    setEditorOpen(true)
  }

  const handleOpenJavFavorites = (event) => {
    event.preventDefault()
    event.stopPropagation()
    onOpenJavFavorites?.(item)
  }

  const handleOpenCustomTags = (event) => {
    event.preventDefault()
    event.stopPropagation()
    setCustomTagEditorOpen(true)
  }

  const handleFavoriteRatingChange = async (event, value) => {
    event?.stopPropagation()
    const javID = Number(item?.id)
    const numericValue = value == null ? 0 : Number(value)
    const nextRating = Math.round(numericValue * 2) / 2
    if (
      favoriteRatingSaving ||
      !Number.isFinite(javID) ||
      javID <= 0 ||
      !Number.isFinite(nextRating) ||
      nextRating < 0 ||
      nextRating > 5
    ) {
      return
    }

    const previousRating = favoriteRating
    setFavoriteRating(nextRating)
    setFavoriteRatingSaving(true)
    setFavoriteRatingError('')
    try {
      const updated = await updateJavItem(javID, { favorite_rating: nextRating })
      detailView?.onItemUpdated?.(updated)
      const savedRating = Number(updated?.favorite_rating) || nextRating
      setFavoriteRating(savedRating)
      useStore.getState().patchJavItem(updated)
    } catch (error) {
      const message = getErrorMessage(error)
      setFavoriteRating(previousRating)
      setFavoriteRatingError(message)
      useStore.getState().setJavError(message)
    } finally {
      setFavoriteRatingSaving(false)
    }
  }

  const handleEditorSaved = (updated, coverUpdated) => {
    detailView?.onItemUpdated?.(updated)
    if (updated?.id) {
      useStore.getState().patchJavItem(updated)
    }
    if (coverUpdated) {
      setCoverVersion(Date.now())
    }
    setEditorOpen(false)
  }

  const handleEditorDeleted = (id, videoIds) => {
    setEditorOpen(false)
    detailView?.onClose?.()
    useStore.getState().removeJavVideos(id, videoIds)
  }

  const handleCustomTagsSaved = (updated) => {
    detailView?.onItemUpdated?.(updated)
    if (updated?.id) {
      useStore.getState().patchJavItem(updated)
    }
    setCustomTagEditorOpen(false)
  }

  const canPlay = Boolean(primaryVideo && primaryVideo.id)
  const handlePlay = (event) => {
    event?.stopPropagation()
    if (!canPlay) return
    onPlay?.(primaryVideo, item)
  }
  const tags = useMemo(() => {
    const rawTags = Array.isArray(item?.tags) ? item.tags : []
    const userTags = rawTags.filter((tag) => isUserJavTag(tag))
    const scrapedTags = rawTags.filter((tag) => !isUserJavTag(tag))
    return [...userTags, ...scrapedTags]
  }, [item?.tags])
  const [previewIdol, setPreviewIdol] = useState(null)
  const [idolHoverAnchorEl, setIdolHoverAnchorEl] = useState(null)
  const [previewStudio, setPreviewStudio] = useState(null)
  const [studioHoverAnchorEl, setStudioHoverAnchorEl] = useState(null)
  const [previewSeries, setPreviewSeries] = useState(null)
  const [seriesHoverAnchorEl, setSeriesHoverAnchorEl] = useState(null)
  const [idolCoverEditorItem, setIdolCoverEditorItem] = useState(null)
  const [idolEditorItem, setIdolEditorItem] = useState(null)
  const closeTimerRef = useRef(null)
  const activeIdolHoverIdRef = useRef(null)
  const activeStudioHoverIdRef = useRef(null)
  const activeSeriesHoverIdRef = useRef(null)

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
        seriesId: null,
        seriesName: '',
        prefix: '',
        favoriteRatingEnabled: false,
        random: false,
        tempSort: '',
      }) || '#'
    )
  }

  const buildStudioFilterHref = (studio) => {
    const id = Number(studio?.id)
    if (!Number.isFinite(id) || id <= 0) return '#'
    return (
      buildJavUrl?.({
        tab: 'list',
        page: 1,
        search: '',
        idolIds: [],
        tagIds: [],
        studioId: id,
        studioName: studio?.name || '',
        seriesId: null,
        seriesName: '',
        prefix: '',
        favoriteRatingEnabled: false,
        random: false,
        tempSort: '',
      }) || '#'
    )
  }

  const buildSeriesFilterHref = (series) => {
    const id = Number(series?.id)
    if (!Number.isFinite(id) || id <= 0) return '#'
    return (
      buildJavUrl?.({
        tab: 'list',
        page: 1,
        search: '',
        idolIds: [],
        tagIds: [],
        studioId: null,
        studioName: '',
        seriesId: id,
        seriesName: series?.name || '',
        prefix: '',
        favoriteRatingEnabled: false,
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
        seriesId: null,
        seriesName: '',
        prefix: '',
        favoriteRatingEnabled: false,
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

  const clearHoverCloseTimer = () => {
    if (closeTimerRef.current) {
      window.clearTimeout(closeTimerRef.current)
      closeTimerRef.current = null
    }
  }

  const clearHoverPreview = () => {
    activeIdolHoverIdRef.current = null
    activeStudioHoverIdRef.current = null
    activeSeriesHoverIdRef.current = null
    setPreviewIdol(null)
    setIdolHoverAnchorEl(null)
    setPreviewStudio(null)
    setStudioHoverAnchorEl(null)
    setPreviewSeries(null)
    setSeriesHoverAnchorEl(null)
  }

  const scheduleHoverClose = () => {
    clearHoverCloseTimer()
    closeTimerRef.current = window.setTimeout(() => {
      clearHoverPreview()
      closeTimerRef.current = null
    }, 120)
  }

  const handleIdolHoverStart = (idol, event) => {
    clearHoverCloseTimer()
    const idolId = Number(idol?.id)
    activeIdolHoverIdRef.current = Number.isFinite(idolId) ? idolId : null
    activeStudioHoverIdRef.current = null
    activeSeriesHoverIdRef.current = null
    setPreviewIdol(idol || null)
    setIdolHoverAnchorEl(event.currentTarget)
    setPreviewStudio(null)
    setStudioHoverAnchorEl(null)
    setPreviewSeries(null)
    setSeriesHoverAnchorEl(null)

    void loadIdolPreview?.(idol)
      .then((loadedIdol) => {
        if (!loadedIdol) return
        if (activeIdolHoverIdRef.current !== Number(loadedIdol.id)) return
        setPreviewIdol((current) =>
          current && current.id === loadedIdol.id ? { ...current, ...loadedIdol } : current
        )
      })
      .catch((error) => {
        console.warn('load idol preview failed', error)
      })
  }

  const handleOpenIdolCoverEditor = (idol) => {
    clearHoverCloseTimer()
    setIdolCoverEditorItem(idol)
  }

  const updateStoredIdol = (updated) => {
    const updatedId = Number(updated?.id)
    if (!Number.isFinite(updatedId) || updatedId <= 0) return
    useStore.getState().patchJavIdol(updated)
  }

  const handleOpenIdolEditor = (idol) => {
    clearHoverCloseTimer()
    setIdolEditorItem(idol)
  }

  const handleIdolSaved = (updated) => {
    const updatedId = Number(updated?.id)
    if (!Number.isFinite(updatedId) || updatedId <= 0) return
    updateStoredIdol(updated)
    onIdolPreviewUpdated?.(updated)
    setPreviewIdol((current) =>
      current && Number(current.id) === updatedId ? { ...current, ...updated } : current
    )
    setIdolEditorItem(null)
  }

  const handleIdolCoverSaved = (updated) => {
    const updatedId = Number(updated?.id)
    if (!Number.isFinite(updatedId) || updatedId <= 0) return
    updateStoredIdol(updated)
    onIdolPreviewUpdated?.(updated)
    setPreviewIdol((current) =>
      current && Number(current.id) === updatedId ? { ...current, ...updated } : current
    )
  }

  const handleStudioHoverStart = (studio, event) => {
    clearHoverCloseTimer()
    const studioId = Number(studio?.id)
    activeStudioHoverIdRef.current = Number.isFinite(studioId) ? studioId : null
    activeIdolHoverIdRef.current = null
    activeSeriesHoverIdRef.current = null
    setPreviewStudio(studio || null)
    setStudioHoverAnchorEl(event.currentTarget)
    setPreviewIdol(null)
    setIdolHoverAnchorEl(null)
    setPreviewSeries(null)
    setSeriesHoverAnchorEl(null)

    void loadStudioPreview?.(studio)
      .then((loadedStudio) => {
        if (!loadedStudio) return
        if (activeStudioHoverIdRef.current !== Number(loadedStudio.id)) return
        setPreviewStudio((current) =>
          current && current.id === loadedStudio.id ? { ...current, ...loadedStudio } : current
        )
      })
      .catch((error) => {
        console.warn('load studio preview failed', error)
      })
  }

  const handleSeriesHoverStart = (series, event) => {
    clearHoverCloseTimer()
    const seriesId = Number(series?.id)
    activeSeriesHoverIdRef.current = Number.isFinite(seriesId) ? seriesId : null
    activeIdolHoverIdRef.current = null
    activeStudioHoverIdRef.current = null
    setPreviewSeries(series || null)
    setSeriesHoverAnchorEl(event.currentTarget)
    setPreviewIdol(null)
    setIdolHoverAnchorEl(null)
    setPreviewStudio(null)
    setStudioHoverAnchorEl(null)

    void loadSeriesPreview?.(series)
      .then((loadedSeries) => {
        if (!loadedSeries) return
        if (activeSeriesHoverIdRef.current !== Number(loadedSeries.id)) return
        setPreviewSeries((current) =>
          current && current.id === loadedSeries.id ? { ...current, ...loadedSeries } : current
        )
      })
      .catch((error) => {
        console.warn('load series preview failed', error)
      })
  }

  const showIdolWorkCount =
    typeof previewIdol?.work_count === 'number' && previewIdol.work_count > 0

  return {
    checked,
    cover,
    item,
    handleOpenDetail,
    code,
    handlePlay,
    canPlay,
    setFavoriteRatingEditing,
    setFavoriteRatingPreview,
    favoriteRatingError,
    favoriteRatingPreview,
    hasFavoriteRatingTooltipValue,
    favoriteRatingTooltipValue,
    favoriteRatingSaving,
    favoriteRating,
    favoriteRatingWidth,
    handleFavoriteRatingChange,
    favoriteRatingEditing,
    onToggleSelect,

    selectionDisabled,
    externalLinks,
    handleExternalLinkClick,
    handleOpenCustomTags,
    favoriteCount,
    handleOpenJavFavorites,
    canOpen,
    handleOpenCoverPreview,
    handleOpenScreenshots,
    titleText,
    titleClampStyle,
    codeText,
    mainTitle,
    releaseText,
    durationText,
    studioText,
    buildStudioFilterHref,
    canFilterStudio,
    handleFilterLinkClick,
    onStudioClick,
    handleStudioHoverStart,
    scheduleHoverClose,
    hideSeries,
    seriesText,
    buildSeriesFilterHref,
    preferredSeries,
    canFilterSeries,
    onSeriesClick,
    handleSeriesHoverStart,

    previewStudio,
    studioHoverAnchorEl,
    clearHoverCloseTimer,
    onPrefixClick,
    onOpenStudioFavorites,
    onOpenSeriesFavorites,
    previewSeries,
    seriesHoverAnchorEl,
    hideIdols,

    idolTagMaxRows,
    preferChineseName,
    buildIdolFilterHref,
    onIdolClick,
    handleIdolHoverStart,
    previewIdol,
    idolHoverAnchorEl,
    onOpenFavorites,
    handleOpenIdolCoverEditor,
    handleOpenIdolEditor,
    coverAspectPercent,
    showIdolWorkCount,
    hideTags,
    tags,
    tagMaxRows,
    buildTagFilterHref,
    onTagClick,
    hideActions,
    openFileLabel,
    handleOpenFile,
    handleOpenEditor,
    handleOpenVideoManager,
    detailView,
    onOpenJavFavorites,
    setEditorOpen,
    loadIdolPreview,
    loadStudioPreview,
    loadSeriesPreview,
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
    idolCoverEditorItem,
    setIdolCoverEditorItem,
    handleIdolCoverSaved,
    idolEditorItem,
    setIdolEditorItem,
    handleIdolSaved,
    setPreviewIdol,
    editorOpen,
    handleEditorSaved,
    handleEditorDeleted,
    customTagEditorOpen,
    setCustomTagEditorOpen,
    handleCustomTagsSaved,
  }
}
