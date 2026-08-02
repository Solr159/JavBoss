import { useEffect, useMemo, useRef, useState } from 'react'
import videojs from 'video.js'
import 'video.js/dist/video-js.css'
import { createVideoScreenshot, fetchPlaybackInfo } from '@/api'
import { getVideoDisplayName } from '@/utils/display'
import {
  PLAYER_HOTKEY_ACTIONS,
  normalizePlayerHotkeyKey,
  parsePlayerHotkeys,
} from '@/utils/playerHotkeys'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'

const VOLUME_STORAGE_KEY = 'javboss.player.volume'

export default function PlayerModal({
  video,
  startTime = 0,
  onClose,
  hotkeys = null,
  onPlaybackError,
}) {
  const videoRef = useRef(null)
  const playerRef = useRef(null)
  const hotkeyMapRef = useRef(new Map())
  const screenshotInFlightRef = useRef(false)
  const screenshotNoticeTimerRef = useRef(null)
  // App 侧传入的 onClose / onPlaybackError 通常是内联箭头函数，每次渲染引用都会变。
  // 若直接作为 effect 依赖，会导致播放器反复 dispose/重建（video.js 会移除 DOM，
  // 重建时拿到已脱离文档的元素而失败，播放区变黑），因此用 ref 保存最新回调。
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose
  const onPlaybackErrorRef = useRef(onPlaybackError)
  onPlaybackErrorRef.current = onPlaybackError
  const [playbackInfo, setPlaybackInfo] = useState(null)
  const [playbackError, setPlaybackError] = useState('')
  const [loadingPlayback, setLoadingPlayback] = useState(false)
  const [screenshotNotice, setScreenshotNotice] = useState(false)
  const [videoSize, setVideoSize] = useState(null) // { width, height } of the source video
  const normalizedHotkeys = useMemo(() => parsePlayerHotkeys(hotkeys), [hotkeys])
  const selectedSource = useMemo(() => {
    if (!playbackInfo?.sources?.length) return null
    return (
      playbackInfo.sources.find((item) => item.kind === playbackInfo.preferred_kind) ||
      playbackInfo.sources[0]
    )
  }, [playbackInfo])

  useEffect(() => {
    hotkeyMapRef.current = new Map(normalizedHotkeys.map((item) => [item.key, item]))
  }, [normalizedHotkeys])

  useEffect(() => {
    return () => {
      if (screenshotNoticeTimerRef.current) {
        window.clearTimeout(screenshotNoticeTimerRef.current)
      }
    }
  }, [])

  useEffect(() => {
    if (!video?.id) {
      setPlaybackInfo(null)
      setPlaybackError('')
      setLoadingPlayback(false)
      setScreenshotNotice(false)
      setVideoSize(null)
      return
    }

    let cancelled = false
    setLoadingPlayback(true)
    setPlaybackError('')
    setPlaybackInfo(null)
    setScreenshotNotice(false)
    setVideoSize(null)

    fetchPlaybackInfo(video.id, { locationId: video.location_id })
      .then((info) => {
        if (cancelled) return
        setPlaybackInfo(info)
      })
      .catch((err) => {
        if (cancelled) return
        const message = getErrorMessage(err)
        setPlaybackError(message)
        onPlaybackErrorRef.current?.(message)
      })
      .finally(() => {
        if (cancelled) return
        setLoadingPlayback(false)
      })

    return () => {
      cancelled = true
    }
  }, [video])

  useEffect(() => {
    if (!video || !videoRef.current || !selectedSource?.src) return

    const player = videojs(videoRef.current, {
      controls: true,
      autoplay: true,
      preload: 'auto',
      sources: [
        {
          src: selectedSource.src,
          type: selectedSource.mime_type || 'video/mp4',
        },
      ],
    })

    playerRef.current = player

    const playerEl = player.el()
    const savedVolume = (() => {
      try {
        const raw = localStorage.getItem(VOLUME_STORAGE_KEY)
        if (raw == null) return null
        const value = Number.parseFloat(raw)
        return Number.isFinite(value) ? value : null
      } catch {
        return null
      }
    })()

    if (savedVolume != null) {
      player.volume(Math.min(1, Math.max(0, savedVolume)))
    }

    const seekBy = (offsetSeconds) => {
      const current = player.currentTime() || 0
      const duration = player.duration()
      let next = current + offsetSeconds
      if (Number.isFinite(duration)) {
        next = Math.min(Math.max(0, next), duration)
      } else {
        next = Math.max(0, next)
      }
      player.currentTime(next)
    }

    const adjustVolume = (delta) => {
      const current = player.volume()
      const next = Math.min(1, Math.max(0, current + delta))
      player.volume(next)
    }

    const captureScreenshot = () => {
      if (!video?.id || screenshotInFlightRef.current) return
      const second = Math.max(0, Number(player.currentTime()) || 0)
      screenshotInFlightRef.current = true
      createVideoScreenshot(video.id, { second, locationId: video.location_id })
        .then(() => {
          if (screenshotNoticeTimerRef.current) {
            window.clearTimeout(screenshotNoticeTimerRef.current)
          }
          setScreenshotNotice(true)
          screenshotNoticeTimerRef.current = window.setTimeout(() => {
            setScreenshotNotice(false)
            screenshotNoticeTimerRef.current = null
          }, 1600)
        })
        .catch((err) => {
          console.error(zh('截图失败', 'Failed to capture screenshot'), err)
        })
        .finally(() => {
          screenshotInFlightRef.current = false
        })
    }

    const handleKeyDown = (event) => {
      const target = event.target
      if (
        target instanceof Element &&
        (target.isContentEditable ||
          target.closest('input, textarea, select, [contenteditable="true"]'))
      ) {
        return
      }
      const key = normalizePlayerHotkeyKey(event.key || '')
      const configured = hotkeyMapRef.current.get(key)
      const markHandled = () => {
        event.preventDefault()
        event.stopPropagation()
      }
      if (
        configured &&
        (configured.action === PLAYER_HOTKEY_ACTIONS.SEEK ||
          configured.action === PLAYER_HOTKEY_ACTIONS.VOLUME ||
          configured.action === PLAYER_HOTKEY_ACTIONS.SCREENSHOT)
      ) {
        markHandled()
        if (configured.action === PLAYER_HOTKEY_ACTIONS.SEEK) {
          seekBy(configured.amount)
        } else if (configured.action === PLAYER_HOTKEY_ACTIONS.VOLUME) {
          adjustVolume(configured.amount / 100)
        } else if (configured.action === PLAYER_HOTKEY_ACTIONS.SCREENSHOT) {
          captureScreenshot()
        }
        return
      }
      switch (key) {
        case ' ':
        case 'Spacebar': {
          markHandled()
          if (player.paused()) {
            player.play()
          } else {
            player.pause()
          }
          break
        }
        case 'Escape':
          markHandled()
          onCloseRef.current()
          break
        default:
          return
      }
    }

    const focusPlayer = () => {
      playerEl?.focus({ preventScroll: true })
    }
    // video.js 在 data-vjs-player 包装下会用新的 .vjs-tech 元素替换原 <video> 标签，
    // 因此不能从 videoRef.current 读尺寸（恒为 0），必须走 player API。
    // 同时监听 resize：HLS 等流在 loadedmetadata 时可能还拿不到分辨率，稍后会再触发。
    const handleDimensions = () => {
      const width = player.videoWidth()
      const height = player.videoHeight()
      if (width > 0 && height > 0) {
        setVideoSize({ width, height })
      }
    }
    const applyStartTime = () => {
      const nextStartTime = Number(startTime)
      if (!Number.isFinite(nextStartTime) || nextStartTime <= 0) return
      player.currentTime(nextStartTime)
    }

    if (playerEl && !playerEl.hasAttribute('tabindex')) {
      playerEl.setAttribute('tabindex', '-1')
    }

    window.addEventListener('keydown', handleKeyDown, true)

    const handleVolumeChange = () => {
      try {
        localStorage.setItem(VOLUME_STORAGE_KEY, String(player.volume()))
      } catch {
        return
      }
    }

    player.ready(() => {
      applyStartTime()
      focusPlayer()
    })
    player.on('fullscreenchange', focusPlayer)
    player.on('volumechange', handleVolumeChange)
    player.on('loadedmetadata', handleDimensions)
    player.on('resize', handleDimensions)

    return () => {
      window.removeEventListener('keydown', handleKeyDown, true)
      player.off('fullscreenchange', focusPlayer)
      player.off('volumechange', handleVolumeChange)
      player.off('loadedmetadata', handleDimensions)
      player.off('resize', handleDimensions)
      playerRef.current?.dispose()
      playerRef.current = null
    }
  }, [video, startTime, selectedSource])

  if (!video) return null

  const displayName = getVideoDisplayName(video)
  const aspectRatio =
    videoSize && videoSize.height > 0 ? videoSize.width / videoSize.height : 16 / 9

  return (
    <div className="fixed inset-0 z-[1700] flex items-center justify-center bg-black/70 pointer-coarse:bg-black">
      {/*
        桌面端：白色卡片包裹，标题与关闭按钮在卡片内；
        移动端（触摸设备，含横屏）：隐藏白卡片边框，视频按比例贴边（100vw/100dvh，不超出不拉伸），标题悬浮在视频上方。
        宽度公式在 .player-card 的媒体查询中，比例通过 --player-ar 传入。
      */}
      <div
        className="player-card relative mx-4 rounded-lg bg-white shadow-lg pointer-coarse:mx-0 pointer-coarse:rounded-none pointer-coarse:bg-black pointer-coarse:shadow-none"
        style={{ '--player-ar': `${aspectRatio}` }}
      >
        <button
          aria-label={zh('关闭', 'Close')}
          onClick={onClose}
          className="absolute right-3 top-3 z-20 rounded-full bg-black/60 px-2 py-1 text-sm text-white hover:bg-black/80"
        >
          ×
        </button>
        <div className="flex flex-col gap-4 p-4 pointer-coarse:gap-0 pointer-coarse:p-0">
          <h2
            className="truncate pr-10 text-lg font-semibold pointer-coarse:absolute pointer-coarse:inset-x-0 pointer-coarse:top-0 pointer-coarse:z-10 pointer-coarse:bg-gradient-to-b pointer-coarse:from-black/60 pointer-coarse:to-transparent pointer-coarse:px-12 pointer-coarse:pb-5 pointer-coarse:pt-2 pointer-coarse:text-sm pointer-coarse:font-medium pointer-coarse:text-white"
            title={displayName}
          >
            {displayName}
          </h2>
          <div
            className="player-shell relative w-full bg-black"
            style={{
              aspectRatio: videoSize ? `${videoSize.width} / ${videoSize.height}` : '16 / 9',
            }}
          >
            {screenshotNotice ? (
              <div className="pointer-events-none absolute left-3 top-3 z-10 rounded bg-black/75 px-3 py-1.5 text-sm font-medium text-white shadow">
                {zh('截图成功', 'Screenshot saved')}
              </div>
            ) : null}
            {loadingPlayback ? (
              <div className="flex h-full w-full items-center justify-center text-sm text-white">
                {zh('加载播放信息中…', 'Loading playback info...')}
              </div>
            ) : playbackError ? (
              <div className="flex h-full w-full items-center justify-center px-6 text-center text-sm text-red-200">
                {playbackError}
              </div>
            ) : (
              <div data-vjs-player className="h-full w-full">
                <video
                  ref={videoRef}
                  className="video-js vjs-big-play-centered h-full w-full"
                  playsInline
                >
                  <track kind="captions" />
                </video>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
