import { useEffect, useState } from 'react'
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined'
import VisibilityOffOutlinedIcon from '@mui/icons-material/VisibilityOffOutlined'
import VisibilityOutlinedIcon from '@mui/icons-material/VisibilityOutlined'

import DirectoryManager from '@/components/DirectoryManager'
import PlayerSettingsModal from '@/components/PlayerSettingsModal'
import { parsePlayerHotkeys } from '@/utils/playerHotkeys'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'

const SETTINGS_SECTIONS = [
  {
    id: 'directories',
    title: { zh: '目录管理', en: 'Directory Management' },
    summary: { zh: '管理扫描目录与路径', en: 'Manage watched folders and paths' },
  },
  {
    id: 'display',
    title: { zh: '显示与交互', en: 'Display & Interaction' },
    summary: { zh: '界面提示与交互行为', en: 'Interface hints and interactions' },
  },
  {
    id: 'network',
    title: { zh: '网络与代理', en: 'Network & Proxy' },
    summary: { zh: '网络连接与代理设置', en: 'Network connection and proxy settings' },
  },
  {
    id: 'player',
    title: { zh: '播放器', en: 'Player' },
    summary: { zh: '播放器快捷键与播放控制', en: 'Player shortcuts and playback controls' },
  },
  {
    id: 'security',
    title: { zh: '安全', en: 'Security' },
    summary: { zh: '修改密码与退出登录', en: 'Password and sign-out' },
  },
]

const PLAYER_BASIC_DEFAULTS = {
  windowWidth: 80,
  windowHeight: 80,
  ontop: false,
  reuseWindow: true,
  resumePlayback: true,
  volume: 70,
  showHotkeyHint: true,
}

const DEFAULT_PROXY_HOST = '127.0.0.1'

