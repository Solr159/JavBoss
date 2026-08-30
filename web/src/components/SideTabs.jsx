import { useEffect, useState } from 'react'
import ArrowBackRoundedIcon from '@mui/icons-material/ArrowBackRounded'
import ArrowForwardRoundedIcon from '@mui/icons-material/ArrowForwardRounded'
import CollectionsBookmarkOutlinedIcon from '@mui/icons-material/CollectionsBookmarkOutlined'
import DisplaySettingsOutlinedIcon from '@mui/icons-material/DisplaySettingsOutlined'
import DownloadOutlinedIcon from '@mui/icons-material/DownloadOutlined'
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
import { zh } from '@/utils/i18n'

const tabs = [
  { id: 'video', label: zh('视频', 'Video'), icon: VideoLibraryOutlinedIcon },
  { id: 'list', label: 'JAV', icon: MovieCreationOutlinedIcon },
  { id: 'idol', label: zh('女优', 'Idols'), icon: PeopleAltOutlinedIcon },
  { id: 'studio', label: zh('片商', 'Studios'), icon: VideocamOutlinedIcon },
  { id: 'series', label: zh('系列', 'Series'), icon: CollectionsBookmarkOutlinedIcon },
]

function RailButton({
  active = false,
  badge = '',
  badgeTone = '',
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
      aria-label={badge ? `${label} (${badge})` : label}
      aria-current={active ? 'page' : undefined}
      className={`side-tabs__button ${active ? 'side-tabs__button--active' : ''} ${className}`}
    >
      <Icon className="side-tabs__icon" fontSize="small" />
      <span className="side-tabs__label">{label}</span>
      {badge ? (
        <span className={`side-tabs__badge side-tabs__badge--${badgeTone}`}>{badge}</span>
      ) : null}
    </button>
  )
}

export default function SideTabs({
  activeTab,
  canGoBack,
  canGoForward,
  isJavMode,
  javPrefix = '',
  buildJavPrefixUrl,
  onBrowserBack,
  onBrowserForward,
  onOpenDownload,
  onOpenGlobalSettings,
  onOpenJavSettings,
  onOpenJavTagModal,
  onJavPrefixClick,
  onOpenTagModal,
  onOpenVideoSettings,
  onSelectTab,
  showDirectorySetupHint = false,
}) {
  const [prefixModalOpen, setPrefixModalOpen] = useState(false)
  const [prefixItems, setPrefixItems] = useState([])
  const [prefixLoading, setPrefixLoading] = useState(false)
  const [prefixError, setPrefixError] = useState('')

  useEffect(() => {
    if (!prefixModalOpen) return undefined
    let cancelled = false
    setPrefixLoading(true)
    setPrefixError('')
    fetchJavPrefixes()
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
  }, [prefixModalOpen])

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
        <div className="side-tabs__nav-tools">
          <RailButton
            badge={isJavMode ? 'JAV' : zh('视频', 'Video')}
            badgeTone={isJavMode ? 'jav' : 'video'}
            icon={LocalOfferOutlinedIcon}
            label={zh('标签', 'Tags')}
            onClick={isJavMode ? onOpenJavTagModal : onOpenTagModal}
          />
          <RailButton
            icon={NumbersRoundedIcon}
            label={zh('番号', 'JAV codes')}
            onClick={() => setPrefixModalOpen(true)}
          />
          <RailButton
            icon={DisplaySettingsOutlinedIcon}
            label={zh('显示', 'Display')}
            onClick={isJavMode ? onOpenJavSettings : onOpenVideoSettings}
          />
        </div>
      </nav>

      <div className="side-tabs__tools">
        <RailButton
          className="w-full"
          icon={DownloadOutlinedIcon}
          label={zh('下载', 'Downloads')}
          onClick={onOpenDownload}
        />
        <div className="relative w-full">
          {showDirectorySetupHint ? (
            <div className="directory-setup-hint side-tabs__directory-setup-hint" role="status">
              <ArrowBackRoundedIcon
                className="directory-setup-hint__arrow shrink-0"
                fontSize="small"
                aria-hidden="true"
              />
              <span>
                {zh(
                  '您还没有添加目录，点击 “设置” 在 “目录管理” 内添加。',
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
