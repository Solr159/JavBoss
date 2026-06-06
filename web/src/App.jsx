import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  buildUrlFromState,
  generateRandomSeed,
  normalizeUrlStateFromStore,
  parseUrlState,
} from '@/utils/urlState'
import {
  addTagToVideos,
  removeTagFromVideos,
  replaceTagsForVideos,
  renameVideoLocation,
  deleteVideoLocation,
  updateConfig,
  playVideoFile,
  openVideoFile,
  revealVideoLocation,
  createJavTag,
  renameJavTag,
  deleteJavTag,
  replaceJavTagsForItems,
  resolveJavIdols,
  createJavIdolFavoriteGroup,
  deleteJavIdolFavoriteGroup,
  fetchJavIdolFavoriteGroupIdols,
  fetchJavIdolFavoriteSelection,
  renameJavIdolFavoriteGroup,
  removeJavIdolFavoriteGroupIdols,
  reorderJavIdolFavoriteGroupIdols,
  reorderJavIdolFavoriteGroups,
  replaceJavIdolFavoriteGroups,
} from '@/api'
import GlobalSettingsModal from '@/components/GlobalSettingsModal'
import JavIdolFavoriteManageModal from '@/components/JavIdolFavoriteManageModal'
import JavIdolView from '@/components/JavIdolView'
import JavIdolFavoriteModal from '@/components/JavIdolFavoriteModal'
import JavQueryEditorModal from '@/components/JavQueryEditorModal'
import JavSettingsModal from '@/components/JavSettingsModal'
import JavSeriesView from '@/components/JavSeriesView'
import JavStudioView from '@/components/JavStudioView'
import JavTagModal from '@/components/JavTagModal'
import JavVideoPickerModal from '@/components/JavVideoPickerModal'
import JavView from '@/components/JavView'
import SelectionOpsModal from '@/components/SelectionOpsModal'
import SelectionTagsModal from '@/components/SelectionTagsModal'
import TagPickerModal from '@/components/TagPickerModal'
import Toast from '@/components/Toast'
import TopBar from '@/components/TopBar'
import VideoSettingsModal from '@/components/VideoSettingsModal'
import VideoScreenshotsModal from '@/components/VideoScreenshotsModal'
import VideoTagModal from '@/components/VideoTagModal'
import VideoView from '@/components/VideoView'
import { isUserJavTag, normalizeIdolSort, normalizeJavSort } from '@/constants/jav'
import { normalizeVideoSort } from '@/constants/video'
import { isChineseLocale, zh } from '@/utils/i18n'
import { directoryQueryIds, useStore, videoSelectionKey } from '@/store'

const JAV_STUDIO_PAGE_SIZE = 24
const HISTORY_INDEX_KEY = '__pornbossHistoryIndex'
const HISTORY_SCROLL_KEY = '__pornbossScroll'
const SCROLL_RESTORE_MAX_ATTEMPTS = 30

const normalizeDefaultPlayer = (value) =>
  String(value || '')
    .trim()
    .toLowerCase() === 'system'
    ? 'system'
    : 'mpv'

const normalizeInitialViewMode = (value) =>
  String(value || '')
    .trim()
    .toLowerCase() === 'jav'
    ? 'jav'
    : 'video'

