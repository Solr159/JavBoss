import { useStore } from '@/store'
import { configFlag } from '@/utils/config'
import { normalizeDefaultPlayer } from '@/features/playback/model'
import { zh } from '@/utils/i18n'

export default function usePlaybackCapabilities() {
  const config = useStore((s) => s.config)
  const browserPlaybackOnly = configFlag(config?.browser_playback_only)

  const remoteAccess = configFlag(config?.runtime_remote_request)

  const clientMode = configFlag(config?.runtime_client)

  const containerMode = configFlag(config?.runtime_container)

  const desktopIntegrationEnabled = configFlag(config?.desktop_integration_enabled, true)

  const mpvEnabled = configFlag(config?.mpv_enabled, true)

  const defaultPlayer = browserPlaybackOnly
    ? 'browser'
    : normalizeDefaultPlayer(config?.default_player)

  const bulkPlayer = defaultPlayer === 'system' ? (remoteAccess ? 'browser' : 'mpv') : defaultPlayer

  const alternatePlayer = browserPlaybackOnly
    ? ''
    : defaultPlayer === 'system'
      ? mpvEnabled
        ? 'mpv'
        : ''
      : defaultPlayer === 'browser'
        ? clientMode
          ? mpvEnabled
            ? 'mpv'
            : desktopIntegrationEnabled
              ? 'system'
              : ''
          : desktopIntegrationEnabled
            ? 'system'
            : ''
        : desktopIntegrationEnabled
          ? 'system'
          : ''

  const alternatePlayerLabel =
    alternatePlayer === 'mpv'
      ? zh('使用MPV播放器播放', 'Play with MPV player')
      : alternatePlayer === 'system'
        ? zh('用默认程序打开', 'Open with default app')
        : ''

  return {
    browserPlaybackOnly,
    remoteAccess,
    clientMode,
    containerMode,
    desktopIntegrationEnabled,
    mpvEnabled,
    defaultPlayer,
    bulkPlayer,
    bulkPlaybackEnabled: bulkPlayer === 'browser' || mpvEnabled,
    alternatePlayer,
    alternatePlayerLabel,
  }
}