export default function GlobalSettingsModal({
  open,
  onClose,
  directories,
  browserPlaybackOnly = false,
  directoryPickerEnabled = true,
  hostPathPrefixEnabled = false,
  mpvEnabled = true,
  onCreateDirectory,
  onUpdateDirectory,
  onDeleteDirectory,
  proxyHost,
  proxyPort,
  onSaveProxySettings,
  javMetadataLanguage,
  onSaveJavMetadataLanguage,
  defaultPlayer,
  onSaveDefaultPlayer,
  initialViewMode,
  onSaveInitialViewMode,
  showTopBarButtonTooltips = true,
  onSaveShowTopBarButtonTooltips,
  playerWindowWidth,
  playerWindowHeight,
  playerOntop,
  playerReuseWindow,
  playerResumePlayback,
  playerVolume,
  playerShowHotkeyHint,
  onSavePlayerBasicSettings,
  playerHotkeys,
  onSavePlayerHotkeys,
  onChangePassword,
  onLogout,
}) {
  const [proxyHostInput, setProxyHostInput] = useState('')
  const [proxyInput, setProxyInput] = useState('')
  const [proxyError, setProxyError] = useState('')
  const [savingProxy, setSavingProxy] = useState(false)
  const [proxyEditing, setProxyEditing] = useState(false)
  const [proxyEnabledInput, setProxyEnabledInput] = useState(false)
  const [javMetadataLanguageInput, setJavMetadataLanguageInput] = useState('zh')
  const [javMetadataLanguageError, setJavMetadataLanguageError] = useState('')
  const [savingJavMetadataLanguage, setSavingJavMetadataLanguage] = useState(false)
  const [activeSection, setActiveSection] = useState('directories')
  const [defaultPlayerInput, setDefaultPlayerInput] = useState('mpv')
  const [defaultPlayerError, setDefaultPlayerError] = useState('')
  const [savingDefaultPlayer, setSavingDefaultPlayer] = useState(false)
  const [initialViewModeInput, setInitialViewModeInput] = useState('video')
  const [initialViewModeError, setInitialViewModeError] = useState('')
  const [savingInitialViewMode, setSavingInitialViewMode] = useState(false)
  const [showTopBarButtonTooltipsInput, setShowTopBarButtonTooltipsInput] = useState(true)
  const [showTopBarButtonTooltipsError, setShowTopBarButtonTooltipsError] = useState('')
  const [savingShowTopBarButtonTooltips, setSavingShowTopBarButtonTooltips] = useState(false)
  const [playerTab, setPlayerTab] = useState('basic')
  const [playerBasicError, setPlayerBasicError] = useState('')
  const [playerBasicSuccess, setPlayerBasicSuccess] = useState('')
  const [savingPlayerBasic, setSavingPlayerBasic] = useState(false)
  const [playerWindowWidthInput, setPlayerWindowWidthInput] = useState('')
  const [playerWindowHeightInput, setPlayerWindowHeightInput] = useState('')
  const [playerOntopInput, setPlayerOntopInput] = useState(false)
  const [playerReuseWindowInput, setPlayerReuseWindowInput] = useState(true)
  const [playerResumePlaybackInput, setPlayerResumePlaybackInput] = useState(true)
  const [playerVolumeInput, setPlayerVolumeInput] = useState('')
  const [playerShowHotkeyHintInput, setPlayerShowHotkeyHintInput] = useState(true)
  const [passwordDialogOpen, setPasswordDialogOpen] = useState(false)
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [visiblePasswords, setVisiblePasswords] = useState({
    current: false,
    new: false,
    confirm: false,
  })
  const [passwordError, setPasswordError] = useState('')
  const [savingPassword, setSavingPassword] = useState(false)

  const normalizedPlayerHotkeys = parsePlayerHotkeys(playerHotkeys)

  const resetPlayerBasicInputs = () => {
    setPlayerWindowWidthInput(String(PLAYER_BASIC_DEFAULTS.windowWidth))
    setPlayerWindowHeightInput(String(PLAYER_BASIC_DEFAULTS.windowHeight))
    setPlayerOntopInput(PLAYER_BASIC_DEFAULTS.ontop)
    setPlayerReuseWindowInput(PLAYER_BASIC_DEFAULTS.reuseWindow)
    setPlayerResumePlaybackInput(PLAYER_BASIC_DEFAULTS.resumePlayback)
    setPlayerVolumeInput(String(PLAYER_BASIC_DEFAULTS.volume))
    setPlayerShowHotkeyHintInput(PLAYER_BASIC_DEFAULTS.showHotkeyHint)
    setPlayerBasicError('')
    setPlayerBasicSuccess('')
  }

  useEffect(() => {
    if (open) {
      setPlayerTab('basic')
      setPlayerBasicError('')
      setPlayerBasicSuccess('')
    }
  }, [open])

  useEffect(() => {
    if (open) {
      setProxyHostInput(proxyHost || DEFAULT_PROXY_HOST)
      setProxyInput(proxyPort ? String(proxyPort) : '')
      setProxyEnabledInput(Boolean(proxyPort))
      setProxyEditing(false)
      setProxyError('')
      setJavMetadataLanguageInput(javMetadataLanguage === 'en' ? 'en' : 'zh')
      setJavMetadataLanguageError('')
      setDefaultPlayerInput(defaultPlayer === 'system' ? 'system' : 'mpv')
      setDefaultPlayerError('')
      setInitialViewModeInput(initialViewMode === 'jav' ? 'jav' : 'video')
      setInitialViewModeError('')
      setShowTopBarButtonTooltipsInput(showTopBarButtonTooltips !== false)
      setShowTopBarButtonTooltipsError('')
      setPlayerWindowWidthInput(String(playerWindowWidth ?? PLAYER_BASIC_DEFAULTS.windowWidth))
      setPlayerWindowHeightInput(String(playerWindowHeight ?? PLAYER_BASIC_DEFAULTS.windowHeight))
      setPlayerOntopInput(playerOntop ?? PLAYER_BASIC_DEFAULTS.ontop)
      setPlayerReuseWindowInput(playerReuseWindow ?? PLAYER_BASIC_DEFAULTS.reuseWindow)
      setPlayerResumePlaybackInput(playerResumePlayback ?? PLAYER_BASIC_DEFAULTS.resumePlayback)
      setPlayerVolumeInput(String(playerVolume ?? PLAYER_BASIC_DEFAULTS.volume))
      setPlayerShowHotkeyHintInput(playerShowHotkeyHint ?? PLAYER_BASIC_DEFAULTS.showHotkeyHint)
      setPasswordDialogOpen(false)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
      setVisiblePasswords({ current: false, new: false, confirm: false })
      setPasswordError('')
    }
  }, [
    open,
    proxyHost,
    proxyPort,
    javMetadataLanguage,
    defaultPlayer,
    initialViewMode,
    showTopBarButtonTooltips,
    playerWindowWidth,
    playerWindowHeight,
    playerOntop,
    playerReuseWindow,
    playerResumePlayback,
    playerVolume,
    playerShowHotkeyHint,
    mpvEnabled,
    browserPlaybackOnly,
  ])

  if (!open) return null

  const handleSaveProxy = async () => {
    setProxyError('')
    const host = proxyHostInput.trim()
    const raw = proxyInput.trim()
    let port = 0
    let nextHost = ''
    if (proxyEnabledInput) {
      if (host === '') {
        setProxyError(zh('请输入代理 IP 或主机名', 'Enter a proxy IP or host'))
        return
      }
      if (raw === '') {
        setProxyError(zh('请输入 1-65535 的端口号', 'Enter a port between 1 and 65535'))
        return
      }
      const parsed = /^\d+$/.test(raw) ? Number(raw) : NaN
      if (!Number.isFinite(parsed) || parsed <= 0 || parsed > 65535) {
        setProxyError(zh('请输入 1-65535 的端口号', 'Enter a port between 1 and 65535'))
        return
      }
      port = parsed
      nextHost = host
    }
    setSavingProxy(true)
    try {
      await onSaveProxySettings?.({ host: nextHost, port })
      setProxyEditing(false)
    } catch (err) {
      setProxyError(getErrorMessage(err))
    } finally {
      setSavingProxy(false)
    }
  }

  const currentProxyHost = proxyHost || DEFAULT_PROXY_HOST
  const proxyHostInputTrimmed = proxyHostInput.trim()
  const proxyInputTrimmed = proxyInput.trim()
  const desiredHostText = proxyEnabledInput ? proxyHostInputTrimmed : ''
  const desiredPortText = proxyEnabledInput ? proxyInputTrimmed : ''
  const currentHostText = proxyPort ? currentProxyHost : ''
  const currentPortText = proxyPort ? String(proxyPort) : ''
  const proxyUnchanged = desiredHostText === currentHostText && desiredPortText === currentPortText
  const proxyHostMissing = proxyEnabledInput && proxyHostInputTrimmed === ''
  const proxyInputMissing = proxyEnabledInput && proxyInputTrimmed === ''
  const visibleSections = SETTINGS_SECTIONS
  const currentSection = visibleSections.some((section) => section.id === activeSection)
    ? activeSection
    : 'directories'
  const activeTitle = visibleSections.find((item) => item.id === currentSection)?.title || {
    zh: '全局设置',
    en: 'Global Settings',
  }

  const handleSaveDefaultPlayer = async () => {
    const next = defaultPlayerInput === 'system' ? 'system' : 'mpv'
    setDefaultPlayerError('')
    setSavingDefaultPlayer(true)
    try {
      await onSaveDefaultPlayer?.(next)
    } catch (err) {
      setDefaultPlayerError(getErrorMessage(err))
    } finally {
      setSavingDefaultPlayer(false)
    }
  }

  const handleSaveInitialViewMode = async () => {
    const next = initialViewModeInput === 'jav' ? 'jav' : 'video'
    setInitialViewModeError('')
    setSavingInitialViewMode(true)
    try {
      await onSaveInitialViewMode?.(next)
    } catch (err) {
      setInitialViewModeError(getErrorMessage(err))
    } finally {
      setSavingInitialViewMode(false)
    }
  }

  const handleSaveShowTopBarButtonTooltips = async () => {
    setShowTopBarButtonTooltipsError('')
    setSavingShowTopBarButtonTooltips(true)
    try {
      await onSaveShowTopBarButtonTooltips?.(showTopBarButtonTooltipsInput)
    } catch (err) {
      setShowTopBarButtonTooltipsError(getErrorMessage(err))
    } finally {
      setSavingShowTopBarButtonTooltips(false)
    }
  }

  const renderDefaultPlayerSettings = () => {
    const currentDefaultPlayer = defaultPlayer === 'system' ? 'system' : 'mpv'
    const defaultPlayerUnchanged = defaultPlayerInput === currentDefaultPlayer

    return (
      <div className="space-y-4">
        {browserPlaybackOnly ? (
          <div>
            <h4 className="text-sm font-semibold text-zinc-800">
              {zh('默认播放器', 'Default Player')}
            </h4>
            <p className="mt-1 text-sm text-zinc-500">
              {zh(
                '当前部署模式使用浏览器播放视频。',
                'This deployment mode plays videos in the browser.'
              )}
            </p>
          </div>
        ) : (
          <>
            <div className="flex flex-wrap items-center gap-3">
              <h4 className="text-sm font-semibold text-zinc-800">
                {zh('默认播放器', 'Default Player')}
              </h4>
              <span className="relative inline-block">
                <select
                  value={defaultPlayerInput}
                  onChange={(event) => {
                    setDefaultPlayerInput(event.target.value === 'system' ? 'system' : 'mpv')
                    setDefaultPlayerError('')
                  }}
                  className="w-auto appearance-none rounded-xl border border-zinc-200 bg-white py-1.5 pl-3 pr-7 text-sm text-zinc-800 outline-none focus:border-zinc-200 focus:outline-none focus:ring-0 focus-visible:outline-none"
                >
                  <option value="mpv">{zh('MPV播放器', 'MPV Player')}</option>
                  <option value="system">{zh('系统播放器', 'System Player')}</option>
                </select>
                <span
                  aria-hidden="true"
                  className="pointer-events-none absolute right-4 top-1/2 h-1.5 w-1.5 -translate-y-1/2 rotate-45 border-b border-r border-zinc-500"
                />
              </span>
            </div>
            <div>
              <p className="mt-1 text-sm text-zinc-500">
                {zh(
                  '默认播放按钮使用所选播放器，底部播放按钮使用另一个播放器。',
                  'The primary play button uses the selected player, while the bottom play button uses the other player.'
                )}
              </p>
            </div>
          </>
        )}

        {defaultPlayerError && <div className="text-sm text-red-600">{defaultPlayerError}</div>}

        {!browserPlaybackOnly ? (
          <div className="flex justify-end">
            <button
              type="button"
              onClick={handleSaveDefaultPlayer}
              disabled={savingDefaultPlayer || defaultPlayerUnchanged}
              className="rounded-xl bg-blue-600 px-3 py-1.5 text-sm text-white disabled:opacity-60"
            >
              {savingDefaultPlayer ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
            </button>
          </div>
        ) : null}
      </div>
    )
  }

  const renderProxyPanel = () => (
    <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
      <div className="flex flex-col gap-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h4 className="text-sm font-semibold text-zinc-800">
              {zh('代理地址', 'Proxy Address')}
            </h4>
            <p className="mt-1 text-sm text-zinc-500">
              {proxyPort
                ? zh(
                    `当前使用 ${currentProxyHost}:${proxyPort}`,
                    `Currently using ${currentProxyHost}:${proxyPort}`
                  )
                : zh('当前使用自动检测', 'Currently using auto-detection')}
            </p>
          </div>
          {!proxyEditing && (
            <button
              type="button"
              onClick={() => {
                setProxyEditing(true)
                setProxyError('')
              }}
              className="rounded-xl border border-zinc-200 bg-white px-3 py-1.5 text-sm text-zinc-700 hover:bg-zinc-50"
            >
              {zh('编辑', 'Edit')}
            </button>
          )}
        </div>

        {proxyEditing ? (
          <div className="space-y-4 rounded-2xl bg-zinc-50 p-4">
            <label className="flex items-center gap-2 text-sm text-zinc-700">
              <input
                type="checkbox"
                checked={proxyEnabledInput}
                onChange={(e) => {
                  setProxyEnabledInput(e.target.checked)
                  setProxyError('')
                }}
                className="h-4 w-4 rounded"
              />
              <span>{zh('手动设置代理', 'Set proxy manually')}</span>
            </label>

            {proxyEnabledInput && (
              <div className="grid max-w-2xl gap-3 sm:grid-cols-[minmax(0,1fr)_160px]">
                <div>
                  <label className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">
                    {zh('代理IP', 'Proxy IP')}
                  </label>
                  <input
                    value={proxyHostInput}
                    onChange={(e) => setProxyHostInput(e.target.value)}
                    placeholder={DEFAULT_PROXY_HOST}
                    className="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm"
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">
                    {zh('端口号', 'Port')}
                  </label>
                  <input
                    value={proxyInput}
                    onChange={(e) => setProxyInput(e.target.value)}
                    placeholder={zh('输入 1-65535', 'Enter 1-65535')}
                    inputMode="numeric"
                    className="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm"
                  />
                </div>
              </div>
            )}

            {proxyError && <div className="text-sm text-red-600">{proxyError}</div>}

            <div className="flex flex-wrap justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setProxyHostInput(proxyHost || DEFAULT_PROXY_HOST)
                  setProxyInput(proxyPort ? String(proxyPort) : '')
                  setProxyEnabledInput(Boolean(proxyPort))
                  setProxyError('')
                  setProxyEditing(false)
                }}
                className="rounded-xl border border-zinc-200 bg-white px-3 py-1.5 text-sm text-zinc-700 hover:bg-zinc-50"
              >
                {zh('取消', 'Cancel')}
              </button>
              <button
                type="button"
                onClick={handleSaveProxy}
                disabled={savingProxy || proxyUnchanged || proxyHostMissing || proxyInputMissing}
                className="rounded-xl bg-blue-600 px-3 py-1.5 text-sm text-white disabled:opacity-60"
              >
                {savingProxy ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
              </button>
            </div>
          </div>
        ) : null}
      </div>
    </section>
  )

  const renderDisplayPanel = () => {
    const unchanged = showTopBarButtonTooltipsInput === (showTopBarButtonTooltips !== false)
    const currentInitialViewMode = initialViewMode === 'jav' ? 'jav' : 'video'
    const initialViewModeUnchanged = initialViewModeInput === currentInitialViewMode

    return (
      <div className="space-y-5">
        <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-3">
              <h4 className="text-sm font-semibold text-zinc-800">
                {zh('初始页面', 'Initial Page')}
              </h4>
              <span className="relative inline-block">
                <select
                  value={initialViewModeInput}
                  onChange={(event) => {
                    setInitialViewModeInput(event.target.value === 'jav' ? 'jav' : 'video')
                    setInitialViewModeError('')
                  }}
                  className="w-auto appearance-none rounded-xl border border-zinc-200 bg-white py-1.5 pl-3 pr-7 text-sm text-zinc-800 outline-none focus:border-zinc-200 focus:outline-none focus:ring-0 focus-visible:outline-none"
                >
                  <option value="video">{zh('视频模式', 'Video Mode')}</option>
                  <option value="jav">{zh('JAV模式', 'JAV Mode')}</option>
                </select>
                <span
                  aria-hidden="true"
                  className="pointer-events-none absolute right-4 top-1/2 h-1.5 w-1.5 -translate-y-1/2 rotate-45 border-b border-r border-zinc-500"
                />
              </span>
            </div>
            <p className="text-sm text-zinc-500">
              {zh(
                '打开新页面，默认进入所选模式。',
                'When opening a new page, use the selected mode by default.'
              )}
            </p>

            {initialViewModeError && (
              <div className="text-sm text-red-600">{initialViewModeError}</div>
            )}

            <div className="flex justify-end">
              <button
                type="button"
                onClick={handleSaveInitialViewMode}
                disabled={savingInitialViewMode || initialViewModeUnchanged}
                className="rounded-xl bg-blue-600 px-3 py-1.5 text-sm text-white disabled:opacity-60"
              >
                {savingInitialViewMode ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
              </button>
            </div>
          </div>
        </section>
        <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="space-y-4">
            <label className="flex items-start gap-3 text-sm font-semibold text-zinc-800">
              <input
                type="checkbox"
                checked={showTopBarButtonTooltipsInput}
                onChange={(event) => {
                  setShowTopBarButtonTooltipsInput(event.target.checked)
                  setShowTopBarButtonTooltipsError('')
                }}
                className="mt-0.5 h-4 w-4 rounded"
              />
              <span>
                {zh(
                  '鼠标移动到顶部控制栏按钮时显示对应说明',
                  'Show descriptions when hovering over top control bar buttons'
                )}
              </span>
            </label>
            {showTopBarButtonTooltipsError && (
              <div className="text-sm text-red-600">{showTopBarButtonTooltipsError}</div>
            )}

            <div className="flex justify-end">
              <button
                type="button"
                onClick={handleSaveShowTopBarButtonTooltips}
                disabled={savingShowTopBarButtonTooltips || unchanged}
                className="rounded-xl bg-blue-600 px-3 py-1.5 text-sm text-white disabled:opacity-60"
              >
                {savingShowTopBarButtonTooltips ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
              </button>
            </div>
          </div>
        </section>
        {renderJavMetadataSettings()}
      </div>
    )
  }

  const handleSaveJavMetadataLanguage = async () => {
    const next = javMetadataLanguageInput === 'en' ? 'en' : 'zh'
    setJavMetadataLanguageError('')
    setSavingJavMetadataLanguage(true)
    try {
      await onSaveJavMetadataLanguage?.(next)
    } catch (err) {
      setJavMetadataLanguageError(getErrorMessage(err))
    } finally {
      setSavingJavMetadataLanguage(false)
    }
  }

  const renderJavMetadataSettings = () => {
    const currentLanguage = javMetadataLanguage === 'en' ? 'en' : 'zh'
    const unchanged = javMetadataLanguageInput === currentLanguage

    return (
      <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="space-y-4">
          <div className="flex flex-wrap items-center gap-3">
            <h4 className="text-sm font-semibold text-zinc-800">
              {zh('JAV元数据语言', 'JAV Metadata Language')}
            </h4>
            <span className="relative inline-block">
              <select
                value={javMetadataLanguageInput}
                onChange={(event) => {
                  setJavMetadataLanguageInput(event.target.value === 'en' ? 'en' : 'zh')
                  setJavMetadataLanguageError('')
                }}
                className="w-auto appearance-none rounded-xl border border-zinc-200 bg-white py-1.5 pl-3 pr-7 text-sm text-zinc-800 outline-none focus:border-zinc-200 focus:outline-none focus:ring-0 focus-visible:outline-none"
              >
                <option value="en">English</option>
                <option value="zh">{zh('中日文', 'Chinese/Japanese')}</option>
              </select>
              <span
                aria-hidden="true"
                className="pointer-events-none absolute right-4 top-1/2 h-1.5 w-1.5 -translate-y-1/2 rotate-45 border-b border-r border-zinc-500"
              />
            </span>
          </div>
          <p className="text-sm text-zinc-500">
            {zh(
              '控制后台JAV 刮削和前端显示的元数据语言。',
              'Controls the metadata language used by background JAV scraping and frontend display.'
            )}
          </p>

          {javMetadataLanguageError && (
            <div className="text-sm text-red-600">{javMetadataLanguageError}</div>
          )}

          <div className="flex justify-end">
            <button
              type="button"
              onClick={handleSaveJavMetadataLanguage}
              disabled={savingJavMetadataLanguage || unchanged}
              className="rounded-xl bg-blue-600 px-3 py-1.5 text-sm text-white disabled:opacity-60"
            >
              {savingJavMetadataLanguage ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
            </button>
          </div>
        </div>
      </section>
    )
  }

  const renderPlayerPanel = () => {
    const showMPVSettings = mpvEnabled && !browserPlaybackOnly
    const currentPlayerTab =
      playerTab === 'hotkeys' ? 'hotkeys' : showMPVSettings && playerTab === 'mpv' ? 'mpv' : 'basic'

    return (
      <div className="space-y-5">
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => setPlayerTab('basic')}
            className={`rounded-xl px-3 py-1.5 text-sm ${
              currentPlayerTab === 'basic'
                ? 'bg-zinc-900 text-white'
                : 'border border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-50'
            }`}
          >
            {zh('基础设置', 'Basic Settings')}
          </button>
          {showMPVSettings ? (
            <button
              type="button"
              onClick={() => setPlayerTab('mpv')}
              className={`rounded-xl px-3 py-1.5 text-sm ${
                currentPlayerTab === 'mpv'
                  ? 'bg-zinc-900 text-white'
                  : 'border border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-50'
              }`}
            >
              {zh('MPV播放器', 'MPV Player')}
            </button>
          ) : null}
          <button
            type="button"
            onClick={() => setPlayerTab('hotkeys')}
            className={`rounded-xl px-3 py-1.5 text-sm ${
              currentPlayerTab === 'hotkeys'
                ? 'bg-zinc-900 text-white'
                : 'border border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-50'
            }`}
          >
            {zh('快捷键', 'Shortcuts')}
          </button>
        </div>
        <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
          {currentPlayerTab === 'basic' ? (
            renderDefaultPlayerSettings()
          ) : currentPlayerTab === 'mpv' ? (
            <div>
              <div className="space-y-6">
                <section className="space-y-3">
                  <h4 className="text-sm font-semibold text-zinc-800">
                    {zh('初始窗口大小', 'Initial Window Size')}
                  </h4>
                  <div className="flex flex-col gap-3">
                    <div className="flex flex-wrap gap-4">
                      <label className="flex items-center gap-2 text-xs font-medium text-zinc-500">
                        <span className="shrink-0">{zh('宽度', 'Width')}</span>
                        <div className="flex items-center gap-2">
                          <input
                            value={playerWindowWidthInput}
                            onChange={(e) => {
                              setPlayerWindowWidthInput(e.target.value)
                              setPlayerBasicError('')
                              setPlayerBasicSuccess('')
                            }}
                            inputMode="numeric"
                            className="w-28 rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-800"
                          />
                          <span className="text-sm text-zinc-500">%</span>
                        </div>
                      </label>

                      <label className="flex items-center gap-2 text-xs font-medium text-zinc-500">
                        <span className="shrink-0">{zh('高度', 'Height')}</span>
                        <div className="flex items-center gap-2">
                          <input
                            value={playerWindowHeightInput}
                            onChange={(e) => {
                              setPlayerWindowHeightInput(e.target.value)
                              setPlayerBasicError('')
                              setPlayerBasicSuccess('')
                            }}
                            inputMode="numeric"
                            className="w-28 rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-800"
                          />
                          <span className="text-sm text-zinc-500">%</span>
                        </div>
                      </label>
                    </div>

                    <p className="text-xs text-zinc-500">
                      {zh(
                        '设置 mpv 启动时的宽高占据屏幕宽高的比例。',
                        'Set the percentage of screen width and height used by the mpv window on startup.'
                      )}
                    </p>
                  </div>
                </section>

                <section className="space-y-3 border-t border-zinc-200 pt-5">
                  <div className="flex flex-wrap items-center gap-3">
                    <h4 className="text-sm font-semibold text-zinc-800">
                      {zh('初始音量', 'Initial Volume')}
                    </h4>
                    <div className="flex items-center gap-2">
                      <input
                        value={playerVolumeInput}
                        onChange={(e) => {
                          setPlayerVolumeInput(e.target.value)
                          setPlayerBasicError('')
                          setPlayerBasicSuccess('')
                        }}
                        inputMode="numeric"
                        className="w-28 rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-800"
                      />
                      <span className="text-sm text-zinc-500">%</span>
                    </div>
                  </div>
                  <p className="text-xs text-zinc-500">
                    {zh(
                      '控制 mpv 启动时的默认音量，范围 0-130。',
                      'Controls the default mpv startup volume, range 0-130.'
                    )}
                  </p>
                </section>

                <section className="space-y-3 border-t border-zinc-200 pt-5">
                  <label className="flex items-center gap-3 text-sm font-semibold text-zinc-800">
                    <input
                      type="checkbox"
                      checked={playerOntopInput}
                      onChange={(e) => {
                        setPlayerOntopInput(e.target.checked)
                        setPlayerBasicError('')
                        setPlayerBasicSuccess('')
                      }}
                      className="h-4 w-4 rounded"
                    />
                    <span>{zh('播放器强行置顶', 'Keep Player On Top')}</span>
                  </label>
                  <p className="text-xs text-zinc-500">
                    {zh(
                      '开启后，mpv 播放器窗口会保持置顶。',
                      'When enabled, the mpv player window stays on top.'
                    )}
                  </p>
                </section>

                <section className="space-y-3 border-t border-zinc-200 pt-5">
                  <label className="flex items-center gap-3 text-sm font-semibold text-zinc-800">
                    <input
                      type="checkbox"
                      checked={playerReuseWindowInput}
                      onChange={(e) => {
                        setPlayerReuseWindowInput(e.target.checked)
                        setPlayerBasicError('')
                        setPlayerBasicSuccess('')
                      }}
                      className="h-4 w-4 rounded"
                    />
                    <span>
                      {zh(
                        '播放新视频时复用当前播放器窗口',
                        'Reuse Current Player Window When Playing a New Video'
                      )}
                    </span>
                  </label>
                  <p className="text-xs text-zinc-500">
                    {zh(
                      '关闭后，每次播放都会启动新的 mpv 播放器窗口。',
                      'When disabled, each playback starts a new mpv player window.'
                    )}
                  </p>
                </section>

                <section className="space-y-3 border-t border-zinc-200 pt-5">
                  <label className="flex items-center gap-3 text-sm font-semibold text-zinc-800">
                    <input
                      type="checkbox"
                      checked={playerResumePlaybackInput}
                      onChange={(e) => {
                        setPlayerResumePlaybackInput(e.target.checked)
                        setPlayerBasicError('')
                        setPlayerBasicSuccess('')
                      }}
                      className="h-4 w-4 rounded"
                    />
                    <span>{zh('从上次结束位置播放', 'Resume From Last Position')}</span>
                  </label>
                  <p className="text-xs text-zinc-500">
                    {zh(
                      'mpv 会记住每个视频的播放位置，下次播放同一文件时自动恢复。',
                      'mpv remembers each video position and resumes the same file automatically.'
                    )}
                  </p>
                </section>

                <section className="space-y-3 border-t border-zinc-200 pt-5">
                  <label className="flex items-center gap-3 text-sm font-semibold text-zinc-800">
                    <input
                      type="checkbox"
                      checked={playerShowHotkeyHintInput}
                      onChange={(e) => {
                        setPlayerShowHotkeyHintInput(e.target.checked)
                        setPlayerBasicError('')
                        setPlayerBasicSuccess('')
                      }}
                      className="h-4 w-4 rounded"
                    />
                    <span>{zh('启动时显示快捷键配置', 'Show Shortcuts on Startup')}</span>
                  </label>
                  <p className="text-xs text-zinc-500">
                    {zh(
                      '在 mpv 打开视频时显示当前快捷键说明。',
                      'Show the current shortcut guide when mpv opens a video.'
                    )}
                  </p>
                </section>
              </div>

              {playerBasicError && (
                <div className="mt-3 text-sm text-red-600">{playerBasicError}</div>
              )}
              {playerBasicSuccess && (
                <div className="mt-3 text-sm text-emerald-600">{playerBasicSuccess}</div>
              )}

              <div className="mt-4 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={resetPlayerBasicInputs}
                  disabled={savingPlayerBasic}
                  className="rounded-xl border border-zinc-200 bg-white px-3 py-1.5 text-sm text-zinc-700 hover:bg-zinc-50 disabled:opacity-60"
                >
                  {zh('恢复默认', 'Restore Defaults')}
                </button>
                <button
                  type="button"
                  onClick={async () => {
                    setPlayerBasicError('')
                    setPlayerBasicSuccess('')
                    const width = Number.parseInt(playerWindowWidthInput, 10)
                    const height = Number.parseInt(playerWindowHeightInput, 10)
                    const volume = Number.parseInt(playerVolumeInput, 10)
                    if (!Number.isFinite(width) || width < 10 || width > 100) {
                      setPlayerBasicError(
                        zh('初始宽度请输入 10-100', 'Initial width must be between 10 and 100')
                      )
                      return
                    }
                    if (!Number.isFinite(height) || height < 10 || height > 100) {
                      setPlayerBasicError(
                        zh('初始高度请输入 10-100', 'Initial height must be between 10 and 100')
                      )
                      return
                    }
                    if (!Number.isFinite(volume) || volume < 0 || volume > 130) {
                      setPlayerBasicError(
                        zh('初始音量请输入 0-130', 'Initial volume must be between 0 and 130')
                      )
                      return
                    }

                    setSavingPlayerBasic(true)
                    try {
                      await onSavePlayerBasicSettings?.({
                        player_window_width: width,
                        player_window_height: height,
                        player_ontop: playerOntopInput,
                        player_reuse_window: playerReuseWindowInput,
                        player_resume_playback: playerResumePlaybackInput,
                        player_volume: volume,
                        player_show_hotkey_hint: playerShowHotkeyHintInput,
                      })
                      setPlayerBasicSuccess(
                        zh('MPV播放器设置保存成功', 'MPV player settings saved')
                      )
                    } catch (err) {
                      setPlayerBasicError(getErrorMessage(err))
                    } finally {
                      setSavingPlayerBasic(false)
                    }
                  }}
                  disabled={savingPlayerBasic}
                  className="rounded-xl bg-blue-600 px-3 py-1.5 text-sm text-white disabled:opacity-60"
                >
                  {savingPlayerBasic ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
                </button>
              </div>
            </div>
          ) : (
            <>
              <div className="mb-4">
                <h4 className="text-sm font-semibold text-zinc-800">{zh('快捷键', 'Shortcuts')}</h4>
                <p className="mt-1 text-xs text-zinc-500">
                  {zh(
                    '正数表示增加，负数表示减少。`Space` 和 `Escape` 仍固定用于播放/暂停和关闭播放器。',
                    'Positive numbers increase, negative numbers decrease. `Space` and `Escape` remain reserved for play/pause and close.'
                  )}
                </p>
              </div>
              <PlayerSettingsModal hotkeys={normalizedPlayerHotkeys} onSave={onSavePlayerHotkeys} />
            </>
          )}
        </section>
      </div>
    )
  }

  const renderDirectoriesPanel = () => (
    <div className="space-y-2">
      <div className="flex items-center gap-1.5 text-xs font-semibold text-zinc-600">
        <InfoOutlinedIcon fontSize="inherit" className="text-[15px]" aria-hidden="true" />
        {zh(
          '添加本地视频目录让 JavBoss 接管，所有内容将自动为您呈现。',
          'No directories yet. Added folders will be scanned automatically.'
        )}
      </div>
      <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
        <DirectoryManager
          open={open}
          directories={directories}
          onCreate={onCreateDirectory}
          onUpdate={onUpdateDirectory}
          onDelete={onDeleteDirectory}
          directoryPickerEnabled={directoryPickerEnabled}
          useHostPaths={hostPathPrefixEnabled}
        />
      </section>
    </div>
  )

  const renderSecurityPanel = () => {
    const handleChangePassword = async (event) => {
      event.preventDefault()
      setPasswordError('')
      const newPasswordLength = [...newPassword].length
      const newPasswordBytes = new TextEncoder().encode(newPassword).length
      if (newPasswordLength < 6 || newPasswordLength > 20 || newPasswordBytes > 72) {
        setPasswordError(zh('新密码需为 6-20 个字符', 'New password must be 6-20 characters'))
        return
      }
      if (newPassword !== newPassword.trim()) {
        setPasswordError(
          zh('新密码首尾不能包含空格', 'New password cannot start or end with spaces')
        )
        return
      }
      if (newPassword !== confirmPassword) {
        setPasswordError(zh('两次输入的新密码不一致', 'The new passwords do not match'))
        return
      }
      setSavingPassword(true)
      try {
        await onChangePassword?.(currentPassword, newPassword)
        setCurrentPassword('')
        setNewPassword('')
        setConfirmPassword('')
        setVisiblePasswords({ current: false, new: false, confirm: false })
        setPasswordDialogOpen(false)
      } catch (err) {
        setPasswordError(getErrorMessage(err))
      } finally {
        setSavingPassword(false)
      }
    }

    const openPasswordDialog = () => {
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
      setVisiblePasswords({ current: false, new: false, confirm: false })
      setPasswordError('')
      setPasswordDialogOpen(true)
    }

    const closePasswordDialog = () => {
      if (savingPassword) return
      setPasswordDialogOpen(false)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
      setVisiblePasswords({ current: false, new: false, confirm: false })
      setPasswordError('')
    }

    return (
      <>
        <div className="flex flex-wrap gap-3">
          <button
            type="button"
            onClick={openPasswordDialog}
            className="rounded-xl border border-zinc-200 bg-white px-4 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50"
          >
            {zh('修改密码', 'Change password')}
          </button>
          <button
            type="button"
            onClick={() => onLogout?.()}
            className="rounded-xl border border-red-200 bg-white px-4 py-2 text-sm font-medium text-red-600 hover:bg-red-50"
          >
            {zh('退出登录', 'Sign out')}
          </button>
        </div>

        {passwordDialogOpen ? (
          <div
            className="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 px-4"
            role="dialog"
            aria-modal="true"
            aria-labelledby="change-password-title"
          >
            <form
              onSubmit={handleChangePassword}
              className="w-full max-w-md rounded-2xl border border-zinc-200 bg-white p-6 shadow-2xl"
            >
              <div className="mb-5 flex items-center justify-between gap-4">
                <h3 id="change-password-title" className="text-lg font-semibold text-zinc-900">
                  {zh('修改密码', 'Change password')}
                </h3>
                <button
                  type="button"
                  onClick={closePasswordDialog}
                  disabled={savingPassword}
                  className="rounded-lg px-2 py-1 text-zinc-500 hover:bg-zinc-100 disabled:opacity-50"
                  aria-label={zh('关闭', 'Close')}
                >
                  ✕
                </button>
              </div>

              <div className="space-y-4">
                {[
                  {
                    id: 'current-password',
                    label: zh('旧密码', 'Current password'),
                    value: currentPassword,
                    setter: setCurrentPassword,
                    autoComplete: 'current-password',
                    visibilityKey: 'current',
                  },
                  {
                    id: 'new-password',
                    label: zh('新密码', 'New password'),
                    value: newPassword,
                    setter: setNewPassword,
                    autoComplete: 'new-password',
                    visibilityKey: 'new',
                  },
                  {
                    id: 'confirm-password',
                    label: zh('确认新密码', 'Confirm new password'),
                    value: confirmPassword,
                    setter: setConfirmPassword,
                    autoComplete: 'new-password',
                    visibilityKey: 'confirm',
                  },
                ].map((field) => (
                  <div key={field.id}>
                    <label
                      htmlFor={field.id}
                      className="mb-1.5 block text-sm font-medium text-zinc-700"
                    >
                      {field.label}
                    </label>
                    <div className="relative">
                      <input
                        id={field.id}
                        type={visiblePasswords[field.visibilityKey] ? 'text' : 'password'}
                        autoComplete={field.autoComplete}
                        value={field.value}
                        onChange={(event) => {
                          field.setter(event.target.value)
                          setPasswordError('')
                        }}
                        className="w-full rounded-xl border border-zinc-200 bg-white py-2 pl-3 pr-16 text-sm text-zinc-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                      />
                      <button
                        type="button"
                        onClick={() =>
                          setVisiblePasswords((current) => ({
                            ...current,
                            [field.visibilityKey]: !current[field.visibilityKey],
                          }))
                        }
                        className="absolute right-2 top-1/2 flex -translate-y-1/2 items-center justify-center rounded-md p-1 text-zinc-400 hover:bg-zinc-100 hover:text-zinc-700"
                        aria-label={
                          visiblePasswords[field.visibilityKey]
                            ? zh(`隐藏${field.label}`, `Hide ${field.label.toLowerCase()}`)
                            : zh(`显示${field.label}`, `Show ${field.label.toLowerCase()}`)
                        }
                      >
                        {visiblePasswords[field.visibilityKey] ? (
                          <VisibilityOutlinedIcon fontSize="small" aria-hidden="true" />
                        ) : (
                          <VisibilityOffOutlinedIcon fontSize="small" aria-hidden="true" />
                        )}
                      </button>
                    </div>
                  </div>
                ))}
              </div>

              {passwordError ? (
                <div className="mt-4 text-sm text-red-600">{passwordError}</div>
              ) : null}

              <div className="mt-6 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={closePasswordDialog}
                  disabled={savingPassword}
                  className="rounded-xl border border-zinc-200 bg-white px-4 py-2 text-sm text-zinc-700 hover:bg-zinc-50 disabled:opacity-50"
                >
                  {zh('取消', 'Cancel')}
                </button>
                <button
                  type="submit"
                  disabled={savingPassword || !currentPassword || !newPassword || !confirmPassword}
                  className="rounded-xl bg-blue-600 px-4 py-2 text-sm text-white hover:bg-blue-700 disabled:opacity-60"
                >
                  {savingPassword ? zh('保存中…', 'Saving...') : zh('确认修改', 'Change password')}
                </button>
              </div>
            </form>
          </div>
        ) : null}
      </>
    )
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4">
      <div className="flex h-[min(86vh,820px)] w-full max-w-6xl flex-col overflow-hidden rounded-[28px] border border-zinc-200 bg-[#f5f5f7] shadow-2xl">
        <div className="flex items-center justify-between border-b border-zinc-200 bg-white/70 px-6 py-4 backdrop-blur">
          <div>
            <h2 className="text-lg font-semibold text-zinc-900">
              {zh('全局设置', 'Global Settings')}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">{zh(activeTitle.zh, activeTitle.en)}</p>
          </div>
          <button
            onClick={onClose}
            className="rounded-xl border border-zinc-200 bg-white px-3 py-1.5 text-sm text-zinc-600 hover:bg-zinc-50"
          >
            {zh('关闭', 'Close')}
          </button>
        </div>

        <div className="flex min-h-0 flex-1 flex-col md:flex-row">
          <aside className="border-b border-zinc-200 bg-white/60 p-3 backdrop-blur md:w-[280px] md:border-b-0 md:border-r">
            <div className="flex gap-2 overflow-x-auto md:flex-col">
              {visibleSections.map((section) => {
                const selected = currentSection === section.id
                const badgeText =
                  section.id === 'player'
                    ? ''
                    : section.id === 'directories'
                      ? String(directories.length)
                      : ''

                return (
                  <button
                    key={section.id}
                    type="button"
                    onClick={() => setActiveSection(section.id)}
                    className={`min-w-[220px] rounded-2xl border px-4 py-3 text-left transition md:min-w-0 ${
                      selected
                        ? 'border-zinc-200 bg-white shadow-sm'
                        : 'border-transparent bg-transparent hover:border-zinc-200 hover:bg-white/80'
                    }`}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <div className="text-sm font-semibold text-zinc-900">
                          {zh(section.title.zh, section.title.en)}
                        </div>
                      </div>
                      {badgeText ? (
                        <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-xs font-medium text-zinc-600">
                          {badgeText}
                        </span>
                      ) : null}
                    </div>
                  </button>
                )
              })}
            </div>
          </aside>

          <section
            className={`min-h-0 flex-1 overflow-y-auto px-4 pb-4 pt-4 md:px-6 md:pb-6 ${
              currentSection === 'directories' ? 'md:pt-3' : 'md:pt-6'
            }`}
          >
            {currentSection === 'display' && renderDisplayPanel()}
            {currentSection === 'network' && renderProxyPanel()}
            {currentSection === 'player' && renderPlayerPanel()}
            {currentSection === 'directories' && renderDirectoriesPanel()}
            {currentSection === 'security' && renderSecurityPanel()}
          </section>
        </div>
      </div>
    </div>
  )
}
