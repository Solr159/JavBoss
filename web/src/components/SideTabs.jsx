import { useEffect, useMemo, useRef, useState } from 'react'
import ArrowBackRoundedIcon from '@mui/icons-material/ArrowBackRounded'
import ArrowForwardRoundedIcon from '@mui/icons-material/ArrowForwardRounded'
import CollectionsBookmarkOutlinedIcon from '@mui/icons-material/CollectionsBookmarkOutlined'
import DisplaySettingsOutlinedIcon from '@mui/icons-material/DisplaySettingsOutlined'
import FolderOpenOutlinedIcon from '@mui/icons-material/FolderOpenOutlined'
import LocalOfferOutlinedIcon from '@mui/icons-material/LocalOfferOutlined'
import MovieCreationOutlinedIcon from '@mui/icons-material/MovieCreationOutlined'
import NumbersRoundedIcon from '@mui/icons-material/NumbersRounded'
import PeopleAltOutlinedIcon from '@mui/icons-material/PeopleAltOutlined'
import SettingsOutlinedIcon from '@mui/icons-material/SettingsOutlined'
import VideocamOutlinedIcon from '@mui/icons-material/VideocamOutlined'
import VideoLibraryOutlinedIcon from '@mui/icons-material/VideoLibraryOutlined'
import { fetchJavPrefixes } from '@/api'
import JavPrefixModal from '@/components/JavPrefixModal'
import { getErrorMessage } from '@/utils/errors'
import { displayHostPath } from '@/utils/hostPath'
import { zh } from '@/utils/i18n'

const tabs = [
  { id: 'video', label: zh('视频', 'Video'), icon: VideoLibraryOutlinedIcon },
  { id: 'list', label: zh('作品', 'Works'), icon: MovieCreationOutlinedIcon },
  { id: 'idol', label: zh('女优', 'Idols'), icon: PeopleAltOutlinedIcon },
  { id: 'studio', label: zh('片商', 'Studios'), icon: VideocamOutlinedIcon },
  { id: 'series', label: zh('系列', 'Series'), icon: CollectionsBookmarkOutlinedIcon },
]

function RailButton({
  active = false,
  className = '',
  disabled = false,
  icon: Icon,
  label,
  onClick,
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      aria-label={label}
      aria-current={active ? 'page' : undefined}
      className={`side-tabs__button ${active ? 'side-tabs__button--active' : ''} ${className}`}
    >
      <Icon className="side-tabs__icon" fontSize="small" />
      <span className="side-tabs__label">{label}</span>
    </button>
  )
}