export default function App() {
  const isPoppingRef = useRef(false)
  const lastUrlRef = useRef(window.location.pathname + window.location.search)
  const browserInitialCanGoBackRef = useRef(window.history.length > 1)
  const browserHistoryIndexRef = useRef(0)
  const browserHistoryMaxRef = useRef(0)
  const pendingScrollRestoreRef = useRef(null)
  const scrollSaveFrameRef = useRef(null)
  const scrollRestoreFrameRef = useRef(null)
  const scrollRestoreTimerRef = useRef(null)
  const preNavigationScrollSaveUrlRef = useRef(null)
  const pendingVideoTagIdsRef = useRef(null)
  const {
    page,
    pageSize,
    setPage,
    videos,
    config,
    tags,
    selectedTags,
    selectedVideoIds,
    selectedVideoMeta,
    loadVideos,
    loadMoreVideos,
    loadTags,
    toggleTagFilter,
    createTag,
    deleteTag,
    renameTag,
    toggleSelectVideo,
    loading,
    videoLoadingMore,
    error,
    hasNext,
    total,
    setSelectedTags,
    clearSelection,
    searchTerm,
    setSearchTerm,
    sortOrder,
    videoTempSort,
    videoHideJav,
    setVideoTempSort,
    loadJavRandom,
    randomMode,
    randomSeed,
    viewMode,
    setViewMode,
    javTab,
    javPage,
    setJavPage,
    javPageSize,
    javGridColumns,
    javTitleMaxRows,
    javIdolTagMaxRows,
    javTagMaxRows,
    javSearchTerm,
    javIdolIds,
    javTags,
    javStudioId,
    javStudioName,
    javSeriesId,
    javSeriesName,
    javSort,
    javTempSort,
    javRandomMode,
    javRandomSeed,
    idolSort,
    setJavTempSort,
    loadJavs,
    loadMoreJavs,
    javItems,
    javTotal,
    javLoading,
    javLoadingMore,
    javError,
    javTagOptions,
    loadJavTags,
    loadConfig,
    idolPage,
    setIdolPage,
    idolPageSize,
    idolFavoriteGroupId,
    setIdolFavoriteGroupId,
    idolItems,
    idolTotal,
    idolLoading,
    idolLoadingMore,
    idolError,
    idolFavoriteGroups,
    idolFavoriteGroupsLoading,
    idolFavoriteGroupsError,
    loadJavIdols,
    loadMoreJavIdols,
    loadJavIdolFavoriteGroups,
    studioPage,
    setStudioPage,
    studioItems,
    studioTotal,
    studioLoading,
    studioLoadingMore,
    studioError,
    loadJavStudios,
    loadMoreJavStudios,
    seriesPage,
    setSeriesPage,
    seriesItems,
    seriesTotal,
    seriesLoading,
    seriesLoadingMore,
    seriesError,
    loadJavSeries,
    loadMoreJavSeries,
    directories,
    loadDirectories,
    createDirectory,
    updateDirectory,
    deleteDirectory,
    enabledDirectoryIds,
    setEnabledDirectoryIds,
    directoryFilterMode,
  } = useStore()

  const [tagModalOpen, setTagModalOpen] = useState(false)
  const [videoSettingsOpen, setVideoSettingsOpen] = useState(false)
  const [javSettingsOpen, setJavSettingsOpen] = useState(false)
  const [globalSettingsOpen, setGlobalSettingsOpen] = useState(false)
  const [javTagModalOpen, setJavTagModalOpen] = useState(false)
  const [javQueryEditorOpen, setJavQueryEditorOpen] = useState(false)
  const [javVideoPickerOpen, setJavVideoPickerOpen] = useState(false)
  const [javVideoPickerItem, setJavVideoPickerItem] = useState(null)
  const [javVideoPickerAction, setJavVideoPickerAction] = useState('play')
  const [idolFavoriteModalOpen, setIdolFavoriteModalOpen] = useState(false)
  const [idolFavoriteModalItem, setIdolFavoriteModalItem] = useState(null)
  const [idolFavoriteSelectedIds, setIdolFavoriteSelectedIds] = useState([])
  const [idolFavoriteModalLoading, setIdolFavoriteModalLoading] = useState(false)
  const [idolFavoriteModalSaving, setIdolFavoriteModalSaving] = useState(false)
  const [idolFavoriteModalError, setIdolFavoriteModalError] = useState('')
  const [idolFavoriteManageOpen, setIdolFavoriteManageOpen] = useState(false)
  const [locationPickerOpen, setLocationPickerOpen] = useState(false)
  const [locationPickerVideo, setLocationPickerVideo] = useState(null)
  const [locationPickerChoices, setLocationPickerChoices] = useState([])
  const [locationPickerAction, setLocationPickerAction] = useState('play')
  const [screenshotsVideo, setScreenshotsVideo] = useState(null)
  const [searchInput, setSearchInput] = useState('')
  const [javSearchInput, setJavSearchInput] = useState('')
  const [waterfallModes, setWaterfallModes] = useState({
    video: false,
    jav: false,
    idol: false,
    studio: false,
    series: false,
  })
  const [hydrated, setHydrated] = useState(false)
  const [configLoaded, setConfigLoaded] = useState(false)
  const [browserNavigation, setBrowserNavigation] = useState({
    canGoBack: window.history.length > 1,
    canGoForward: false,
  })
  const isJavMode = viewMode === 'jav'
  const isModifiedClick = (e) =>
    e && (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0)

  const setBrowserNavigationFromIndex = useCallback((index, max) => {
    browserHistoryIndexRef.current = index
    browserHistoryMaxRef.current = max
    setBrowserNavigation({
      canGoBack: index > 0 || (index === 0 && browserInitialCanGoBackRef.current),
      canGoForward: index < max,
    })
  }, [])

  const readBrowserHistoryIndex = useCallback((state = window.history.state) => {
    const rawIndex = Number(state?.[HISTORY_INDEX_KEY])
    return Number.isFinite(rawIndex) && rawIndex >= 0 ? Math.floor(rawIndex) : 0
  }, [])

  const readWindowScrollPosition = useCallback(
    () => ({
      x: Math.max(0, Math.round(window.scrollX || window.pageXOffset || 0)),
      y: Math.max(0, Math.round(window.scrollY || window.pageYOffset || 0)),
    }),
    []
  )

  const normalizeHistoryScrollPosition = useCallback((value) => {
    const x = Number(value?.x)
    const y = Number(value?.y)
    return {
      x: Number.isFinite(x) && x > 0 ? Math.round(x) : 0,
      y: Number.isFinite(y) && y > 0 ? Math.round(y) : 0,
    }
  }, [])

  const saveCurrentScrollPosition = useCallback(() => {
    const currentState = window.history.state || {}
    const currentScroll = readWindowScrollPosition()
    const previousScroll = normalizeHistoryScrollPosition(currentState[HISTORY_SCROLL_KEY])
    if (previousScroll.x === currentScroll.x && previousScroll.y === currentScroll.y) return
    window.history.replaceState(
      { ...currentState, [HISTORY_SCROLL_KEY]: currentScroll },
      '',
      window.location.pathname + window.location.search
    )
  }, [normalizeHistoryScrollPosition, readWindowScrollPosition])

  const saveScrollBeforeUrlStateChange = useCallback(() => {
    if (scrollSaveFrameRef.current) {
      window.cancelAnimationFrame(scrollSaveFrameRef.current)
      scrollSaveFrameRef.current = null
    }
    saveCurrentScrollPosition()
    preNavigationScrollSaveUrlRef.current = window.location.pathname + window.location.search
  }, [saveCurrentScrollPosition])

  const ensureBrowserHistoryState = useCallback(() => {
    const currentState = window.history.state || {}
    const hasIndex = Number.isFinite(Number(currentState[HISTORY_INDEX_KEY]))
    const hasScroll =
      currentState[HISTORY_SCROLL_KEY] && typeof currentState[HISTORY_SCROLL_KEY] === 'object'
    const index = hasIndex ? readBrowserHistoryIndex(currentState) : browserHistoryIndexRef.current
    if (!hasIndex || !hasScroll) {
      window.history.replaceState(
        {
          ...currentState,
          [HISTORY_INDEX_KEY]: index,
          [HISTORY_SCROLL_KEY]: hasScroll
            ? normalizeHistoryScrollPosition(currentState[HISTORY_SCROLL_KEY])
            : readWindowScrollPosition(),
        },
        '',
        window.location.pathname + window.location.search
      )
    }
    setBrowserNavigationFromIndex(index, Math.max(browserHistoryMaxRef.current, index))
  }, [
    normalizeHistoryScrollPosition,
    readBrowserHistoryIndex,
    readWindowScrollPosition,
    setBrowserNavigationFromIndex,
  ])

  const handleBrowserBack = useCallback(() => {
    saveCurrentScrollPosition()
    window.history.back()
  }, [saveCurrentScrollPosition])

  const handleBrowserForward = useCallback(() => {
    saveCurrentScrollPosition()
    window.history.forward()
  }, [saveCurrentScrollPosition])

  useEffect(() => {
    if (!('scrollRestoration' in window.history)) return undefined
    const previousScrollRestoration = window.history.scrollRestoration
    window.history.scrollRestoration = 'manual'
    return () => {
      window.history.scrollRestoration = previousScrollRestoration
    }
  }, [])

  useEffect(() => {
    const flushScrollPosition = () => {
      if (scrollSaveFrameRef.current) {
        window.cancelAnimationFrame(scrollSaveFrameRef.current)
        scrollSaveFrameRef.current = null
      }
      saveCurrentScrollPosition()
    }
    const handleScroll = () => {
      if (scrollSaveFrameRef.current) return
      scrollSaveFrameRef.current = window.requestAnimationFrame(() => {
        scrollSaveFrameRef.current = null
        saveCurrentScrollPosition()
      })
    }
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'hidden') {
        flushScrollPosition()
      }
    }

    window.addEventListener('scroll', handleScroll, { passive: true })
    window.addEventListener('pagehide', flushScrollPosition)
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => {
      window.removeEventListener('scroll', handleScroll)
      window.removeEventListener('pagehide', flushScrollPosition)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      if (scrollSaveFrameRef.current) {
        window.cancelAnimationFrame(scrollSaveFrameRef.current)
        scrollSaveFrameRef.current = null
      }
    }
  }, [saveCurrentScrollPosition])

  const cancelScheduledScrollRestore = useCallback(() => {
    if (scrollRestoreFrameRef.current) {
      window.cancelAnimationFrame(scrollRestoreFrameRef.current)
      scrollRestoreFrameRef.current = null
    }
    if (scrollRestoreTimerRef.current) {
      window.clearTimeout(scrollRestoreTimerRef.current)
      scrollRestoreTimerRef.current = null
    }
  }, [])

  const schedulePendingScrollRestore = useCallback(() => {
    if (!pendingScrollRestoreRef.current) return
    cancelScheduledScrollRestore()

    const restore = () => {
      const pending = pendingScrollRestoreRef.current
      if (!pending) return

      const maxY = Math.max(
        0,
        Math.max(document.documentElement.scrollHeight, document.body.scrollHeight) -
          window.innerHeight
      )
      const targetX = Math.max(0, pending.x || 0)
      const targetY = Math.max(0, pending.y || 0)
      const nextY = Math.min(targetY, maxY)
      window.scrollTo({ left: targetX, top: nextY, behavior: 'auto' })

      const reached = Math.abs((window.scrollY || window.pageYOffset || 0) - targetY) <= 2
      const canReach = targetY <= maxY + 2
      if ((canReach && reached) || pending.attempts >= SCROLL_RESTORE_MAX_ATTEMPTS) {
        pendingScrollRestoreRef.current = null
        return
      }

      pending.attempts += 1
      scrollRestoreTimerRef.current = window.setTimeout(() => {
        scrollRestoreTimerRef.current = null
        scrollRestoreFrameRef.current = window.requestAnimationFrame(restore)
      }, 50)
    }

    scrollRestoreFrameRef.current = window.requestAnimationFrame(restore)
  }, [cancelScheduledScrollRestore])

  useEffect(() => cancelScheduledScrollRestore, [cancelScheduledScrollRestore])
  const selectedTagIds = useMemo(
    () =>
      tags
        .filter((t) => selectedTags.includes(t.name))
        .map((t) => t.id)
        .filter((id) => id > 0),
    [tags, selectedTags]
  )
  const tagsByName = useMemo(() => new Map(tags.map((t) => [t.name, t.id])), [tags])
  const directoryTagKey = useMemo(
    () =>
      directoryQueryIds({
        directories,
        enabledDirectoryIds,
        directoryFilterMode,
      }).join(','),
    [directories, enabledDirectoryIds, directoryFilterMode]
  )
  const javQueryDirectoryIds = useMemo(
    () =>
      directoryQueryIds({
        directories,
        enabledDirectoryIds,
        directoryFilterMode,
      }),
    [directories, enabledDirectoryIds, directoryFilterMode]
  )
  const [tagPickerFor, setTagPickerFor] = useState(null)
  const [tagPickerSelected, setTagPickerSelected] = useState([])
  const [javTagPickerItem, setJavTagPickerItem] = useState(null)
  const [javTagPickerSelected, setJavTagPickerSelected] = useState([])
  const [selectionOpsOpen, setSelectionOpsOpen] = useState(false)
  const [selectionTagsOpen, setSelectionTagsOpen] = useState(false)
  const [selectionTagAction, setSelectionTagAction] = useState('add')
  const [selectionTagChoices, setSelectionTagChoices] = useState([])
  const [videoPageSizeInput, setVideoPageSizeInput] = useState(pageSize)
  const [videoSortInput, setVideoSortInput] = useState(sortOrder)
  const [videoHideJavInput, setVideoHideJavInput] = useState(videoHideJav)
  const [javPageSizeInput, setJavPageSizeInput] = useState(javPageSize)
  const [javGridColumnsInput, setJavGridColumnsInput] = useState(javGridColumns)
  const [javTitleMaxRowsInput, setJavTitleMaxRowsInput] = useState(javTitleMaxRows)
  const [javIdolTagMaxRowsInput, setJavIdolTagMaxRowsInput] = useState(javIdolTagMaxRows)
  const [javTagMaxRowsInput, setJavTagMaxRowsInput] = useState(javTagMaxRows)
  const [idolPageSizeInput, setIdolPageSizeInput] = useState(idolPageSize)
  const [javSortInput, setJavSortInput] = useState(javSort)
  const [idolSortInput, setIdolSortInput] = useState(idolSort)
  const [javResolvedIdols, setJavResolvedIdols] = useState({})
  const [toastMessage, setToastMessage] = useState('')
  const javVideoChoices = javVideoPickerItem?.videos || []
  const locationPickerItem = locationPickerVideo
    ? {
        code:
          locationPickerVideo.filename ||
          locationPickerVideo.path ||
          zh('选择文件位置', 'Choose file location'),
        title: zh('选择文件位置', 'Choose file location'),
      }
    : null
  const defaultPlayer = normalizeDefaultPlayer(config?.default_player)
  const initialViewMode = normalizeInitialViewMode(config?.initial_view_mode)
  const alternatePlayer = defaultPlayer === 'system' ? 'mpv' : 'system'
  const alternatePlayerLabel =
    alternatePlayer === 'mpv'
      ? zh('使用MPV播放器播放', 'Play with MPV player')
      : zh('用默认程序打开', 'Open with default app')
  const showToast = useCallback((message) => {
    setToastMessage(String(message || '').trim())
  }, [])
  const closeToast = useCallback(() => {
    setToastMessage('')
  }, [])
  const handleOpenTagModal = useCallback(() => {
    loadTags()
    setTagModalOpen(true)
  }, [loadTags])

  const mapTagIdsToNames = useCallback(
    (ids) => {
      if (!Array.isArray(ids) || ids.length === 0) return []
      const idSet = new Set(ids)
      return tags.filter((t) => idSet.has(t.id)).map((t) => t.name)
    },
    [tags]
  )

  const buildVideoFullPath = useCallback((video) => {
    if (!video) return ''
    const rawPath = String(video.path || '').trim()
    const dirPath = String(video.directory?.path || video.directory_path || '').trim()
    if (!dirPath) return rawPath
    if (!rawPath) return dirPath
    const isAbs = rawPath.startsWith('/') || /^[A-Za-z]:[\\/]/.test(rawPath)
    if (isAbs) return rawPath
    const separator = dirPath.includes('\\') ? '\\' : '/'
    const cleanedDir = dirPath.replace(/[\\/]+$/, '')
    const cleanedRel = rawPath.replace(/^[\\/]+/, '')
    return `${cleanedDir}${separator}${cleanedRel}`
  }, [])

  const getVideoDirPath = useCallback(
    (video) => String(video?.directory?.path || video?.directory_path || '').trim(),
    []
  )

  const getVideoRelPath = useCallback((video) => String(video?.path || '').trim(), [])

  const isVideoOpenable = useCallback(
    (video) => Boolean(getVideoDirPath(video) && getVideoRelPath(video)),
    [getVideoDirPath, getVideoRelPath]
  )

  const getVideoLocationChoices = useCallback(
    (video) => {
      const locations = Array.isArray(video?.locations) ? video.locations : []
      const choices = locations
        .map((location) => {
          const relPath = String(location?.relative_path || '').trim()
          const directory = location?.directory || location?.directory_ref || null
          const dirPath = String(directory?.path || location?.directory_path || '').trim()
          if (!relPath || !dirPath) return null
          return {
            ...video,
            id: video.id,
            location_id: location.id,
            path: relPath,
            directory,
            directory_path: dirPath,
            filename: location?.filename || relPath.split(/[\\/]/).pop() || video.filename,
          }
        })
        .filter(Boolean)
        .filter(isVideoOpenable)
      if (choices.length > 0) return choices
      return isVideoOpenable(video) ? [video] : []
    },
    [isVideoOpenable]
  )

  const openLocationPicker = useCallback((video, action, choices) => {
    setLocationPickerVideo(video)
    setLocationPickerAction(action)
    setLocationPickerChoices(Array.isArray(choices) ? choices : [])
    setLocationPickerOpen(true)
  }, [])

  const closeLocationPicker = useCallback(() => {
    setLocationPickerOpen(false)
    setLocationPickerVideo(null)
    setLocationPickerChoices([])
    setLocationPickerAction('play')
  }, [])

  const playVideoWith = useCallback(
    (video, player) => {
      if (!video || !isVideoOpenable(video)) return
      const payload = {
        id: video.id,
        path: getVideoRelPath(video),
        dirPath: getVideoDirPath(video),
      }
      const useSystemPlayer = player === 'system'
      const action = useSystemPlayer ? openVideoFile : playVideoFile
      action(payload).catch((err) =>
        console.error(
          useSystemPlayer
            ? zh('打开文件失败', 'Failed to open file')
            : zh('播放文件失败', 'Failed to play file'),
          err
        )
      )
    },
    [getVideoDirPath, getVideoRelPath, isVideoOpenable]
  )

  const revealVideoFile = useCallback(
    (video) => {
      if (!video || !isVideoOpenable(video)) return Promise.resolve()
      return revealVideoLocation({
        path: getVideoRelPath(video),
        dirPath: getVideoDirPath(video),
      })
    },
    [getVideoDirPath, getVideoRelPath, isVideoOpenable]
  )

  const playVideoFromTime = useCallback(
    (video, startTime) => {
      if (!video || !isVideoOpenable(video)) return
      playVideoFile({
        id: video.id,
        path: getVideoRelPath(video),
        dirPath: getVideoDirPath(video),
        startTime,
      }).catch((err) => console.error(zh('播放文件失败', 'Failed to play file'), err))
    },
    [getVideoDirPath, getVideoRelPath, isVideoOpenable]
  )

  const handleOpenPlayer = useCallback(
    (video) => {
      const choices = getVideoLocationChoices(video)
      if (choices.length > 1) {
        openLocationPicker(video, 'play', choices)
        return
      }
      playVideoWith(choices[0] || video, defaultPlayer)
    },
    [defaultPlayer, getVideoLocationChoices, openLocationPicker, playVideoWith]
  )

  const handleOpenAlternatePlayer = useCallback(
    (video) => {
      const choices = getVideoLocationChoices(video)
      if (choices.length > 1) {
        openLocationPicker(video, 'open', choices)
        return
      }
      playVideoWith(choices[0] || video, alternatePlayer)
    },
    [alternatePlayer, getVideoLocationChoices, openLocationPicker, playVideoWith]
  )

  const handleRevealVideoFile = useCallback(
    (video) => {
      const choices = getVideoLocationChoices(video)
      if (choices.length > 1) {
        openLocationPicker(video, 'reveal', choices)
        return
      }
      revealVideoFile(choices[0] || video).catch((err) =>
        console.error(zh('打开所在位置失败', 'Failed to reveal file'), err)
      )
    },
    [getVideoLocationChoices, openLocationPicker, revealVideoFile]
  )

  const handleRenameVideo = useCallback(
    async (video) => {
      const locationId = Number(video?.location_id)
      if (!video?.id || !Number.isFinite(locationId) || locationId <= 0) {
        showToast(zh('无法重命名：缺少文件位置', 'Cannot rename: missing file location'))
        return
      }
      const currentName =
        String(video?.filename || '')
          .trim()
          .split(/[\\/]/)
          .pop() ||
        String(video?.path || '')
          .split(/[\\/]/)
          .pop()
      const nextName = window.prompt(zh('重命名视频文件', 'Rename video file'), currentName)
      if (nextName == null) return
      const filename = nextName.trim()
      if (!filename || filename === currentName) return
      try {
        const updated = await renameVideoLocation(video.id, locationId, filename)
        useStore.setState((state) => {
          const targetKey = videoSelectionKey(video)
          const nextVideos = Array.isArray(state.videos)
            ? state.videos.map((item) =>
                videoSelectionKey(item) === targetKey ? { ...item, ...updated } : item
              )
            : state.videos
          const nextMeta = { ...(state.selectedVideoMeta || {}) }
          if (targetKey && nextMeta[targetKey]) {
            nextMeta[targetKey] = {
              ...nextMeta[targetKey],
              label: updated.filename || updated.path || nextMeta[targetKey].label,
            }
          }
          return { videos: nextVideos, selectedVideoMeta: nextMeta }
        })
        await loadVideos({ force: true })
      } catch (err) {
        console.error(zh('重命名视频失败', 'Failed to rename video'), err)
        showToast(err?.message || zh('重命名视频失败', 'Failed to rename video'))
      }
    },
    [loadVideos, showToast]
  )

  const handleDeleteVideo = useCallback(
    async (video) => {
      const locationId = Number(video?.location_id)
      if (!video?.id || !Number.isFinite(locationId) || locationId <= 0) {
        showToast(zh('无法删除：缺少文件位置', 'Cannot delete: missing file location'))
        return
      }
      const label = String(video?.filename || video?.path || `#${video.id}`)
      if (!window.confirm(zh(`确定删除视频文件“${label}”吗？`, `Delete video file "${label}"?`))) {
        return
      }
      try {
        await deleteVideoLocation(video.id, locationId)
        const targetKey = videoSelectionKey(video)
        useStore.setState((state) => {
          const nextIds = new Set(state.selectedVideoIds || [])
          const nextMeta = { ...(state.selectedVideoMeta || {}) }
          if (targetKey) {
            nextIds.delete(targetKey)
            delete nextMeta[targetKey]
          }
          const nextVideos = Array.isArray(state.videos)
            ? state.videos.filter((item) => videoSelectionKey(item) !== targetKey)
            : state.videos
          return {
            videos: nextVideos,
            selectedVideoIds: nextIds,
            selectedVideoMeta: nextMeta,
            total: Math.max(0, Number(state.total || 0) - 1),
          }
        })
        await loadVideos({ force: true })
      } catch (err) {
        console.error(zh('删除视频失败', 'Failed to delete video'), err)
        showToast(err?.message || zh('删除视频失败', 'Failed to delete video'))
      }
    },
    [loadVideos, showToast]
  )

  const closeJavVideoPicker = useCallback(() => {
    setJavVideoPickerOpen(false)
    setJavVideoPickerItem(null)
    setJavVideoPickerAction('play')
  }, [])

  const handleVideoTagClick = useCallback(
    (name) => {
      if (!name) return
      saveScrollBeforeUrlStateChange()
      setSearchTerm('', { resetPage: false, triggerLoad: false })
      setSelectedTags([name])
    },
    [saveScrollBeforeUrlStateChange, setSearchTerm, setSelectedTags]
  )

  const handleJavPlay = useCallback(
    (video, item) => {
      const videos = item?.videos || []
      if (videos.length > 1) {
        setJavVideoPickerAction('play')
        setJavVideoPickerItem(item)
        setJavVideoPickerOpen(true)
        return
      }
      const target = video || videos[0]
      if (target) {
        handleOpenPlayer(target)
      }
    },
    [handleOpenPlayer]
  )

  const handleJavOpenFile = useCallback(
    (video, item) => {
      const videos = item?.videos || (video ? [video] : [])
      if (videos.length > 1) {
        setJavVideoPickerAction('open')
        setJavVideoPickerItem(item)
        setJavVideoPickerOpen(true)
        return
      }
      const target = video && isVideoOpenable(video) ? video : videos.find(isVideoOpenable)
      if (!target) return
      handleOpenAlternatePlayer(target)
    },
    [handleOpenAlternatePlayer, isVideoOpenable]
  )

  const handleJavRevealFile = useCallback(
    (video, item) => {
      const videos = item?.videos || (video ? [video] : [])
      if (videos.length > 1) {
        setJavVideoPickerAction('reveal')
        setJavVideoPickerItem(item)
        setJavVideoPickerOpen(true)
        return
      }
      const target = video && isVideoOpenable(video) ? video : videos.find(isVideoOpenable)
      if (!target) return
      handleRevealVideoFile(target)
    },
    [handleRevealVideoFile, isVideoOpenable]
  )

  const handleJavOpenScreenshots = useCallback(
    (video, item) => {
      const videos = item?.videos || (video ? [video] : [])
      if (videos.length > 1) {
        setJavVideoPickerAction('screenshots')
        setJavVideoPickerItem(item)
        setJavVideoPickerOpen(true)
        return
      }
      const target = video && isVideoOpenable(video) ? video : videos.find(isVideoOpenable)
      if (!target) return
      setScreenshotsVideo(target)
    },
    [isVideoOpenable]
  )

  const handleSelectJavVideo = useCallback(
    async (video) => {
      if (!video) return
      if (javVideoPickerAction === 'play') {
        handleOpenPlayer(video)
        closeJavVideoPicker()
        return
      }
      if (javVideoPickerAction === 'open') {
        handleOpenAlternatePlayer(video)
        closeJavVideoPicker()
        return
      }
      if (javVideoPickerAction === 'screenshots') {
        if (isVideoOpenable(video)) {
          setScreenshotsVideo(video)
          closeJavVideoPicker()
        }
        return
      }
      try {
        if (javVideoPickerAction === 'reveal') {
          handleRevealVideoFile(video)
        }
      } catch (err) {
        console.error(
          javVideoPickerAction === 'open'
            ? zh('打开文件失败', 'Failed to open file')
            : zh('打开所在位置失败', 'Failed to reveal file'),
          err
        )
      } finally {
        closeJavVideoPicker()
      }
    },
    [
      closeJavVideoPicker,
      handleOpenAlternatePlayer,
      handleOpenPlayer,
      handleRevealVideoFile,
      isVideoOpenable,
      javVideoPickerAction,
    ]
  )

  const handleSelectVideoLocation = useCallback(
    async (video) => {
      if (!video) return
      if (locationPickerAction === 'play') {
        playVideoWith(video, defaultPlayer)
        closeLocationPicker()
        return
      }
      if (locationPickerAction === 'open') {
        playVideoWith(video, alternatePlayer)
        closeLocationPicker()
        return
      }
      try {
        if (locationPickerAction === 'reveal') {
          await revealVideoFile(video)
        }
      } catch (err) {
        console.error(zh('打开所在位置失败', 'Failed to reveal file'), err)
      } finally {
        closeLocationPicker()
      }
    },
    [
      alternatePlayer,
      closeLocationPicker,
      defaultPlayer,
      locationPickerAction,
      playVideoWith,
      revealVideoFile,
    ]
  )
  useEffect(() => {
    let mounted = true
    loadConfig().finally(() => {
      if (mounted) setConfigLoaded(true)
    })
    return () => {
      mounted = false
    }
  }, [loadConfig])
  const buildVideoUrl = useCallback(
    (options = {}) => {
      const {
        page: pageOverride,
        search: searchOverride,
        random: randomOverride,
        seed: seedOverride,
        tagIds: tagIdsOverride,
        tempSort: tempSortOverride,
      } = options
      const sp = new URLSearchParams()
      sp.set('view', 'video')
      const searchVal = (searchOverride ?? searchTerm).trim()
      if (searchVal) {
        sp.set('search', searchVal)
      }
      const hasTempSortOverride = Object.prototype.hasOwnProperty.call(options, 'tempSort')
      const tempSortVal = hasTempSortOverride
        ? normalizeVideoSort(tempSortOverride, '')
        : videoTempSort
      const tagIds = tagIdsOverride ?? selectedTagIds
      if (tagIds.length > 0) {
        sp.set('tag_ids', [...tagIds].sort((a, b) => a - b).join(','))
      }
      const randomFlag = randomOverride ?? randomMode
      if (randomFlag) {
        sp.set('random', '1')
        const seedValue = seedOverride ?? randomSeed
        if (seedValue) {
          sp.set('seed', String(seedValue))
        }
      } else {
        if (tempSortVal) {
          sp.set('temp_sort', tempSortVal)
        }
        sp.delete('random')
        sp.delete('seed')
        const targetPage = pageOverride ?? page
        sp.set('page', String(targetPage))
      }
      const query = sp.toString()
      return `${window.location.pathname}${query ? `?${query}` : ''}`
    },
    [page, randomMode, randomSeed, searchTerm, selectedTagIds, videoTempSort]
  )

  const buildJavUrl = useCallback(
    (options = {}) => {
      const {
        page: pageOverride,
        search: searchOverride,
        tab: tabOverride,
        idolIds: idolIdsOverride,
        studioId: studioIdOverride,
        studioName: studioNameOverride,
        seriesId: seriesIdOverride,
        seriesName: seriesNameOverride,
        favoriteGroupId: favoriteGroupIdOverride,
        tagIds: tagIdsOverride,
        random: randomOverride,
        seed: seedOverride,
        tempSort: tempSortOverride,
      } = options
      const sp = new URLSearchParams()
      sp.set('view', 'jav')
      const tab = tabOverride ?? javTab
      if (tab === 'idol' || tab === 'studio' || tab === 'series') {
        sp.set('tab', tab)
      }
      const searchVal = (searchOverride ?? javSearchTerm).trim()
      if (searchVal) {
        sp.set('search', searchVal)
      }
      const idolIdList = idolIdsOverride ?? javIdolIds
      if (tab === 'list' && idolIdList && idolIdList.length > 0) {
        sp.set('idol_ids', idolIdList.join(','))
      }
      const tagList = tagIdsOverride ?? javTags
      if (tab === 'list' && tagList && tagList.length > 0) {
        sp.set('tag_ids', tagList.join(','))
      }
      const hasStudioIdOverride = Object.prototype.hasOwnProperty.call(options, 'studioId')
      const studioId = hasStudioIdOverride ? studioIdOverride : javStudioId
      if (tab === 'list' && studioId) {
        sp.set('studio_id', String(studioId))
        const studioName =
          studioNameOverride ?? (hasStudioIdOverride ? '' : String(javStudioName || '').trim())
        if (studioName) {
          sp.set('studio_name', studioName)
        }
      }
      const hasSeriesIdOverride = Object.prototype.hasOwnProperty.call(options, 'seriesId')
      const seriesId = hasSeriesIdOverride ? seriesIdOverride : javSeriesId
      if (tab === 'list' && seriesId) {
        sp.set('series_id', String(seriesId))
        const seriesName =
          seriesNameOverride ?? (hasSeriesIdOverride ? '' : String(javSeriesName || '').trim())
        if (seriesName) {
          sp.set('series_name', seriesName)
        }
      }
      const hasFavoriteGroupIdOverride = Object.prototype.hasOwnProperty.call(
        options,
        'favoriteGroupId'
      )
      const favoriteGroupId = hasFavoriteGroupIdOverride
        ? favoriteGroupIdOverride
        : idolFavoriteGroupId
      if (tab === 'idol' && favoriteGroupId) {
        sp.set('favorite_group_id', String(favoriteGroupId))
      }
      const hasTempSortOverride = Object.prototype.hasOwnProperty.call(options, 'tempSort')
      const tempSortVal = hasTempSortOverride ? normalizeJavSort(tempSortOverride, '') : javTempSort
      const randomFlag = randomOverride ?? javRandomMode
      if (tab === 'list' && randomFlag) {
        sp.set('random', '1')
        const seedValue = seedOverride ?? javRandomSeed
        if (seedValue) {
          sp.set('seed', String(seedValue))
        }
      } else {
        if (tab === 'list' && tempSortVal) {
          sp.set('temp_sort', tempSortVal)
        }
        sp.delete('random')
        sp.delete('seed')
        const targetPage =
          pageOverride ??
          (tab === 'idol'
            ? idolPage
            : tab === 'studio'
              ? studioPage
              : tab === 'series'
                ? seriesPage
                : javPage)
        sp.set('page', String(targetPage))
      }
      const query = sp.toString()
      return `${window.location.pathname}${query ? `?${query}` : ''}`
    },
    [
      idolPage,
      idolFavoriteGroupId,
      studioPage,
      seriesPage,
      javIdolIds,
      javStudioId,
      javStudioName,
      javSeriesId,
      javSeriesName,
      javPage,
      javTempSort,
      javSearchTerm,
      javTab,
      javTags,
      javRandomMode,
      javRandomSeed,
    ]
  )

  const applyJavTagFilter = useCallback(
    (tagIds) => {
      const clean = Array.from(
        new Set(
          (tagIds || [])
            .map((id) => Number.parseInt(String(id), 10))
            .filter((value) => Number.isFinite(value) && value > 0)
        )
      )
      saveScrollBeforeUrlStateChange()
      useStore.setState({
        viewMode: 'jav',
        videoTempSort: '',
        javTab: 'list',
        javTempSort: '',
        javRandomMode: false,
        javRandomSeed: null,
        javIdolIds: [],
        javStudioId: null,
        javStudioName: '',
        javSeriesId: null,
        javSeriesName: '',
        idolFavoriteGroupId: null,
        javTags: clean,
        javSearchTerm: '',
        javPage: 1,
        idolPage: 1,
        studioPage: 1,
        seriesPage: 1,
      })
    },
    [saveScrollBeforeUrlStateChange]
  )

  const applyUrlState = useCallback(
    (parsed, { fromPopstate = false } = {}) => {
      isPoppingRef.current = fromPopstate
      lastUrlRef.current = window.location.pathname + window.location.search
      useStore.getState().setDirectoryFilterFromUrl(parsed.directoryIds)
      const mapTagIdsToNamesFromStore = (ids) => {
        if (!Array.isArray(ids) || ids.length === 0) return []
        const { tags: storeTags } = useStore.getState()
        const idSet = new Set(ids)
        return (storeTags || []).filter((t) => idSet.has(t.id)).map((t) => t.name)
      }
      if (parsed.view === 'jav') {
        const { jav } = parsed
        useStore.setState({
          viewMode: 'jav',
          videoTempSort: '',
          javTab: jav.tab,
          javRandomMode: jav.tab === 'list' ? jav.random : false,
          javRandomSeed: jav.tab === 'list' && jav.random ? jav.seed : null,
          javSearchTerm: jav.search,
          javIdolIds: jav.tab === 'list' ? jav.idolIds : [],
          javTags: jav.tab === 'list' ? jav.tagIds : [],
          javStudioId: jav.tab === 'list' ? jav.studioId : null,
          javStudioName: jav.tab === 'list' && jav.studioId ? jav.studioName : '',
          javSeriesId: jav.tab === 'list' ? jav.seriesId : null,
          javSeriesName: jav.tab === 'list' && jav.seriesId ? jav.seriesName : '',
          javPage: jav.random ? 1 : jav.page,
          idolPage: jav.tab === 'idol' ? jav.page : 1,
          idolFavoriteGroupId: jav.tab === 'idol' ? jav.idolFavoriteGroupId : null,
          studioPage: jav.tab === 'studio' ? jav.page : 1,
          seriesPage: jav.tab === 'series' ? jav.page : 1,
          javTempSort: jav.tab !== 'list' || jav.random ? '' : jav.tempSort,
        })
        setJavSearchInput(jav.search)
        if (jav.tab === 'list' && jav.random) {
          useStore.getState().loadJavRandom(jav.seed ?? undefined)
        }
        setHydrated(true)
        return
      }

      const { video } = parsed
      useStore.setState({
        viewMode: 'video',
        javTempSort: '',
        videoTempSort: video.random ? '' : video.tempSort,
        randomMode: video.random,
        randomSeed: video.random ? video.seed : null,
        searchTerm: video.random ? '' : video.search,
        page: video.random ? 1 : video.page,
      })
      setSearchInput(video.search)
      const names = mapTagIdsToNamesFromStore(video.tagIds)
      if (names.length || video.tagIds.length === 0) {
        useStore.getState().setSelectedTags(names, { resetPage: false, preserveTempSort: true })
      } else {
        pendingVideoTagIdsRef.current = video.tagIds
      }
      if (video.random) {
        useStore.getState().loadRandom(video.seed ?? undefined)
      }
      setHydrated(true)
    },
    [setJavSearchInput, setSearchInput]
  )

  useEffect(() => {
    if (!configLoaded) return
    ensureBrowserHistoryState()
    const apply = (fromPopstate = false) => {
      const parsed = parseUrlState(window.location.search, { defaultView: initialViewMode })
      if (parsed.view === 'jav') {
        useStore.setState({ viewMode: 'jav' })
      } else {
        useStore.setState({ viewMode: 'video' })
      }
      applyUrlState(parsed, { fromPopstate })
    }
    apply(false)
    const onPop = (event) => {
      const index = readBrowserHistoryIndex(event.state)
      const max = Math.max(browserHistoryMaxRef.current, index)
      pendingScrollRestoreRef.current = {
        ...normalizeHistoryScrollPosition(event.state?.[HISTORY_SCROLL_KEY]),
        attempts: 0,
      }
      cancelScheduledScrollRestore()
      setBrowserNavigationFromIndex(index, max)
      apply(true)
    }
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [
    applyUrlState,
    cancelScheduledScrollRestore,
    ensureBrowserHistoryState,
    normalizeHistoryScrollPosition,
    readBrowserHistoryIndex,
    setBrowserNavigationFromIndex,
    configLoaded,
    initialViewMode,
  ])

  useEffect(() => {
    setSearchInput(searchTerm)
  }, [searchTerm])

  useEffect(() => {
    setJavSearchInput(javSearchTerm)
  }, [javSearchTerm])

  useEffect(() => {
    loadDirectories()
  }, [loadDirectories])

  useEffect(() => {
    loadTags({ skipUnchanged: true })
    loadJavTags({ skipUnchanged: true })
  }, [loadTags, loadJavTags, directoryTagKey, videoHideJav])

  useEffect(() => {
    if (!pendingVideoTagIdsRef.current || !tags.length) return
    const names = mapTagIdsToNames(pendingVideoTagIdsRef.current)
    setSelectedTags(names, { resetPage: false, preserveTempSort: true })
    pendingVideoTagIdsRef.current = null
  }, [mapTagIdsToNames, setSelectedTags, tags])

  useEffect(() => {
    if (hydrated && configLoaded && !isJavMode) {
      loadVideos()
    }
  }, [
    configLoaded,
    hydrated,
    isJavMode,
    loadVideos,
    page,
    pageSize,
    randomMode,
    randomSeed,
    searchTerm,
    selectedTags,
    enabledDirectoryIds,
    directoryFilterMode,
    sortOrder,
    videoTempSort,
    videoHideJav,
  ])

  useEffect(() => {
    if (!hydrated || !configLoaded || !isJavMode) return
    if (javTab === 'idol') {
      loadJavIdols()
      loadJavIdolFavoriteGroups()
    } else if (javTab === 'studio') {
      loadJavStudios()
    } else if (javTab === 'series') {
      loadJavSeries()
    } else {
      loadJavs()
    }
  }, [
    hydrated,
    isJavMode,
    javTab,
    javPage,
    javPageSize,
    javSearchTerm,
    javIdolIds,
    javTags,
    javStudioId,
    javSeriesId,
    javSort,
    javTempSort,
    javRandomMode,
    javRandomSeed,
    idolSort,
    idolPage,
    idolPageSize,
    idolFavoriteGroupId,
    studioPage,
    seriesPage,
    enabledDirectoryIds,
    directoryFilterMode,
    loadJavs,
    loadJavIdols,
    loadJavIdolFavoriteGroups,
    loadJavStudios,
    loadJavSeries,
    configLoaded,
  ])

  const forceReloadVideos = useCallback(() => {
    if (!hydrated || !configLoaded) return
    loadVideos({ force: true })
  }, [configLoaded, hydrated, loadVideos])

  const forceReloadJavByTab = useCallback(
    (tab) => {
      if (!hydrated || !configLoaded) return
      if (tab === 'idol') {
        loadJavIdols({ force: true })
        loadJavIdolFavoriteGroups({ force: true })
      } else if (tab === 'studio') {
        loadJavStudios({ force: true })
      } else if (tab === 'series') {
        loadJavSeries({ force: true })
      } else {
        loadJavs({ force: true })
      }
    },
    [
      configLoaded,
      hydrated,
      loadJavIdolFavoriteGroups,
      loadJavIdols,
      loadJavSeries,
      loadJavStudios,
      loadJavs,
    ]
  )

  const setWaterfallMode = useCallback(
    (key, enabled) => {
      setWaterfallModes((current) => ({ ...current, [key]: enabled }))
      if (enabled || !hydrated || !configLoaded) return
      if (key === 'video') {
        loadVideos({ force: true })
      } else if (key === 'jav') {
        loadJavs({ force: true })
      } else if (key === 'idol') {
        loadJavIdols({ force: true })
      } else if (key === 'studio') {
        loadJavStudios({ force: true })
      } else if (key === 'series') {
        loadJavSeries({ force: true })
      }
    },
    [configLoaded, hydrated, loadJavIdols, loadJavSeries, loadJavStudios, loadJavs, loadVideos]
  )

  const currentUrlState = useMemo(
    () =>
      normalizeUrlStateFromStore(
        {
          viewMode,
          page,
          searchTerm,
          videoTempSort,
          selectedTags,
          randomMode,
          randomSeed,
          javTab,
          javPage,
          javSearchTerm,
          javIdolIds,
          javTags,
          javStudioId,
          javStudioName,
          javSeriesId,
          javSeriesName,
          javTempSort,
          javRandomMode,
          javRandomSeed,
          idolPage,
          idolFavoriteGroupId,
          studioPage,
          seriesPage,
          directories,
          enabledDirectoryIds,
          directoryFilterMode,
        },
        tagsByName
      ),
    [
      directories,
      directoryFilterMode,
      enabledDirectoryIds,
      idolFavoriteGroupId,
      idolPage,
      studioPage,
      seriesPage,
      javIdolIds,
      javStudioId,
      javSeriesId,
      javPage,
      javRandomMode,
      javRandomSeed,
      javSearchTerm,
      javTempSort,
      javTab,
      javTags,
      page,
      randomMode,
      randomSeed,
      searchTerm,
      selectedTags,
      videoTempSort,
      tagsByName,
      viewMode,
      javStudioName,
      javSeriesName,
    ]
  )

  useEffect(() => {
    if (!hydrated) return
    const nextUrl = buildUrlFromState(currentUrlState)
    const currentUrl = window.location.pathname + window.location.search
    if (nextUrl === currentUrl) {
      preNavigationScrollSaveUrlRef.current = null
      lastUrlRef.current = nextUrl
      isPoppingRef.current = false
      return
    }
    if (isPoppingRef.current) {
      preNavigationScrollSaveUrlRef.current = null
      lastUrlRef.current = nextUrl
      isPoppingRef.current = false
      return
    }
    pendingScrollRestoreRef.current = null
    cancelScheduledScrollRestore()
    if (preNavigationScrollSaveUrlRef.current === currentUrl) {
      preNavigationScrollSaveUrlRef.current = null
    } else {
      saveCurrentScrollPosition()
    }
    const nextIndex = browserHistoryIndexRef.current + 1
    const nextScroll = readWindowScrollPosition()
    window.history.pushState(
      {
        ...(window.history.state || {}),
        [HISTORY_INDEX_KEY]: nextIndex,
        [HISTORY_SCROLL_KEY]: nextScroll,
      },
      '',
      nextUrl
    )
    setBrowserNavigationFromIndex(nextIndex, nextIndex)
    lastUrlRef.current = nextUrl
  }, [
    cancelScheduledScrollRestore,
    currentUrlState,
    hydrated,
    readWindowScrollPosition,
    saveCurrentScrollPosition,
    setBrowserNavigationFromIndex,
  ])

  const canPrev = page > 1
  const canNext = hasNext
  const lastPage = Math.max(1, Math.ceil((total || 0) / pageSize))

  const navigateVideoPage = useCallback(
    (targetPage) => {
      if (!targetPage || targetPage === page) return
      saveScrollBeforeUrlStateChange()
      setPage(targetPage)
    },
    [page, saveScrollBeforeUrlStateChange, setPage]
  )
  const selectedCount = useMemo(() => selectedVideoIds.size, [selectedVideoIds])
  const selectedList = useMemo(() => {
    const keys = Array.from(selectedVideoIds)
    return keys.map((key) => {
      const v = videos.find((item) => videoSelectionKey(item) === String(key))
      const meta = selectedVideoMeta?.[key]
      const labelFromMeta = meta && typeof meta === 'object' ? meta.label : meta
      return {
        id: key,
        label: labelFromMeta || v?.filename || v?.path || `#${key}`,
        video: v,
      }
    })
  }, [selectedVideoIds, videos, selectedVideoMeta])
  const javLastPage = Math.max(1, Math.ceil((javTotal || 0) / javPageSize))
  const javHasPrev = javPage > 1
  const javHasNext = javPage < javLastPage
  const idolLastPage = Math.max(1, Math.ceil((idolTotal || 0) / idolPageSize))
  const idolHasPrev = idolPage > 1
  const idolHasNext = idolPage < idolLastPage
  const studioLastPage = Math.max(1, Math.ceil((studioTotal || 0) / JAV_STUDIO_PAGE_SIZE))
  const studioHasPrev = studioPage > 1
  const studioHasNext = studioPage < studioLastPage
  const seriesLastPage = Math.max(1, Math.ceil((seriesTotal || 0) / JAV_STUDIO_PAGE_SIZE))
  const seriesHasPrev = seriesPage > 1
  const seriesHasNext = seriesPage < seriesLastPage
  const videoWaterfallHasMore =
    !randomMode && (page - 1) * pageSize + (videos?.length || 0) < (total || 0)
  const javWaterfallHasMore =
    !javRandomMode && (javPage - 1) * javPageSize + (javItems?.length || 0) < (javTotal || 0)
  const idolWaterfallHasMore =
    (idolPage - 1) * idolPageSize + (idolItems?.length || 0) < (idolTotal || 0)
  const studioWaterfallHasMore =
    (studioPage - 1) * JAV_STUDIO_PAGE_SIZE + (studioItems?.length || 0) < (studioTotal || 0)
  const seriesWaterfallHasMore =
    (seriesPage - 1) * JAV_STUDIO_PAGE_SIZE + (seriesItems?.length || 0) < (seriesTotal || 0)
  const javTagNameMap = useMemo(
    () => new Map((javTagOptions || []).map((tag) => [tag.id, tag.name])),
    [javTagOptions]
  )
  const javDirectoryKey = javQueryDirectoryIds.join(',')
  const javIdolOptionMap = useMemo(() => {
    const map = new Map()
    const addIdol = (idol) => {
      const id = Number(idol?.id)
      if (!Number.isFinite(id) || id <= 0 || map.has(id)) return
      map.set(id, idol)
    }
    Object.values(javResolvedIdols || {}).forEach(addIdol)
    ;(idolItems || []).forEach(addIdol)
    ;(javItems || []).forEach((item) => {
      ;(item?.idols || []).forEach(addIdol)
    })
    return map
  }, [idolItems, javItems, javResolvedIdols])
  const javIdolFilterOptions = useMemo(
    () => Array.from(javIdolOptionMap.values()),
    [javIdolOptionMap]
  )
  const javUserTagOptions = useMemo(
    () => (javTagOptions || []).filter((tag) => isUserJavTag(tag)),
    [javTagOptions]
  )
  const showJavFilterRandomButton =
    isJavMode &&
    javTab === 'list' &&
    (javIdolIds.length > 0 ||
      javTags.length > 0 ||
      Boolean(javStudioId) ||
      Boolean(javSeriesId) ||
      Boolean((javSearchTerm || '').trim()))
  useEffect(() => {
    setJavResolvedIdols({})
  }, [config?.jav_metadata_language, javDirectoryKey])

  useEffect(() => {
    if (!isJavMode || javTab !== 'list' || javIdolIds.length === 0) return undefined
    const missingIds = javIdolIds
      .map((id) => Number(id))
      .filter((id) => Number.isFinite(id) && id > 0 && !javIdolOptionMap.has(id))
    if (missingIds.length === 0) return undefined

    let cancelled = false
    resolveJavIdols(missingIds)
      .then((items) => {
        if (cancelled) return
        const loaded = {}
        for (const idol of items || []) {
          const id = Number(idol?.id)
          if (Number.isFinite(id) && id > 0) loaded[id] = idol
        }
        if (Object.keys(loaded).length > 0) {
          setJavResolvedIdols((current) => ({ ...current, ...loaded }))
        }
      })
      .catch((err) => {
        console.warn('resolve jav idol names failed', err)
      })

    return () => {
      cancelled = true
    }
  }, [isJavMode, javTab, javIdolIds, javIdolOptionMap, javQueryDirectoryIds])

  const searchHref = buildVideoUrl({
    search: searchInput,
    page: 1,
    random: false,
    tagIds: [],
    tempSort: '',
  })
  const randomHref = buildVideoUrl({ random: true, page: 1, tagIds: [], search: '' })
  const javSearchHref = buildJavUrl({
    search: javSearchInput,
    page: 1,
    tab: javTab,
    idolIds: [],
    tagIds: [],
    studioId: null,
    seriesId: null,
    random: false,
    tempSort: '',
  })
  const javRandomHref = buildJavUrl({
    random: true,
    page: 1,
    tab: 'list',
    idolIds: [],
    tagIds: [],
    studioId: null,
    seriesId: null,
    search: '',
  })
  const handleJavRandomClick = useCallback(() => {
    const nextSeed = generateRandomSeed()
    useStore.setState({
      viewMode: 'jav',
      videoTempSort: '',
      javTab: 'list',
      javTempSort: '',
      javIdolIds: [],
      javTags: [],
      javStudioId: null,
      javStudioName: '',
      javSeriesId: null,
      javSeriesName: '',
      idolFavoriteGroupId: null,
      javSearchTerm: '',
      javPage: 1,
      idolPage: 1,
      studioPage: 1,
      seriesPage: 1,
    })
    setJavSearchInput('')
    loadJavRandom(nextSeed)
  }, [loadJavRandom])

  const handleJavFilterRandomClick = useCallback(() => {
    const nextSeed = generateRandomSeed()
    useStore.setState({
      viewMode: 'jav',
      videoTempSort: '',
      javTab: 'list',
      idolFavoriteGroupId: null,
      idolPage: 1,
      studioPage: 1,
      seriesPage: 1,
    })
    loadJavRandom(nextSeed)
  }, [loadJavRandom])

  const handleCancelJavFilterRandom = useCallback(() => {
    useStore.setState({
      javTempSort: '',
      javRandomMode: false,
      javRandomSeed: null,
      javPage: 1,
    })
  }, [])

  const handleVideoRandomClick = useCallback(() => {
    const nextSeed = generateRandomSeed()
    setSearchInput('')
    useStore.setState({
      viewMode: 'video',
      selectedTags: [],
      searchTerm: '',
      videoTempSort: '',
      page: 1,
      randomMode: true,
      randomSeed: nextSeed,
    })
  }, [])
  const filterSummary = useMemo(() => {
    const formatList = (items) => {
      if (!items || items.length === 0) return ''
      const separator = isChineseLocale() ? '、' : ', '
      return items.join(separator)
    }
    if (isJavMode) {
      const parts = []
      if (javTab === 'list') {
        const idolNames = javIdolIds
          .map((id) => javIdolOptionMap.get(Number(id))?.name)
          .filter(Boolean)
        const idolsLabel = formatList(idolNames)
        if (idolsLabel) parts.push(zh(`女优: ${idolsLabel}`, `Idols: ${idolsLabel}`))
        const tagNames = javTags.map((id) => javTagNameMap.get(id)).filter(Boolean)
        const tagsLabel = formatList(tagNames)
        if (tagsLabel) parts.push(zh(`标签: ${tagsLabel}`, `Tags: ${tagsLabel}`))
        if (javStudioId) {
          const loadedStudioName =
            javItems.find((item) => Number(item?.studio?.id) === Number(javStudioId))?.studio
              ?.name || ''
          const label = javStudioName || loadedStudioName || `#${javStudioId}`
          parts.push(zh(`片商: ${label}`, `Studio: ${label}`))
        }
        if (javSeriesId) {
          const loadedSeriesItem = javItems.find(
            (item) =>
              Number(item?.series?.id) === Number(javSeriesId) ||
              Number(item?.series_en?.id) === Number(javSeriesId)
          )
          const loadedSeriesName =
            Number(loadedSeriesItem?.series?.id) === Number(javSeriesId)
              ? loadedSeriesItem?.series?.name || ''
              : loadedSeriesItem?.series_en?.name || ''
          const label = javSeriesName || loadedSeriesName || `#${javSeriesId}`
          parts.push(zh(`系列: ${label}`, `Series: ${label}`))
        }
      }
      const searchLabel = (javSearchTerm || '').trim()
      if (searchLabel) parts.push(zh(`搜索: ${searchLabel}`, `Search: ${searchLabel}`))
      if (javTab === 'list' && javRandomMode && parts.length === 0) {
        parts.push(zh('随机', 'Random'))
      }
      return parts.length ? parts.join(isChineseLocale() ? '；' : '; ') : ''
    }
    const parts = []
    const tagsLabel = formatList(selectedTags)
    if (tagsLabel) parts.push(zh(`标签: ${tagsLabel}`, `Tags: ${tagsLabel}`))
    const searchLabel = (searchTerm || '').trim()
    if (searchLabel) parts.push(zh(`搜索: ${searchLabel}`, `Search: ${searchLabel}`))
    if (randomMode && parts.length === 0) {
      parts.push(zh('随机', 'Random'))
    }
    return parts.length ? parts.join(isChineseLocale() ? '；' : '; ') : ''
  }, [
    isJavMode,
    javTab,
    javIdolIds,
    javIdolOptionMap,
    javTags,
    javStudioId,
    javStudioName,
    javSeriesId,
    javSeriesName,
    javItems,
    javTagNameMap,
    javSearchTerm,
    javRandomMode,
    selectedTags,
    searchTerm,
    randomMode,
  ])

  const openVideoSettings = useCallback(() => {
    setVideoPageSizeInput(pageSize)
    setVideoSortInput(sortOrder)
    setVideoHideJavInput(videoHideJav)
    setVideoSettingsOpen(true)
  }, [pageSize, sortOrder, videoHideJav])

  const openJavSettings = useCallback(() => {
    setJavPageSizeInput(javPageSize)
    setJavGridColumnsInput(javGridColumns)
    setJavTitleMaxRowsInput(javTitleMaxRows)
    setJavIdolTagMaxRowsInput(javIdolTagMaxRows)
    setJavTagMaxRowsInput(javTagMaxRows)
    setIdolPageSizeInput(idolPageSize)
    setJavSortInput(javSort)
    setIdolSortInput(idolSort)
    setJavSettingsOpen(true)
  }, [
    javPageSize,
    javGridColumns,
    javTitleMaxRows,
    javIdolTagMaxRows,
    javTagMaxRows,
    idolPageSize,
    javSort,
    idolSort,
  ])

  const submitSearch = (e) => {
    e?.preventDefault()
    const nextSearch = (searchInput || '').trim()
    useStore.setState({
      viewMode: 'video',
      selectedTags: [],
      searchTerm: nextSearch,
      videoTempSort: '',
      page: 1,
      randomMode: false,
      randomSeed: null,
    })
  }

  const submitJavSearch = (e) => {
    e?.preventDefault()
    useStore.setState({
      viewMode: 'jav',
      videoTempSort: '',
      javTempSort: '',
      javRandomMode: false,
      javRandomSeed: null,
      javIdolIds: [],
      javTags: [],
      javStudioId: null,
      javStudioName: '',
      javSeriesId: null,
      javSeriesName: '',
      javSearchTerm: (javSearchInput || '').trim(),
      javPage: 1,
      idolPage: 1,
      studioPage: 1,
      seriesPage: 1,
    })
  }

  const handleSaveVideoSettings = async () => {
    const size = Math.max(1, parseInt(videoPageSizeInput, 10) || pageSize)
    const normalizedSort = normalizeVideoSort(videoSortInput)
    try {
      await updateConfig({
        video_page_size: size,
        video_sort: normalizedSort,
        video_hide_jav: videoHideJavInput,
      })
      const prevPage = page
      // ensure current page does not exceed last page after page size change
      const lastPage = Math.max(1, Math.ceil((total || 0) / size))
      const filterChanged = videoHideJavInput !== videoHideJav
      const nextPage = filterChanged ? 1 : prevPage > lastPage ? lastPage : prevPage

      useStore.setState({
        pageSize: size,
        sortOrder: normalizedSort,
        videoHideJav: videoHideJavInput,
        videoTempSort: '',
        page: nextPage,
        randomMode: false,
        randomSeed: null,
      })
      setVideoSettingsOpen(false)
    } catch (err) {
      alert(err.message || zh('保存失败', 'Save failed'))
    }
  }

  const handleSaveJavSettings = async () => {
    const javSize = Math.max(1, parseInt(javPageSizeInput, 10) || javPageSize)
    const javGridColumnsRaw = parseInt(javGridColumnsInput, 10)
    const javColumns =
      Number.isFinite(javGridColumnsRaw) && javGridColumnsRaw > 0
        ? Math.min(javGridColumnsRaw, 12)
        : 0
    const javIdolTagRowsRaw = parseInt(javIdolTagMaxRowsInput, 10)
    const javIdolTagRows =
      Number.isFinite(javIdolTagRowsRaw) && javIdolTagRowsRaw > 0
        ? Math.min(javIdolTagRowsRaw, 12)
        : 0
    const javTitleRowsRaw = parseInt(javTitleMaxRowsInput, 10)
    const javTitleRows =
      Number.isFinite(javTitleRowsRaw) && javTitleRowsRaw >= 0 ? Math.min(javTitleRowsRaw, 12) : 2
    const javTagRowsRaw = parseInt(javTagMaxRowsInput, 10)
    const javTagRows =
      Number.isFinite(javTagRowsRaw) && javTagRowsRaw >= 0 ? Math.min(javTagRowsRaw, 12) : 2
    const idolSize = Math.max(1, parseInt(idolPageSizeInput, 10) || idolPageSize)
    const normalizedSort = normalizeJavSort(javSortInput)
    const normalizedIdolSort = normalizeIdolSort(idolSortInput)
    try {
      const cfg = await updateConfig({
        jav_page_size: javSize,
        jav_grid_columns: javColumns,
        jav_title_max_rows: javTitleRows,
        jav_idol_tag_max_rows: javIdolTagRows,
        jav_tag_max_rows: javTagRows,
        idol_page_size: idolSize,
        jav_sort: normalizedSort,
        idol_sort: normalizedIdolSort,
      })
      const prevJavPage = javPage
      const prevIdolPage = idolPage
      const prevStudioPage = studioPage
      const prevSeriesPage = seriesPage
      const javLast = Math.max(1, Math.ceil((javTotal || 0) / javSize))
      const idolLast = Math.max(1, Math.ceil((idolTotal || 0) / idolSize))
      const studioLast = Math.max(1, Math.ceil((studioTotal || 0) / JAV_STUDIO_PAGE_SIZE))
      const seriesLast = Math.max(1, Math.ceil((seriesTotal || 0) / JAV_STUDIO_PAGE_SIZE))
      useStore.setState({
        javPageSize: javSize,
        javGridColumns: javColumns,
        javTitleMaxRows: javTitleRows,
        javIdolTagMaxRows: javIdolTagRows,
        javTagMaxRows: javTagRows,
        idolPageSize: idolSize,
        javSort: normalizedSort,
        javTempSort: '',
        idolSort: normalizedIdolSort,
        javPage: Math.min(prevJavPage, javLast),
        idolPage: Math.min(prevIdolPage, idolLast),
        studioPage: Math.min(prevStudioPage, studioLast),
        seriesPage: Math.min(prevSeriesPage, seriesLast),
        javRandomMode: false,
        javRandomSeed: null,
        config: cfg,
      })
      setJavSettingsOpen(false)
    } catch (err) {
      alert(err.message || zh('保存失败', 'Save failed'))
    }
  }

  useEffect(() => {
    if (videoSettingsOpen) {
      setVideoPageSizeInput(pageSize)
      setVideoSortInput(sortOrder)
      setVideoHideJavInput(videoHideJav)
    }
  }, [videoSettingsOpen, pageSize, sortOrder, videoHideJav])

  useEffect(() => {
    if (javSettingsOpen) {
      setJavPageSizeInput(javPageSize)
      setJavGridColumnsInput(javGridColumns)
      setJavTitleMaxRowsInput(javTitleMaxRows)
      setJavIdolTagMaxRowsInput(javIdolTagMaxRows)
      setJavTagMaxRowsInput(javTagMaxRows)
      setIdolPageSizeInput(idolPageSize)
      setJavSortInput(javSort)
      setIdolSortInput(idolSort)
    }
  }, [
    javSettingsOpen,
    javPageSize,
    javGridColumns,
    javTitleMaxRows,
    javIdolTagMaxRows,
    javTagMaxRows,
    idolPageSize,
    javSort,
    idolSort,
  ])

  useEffect(() => {
    if (selectedCount !== 0) return
    setSelectionOpsOpen(false)
    setSelectionTagsOpen(false)
    setSelectionTagAction('add')
    setSelectionTagChoices([])
  }, [selectedCount])

  const openTagEditor = useCallback(
    (videoId) => {
      setTagPickerFor(videoId)
      const target = videos.find((v) => v.id === videoId)
      const initial = Array.isArray(target?.tags) ? target.tags.map((t) => String(t.id)) : []
      setTagPickerSelected(initial)
    },
    [videos]
  )

  const openJavTagEditor = useCallback(
    (item) => {
      if (!item) return
      const initial = Array.isArray(item?.tags)
        ? item.tags.filter((tag) => isUserJavTag(tag)).map((tag) => String(tag.id))
        : []
      setJavTagPickerItem(item)
      setJavTagPickerSelected(initial)
      loadJavTags()
    },
    [loadJavTags]
  )

  const tagPickerExisting = useMemo(() => {
    if (!tagPickerFor) return []
    const target = videos.find((v) => v.id === tagPickerFor)
    return Array.isArray(target?.tags) ? target.tags.map((t) => String(t.id)) : []
  }, [tagPickerFor, videos])

  const tagPickerDirty = useMemo(() => {
    if (!tagPickerFor) return false
    const current = new Set(tagPickerExisting)
    const selected = new Set(tagPickerSelected)
    if (current.size !== selected.size) return true
    for (const id of current) {
      if (!selected.has(id)) return true
    }
    return false
  }, [tagPickerExisting, tagPickerFor, tagPickerSelected])

  const javTagPickerExisting = useMemo(() => {
    if (!javTagPickerItem) return []
    return Array.isArray(javTagPickerItem?.tags)
      ? javTagPickerItem.tags.filter((tag) => isUserJavTag(tag)).map((tag) => String(tag.id))
      : []
  }, [javTagPickerItem])

  const javTagPickerDirty = useMemo(() => {
    if (!javTagPickerItem) return false
    const current = new Set(javTagPickerExisting)
    const selected = new Set(javTagPickerSelected)
    if (current.size !== selected.size) return true
    for (const id of current) {
      if (!selected.has(id)) return true
    }
    return false
  }, [javTagPickerExisting, javTagPickerItem, javTagPickerSelected])

  const handleApplyTags = async () => {
    if (!tagPickerFor) {
      setTagPickerFor(null)
      setTagPickerSelected([])
      return
    }
    const selectedIds = tagPickerSelected.map((t) => Number(t)).filter(Boolean)
    if (!tagPickerDirty) {
      setTagPickerFor(null)
      setTagPickerSelected([])
      return
    }
    try {
      await replaceTagsForVideos([tagPickerFor], selectedIds)
      useStore.setState((state) => {
        if (!Array.isArray(state.videos)) return {}
        const tagLookup = new Map((tags || []).map((tag) => [tag.id, tag]))
        const nextVideos = state.videos.map((video) => {
          if (video.id !== tagPickerFor) return video
          const nextTags = selectedIds.map((id) => tagLookup.get(id)).filter(Boolean)
          return { ...video, tags: nextTags }
        })
        return { videos: nextVideos }
      })
    } catch (err) {
      console.error('update tags failed', err)
    } finally {
      setTagPickerFor(null)
      setTagPickerSelected([])
    }
  }

  const handleApplyJavTags = async () => {
    if (!javTagPickerItem) {
      setJavTagPickerItem(null)
      setJavTagPickerSelected([])
      return
    }
    const javId = javTagPickerItem.id
    if (!javId) {
      setJavTagPickerItem(null)
      setJavTagPickerSelected([])
      return
    }
    const selectedIds = javTagPickerSelected.map((t) => Number(t)).filter(Boolean)
    if (!javTagPickerDirty) {
      setJavTagPickerItem(null)
      setJavTagPickerSelected([])
      return
    }
    try {
      await replaceJavTagsForItems([javId], selectedIds)
      useStore.setState((state) => {
        if (!Array.isArray(state.javItems)) return {}
        const userTagMap = new Map(javUserTagOptions.map((tag) => [tag.id, tag]))
        const next = state.javItems.map((item) => {
          if (item.id !== javId) return item
          const existingTags = Array.isArray(item.tags) ? item.tags : []
          for (const tag of existingTags) {
            if (isUserJavTag(tag) && !userTagMap.has(tag.id)) {
              userTagMap.set(tag.id, tag)
            }
          }
          const nextUserTags = selectedIds.map((id) => userTagMap.get(id)).filter(Boolean)
          const nonUserTags = existingTags.filter((tag) => !isUserJavTag(tag))
          return { ...item, tags: [...nonUserTags, ...nextUserTags] }
        })
        return { javItems: next }
      })
    } catch (err) {
      console.error('update jav tags failed', err)
    } finally {
      setJavTagPickerItem(null)
      setJavTagPickerSelected([])
    }
  }

  const handleTagPickerClose = () => {
    setTagPickerFor(null)
    setTagPickerSelected([])
  }

  const handleJavTagPickerClose = () => {
    setJavTagPickerItem(null)
    setJavTagPickerSelected([])
  }

  const handleTagPickerToggle = (tagId, checked) => {
    setTagPickerSelected((prev) => {
      const set = new Set(prev)
      if (checked) set.add(String(tagId))
      else set.delete(String(tagId))
      return Array.from(set)
    })
  }

  const handleJavTagPickerToggle = (tagId, checked) => {
    setJavTagPickerSelected((prev) => {
      const set = new Set(prev)
      if (checked) set.add(String(tagId))
      else set.delete(String(tagId))
      return Array.from(set)
    })
  }

  const handleSelectionTagsClose = () => {
    setSelectionTagsOpen(false)
    setSelectionTagAction('add')
    setSelectionTagChoices([])
  }

  const handleSelectionTagChoiceToggle = (tagId, checked) => {
    setSelectionTagChoices((prev) => {
      const set = new Set(prev)
      if (checked) set.add(String(tagId))
      else set.delete(String(tagId))
      return Array.from(set)
    })
  }

  const handleApplySelectionTags = async () => {
    const ids = selectionTagChoices.map((t) => Number(t)).filter(Boolean)
    const selectedKeys = Array.from(selectedVideoIds)
    const vidIds = Array.from(
      new Set(
        selectedKeys
          .map((key) => {
            const meta = selectedVideoMeta?.[key]
            const raw = meta && typeof meta === 'object' ? meta.video_id : key
            const parsed = Number(raw)
            return Number.isFinite(parsed) && parsed > 0 ? parsed : null
          })
          .filter(Boolean)
      )
    )
    try {
      if (selectionTagAction === 'remove') {
        await Promise.all(ids.map((tid) => removeTagFromVideos(tid, vidIds)))
        const removedIds = new Set(ids)
        useStore.setState(({ videos }) => {
          const next = videos.map((v) => {
            if (!vidIds.includes(v.id)) return v
            const existing = Array.isArray(v.tags) ? v.tags : []
            const nextTags = existing.filter((tag) => !removedIds.has(tag.id))
            return nextTags.length === existing.length ? v : { ...v, tags: nextTags }
          })
          return { videos: next }
        })
      } else {
        await Promise.all(ids.map((tid) => addTagToVideos(tid, vidIds)))
        const addedTags = tags.filter((t) => ids.includes(t.id))
        useStore.setState(({ videos }) => {
          const next = videos.map((v) => {
            if (!vidIds.includes(v.id)) return v
            const existing = Array.isArray(v.tags) ? v.tags : []
            const mergedById = new Map()
            for (const tag of existing) mergedById.set(tag.id, tag)
            for (const tag of addedTags) mergedById.set(tag.id, tag)
            return { ...v, tags: Array.from(mergedById.values()) }
          })
          return { videos: next }
        })
      }
    } catch (err) {
      console.error(`${selectionTagAction} tags for selection failed`, err)
    } finally {
      setSelectionTagsOpen(false)
      setSelectionTagAction('add')
      setSelectionTagChoices([])
      setSelectionOpsOpen(false)
      clearSelection()
    }
  }

  const handleHomeClick = () => {
    setTagModalOpen(false)
    setVideoSettingsOpen(false)
    setJavSettingsOpen(false)
    setGlobalSettingsOpen(false)
    if (isJavMode) {
      useStore.setState({
        viewMode: 'jav',
        javTab: 'list',
        videoTempSort: '',
        javTempSort: '',
        javRandomMode: false,
        javRandomSeed: null,
        javIdolIds: [],
        javTags: [],
        javStudioId: null,
        javStudioName: '',
        javSeriesId: null,
        javSeriesName: '',
        idolFavoriteGroupId: null,
        javSearchTerm: '',
        javPage: 1,
        idolPage: 1,
        studioPage: 1,
        seriesPage: 1,
      })
      setJavSearchInput('')
      forceReloadJavByTab('list')
    } else {
      useStore.setState({
        viewMode: 'video',
        videoTempSort: '',
        randomMode: false,
        randomSeed: null,
        selectedTags: [],
        searchTerm: '',
        page: 1,
        selectedVideoIds: new Set(),
        selectedVideoMeta: {},
      })
      setSearchInput('')
      forceReloadVideos()
    }
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  const handleSwitchToJav = () => {
    const targetTab =
      javTab === 'idol' || javTab === 'studio' || javTab === 'series' ? javTab : 'list'
    saveScrollBeforeUrlStateChange()
    useStore.setState({ viewMode: 'jav', videoTempSort: '', javTab: targetTab, javTempSort: '' })
    forceReloadJavByTab(targetTab)
  }

  const handleSwitchJavTab = (tab) => {
    const nextTab =
      tab === 'idol' ? 'idol' : tab === 'studio' ? 'studio' : tab === 'series' ? 'series' : 'list'
    const shouldResetRandomList = nextTab === 'list' && javRandomMode
    const shouldClearSearch = nextTab === 'list' || nextTab !== javTab || shouldResetRandomList
    const nextRandomMode = nextTab === 'list' && !shouldResetRandomList ? javRandomMode : false
    const nextRandomSeed = nextTab === 'list' && !shouldResetRandomList ? javRandomSeed : null
    const updates = {
      javTab: nextTab,
      javTempSort: '',
      javIdolIds: [],
      javTags: [],
      javStudioId: null,
      javStudioName: '',
      javSeriesId: null,
      javSeriesName: '',
      idolFavoriteGroupId: null,
      javRandomMode: nextRandomMode,
      javRandomSeed: nextRandomSeed,
      javPage: 1,
      idolPage: 1,
      studioPage: 1,
      seriesPage: 1,
    }
    if (shouldClearSearch) {
      updates.javSearchTerm = ''
      setJavSearchInput('')
    }
    saveScrollBeforeUrlStateChange()
    useStore.setState(updates)
    forceReloadJavByTab(nextTab)
  }

  const handleToggleMode = () => {
    if (isJavMode) {
      saveScrollBeforeUrlStateChange()
      setViewMode('video')
      forceReloadVideos()
    } else {
      handleSwitchToJav()
    }
  }

  const handleSelectIdol = (idol) => {
    const id = Number(idol?.id)
    if (!Number.isFinite(id) || id <= 0) return
    saveScrollBeforeUrlStateChange()
    useStore.setState({
      viewMode: 'jav',
      videoTempSort: '',
      javTab: 'list',
      javTempSort: '',
      javRandomMode: false,
      javRandomSeed: null,
      javIdolIds: [id],
      javTags: [],
      javStudioId: null,
      javStudioName: '',
      javSeriesId: null,
      javSeriesName: '',
      idolFavoriteGroupId: null,
      javSearchTerm: '',
      javPage: 1,
      idolPage: 1,
      studioPage: 1,
      seriesPage: 1,
    })
  }

  const handleOpenIdolFavoriteModal = useCallback(
    async (idol) => {
      const id = Number(idol?.id)
      if (!Number.isFinite(id) || id <= 0) return
      setIdolFavoriteModalItem(idol)
      setIdolFavoriteSelectedIds([])
      setIdolFavoriteModalError('')
      setIdolFavoriteModalOpen(true)
      setIdolFavoriteModalLoading(true)
      try {
        const [selectedIds] = await Promise.all([
          fetchJavIdolFavoriteSelection(id),
          loadJavIdolFavoriteGroups({ force: true }),
        ])
        setIdolFavoriteSelectedIds(
          (selectedIds || []).map((value) => Number(value)).filter((value) => value > 0)
        )
      } catch (err) {
        setIdolFavoriteModalError(
          err.message || zh('加载女优收藏夹失败', 'Failed to load idol favorites')
        )
      } finally {
        setIdolFavoriteModalLoading(false)
      }
    },
    [loadJavIdolFavoriteGroups]
  )

  const handleCloseIdolFavoriteModal = useCallback(() => {
    if (idolFavoriteModalSaving) return
    setIdolFavoriteModalOpen(false)
    setIdolFavoriteModalItem(null)
    setIdolFavoriteSelectedIds([])
    setIdolFavoriteModalError('')
    setIdolFavoriteModalLoading(false)
  }, [idolFavoriteModalSaving])

  const handleCreateIdolFavoriteGroup = useCallback(async (name) => {
    const group = await createJavIdolFavoriteGroup(name)
    useStore.setState((state) => {
      const current = Array.isArray(state.idolFavoriteGroups) ? state.idolFavoriteGroups : []
      const exists = current.some((item) => Number(item?.id) === Number(group?.id))
      const next = exists ? current : [...current, { ...group, count: group?.count || 0 }]
      next.sort((a, b) => {
        const orderA = Number(a?.sort_order) || 0
        const orderB = Number(b?.sort_order) || 0
        if (orderA !== orderB) return orderA - orderB
        return String(a?.name || '').localeCompare(String(b?.name || ''))
      })
      return { idolFavoriteGroups: next }
    })
    return group
  }, [])

  const handleSaveIdolFavoriteGroups = useCallback(
    async (groupIds) => {
      const idolID = Number(idolFavoriteModalItem?.id)
      if (!Number.isFinite(idolID) || idolID <= 0) return
      setIdolFavoriteModalSaving(true)
      setIdolFavoriteModalError('')
      try {
        await replaceJavIdolFavoriteGroups(idolID, groupIds)
        setIdolFavoriteModalOpen(false)
        setIdolFavoriteModalItem(null)
        setIdolFavoriteSelectedIds([])
        await Promise.all([
          loadJavIdolFavoriteGroups({ force: true }),
          isJavMode && javTab === 'idol' ? loadJavIdols({ force: true }) : Promise.resolve(),
        ])
      } catch (err) {
        setIdolFavoriteModalError(
          err.message || zh('保存女优收藏夹失败', 'Failed to save idol favorites')
        )
      } finally {
        setIdolFavoriteModalSaving(false)
      }
    },
    [idolFavoriteModalItem, isJavMode, javTab, loadJavIdolFavoriteGroups, loadJavIdols]
  )

  const handleReorderIdolFavoriteGroups = useCallback(
    async (groupIds) => {
      await reorderJavIdolFavoriteGroups(groupIds)
      await loadJavIdolFavoriteGroups({ force: true })
    },
    [loadJavIdolFavoriteGroups]
  )

  const handleRenameIdolFavoriteGroup = useCallback(
    async (groupId, name) => {
      await renameJavIdolFavoriteGroup(groupId, name)
      useStore.setState((state) => ({
        idolFavoriteGroups: (state.idolFavoriteGroups || []).map((group) =>
          Number(group.id) === Number(groupId) ? { ...group, name } : group
        ),
      }))
      await loadJavIdolFavoriteGroups({ force: true })
    },
    [loadJavIdolFavoriteGroups]
  )

  const handleDeleteIdolFavoriteGroup = useCallback(
    async (groupId) => {
      await deleteJavIdolFavoriteGroup(groupId)
      if (Number(idolFavoriteGroupId) === Number(groupId)) {
        setIdolFavoriteGroupId(null)
      }
      await Promise.all([
        loadJavIdolFavoriteGroups({ force: true }),
        isJavMode && javTab === 'idol' ? loadJavIdols({ force: true }) : Promise.resolve(),
      ])
    },
    [
      idolFavoriteGroupId,
      isJavMode,
      javTab,
      loadJavIdolFavoriteGroups,
      loadJavIdols,
      setIdolFavoriteGroupId,
    ]
  )

  const handleLoadIdolFavoriteGroupIdols = useCallback(
    (groupId) => fetchJavIdolFavoriteGroupIdols(groupId, { directoryIds: javQueryDirectoryIds }),
    [javQueryDirectoryIds]
  )

  const handleReorderIdolFavoriteGroupIdols = useCallback(
    async (groupId, idolIds) => {
      await reorderJavIdolFavoriteGroupIdols(groupId, idolIds)
      if (Number(idolFavoriteGroupId) === Number(groupId) && isJavMode && javTab === 'idol') {
        await loadJavIdols({ force: true })
      }
    },
    [idolFavoriteGroupId, isJavMode, javTab, loadJavIdols]
  )

  const handleRemoveIdolFavoriteGroupIdols = useCallback(
    async (groupId, idolIds) => {
      await removeJavIdolFavoriteGroupIdols(groupId, idolIds)
      await loadJavIdolFavoriteGroups({ force: true })
      if (Number(idolFavoriteGroupId) === Number(groupId) && isJavMode && javTab === 'idol') {
        await loadJavIdols({ force: true })
      }
    },
    [idolFavoriteGroupId, isJavMode, javTab, loadJavIdolFavoriteGroups, loadJavIdols]
  )

  const handleJavIdolClick = useCallback(
    (idol) => {
      const id = Number(idol?.id ?? idol)
      if (!Number.isFinite(id) || id <= 0) return
      saveScrollBeforeUrlStateChange()
      useStore.setState({
        viewMode: 'jav',
        videoTempSort: '',
        javTab: 'list',
        javTempSort: '',
        javRandomMode: false,
        javRandomSeed: null,
        javIdolIds: [id],
        javTags: [],
        javStudioId: null,
        javStudioName: '',
        javSeriesId: null,
        javSeriesName: '',
        idolFavoriteGroupId: null,
        javSearchTerm: '',
        javPage: 1,
        idolPage: 1,
        studioPage: 1,
        seriesPage: 1,
      })
    },
    [saveScrollBeforeUrlStateChange]
  )

  const handleSelectStudio = (studio) => {
    const id = Number(studio?.id)
    if (!Number.isFinite(id) || id <= 0) return
    saveScrollBeforeUrlStateChange()
    useStore.setState({
      viewMode: 'jav',
      videoTempSort: '',
      javTab: 'list',
      javTempSort: '',
      javRandomMode: false,
      javRandomSeed: null,
      javIdolIds: [],
      javTags: [],
      javStudioId: id,
      javStudioName: String(studio?.name || '').trim(),
      javSeriesId: null,
      javSeriesName: '',
      idolFavoriteGroupId: null,
      javSearchTerm: '',
      javPage: 1,
      idolPage: 1,
      studioPage: 1,
      seriesPage: 1,
    })
  }

  const handleSelectSeries = (series) => {
    const id = Number(series?.id)
    if (!Number.isFinite(id) || id <= 0) return
    saveScrollBeforeUrlStateChange()
    useStore.setState({
      viewMode: 'jav',
      videoTempSort: '',
      javTab: 'list',
      javTempSort: '',
      javRandomMode: false,
      javRandomSeed: null,
      javIdolIds: [],
      javTags: [],
      javStudioId: null,
      javStudioName: '',
      javSeriesId: id,
      javSeriesName: String(series?.name || '').trim(),
      idolFavoriteGroupId: null,
      javSearchTerm: '',
      javPage: 1,
      idolPage: 1,
      studioPage: 1,
      seriesPage: 1,
    })
  }

  const handleJavTagClick = useCallback(
    (tag) => {
      const raw = typeof tag === 'object' ? tag?.id : tag
      const parsed = Number.parseInt(String(raw), 10)
      if (!Number.isFinite(parsed) || parsed <= 0) return
      applyJavTagFilter([parsed])
    },
    [applyJavTagFilter]
  )

  const handleOpenJavTagModal = useCallback(() => {
    setJavTagModalOpen(true)
    loadJavTags()
  }, [loadJavTags])

  const handleApplyJavQuery = useCallback(
    (query) => {
      const nextSearch = String(query?.search || '').trim()
      const nextIdolIds = Array.from(
        new Set(
          (query?.idolIds || [])
            .map((id) => Number(id))
            .filter((id) => Number.isFinite(id) && id > 0)
        )
      )
      const nextTags = Array.from(
        new Set(
          (query?.tagIds || [])
            .map((id) => Number(id))
            .filter((id) => Number.isFinite(id) && id > 0)
        )
      )
      const nextStudioId = Number(query?.studio?.id)
      const hasStudio = Number.isFinite(nextStudioId) && nextStudioId > 0
      const nextStudioName = hasStudio ? String(query?.studio?.name || '').trim() : ''
      const nextSeriesId = Number(query?.series?.id)
      const hasSeries = Number.isFinite(nextSeriesId) && nextSeriesId > 0
      const nextSeriesName = hasSeries ? String(query?.series?.name || '').trim() : ''
      saveScrollBeforeUrlStateChange()
      useStore.setState({
        viewMode: 'jav',
        videoTempSort: '',
        javTab: 'list',
        javTempSort: '',
        javRandomMode: false,
        javRandomSeed: null,
        javSearchTerm: nextSearch,
        javIdolIds: nextIdolIds,
        javTags: nextTags,
        javStudioId: hasStudio ? nextStudioId : null,
        javStudioName: nextStudioName,
        javSeriesId: hasSeries ? nextSeriesId : null,
        javSeriesName: nextSeriesName,
        idolFavoriteGroupId: null,
        javPage: 1,
        idolPage: 1,
        studioPage: 1,
        seriesPage: 1,
      })
      setJavSearchInput(nextSearch)
      setJavQueryEditorOpen(false)
    },
    [saveScrollBeforeUrlStateChange]
  )

  const handleToggleSelectPage = useCallback(() => {
    if (!Array.isArray(videos) || videos.length === 0) return
    useStore.setState((state) => {
      const pageKeys = videos.map((video) => videoSelectionKey(video)).filter(Boolean)
      if (pageKeys.length === 0) return {}
      const nextIds = new Set(state.selectedVideoIds)
      const nextMeta = { ...state.selectedVideoMeta }
      const allSelected = pageKeys.every((key) => nextIds.has(key))
      if (allSelected) {
        pageKeys.forEach((key) => {
          nextIds.delete(key)
          delete nextMeta[key]
        })
      } else {
        videos.forEach((video) => {
          const key = videoSelectionKey(video)
          if (!video?.id || !key) return
          nextIds.add(key)
          nextMeta[key] = {
            label: video.filename || video.path || `#${video.id}`,
            video_id: video.id,
            location_id: video.location_id || null,
          }
        })
      }
      return { selectedVideoIds: nextIds, selectedVideoMeta: nextMeta }
    })
  }, [videos])

  const activeError = isJavMode
    ? javTab === 'idol'
      ? idolError
      : javTab === 'studio'
        ? studioError
        : javTab === 'series'
          ? seriesError
          : javError
    : error
  const showDirectorySetupHint =
    hydrated &&
    configLoaded &&
    !loading &&
    !activeError &&
    Array.isArray(directories) &&
    directories.length === 0 &&
    Array.isArray(videos) &&
    videos.length === 0

  const activeJavLoading =
    javTab === 'idol'
      ? idolLoading
      : javTab === 'studio'
        ? studioLoading
        : javTab === 'series'
          ? seriesLoading
          : javLoading
  const activeLoadingMore = isJavMode
    ? javTab === 'idol'
      ? idolLoadingMore
      : javTab === 'studio'
        ? studioLoadingMore
        : javTab === 'series'
          ? seriesLoadingMore
          : javLoadingMore
    : videoLoadingMore
  useEffect(() => {
    if (!hydrated || !configLoaded || !pendingScrollRestoreRef.current) return
    if ((isJavMode ? activeJavLoading : loading) || activeLoadingMore) return

    const pending = pendingScrollRestoreRef.current
    const maxY = Math.max(
      0,
      Math.max(document.documentElement.scrollHeight, document.body.scrollHeight) -
        window.innerHeight
    )
    const needsMoreContent = (pending.y || 0) > maxY + 24
    if (needsMoreContent) {
      if (!isJavMode && waterfallModes.video && videoWaterfallHasMore) {
        loadMoreVideos()
        return
      }
      if (isJavMode && javTab === 'list' && waterfallModes.jav && javWaterfallHasMore) {
        loadMoreJavs()
        return
      }
      if (isJavMode && javTab === 'idol' && waterfallModes.idol && idolWaterfallHasMore) {
        loadMoreJavIdols()
        return
      }
      if (isJavMode && javTab === 'studio' && waterfallModes.studio && studioWaterfallHasMore) {
        loadMoreJavStudios()
        return
      }
      if (isJavMode && javTab === 'series' && waterfallModes.series && seriesWaterfallHasMore) {
        loadMoreJavSeries()
        return
      }
    }

    schedulePendingScrollRestore()
  }, [
    activeJavLoading,
    activeLoadingMore,
    configLoaded,
    hydrated,
    idolItems.length,
    idolWaterfallHasMore,
    isJavMode,
    javItems.length,
    javTab,
    javWaterfallHasMore,
    loadMoreJavIdols,
    loadMoreJavSeries,
    loadMoreJavStudios,
    loadMoreJavs,
    loadMoreVideos,
    loading,
    schedulePendingScrollRestore,
    seriesItems.length,
    seriesWaterfallHasMore,
    studioItems.length,
    studioWaterfallHasMore,
    videoWaterfallHasMore,
    videos.length,
    waterfallModes.idol,
    waterfallModes.jav,
    waterfallModes.series,
    waterfallModes.studio,
    waterfallModes.video,
  ])
  const javVideoPickerTitle =
    javVideoPickerAction === 'open'
      ? alternatePlayer === 'mpv'
        ? zh('选择使用MPV播放器播放的文件', 'Choose a file to play with MPV player')
        : zh('选择使用系统播放器播放的文件', 'Choose a file to play with system player')
      : javVideoPickerAction === 'screenshots'
        ? zh('选择查看截图的文件', 'Choose a file to view screenshots')
        : javVideoPickerAction === 'reveal'
          ? zh('选择定位文件', 'Choose a file to reveal')
          : defaultPlayer === 'system'
            ? zh('选择使用系统播放器播放的文件', 'Choose a file to play with system player')
            : zh('选择使用MPV播放器播放的文件', 'Choose a file to play with MPV player')
  const javVideoPickerEmptyText =
    javVideoPickerAction === 'play'
      ? zh('暂无可播放文件', 'No playable files')
      : javVideoPickerAction === 'screenshots'
        ? zh('暂无可查看截图的文件', 'No files with screenshots available')
        : zh('暂无可用文件', 'No available files')

  return (
    <div className="min-h-screen">
      <TopBar
        onHome={handleHomeClick}
        canGoBack={browserNavigation.canGoBack}
        canGoForward={browserNavigation.canGoForward}
        onBrowserBack={handleBrowserBack}
        onBrowserForward={handleBrowserForward}
        isJavMode={isJavMode}
        onToggleMode={handleToggleMode}
        videoSearchInput={searchInput}
        onVideoSearchInputChange={setSearchInput}
        onSubmitVideoSearch={submitSearch}
        videoSearchHref={searchHref}
        randomMode={randomMode}
        randomHref={randomHref}
        onRandomClick={handleVideoRandomClick}
        onOpenTagModal={handleOpenTagModal}
        onOpenJavTagModal={handleOpenJavTagModal}
        onOpenVideoSettings={openVideoSettings}
        onOpenJavSettings={openJavSettings}
        onOpenGlobalSettings={() => setGlobalSettingsOpen(true)}
        javSearchInput={javSearchInput}
        onJavSearchInputChange={setJavSearchInput}
        onSubmitJavSearch={submitJavSearch}
        javSearchHref={javSearchHref}
        javRandomHref={javRandomHref}
        javRandomMode={javRandomMode}
        onJavRandomClick={handleJavRandomClick}
        onJavFilterRandomClick={handleJavFilterRandomClick}
        onCancelJavFilterRandom={handleCancelJavFilterRandom}
        showJavFilterRandomButton={showJavFilterRandomButton}
        isModifiedClick={isModifiedClick}
        javTab={javTab}
        onSwitchJavTab={handleSwitchJavTab}
        filterSummary={filterSummary}
        onOpenJavQueryEditor={() => {
          setJavQueryEditorOpen(true)
          loadJavTags()
        }}
        showDirectorySetupHint={showDirectorySetupHint}
        directories={directories}
        enabledDirectoryIds={enabledDirectoryIds}
        onEnabledDirectoryIdsChange={setEnabledDirectoryIds}
      />

      <main className="mx-auto max-w-screen-2xl px-6 pb-6 pt-0">
        {activeError && (
          <div
            role="alert"
            className="mb-4 rounded border border-red-200 bg-red-50 p-3 text-red-700"
          >
            {String(activeError)}
          </div>
        )}

        {isJavMode ? (
          javTab === 'idol' ? (
            <JavIdolView
              page={idolPage}
              lastPage={idolLastPage}
              hasPrev={idolHasPrev}
              hasNext={idolHasNext}
              loading={idolLoading}
              buildPageUrl={({ page: targetPage }) =>
                buildJavUrl({ page: targetPage, tab: 'idol' })
              }
              buildGroupUrl={(groupId) =>
                buildJavUrl({ page: 1, tab: 'idol', favoriteGroupId: groupId || null })
              }
              buildIdolUrl={(idol) =>
                buildJavUrl({
                  page: 1,
                  search: '',
                  tab: 'list',
                  idolIds: [idol.id],
                  tagIds: [],
                  tempSort: '',
                })
              }
              onFirst={() => setIdolPage(1)}
              onPrev={() => idolHasPrev && setIdolPage(idolPage - 1)}
              onGoToPage={(p) => setIdolPage(p)}
              onNext={() => idolHasNext && setIdolPage(idolPage + 1)}
              onLast={() => setIdolPage(idolLastPage)}
              items={idolItems}
              favoriteGroups={idolFavoriteGroups}
              selectedFavoriteGroupId={idolFavoriteGroupId}
              favoriteGroupsLoading={idolFavoriteGroupsLoading}
              favoriteGroupsError={idolFavoriteGroupsError}
              javMetadataLanguage={config?.jav_metadata_language === 'en' ? 'en' : 'zh'}
              onSelectIdol={handleSelectIdol}
              onFavoriteGroupSelect={(groupId) => setIdolFavoriteGroupId(groupId)}
              onOpenFavorites={handleOpenIdolFavoriteModal}
              onOpenFavoriteManager={() => setIdolFavoriteManageOpen(true)}
              waterfallMode={waterfallModes.idol}
              onWaterfallModeChange={(enabled) => setWaterfallMode('idol', enabled)}
              onLoadMore={loadMoreJavIdols}
              loadingMore={idolLoadingMore}
              hasMore={idolWaterfallHasMore}
            />
          ) : javTab === 'studio' ? (
            <JavStudioView
              page={studioPage}
              lastPage={studioLastPage}
              hasPrev={studioHasPrev}
              hasNext={studioHasNext}
              loading={studioLoading}
              buildPageUrl={({ page: targetPage }) =>
                buildJavUrl({ page: targetPage, tab: 'studio' })
              }
              buildStudioUrl={(studio) =>
                buildJavUrl({
                  page: 1,
                  search: '',
                  tab: 'list',
                  idolIds: [],
                  tagIds: [],
                  studioId: studio.id,
                  studioName: studio.name,
                  tempSort: '',
                })
              }
              onFirst={() => setStudioPage(1)}
              onPrev={() => studioHasPrev && setStudioPage(studioPage - 1)}
              onGoToPage={(p) => setStudioPage(p)}
              onNext={() => studioHasNext && setStudioPage(studioPage + 1)}
              onLast={() => setStudioPage(studioLastPage)}
              items={studioItems}
              onSelectStudio={handleSelectStudio}
              waterfallMode={waterfallModes.studio}
              onWaterfallModeChange={(enabled) => setWaterfallMode('studio', enabled)}
              onLoadMore={loadMoreJavStudios}
              loadingMore={studioLoadingMore}
              hasMore={studioWaterfallHasMore}
            />
          ) : javTab === 'series' ? (
            <JavSeriesView
              page={seriesPage}
              lastPage={seriesLastPage}
              hasPrev={seriesHasPrev}
              hasNext={seriesHasNext}
              loading={seriesLoading}
              buildPageUrl={({ page: targetPage }) =>
                buildJavUrl({ page: targetPage, tab: 'series' })
              }
              buildSeriesUrl={(series) =>
                buildJavUrl({
                  page: 1,
                  search: '',
                  tab: 'list',
                  idolIds: [],
                  tagIds: [],
                  studioId: null,
                  seriesId: series.id,
                  seriesName: series.name,
                  tempSort: '',
                })
              }
              onFirst={() => setSeriesPage(1)}
              onPrev={() => seriesHasPrev && setSeriesPage(seriesPage - 1)}
              onGoToPage={(p) => setSeriesPage(p)}
              onNext={() => seriesHasNext && setSeriesPage(seriesPage + 1)}
              onLast={() => setSeriesPage(seriesLastPage)}
              items={seriesItems}
              onSelectSeries={handleSelectSeries}
              onSelectStudio={handleSelectStudio}
              waterfallMode={waterfallModes.series}
              onWaterfallModeChange={(enabled) => setWaterfallMode('series', enabled)}
              onLoadMore={loadMoreJavSeries}
              loadingMore={seriesLoadingMore}
              hasMore={seriesWaterfallHasMore}
            />
          ) : (
            <JavView
              javPage={javPage}
              javLastPage={javLastPage}
              javHasPrev={javHasPrev}
              javHasNext={javHasNext}
              javLoading={activeJavLoading}
              javRandomMode={javRandomMode}
              javTempSort={javTempSort}
              javGlobalSort={javSort}
              buildJavUrl={buildJavUrl}
              setJavPage={setJavPage}
              setJavTempSort={setJavTempSort}
              javItems={javItems}
              javGridColumns={javGridColumns}
              javTitleMaxRows={javTitleMaxRows}
              javIdolTagMaxRows={javIdolTagMaxRows}
              javTagMaxRows={javTagMaxRows}
              onPlay={handleJavPlay}
              onOpenFile={handleJavOpenFile}
              openFileLabel={alternatePlayerLabel}
              onRevealFile={handleJavRevealFile}
              onOpenScreenshots={handleJavOpenScreenshots}
              onIdolClick={handleJavIdolClick}
              onOpenFavorites={handleOpenIdolFavoriteModal}
              onStudioClick={handleSelectStudio}
              onSeriesClick={handleSelectSeries}
              onTagClick={handleJavTagClick}
              onEditTags={openJavTagEditor}
              waterfallMode={waterfallModes.jav}
              onWaterfallModeChange={(enabled) => setWaterfallMode('jav', enabled)}
              onLoadMore={loadMoreJavs}
              loadingMore={javLoadingMore}
              hasMore={javWaterfallHasMore}
            />
          )
        ) : (
          <VideoView
            selectedCount={selectedCount}
            clearSelection={clearSelection}
            setSelectionOpsOpen={setSelectionOpsOpen}
            page={page}
            lastPage={lastPage}
            canPrev={canPrev}
            canNext={canNext}
            loading={loading}
            randomMode={randomMode}
            videoTempSort={videoTempSort}
            videoGlobalSort={sortOrder}
            buildVideoUrl={buildVideoUrl}
            setPage={navigateVideoPage}
            setVideoTempSort={setVideoTempSort}
            goToLastPage={() => navigateVideoPage(lastPage)}
            videos={videos}
            selectedVideoIds={selectedVideoIds}
            toggleSelectVideo={toggleSelectVideo}
            onToggleSelectPage={handleToggleSelectPage}
            openPlayer={handleOpenPlayer}
            openAlternatePlayer={handleOpenAlternatePlayer}
            revealFile={handleRevealVideoFile}
            alternatePlayerLabel={alternatePlayerLabel}
            setTagPickerFor={openTagEditor}
            onOpenScreenshots={setScreenshotsVideo}
            onRenameVideo={handleRenameVideo}
            onDeleteVideo={handleDeleteVideo}
            onTagClick={handleVideoTagClick}
            waterfallMode={waterfallModes.video}
            onWaterfallModeChange={(enabled) => setWaterfallMode('video', enabled)}
            onLoadMore={loadMoreVideos}
            loadingMore={videoLoadingMore}
            hasMore={videoWaterfallHasMore}
          />
        )}
      </main>

      <JavQueryEditorModal
        open={javQueryEditorOpen}
        onClose={() => setJavQueryEditorOpen(false)}
        onApply={handleApplyJavQuery}
        search={javSearchTerm}
        idolIds={javIdolIds}
        idolOptions={javIdolFilterOptions}
        tagIds={javTags}
        tagOptions={javTagOptions}
        studioId={javStudioId}
        studioName={javStudioName}
        seriesId={javSeriesId}
        seriesName={javSeriesName}
        directoryIds={javQueryDirectoryIds}
      />

      <VideoSettingsModal
        open={videoSettingsOpen}
        onClose={() => setVideoSettingsOpen(false)}
        pageSizeInput={videoPageSizeInput}
        onPageSizeChange={setVideoPageSizeInput}
        sortInput={videoSortInput}
        onSortChange={setVideoSortInput}
        hideJavInput={videoHideJavInput}
        onHideJavChange={setVideoHideJavInput}
        onSave={handleSaveVideoSettings}
      />

      <VideoScreenshotsModal
        video={screenshotsVideo}
        playerHotkeys={config?.player_hotkeys}
        onClose={() => setScreenshotsVideo(null)}
        onPlayAtTime={playVideoFromTime}
      />

      <JavSettingsModal
        open={javSettingsOpen}
        onClose={() => setJavSettingsOpen(false)}
        javPageSizeInput={javPageSizeInput}
        onJavPageSizeChange={setJavPageSizeInput}
        javGridColumnsInput={javGridColumnsInput}
        onJavGridColumnsChange={setJavGridColumnsInput}
        javTitleMaxRowsInput={javTitleMaxRowsInput}
        onJavTitleMaxRowsChange={setJavTitleMaxRowsInput}
        javIdolTagMaxRowsInput={javIdolTagMaxRowsInput}
        onJavIdolTagMaxRowsChange={setJavIdolTagMaxRowsInput}
        javTagMaxRowsInput={javTagMaxRowsInput}
        onJavTagMaxRowsChange={setJavTagMaxRowsInput}
        idolPageSizeInput={idolPageSizeInput}
        onIdolPageSizeChange={setIdolPageSizeInput}
        javSortInput={javSortInput}
        onJavSortChange={setJavSortInput}
        idolSortInput={idolSortInput}
        onIdolSortChange={setIdolSortInput}
        onSave={handleSaveJavSettings}
      />

      <JavVideoPickerModal
        open={javVideoPickerOpen}
        title={javVideoPickerTitle}
        onClose={closeJavVideoPicker}
        item={javVideoPickerItem}
        choices={javVideoChoices}
        emptyText={javVideoPickerEmptyText}
        action={javVideoPickerAction}
        buildVideoFullPath={buildVideoFullPath}
        isVideoOpenable={isVideoOpenable}
        onSelectVideo={handleSelectJavVideo}
        javMetadataLanguage={config?.jav_metadata_language === 'en' ? 'en' : 'zh'}
      />

      <JavIdolFavoriteModal
        open={idolFavoriteModalOpen}
        idol={idolFavoriteModalItem}
        groups={idolFavoriteGroups}
        selectedIds={idolFavoriteSelectedIds}
        loading={idolFavoriteModalLoading || idolFavoriteGroupsLoading}
        saving={idolFavoriteModalSaving}
        error={idolFavoriteModalError || idolFavoriteGroupsError || ''}
        onClose={handleCloseIdolFavoriteModal}
        onCreateGroup={handleCreateIdolFavoriteGroup}
        onSave={handleSaveIdolFavoriteGroups}
      />

      <JavIdolFavoriteManageModal
        open={idolFavoriteManageOpen}
        groups={idolFavoriteGroups}
        selectedGroupId={idolFavoriteGroupId}
        loading={idolFavoriteGroupsLoading}
        onClose={() => setIdolFavoriteManageOpen(false)}
        onCreateGroup={handleCreateIdolFavoriteGroup}
        onReorderGroups={handleReorderIdolFavoriteGroups}
        onRenameGroup={handleRenameIdolFavoriteGroup}
        onDeleteGroup={handleDeleteIdolFavoriteGroup}
        onLoadGroupIdols={handleLoadIdolFavoriteGroupIdols}
        onReorderGroupIdols={handleReorderIdolFavoriteGroupIdols}
        onRemoveGroupIdols={handleRemoveIdolFavoriteGroupIdols}
      />

      <JavVideoPickerModal
        open={locationPickerOpen}
        title={
          locationPickerAction === 'reveal'
            ? zh('选择定位文件', 'Choose a file to reveal')
            : locationPickerAction === 'open'
              ? alternatePlayer === 'mpv'
                ? zh('选择使用MPV播放器播放的文件', 'Choose a file to play with MPV player')
                : zh('选择使用系统播放器播放的文件', 'Choose a file to play with system player')
              : defaultPlayer === 'system'
                ? zh('选择使用系统播放器播放的文件', 'Choose a file to play with system player')
                : zh('选择使用MPV播放器播放的文件', 'Choose a file to play with MPV player')
        }
        onClose={closeLocationPicker}
        item={locationPickerItem}
        choices={locationPickerChoices}
        emptyText={zh('暂无可用文件', 'No available files')}
        action={locationPickerAction}
        buildVideoFullPath={buildVideoFullPath}
        isVideoOpenable={isVideoOpenable}
        onSelectVideo={handleSelectVideoLocation}
      />

      <SelectionOpsModal
        open={selectionOpsOpen}
        onClose={() => setSelectionOpsOpen(false)}
        selectedList={selectedList}
        selectedCount={selectedCount}
        onOpenTags={() => {
          loadTags()
          setSelectionTagAction('add')
          setSelectionTagChoices([])
          setSelectionOpsOpen(false)
          setSelectionTagsOpen(true)
        }}
        onOpenRemoveTags={() => {
          loadTags()
          setSelectionTagAction('remove')
          setSelectionTagChoices([])
          setSelectionOpsOpen(false)
          setSelectionTagsOpen(true)
        }}
      />

      <SelectionTagsModal
        open={selectionTagsOpen}
        onClose={handleSelectionTagsClose}
        tags={tags}
        action={selectionTagAction}
        selectedChoices={selectionTagChoices}
        onToggleChoice={handleSelectionTagChoiceToggle}
        onConfirm={handleApplySelectionTags}
        confirmDisabled={!selectionTagChoices.length || selectedVideoIds.size === 0}
      />

      <TagPickerModal
        open={Boolean(tagPickerFor)}
        tags={tags}
        selectedIds={tagPickerSelected}
        onToggleChoice={handleTagPickerToggle}
        onClose={handleTagPickerClose}
        onSave={handleApplyTags}
        saveDisabled={!tagPickerDirty}
      />
      <TagPickerModal
        open={Boolean(javTagPickerItem)}
        tags={javUserTagOptions}
        selectedIds={javTagPickerSelected}
        onToggleChoice={handleJavTagPickerToggle}
        onClose={handleJavTagPickerClose}
        onSave={handleApplyJavTags}
        saveDisabled={!javTagPickerDirty}
      />

      <VideoTagModal
        open={tagModalOpen}
        onClose={() => setTagModalOpen(false)}
        tags={tags}
        onToggleFilter={(name) => toggleTagFilter(name)}
        onCreateTag={async (name) => {
          await createTag(name)
          await loadTags()
        }}
        onRenameTag={async (id, name) => {
          const oldName = tags.find((t) => t.id === id)?.name || ''
          await renameTag(id, name)
          useStore.setState((state) => {
            const nextTags = Array.isArray(state.tags)
              ? state.tags.map((tag) => (tag.id === id ? { ...tag, name } : tag))
              : state.tags
            const nextVideos = Array.isArray(state.videos)
              ? state.videos.map((video) => {
                  if (!Array.isArray(video.tags)) return video
                  const nextVideoTags = video.tags.map((tag) =>
                    tag.id === id ? { ...tag, name } : tag
                  )
                  return nextVideoTags === video.tags ? video : { ...video, tags: nextVideoTags }
                })
              : state.videos
            const nextSelectedTags =
              oldName && Array.isArray(state.selectedTags)
                ? state.selectedTags.map((tagName) => (tagName === oldName ? name : tagName))
                : state.selectedTags
            return { tags: nextTags, videos: nextVideos, selectedTags: nextSelectedTags }
          })
          await loadTags()
        }}
        onDeleteTag={async (tag) => {
          const id = typeof tag === 'object' ? tag?.id : tag
          if (!id) return
          const name =
            typeof tag === 'object' ? tag?.name : tags.find((item) => item.id === id)?.name || ''
          await deleteTag(id)
          if (name) {
            useStore.setState((state) => ({
              selectedTags: state.selectedTags.filter((tagName) => tagName !== name),
            }))
          }
          await loadTags()
        }}
        onApplyTagFilter={(names) => {
          setSearchTerm('', { resetPage: false, triggerLoad: false })
          setSelectedTags(names)
        }}
      />
      <JavTagModal
        open={javTagModalOpen}
        onClose={() => setJavTagModalOpen(false)}
        tags={javTagOptions}
        onApplyTagFilter={applyJavTagFilter}
        onCreateTag={async (name) => {
          await createJavTag(name)
          await loadJavTags()
        }}
        onRenameTag={async (id, name) => {
          await renameJavTag(id, name)
          useStore.setState((state) => {
            const options = Array.isArray(state.javTagOptions) ? state.javTagOptions : []
            const items = Array.isArray(state.javItems) ? state.javItems : []
            const nextOptions = options.map((tag) => (tag.id === id ? { ...tag, name } : tag))
            const nextItems = items.map((item) => {
              if (!Array.isArray(item.tags)) return item
              const nextTags = item.tags.map((tag) => (tag.id === id ? { ...tag, name } : tag))
              return nextTags === item.tags ? item : { ...item, tags: nextTags }
            })
            return { javTagOptions: nextOptions, javItems: nextItems }
          })
          await loadJavTags()
        }}
        onDeleteTag={async (tag) => {
          const id = typeof tag === 'object' ? tag?.id : tag
          if (!id) return
          await deleteJavTag(id)
          useStore.setState((state) => {
            const nextOptions = Array.isArray(state.javTagOptions)
              ? state.javTagOptions.filter((item) => item.id !== id)
              : state.javTagOptions
            const nextItems = Array.isArray(state.javItems)
              ? state.javItems.map((item) => {
                  if (!Array.isArray(item.tags)) return item
                  const nextTags = item.tags.filter((tagItem) => tagItem.id !== id)
                  return nextTags === item.tags ? item : { ...item, tags: nextTags }
                })
              : state.javItems
            const nextFilters = Array.isArray(state.javTags)
              ? state.javTags.filter((tagId) => tagId !== id)
              : state.javTags
            return { javTagOptions: nextOptions, javItems: nextItems, javTags: nextFilters }
          })
          await loadJavTags()
        }}
      />
      <GlobalSettingsModal
        open={globalSettingsOpen}
        onClose={() => setGlobalSettingsOpen(false)}
        directories={directories}
        enabledDirectoryIds={enabledDirectoryIds}
        onEnabledDirectoryIdsChange={setEnabledDirectoryIds}
        onCreateDirectory={async (payload) => {
          const created = await createDirectory(payload)
          await loadDirectories()
          showToast(
            zh(
              '目录添加成功，首次扫描目录里的视频需要一定时间，请耐心等待，您可手动刷新页面查看扫描进度',
              'Directory added. The first scan may take some time. You can refresh manually to check progress.'
            )
          )
          return created
        }}
        onUpdateDirectory={async (id, payload) => {
          const updated = await updateDirectory(id, payload)
          await loadDirectories()
          return updated
        }}
        onDeleteDirectory={async (id) => {
          const deleted = await deleteDirectory(id)
          await loadDirectories()
          return deleted
        }}
        proxyHost={config?.proxy_host || ''}
        proxyPort={Number.parseInt(config?.proxy_port, 10) || 0}
        onSaveProxySettings={async ({ host, port }) => {
          const cfg = await updateConfig({ proxy_host: host, proxy_port: port })
          useStore.setState({ config: cfg })
        }}
        javMetadataLanguage={config?.jav_metadata_language === 'en' ? 'en' : 'zh'}
        onSaveJavMetadataLanguage={async (language) => {
          const cfg = await updateConfig({
            jav_metadata_language: language === 'en' ? 'en' : 'zh',
          })
          useStore.setState({
            config: cfg,
            javTempSort: '',
            javTags: [],
            javPage: 1,
            javRandomMode: false,
            javRandomSeed: null,
          })
          await loadJavTags({ force: true })
          forceReloadJavByTab(javTab)
        }}
        defaultPlayer={defaultPlayer}
        onSaveDefaultPlayer={async (player) => {
          const cfg = await updateConfig({ default_player: normalizeDefaultPlayer(player) })
          useStore.setState({ config: cfg })
        }}
        initialViewMode={initialViewMode}
        onSaveInitialViewMode={async (mode) => {
          const cfg = await updateConfig({ initial_view_mode: normalizeInitialViewMode(mode) })
          useStore.setState({ config: cfg })
        }}
        playerWindowWidth={
          Number.parseInt(config?.player_window_width, 10) ||
          Number.parseInt(config?.player_window_size, 10) ||
          80
        }
        playerWindowHeight={
          Number.parseInt(config?.player_window_height, 10) ||
          Number.parseInt(config?.player_window_size, 10) ||
          80
        }
        playerWindowUseAutofit={
          config?.player_window_use_autofit == null
            ? false
            : !['0', 'false', 'no', 'off'].includes(
                String(config.player_window_use_autofit).trim().toLowerCase()
              )
        }
        playerOntop={
          config?.player_ontop == null
            ? true
            : !['0', 'false', 'no', 'off'].includes(
                String(config.player_ontop).trim().toLowerCase()
              )
        }
        playerVolume={
          config?.player_volume === '0' ? 0 : Number.parseInt(config?.player_volume, 10) || 70
        }
        playerShowHotkeyHint={
          config?.player_show_hotkey_hint == null
            ? true
            : !['0', 'false', 'no', 'off'].includes(
                String(config.player_show_hotkey_hint).trim().toLowerCase()
              )
        }
        onSavePlayerBasicSettings={async (payload) => {
          const cfg = await updateConfig(payload)
          useStore.setState({ config: cfg })
        }}
        playerHotkeys={config?.player_hotkeys}
        onSavePlayerHotkeys={async (hotkeys) => {
          const cfg = await updateConfig({ player_hotkeys: hotkeys })
          useStore.setState({ config: cfg })
        }}
      />
      <Toast open={Boolean(toastMessage)} message={toastMessage} onClose={closeToast} />
    </div>
  )
}
