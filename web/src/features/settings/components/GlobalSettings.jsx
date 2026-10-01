import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import usePlaybackCapabilities from '@/features/playback/usePlaybackCapabilities'
import { useAuth } from '@/auth'
import { normalizeInitialViewMode, configFlag } from '@/utils/config'
import GlobalSettingsModal from '@/features/settings/components/GlobalSettingsModal'
import { zh } from '@/utils/i18n'
import { processDirectory, scanDirectory } from '@/features/directories/api'
import { updateConfig } from '@/features/settings/api'
import { normalizeDefaultPlayer } from '@/features/playback/model'

export default function GlobalSettings({ onToast, open, onClose }) {
  const {
    config,
    directories,
    createDirectory,
    loadDirectories,
    updateDirectory,
    deleteDirectory,
  } = useStore(
    useShallow((state) => ({
      config: state.config,
      directories: state.directories,
      createDirectory: state.createDirectory,
      loadDirectories: state.loadDirectories,
      updateDirectory: state.updateDirectory,
      deleteDirectory: state.deleteDirectory,
    }))
  )
  const {
    browserPlaybackOnly,
    desktopIntegrationEnabled,
    containerMode,
    directoryPickerEnabled,
    mpvEnabled,
    defaultPlayer,
  } = usePlaybackCapabilities()
  const { changePassword, logout } = useAuth()
  const initialViewMode = normalizeInitialViewMode(config?.initial_view_mode)

  return (
    <GlobalSettingsModal
      onToast={onToast}
      open={open}
      onClose={onClose}
      directories={directories}
      browserPlaybackOnly={browserPlaybackOnly}
      desktopIntegrationEnabled={desktopIntegrationEnabled}
      containerMode={containerMode}
      directoryPickerEnabled={directoryPickerEnabled}
      serverOS={config?.runtime_os}
      mpvEnabled={mpvEnabled}
      onCreateDirectory={async (payload) => {
        const created = await createDirectory(payload)
        await loadDirectories()
        onToast(
          zh(
            '目录添加成功，首次扫描目录里的视频需要一定时间，请耐心等待，您可手动刷新页面查看扫描进度',
            'Directory added. The first scan may take some time. You can refresh manually to check progress.'
          ),
          4000
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
      onProcessDirectory={async (id, mode, layout) => {
        const result = await processDirectory(id, mode, layout)
        await loadDirectories()
        onToast(zh('目录任务已启动', 'Directory task started'), 4000)
        return result
      }}
      onScanDirectory={async (id) => {
        const result = await scanDirectory(id)
        await loadDirectories()
        onToast(zh('目录扫描已启动', 'Directory scan started'), 4000)
        return result
      }}
      onRefreshDirectories={loadDirectories}
      proxyMode={config?.proxy_mode}
      proxyHost={config?.proxy_host || ''}
      proxyPort={Number.parseInt(config?.proxy_port, 10) || 0}
      onSaveProxySettings={async ({ mode, host, port }) => {
        const cfg = await updateConfig({
          proxy_mode: mode,
          ...(mode === 'manual' ? { proxy_host: host, proxy_port: port } : {}),
        })
        useStore.setState({ config: cfg })
      }}
      allowLANAccess={configFlag(config?.allow_lan_access)}
      onSaveAllowLANAccess={async (enabled) => {
        const cfg = await updateConfig({ allow_lan_access: Boolean(enabled) })
        useStore.setState({ config: cfg })
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
      playerOntop={
        config?.player_ontop == null
          ? false
          : !['0', 'false', 'no', 'off'].includes(String(config.player_ontop).trim().toLowerCase())
      }
      playerReuseWindow={
        config?.player_reuse_window == null
          ? true
          : !['0', 'false', 'no', 'off'].includes(
              String(config.player_reuse_window).trim().toLowerCase()
            )
      }
      playerResumePlayback={
        config?.player_resume_playback == null
          ? true
          : !['0', 'false', 'no', 'off'].includes(
              String(config.player_resume_playback).trim().toLowerCase()
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
      browserPlayerShowHotkeyHint={configFlag(config?.browser_player_show_hotkey_hint, true)}
      onSaveBrowserPlayerSettings={async (payload) => {
        const cfg = await updateConfig(payload)
        useStore.setState({ config: cfg })
      }}
      playerHotkeys={config?.player_hotkeys}
      onSavePlayerHotkeys={async (hotkeys) => {
        const cfg = await updateConfig({ player_hotkeys: hotkeys })
        useStore.setState({ config: cfg })
      }}
      webHotkeys={config?.web_hotkeys}
      onSaveWebHotkeys={async (hotkeys) => {
        const cfg = await updateConfig({ web_hotkeys: hotkeys })
        useStore.setState({ config: cfg })
      }}
      onChangePassword={changePassword}
      onLogout={logout}
    />
  )
}