export default function SideTabs({
  activeTab,
  canGoBack,
  canGoForward,
  directories = [],
  enabledDirectoryIds = [],
  hostPathPrefixEnabled = false,
  isJavMode,
  javPrefix = '',
  javPrefixDirectoryIds = [],
  buildJavPrefixUrl,
  onBrowserBack,
  onBrowserForward,
  onEnabledDirectoryIdsChange,
  onOpenGlobalSettings,
  onOpenJavSettings,
  onOpenJavTagModal,
  onJavPrefixClick,
  onOpenTagModal,
  onOpenVideoSettings,
  onSelectTab,
  showDirectorySetupHint = false,
}) {
  const directoryMenuRef = useRef(null)
  const [directoryMenuOpen, setDirectoryMenuOpen] = useState(false)
  const [prefixModalOpen, setPrefixModalOpen] = useState(false)
  const [prefixItems, setPrefixItems] = useState([])
  const [prefixLoading, setPrefixLoading] = useState(false)
  const [prefixError, setPrefixError] = useState('')
  const activeDirectories = useMemo(
    () => directories.filter((directory) => !directory?.is_delete),
    [directories]
  )
  const activeDirectoryIds = useMemo(
    () =>
      activeDirectories
        .map((directory) => Number(directory.id))
        .filter((id) => Number.isFinite(id) && id > 0),
    [activeDirectories]
  )
  const enabledDirectorySet = useMemo(
    () => new Set(enabledDirectoryIds.map((id) => Number(id))),
    [enabledDirectoryIds]
  )
  const enabledDirectoryCount = activeDirectoryIds.filter((id) =>
    enabledDirectorySet.has(id)
  ).length
  const directorySummary =
    activeDirectories.length === 0
      ? zh('无目录', 'No directories')
      : enabledDirectoryCount === activeDirectories.length
        ? zh('全部目录', 'All directories')
        : enabledDirectoryCount === 0
          ? zh('未启用目录', 'No directories')
          : zh(
              `${enabledDirectoryCount}/${activeDirectories.length} 个目录`,
              `${enabledDirectoryCount}/${activeDirectories.length} directories`
            )

  useEffect(() => {
    if (!directoryMenuOpen) return undefined
    const handlePointerDown = (event) => {
      if (directoryMenuRef.current?.contains(event.target)) return
      setDirectoryMenuOpen(false)
    }
    const handleKeyDown = (event) => {
      if (event.key === 'Escape') setDirectoryMenuOpen(false)
    }
    document.addEventListener('mousedown', handlePointerDown)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handlePointerDown)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [directoryMenuOpen])

  useEffect(() => {
    if (!prefixModalOpen) return undefined
    let cancelled = false
    setPrefixLoading(true)
    setPrefixError('')
    fetchJavPrefixes({ directoryIds: javPrefixDirectoryIds })
      .then((items) => {
        if (!cancelled) setPrefixItems(Array.isArray(items) ? items : [])
      })
      .catch((error) => {
        if (!cancelled) setPrefixError(getErrorMessage(error))
      })
      .finally(() => {
        if (!cancelled) setPrefixLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [javPrefixDirectoryIds, prefixModalOpen])

  const setDirectoryEnabled = (id, checked) => {
    const next = new Set(enabledDirectorySet)
    if (checked) next.add(id)
    else next.delete(id)
    onEnabledDirectoryIdsChange?.(Array.from(next))
  }

  return (
    <aside className="side-tabs" aria-label={zh('主导航', 'Primary navigation')}>
      <div className="side-tabs__history" aria-label={zh('浏览历史', 'Navigation history')}>
        <button
          type="button"
          onClick={onBrowserBack}
          disabled={!canGoBack}
          aria-label={zh('后退', 'Back')}
        >
          <ArrowBackRoundedIcon fontSize="small" />
        </button>
        <button
          type="button"
          onClick={onBrowserForward}
          disabled={!canGoForward}
          aria-label={zh('前进', 'Forward')}
        >
          <ArrowForwardRoundedIcon fontSize="small" />
        </button>
      </div>

      <nav className="side-tabs__nav">
        {tabs.map((tab) => (
          <RailButton
            key={tab.id}
            active={activeTab === tab.id}
            icon={tab.icon}
            label={tab.label}
            onClick={() => onSelectTab?.(tab.id)}
          />
        ))}
      </nav>

      <div className="side-tabs__tools">
        <RailButton
          icon={LocalOfferOutlinedIcon}
          label={zh('标签', 'Tags')}
          onClick={isJavMode ? onOpenJavTagModal : onOpenTagModal}
        />
        {isJavMode ? (
          <RailButton
            icon={NumbersRoundedIcon}
            label={zh('番号', 'JAV codes')}
            onClick={() => setPrefixModalOpen(true)}
          />
        ) : null}
        <RailButton
          icon={DisplaySettingsOutlinedIcon}
          label={zh('显示', 'Display')}
          onClick={isJavMode ? onOpenJavSettings : onOpenVideoSettings}
        />
        <div ref={directoryMenuRef} className="relative">
          <button
            type="button"
            onClick={() => setDirectoryMenuOpen((open) => !open)}
            className="side-tabs__button w-full"
            aria-label={zh('选择启用目录', 'Choose enabled directories')}
            aria-haspopup="menu"
            aria-expanded={directoryMenuOpen}
          >
            <FolderOpenOutlinedIcon className="side-tabs__icon" fontSize="small" />
            <span className="side-tabs__label">{zh('目录', 'Folders')}</span>
            {enabledDirectoryCount !== activeDirectories.length ? (
              <span className="side-tabs__directory-count">
                {enabledDirectoryCount}/{activeDirectories.length}
              </span>
            ) : null}
          </button>

          {directoryMenuOpen ? (
            <div role="menu" className="filter-menu side-tabs__directory-menu">
              <div className="filter-menu__header">
                <div>
                  <div className="text-sm font-semibold text-slate-800">
                    {zh('启用目录', 'Enabled directories')}
                  </div>
                  <div className="text-xs text-slate-500">{directorySummary}</div>
                </div>
                {activeDirectories.length > 0 ? (
                  <div className="flex gap-1">
                    <button
                      type="button"
                      className="filter-menu__small-action"
                      onClick={() => onEnabledDirectoryIdsChange?.(activeDirectoryIds)}
                    >
                      {zh('全选', 'All')}
                    </button>
                    <button
                      type="button"
                      className="filter-menu__small-action"
                      onClick={() => onEnabledDirectoryIdsChange?.([])}
                    >
                      {zh('清空', 'None')}
                    </button>
                  </div>
                ) : null}
              </div>
              <div className="max-h-[60vh] overflow-y-auto py-1">
                {activeDirectories.length === 0 ? (
                  <div className="px-3 py-4 text-sm text-amber-700">
                    {zh(
                      '还没有添加目录，请在“设置”->“目录管理”中添加。',
                      'No directories. Add one in Settings > Directory Management.'
                    )}
                  </div>
                ) : (
                  activeDirectories.map((directory) => {
                    const id = Number(directory.id)
                    const path = displayHostPath(directory.path, hostPathPrefixEnabled)
                    return (
                      <label key={directory.id} className="filter-menu__check-row">
                        <input
                          type="checkbox"
                          checked={enabledDirectorySet.has(id)}
                          onChange={(event) => setDirectoryEnabled(id, event.target.checked)}
                          className="mt-0.5 h-4 w-4 rounded border-slate-300 text-blue-600"
                        />
                        <span className="min-w-0 flex-1 break-all">
                          {path}
                          {directory.missing ? (
                            <span className="ml-2 text-xs text-rose-600">
                              {zh('目录缺失', 'Missing')}
                            </span>
                          ) : null}
                        </span>
                      </label>
                    )
                  })
                )}
              </div>
            </div>
          ) : null}
        </div>
        <div className="relative w-full">
          {showDirectorySetupHint && !directoryMenuOpen ? (
            <div className="directory-setup-hint side-tabs__directory-setup-hint" role="status">
              <ArrowBackRoundedIcon
                className="directory-setup-hint__arrow shrink-0"
                fontSize="small"
                aria-hidden="true"
              />
              <span>
                {zh(
                  '您还没有添加目录，点击此处在目录管理内添加',
                  'No directories yet. Click here to add one in Directory Management'
                )}
              </span>
            </div>
          ) : null}
          <RailButton
            className="w-full"
            icon={SettingsOutlinedIcon}
            label={zh('设置', 'Settings')}
            onClick={onOpenGlobalSettings}
          />
        </div>
      </div>
      <JavPrefixModal
        open={prefixModalOpen}
        items={prefixItems}
        loading={prefixLoading}
        error={prefixError}
        activePrefix={javPrefix}
        buildPrefixUrl={buildJavPrefixUrl}
        onSelectPrefix={(item) => {
          setPrefixModalOpen(false)
          onJavPrefixClick?.(item)
        }}
        onClose={() => setPrefixModalOpen(false)}
      />
    </aside>
  )
}
