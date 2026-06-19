import { useEffect, useMemo, useRef, useState } from 'react'
import videojs from 'video.js'
import 'video.js/dist/video-js.css'
import { fetchPlaybackInfo } from '@/api'
import { getVideoDisplayName } from '@/utils/display'
import {
  PLAYER_HOTKEY_ACTIONS,
  normalizePlayerHotkeyKey,
  normalizePlayerHotkeysList,
} from '@/utils/playerHotkeys'
import { zh } from '@/utils/i18n'

const VOLUME_STORAGE_KEY = 'javboss.player.volume'

export default function PlayerModal({ video, onClose, hotkeys = [] }) {
  const videoRef = useRef(null)
  const playerRef = useRef(null)
  const hotkeyMapRef = useRef(new Map())
  const [playbackInfo, setPlaybackInfo] = useState(null)
  const [playbackError, setPlaybackError] = useState('')
  const [loadingPlayback, setLoadingPlayback] = useState(false)
  const normalizedHotkeys = useMemo(() => normalizePlayerHotkeysList(hotkeys), [hotkeys])
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
    if (!video?.id) {
      setPlaybackInfo(null)
      setPlaybackError('')
      setLoadingPlayback(false)
      return
    }

    let cancelled = false
    setLoadingPlayback(true)
    setPlaybackError('')
    setPlaybackInfo(null)

    fetchPlaybackInfo(video.id)
      .then((info) => {
        if (cancelled) return
        setPlaybackInfo(info)
      })
      .catch((err) => {
        if (cancelled) return
        setPlaybackError(
          err instanceof Error
            ? err.message
            : zh('加载播放信息失败', 'Failed to load playback info')
        )
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

    const handleKeyDown = (event) => {
      const target = event.target
      if (
        target instanceof HTMLElement &&
        (target.isContentEditable ||
          target.closest('input, textarea, select, [contenteditable="true"]'))
      ) {
        return
      }
      const key = normalizePlayerHotkeyKey(event.key || '')
      const configured = hotkeyMapRef.current.get(key)
      if (
        configured &&
        (configured.action === PLAYER_HOTKEY_ACTIONS.SEEK ||
          configured.action === PLAYER_HOTKEY_ACTIONS.VOLUME)
      ) {
        event.preventDefault()
        if (configured.action === PLAYER_HOTKEY_ACTIONS.SEEK) {
          seekBy(configured.amount)
        } else if (configured.action === PLAYER_HOTKEY_ACTIONS.VOLUME) {
          adjustVolume(configured.amount / 100)
        }
        event.stopPropagation()
        return
      }
      switch (key) {
        case ' ':
        case 'Spacebar': {
          event.preventDefault()
          if (player.paused()) {
            player.play()
          } else {
            player.pause()
          }
          break
        }
        case 'Escape':
          event.preventDefault()
          onClose()
          break
        default:
          return
      }
      event.stopPropagation()
    }

    const focusPlayer = () => {
      playerEl?.focus({ preventScroll: true })
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

    player.ready(focusPlayer)
    player.on('fullscreenchange', focusPlayer)
    player.on('volumechange', handleVolumeChange)

    return () => {
      window.removeEventListener('keydown', handleKeyDown, true)
      player.off('fullscreenchange', focusPlayer)
      player.off('volumechange', handleVolumeChange)
      playerRef.current?.dispose()
      playerRef.current = null
    }
  }, [video, onClose, selectedSource])

  if (!video) return null

  const displayName = getVideoDisplayName(video)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70">
      <div className="relative mx-4 w-full max-w-6xl rounded-lg bg-white shadow-lg">
        <button
          aria-label={zh('关闭', 'Close')}
          onClick={onClose}
          className="absolute right-3 top-3 rounded-full bg-black/60 px-2 py-1 text-sm text-white hover:bg-black/80"
        >
          ×
        </button>
        <div className="flex flex-col gap-4 p-4">
          <h2 className="truncate text-lg font-semibold" title={displayName}>
            {displayName}
          </h2>
          <div className="player-shell relative w-full bg-black">
            {loadingPlayback ? (
              <div className="flex aspect-video items-center justify-center text-sm text-white">
                {zh('加载播放信息中…', 'Loading playback info...')}
              </div>
            ) : playbackError ? (
              <div className="flex aspect-video items-center justify-center px-6 text-center text-sm text-red-200">
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
