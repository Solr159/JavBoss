import { useState, useCallback } from 'react'
import { zh } from '@/utils/i18n'
import usePlaybackCapabilities from '@/features/playback/hooks/usePlaybackCapabilities'
import { canOpenAlternatePlayer } from '@/utils/playbackCapabilities'
import {
  openVideoFile,
  playVideoFile,
  revealVideoLocation,
  playVideoPlaylist,
} from '@/features/video/api'
import { getErrorMessage } from '@/utils/errors'
import { confirmLargeMPVPlaylist } from '@/features/playback/model'
import { useStore } from '@/store'

export default function usePlayback({ showCenterToast, showToast }) {
  const [javVideoPickerOpen, setJavVideoPickerOpen] = useState(false)

  const [javVideoPickerItem, setJavVideoPickerItem] = useState(null)

  const [javVideoPickerAction, setJavVideoPickerAction] = useState('play')

  const [locationPickerOpen, setLocationPickerOpen] = useState(false)

  const [locationPickerVideo, setLocationPickerVideo] = useState(null)

  const [locationPickerChoices, setLocationPickerChoices] = useState([])

  const [locationPickerAction, setLocationPickerAction] = useState('play')

  const [playerVideo, setPlayerVideo] = useState(null)

  const [playerStartTime, setPlayerStartTime] = useState(0)

  const [screenshotsVideo, setScreenshotsVideo] = useState(null)

  const [screenshotsAllowSetCover, setScreenshotsAllowSetCover] = useState(true)

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

  const {
    browserPlaybackOnly,
    remoteAccess,
    clientMode,
    containerMode,
    desktopIntegrationEnabled,
    directoryPickerEnabled,
    mpvEnabled,
    defaultPlayer,
    alternatePlayer,
    alternatePlayerLabel,
  } = usePlaybackCapabilities()
  const ensureMPVPlaylistAvailable = useCallback(() => {
    if (!remoteAccess || clientMode) return true
    showCenterToast(
      zh(
        '非本机访问时无法使用 MPV 批量播放，请使用 client 模式',
        'MPV batch playback is unavailable for remote access. Please use client mode.'
      )
    )
    return false
  }, [remoteAccess, clientMode, showCenterToast])

  const ensureOpenFileAvailable = useCallback(() => {
    if (canOpenAlternatePlayer({ containerMode, clientMode, alternatePlayer })) return true
    showCenterToast(
      zh(
        'Docker 模式不支持用默认程序打开文件，请使用浏览器播放。',
        'Opening files with the default app is unavailable in Docker mode. Use browser playback.'
      )
    )
    return false
  }, [containerMode, clientMode, alternatePlayer, showCenterToast])

  const ensureRevealAvailable = useCallback(() => {
    if (containerMode) {
      showCenterToast(
        zh(
          'Docker 模式不支持打开文件所在位置，请在宿主机中访问该目录。',
          'Revealing file locations is unavailable in Docker mode. Open the directory on the host.'
        )
      )
      return false
    }
    if (!remoteAccess) return true
    showCenterToast(
      zh(
        '通过局域网访问时无法打开文件所在位置',
        'Cannot reveal file locations when accessing over the local network'
      )
    )
    return false
  }, [containerMode, remoteAccess, showCenterToast])

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
      if (!video) return
      if (player === 'browser') {
        setPlayerStartTime(0)
        setPlayerVideo(video)
        return
      }
      const payload = {
        id: video.id,
        locationId: video.location_id,
        path: getVideoRelPath(video),
        dirPath: getVideoDirPath(video),
      }
      const useSystemPlayer = player === 'system'
      const action = useSystemPlayer ? openVideoFile : playVideoFile
      action(payload).catch((err) => {
        console.error(
          useSystemPlayer
            ? zh('打开文件失败', 'Failed to open file')
            : zh('播放文件失败', 'Failed to play file'),
          err
        )
        showCenterToast(getErrorMessage(err))
      })
    },
    [getVideoDirPath, getVideoRelPath, showCenterToast]
  )

  const revealVideoFile = useCallback(
    (video) => {
      if (!ensureRevealAvailable()) return Promise.resolve()
      if (!video || !isVideoOpenable(video)) return Promise.resolve()
      return revealVideoLocation({
        path: getVideoRelPath(video),
        dirPath: getVideoDirPath(video),
      })
    },
    [ensureRevealAvailable, getVideoDirPath, getVideoRelPath, isVideoOpenable]
  )

  const playVideoFromTime = useCallback(
    (video, startTime) => {
      if (!video) return
      if (browserPlaybackOnly || defaultPlayer === 'browser') {
        setPlayerStartTime(startTime || 0)
        setPlayerVideo(video)
        return
      }
      playVideoFile({
        id: video.id,
        locationId: video.location_id,
        path: getVideoRelPath(video),
        dirPath: getVideoDirPath(video),
        startTime,
      }).catch((err) => {
        console.error(zh('播放文件失败', 'Failed to play file'), err)
        showCenterToast(getErrorMessage(err))
      })
    },
    [browserPlaybackOnly, defaultPlayer, getVideoDirPath, getVideoRelPath, showCenterToast]
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
      if (!ensureOpenFileAvailable()) return
      if (!alternatePlayer) return
      const choices = getVideoLocationChoices(video)
      if (choices.length > 1) {
        openLocationPicker(video, 'open', choices)
        return
      }
      playVideoWith(choices[0] || video, alternatePlayer)
    },
    [
      ensureOpenFileAvailable,
      alternatePlayer,
      getVideoLocationChoices,
      openLocationPicker,
      playVideoWith,
    ]
  )

  const handleRevealVideoFile = useCallback(
    (video) => {
      if (!ensureRevealAvailable()) return
      const choices = getVideoLocationChoices(video)
      if (choices.length > 1) {
        openLocationPicker(video, 'reveal', choices)
        return
      }
      revealVideoFile(choices[0] || video).catch((err) => {
        console.error(zh('打开所在位置失败', 'Failed to reveal file'), err)
        showCenterToast(getErrorMessage(err))
      })
    },
    [
      ensureRevealAvailable,
      getVideoLocationChoices,
      openLocationPicker,
      revealVideoFile,
      showCenterToast,
    ]
  )

  const closeJavVideoPicker = useCallback(() => {
    setJavVideoPickerOpen(false)
    setJavVideoPickerItem(null)
    setJavVideoPickerAction('play')
  }, [])

  const playVideosWithMPV = useCallback(
    async (items) => {
      if (!ensureMPVPlaylistAvailable()) return
      const list = Array.isArray(items) ? items : []
      const targets = list
        .map((video) => {
          const videoId = Number(video?.id)
          const locationId = Number(video?.location_id || 0)
          if (!Number.isFinite(videoId) || videoId <= 0) return null
          return {
            video_id: videoId,
            location_id: Number.isFinite(locationId) && locationId > 0 ? locationId : 0,
            title: video?.filename || `Video #${videoId}`,
          }
        })
        .filter(Boolean)
      if (targets.length !== list.length || targets.length === 0) {
        showCenterToast(
          zh(
            '无法播放：部分视频缺少文件信息',
            'Cannot play: some videos are missing file information'
          )
        )
        return
      }
      if (!confirmLargeMPVPlaylist(targets.length)) return

      const result = await playVideoPlaylist(targets)
      const count = Number(result?.count) || targets.length
      showToast(
        zh(`已将 ${count} 个视频加入 MPV 播放列表`, `Added ${count} videos to the MPV playlist`)
      )
      return true
    },
    [ensureMPVPlaylistAvailable, showCenterToast, showToast]
  )

  const handleJavPlay = useCallback(
    (video, item) => {
      const videos = item?.videos || []
      if (videos.length > 1) {
        if (defaultPlayer === 'mpv') {
          playVideosWithMPV(videos).catch((err) => {
            showCenterToast(getErrorMessage(err))
          })
          return
        }
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
    [defaultPlayer, playVideosWithMPV, showCenterToast, handleOpenPlayer]
  )

  const handleJavOpenFile = useCallback(
    (video, item) => {
      if (!ensureOpenFileAvailable()) return
      const videos = item?.videos || (video ? [video] : [])
      if (videos.length > 1) {
        if (alternatePlayer === 'mpv') {
          playVideosWithMPV(videos).catch((err) => {
            showCenterToast(getErrorMessage(err))
          })
          return
        }
        setJavVideoPickerAction('open')
        setJavVideoPickerItem(item)
        setJavVideoPickerOpen(true)
        return
      }
      const target = video && isVideoOpenable(video) ? video : videos.find(isVideoOpenable)
      if (!target) return
      handleOpenAlternatePlayer(target)
    },
    [
      ensureOpenFileAvailable,
      alternatePlayer,
      playVideosWithMPV,
      showCenterToast,
      handleOpenAlternatePlayer,
      isVideoOpenable,
    ]
  )

  const handleJavRevealFile = useCallback(
    (video, item) => {
      if (!ensureRevealAvailable()) return
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
    [ensureRevealAvailable, handleRevealVideoFile, isVideoOpenable]
  )

  const openVideoScreenshots = useCallback((video) => {
    setScreenshotsAllowSetCover(true)
    setScreenshotsVideo(video)
  }, [])

  const openJavScreenshots = useCallback((video) => {
    setScreenshotsAllowSetCover(false)
    setScreenshotsVideo(video)
  }, [])

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
      openJavScreenshots(target)
    },
    [isVideoOpenable, openJavScreenshots]
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
          openJavScreenshots(video)
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
        showCenterToast(getErrorMessage(err))
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
      openJavScreenshots,
      showCenterToast,
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
        showCenterToast(getErrorMessage(err))
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
      showCenterToast,
    ]
  )

  const handleVideoCoverChanged = useCallback(
    (updated) => {
      const targetID = Number(updated?.id || screenshotsVideo?.id || 0)
      if (!targetID) return
      const coverScreenshotName =
        typeof updated?.cover_screenshot_name === 'string' ? updated.cover_screenshot_name : ''
      const updatedAt = updated?.updated_at || new Date().toISOString()

      if (screenshotsVideo?.id === targetID) {
        setScreenshotsVideo((current) =>
          current?.id === targetID
            ? {
                ...current,
                ...(updated || {}),
                cover_screenshot_name: coverScreenshotName,
                updated_at: updatedAt,
              }
            : current
        )
      }
      useStore.setState((state) => ({
        videos: Array.isArray(state.videos)
          ? state.videos.map((video) =>
              video?.id === targetID
                ? {
                    ...video,
                    ...(updated || {}),
                    cover_screenshot_name: coverScreenshotName,
                    updated_at: updatedAt,
                  }
                : video
            )
          : state.videos,
        javItems: Array.isArray(state.javItems)
          ? state.javItems.map((item) => {
              if (!Array.isArray(item?.videos)) return item
              let changed = false
              const nextVideos = item.videos.map((video) => {
                if (video?.id !== targetID) return video
                changed = true
                return {
                  ...video,
                  ...(updated || {}),
                  cover_screenshot_name: coverScreenshotName,
                  updated_at: updatedAt,
                }
              })
              return changed ? { ...item, videos: nextVideos } : item
            })
          : state.javItems,
      }))
    },
    [screenshotsVideo?.id]
  )

  const javVideoPickerTitle =
    javVideoPickerAction === 'open'
      ? alternatePlayer === 'mpv'
        ? zh('选择使用MPV播放器播放的文件', 'Choose a file to play with MPV player')
        : alternatePlayer === 'system'
          ? zh('选择使用系统播放器播放的文件', 'Choose a file to play with system player')
          : zh('选择使用浏览器播放的文件', 'Choose a file to play in the browser')
      : javVideoPickerAction === 'screenshots'
        ? zh('选择查看截图的文件', 'Choose a file to view screenshots')
        : javVideoPickerAction === 'reveal'
          ? zh('选择定位文件', 'Choose a file to reveal')
          : defaultPlayer === 'system'
            ? zh('选择使用系统播放器播放的文件', 'Choose a file to play with system player')
            : defaultPlayer === 'browser'
              ? zh('选择使用浏览器播放的文件', 'Choose a file to play in the browser')
              : zh('选择使用MPV播放器播放的文件', 'Choose a file to play with MPV player')

  const javVideoPickerEmptyText =
    javVideoPickerAction === 'play'
      ? zh('暂无可播放文件', 'No playable files')
      : javVideoPickerAction === 'screenshots'
        ? zh('暂无可查看截图的文件', 'No files with screenshots available')
        : zh('暂无可用文件', 'No available files')
  return {
    javVideoPickerOpen,
    javVideoPickerItem,
    javVideoPickerAction,
    locationPickerOpen,
    locationPickerChoices,
    locationPickerAction,
    playerVideo,
    setPlayerVideo,
    playerStartTime,
    setPlayerStartTime,
    screenshotsVideo,
    setScreenshotsVideo,
    screenshotsAllowSetCover,
    javVideoChoices,
    locationPickerItem,
    browserPlaybackOnly,
    containerMode,
    desktopIntegrationEnabled,
    directoryPickerEnabled,
    mpvEnabled,
    defaultPlayer,
    alternatePlayer,
    alternatePlayerLabel,
    ensureMPVPlaylistAvailable,
    isVideoOpenable,
    closeLocationPicker,
    playVideoFromTime,
    handleOpenPlayer,
    handleOpenAlternatePlayer,
    handleRevealVideoFile,
    closeJavVideoPicker,
    playVideosWithMPV,
    handleJavPlay,
    handleJavOpenFile,
    handleJavRevealFile,
    openVideoScreenshots,
    openJavScreenshots,
    handleJavOpenScreenshots,
    handleSelectJavVideo,
    handleSelectVideoLocation,
    handleVideoCoverChanged,
    javVideoPickerTitle,
    javVideoPickerEmptyText,
  }
}
