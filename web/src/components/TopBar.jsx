import { useEffect, useMemo, useRef, useState } from 'react'
import { Button, Tooltip } from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import LocalOfferOutlinedIcon from '@mui/icons-material/LocalOfferOutlined'
import ShuffleOutlinedIcon from '@mui/icons-material/ShuffleOutlined'
import SettingsOutlinedIcon from '@mui/icons-material/SettingsOutlined'
import SwapHorizOutlinedIcon from '@mui/icons-material/SwapHorizOutlined'
import DisplaySettingsIcon from '@mui/icons-material/DisplaySettings'
import FolderOpenOutlinedIcon from '@mui/icons-material/FolderOpenOutlined'
import ArrowBackRoundedIcon from '@mui/icons-material/ArrowBackRounded'
import ArrowForwardRoundedIcon from '@mui/icons-material/ArrowForwardRounded'
import KeyboardArrowDownRoundedIcon from '@mui/icons-material/KeyboardArrowDownRounded'
import CloseRoundedIcon from '@mui/icons-material/CloseRounded'
import TuneOutlinedIcon from '@mui/icons-material/TuneOutlined'
import { zh } from '@/utils/i18n'

export default function TopBar({
  onHome,
  canGoBack,
  canGoForward,
  onBrowserBack,
  onBrowserForward,
  isJavMode,
  onToggleMode,
  videoSearchInput,
  onVideoSearchInputChange,
  onSubmitVideoSearch,
  videoSearchHref,
  randomHref,
  onRandomClick,
  onOpenTagModal,
  onOpenJavTagModal,
  onOpenVideoSettings,
  onOpenJavSettings,
  onOpenGlobalSettings,
  javSearchInput,
  onJavSearchInputChange,
  onSubmitJavSearch,
  javSearchHref,
  javRandomHref,
  javRandomMode,
  onJavRandomClick,
  onJavFilterRandomClick,
  onCancelJavFilterRandom,
  showJavFilterRandomButton,
  isModifiedClick,
  javTab,
  onSwitchJavTab,
  filterSummary,
  onOpenJavQueryEditor,
  showDirectorySetupHint,
  directories = [],
  enabledDirectoryIds = [],
  onEnabledDirectoryIdsChange,
}) {
  const headerRef = useRef(null)
  const directoryMenuRef = useRef(null)
  const [directoryMenuOpen, setDirectoryMenuOpen] = useState(false)
  const headerClassName = ['sticky top-0 z-40 border-b bg-white/80 backdrop-blur']
    .filter(Boolean)
    .join(' ')
  const activeDirectories = useMemo(
    () =>
      Array.isArray(directories) ? directories.filter((directory) => !directory?.is_delete) : [],
    [directories]
  )
  const enabledDirectorySet = useMemo(
    () => new Set((enabledDirectoryIds || []).map((id) => Number(id))),
    [enabledDirectoryIds]
  )
  const activeDirectoryIds = useMemo(
    () =>
      activeDirectories
        .map((directory) => Number(directory.id))
        .filter((id) => Number.isFinite(id)),
    [activeDirectories]
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
          ? zh('未启用目录', 'No directories enabled')
          : zh(
              `启用 ${enabledDirectoryCount}/${activeDirectories.length}`,
              `${enabledDirectoryCount}/${activeDirectories.length} enabled`
            )

  const updateTopbarOffset = () => {
    const height = headerRef.current?.getBoundingClientRect().height || 0
    document.documentElement.style.setProperty('--topbar-height', `${Math.round(height)}px`)
  }

  useEffect(() => {
    updateTopbarOffset()
    window.addEventListener('resize', updateTopbarOffset)
    return () => window.removeEventListener('resize', updateTopbarOffset)
  }, [])

  useEffect(() => {
    updateTopbarOffset()
  }, [isJavMode, javTab, javRandomMode])

  useEffect(() => {
    if (!directoryMenuOpen) return

    const handlePointerDown = (event) => {
      if (directoryMenuRef.current?.contains(event.target)) return
      setDirectoryMenuOpen(false)
    }
    const handleKeyDown = (event) => {
      if (event.key === 'Escape') {
        setDirectoryMenuOpen(false)
      }
    }

    document.addEventListener('mousedown', handlePointerDown)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handlePointerDown)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [directoryMenuOpen])

  const handleSettingsClick = () => {
    if (isJavMode) {
      onOpenJavSettings?.()
    } else {
      onOpenVideoSettings?.()
    }
  }

  const filterLabelPrefix = zh('筛选条件：', 'Filters:')

  const setDirectoryEnabled = (id, checked) => {
    const next = new Set(enabledDirectorySet)
    if (checked) {
      next.add(id)
    } else {
      next.delete(id)
    }
    onEnabledDirectoryIdsChange?.(Array.from(next))
  }

  return (
    <header ref={headerRef} className={headerClassName}>
      <div className="mx-auto flex max-w-screen-2xl flex-col gap-3 px-6 py-3">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex min-w-0 flex-1 flex-wrap items-center gap-3">
            <button
              type="button"
              onClick={onHome}
              className="cursor-pointer select-none rounded text-left text-xl font-bold focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
            >
              Pornboss
            </button>

            <div className="flex min-w-0 flex-1 items-center gap-2">
              <div className="flex flex-wrap items-center gap-2">
                {isJavMode ? (
                  <div className="flex items-center gap-2">
                    <Button
                      variant={javTab === 'list' ? 'contained' : 'outlined'}
                      onClick={() => onSwitchJavTab('list')}
                    >
                      {zh('作品', 'JAV')}
                    </Button>
                    <Button
                      variant={javTab === 'idol' ? 'contained' : 'outlined'}
                      onClick={() => onSwitchJavTab('idol')}
                    >
                      {zh('女优', 'Idol')}
                    </Button>
                    <Button
                      variant={javTab === 'studio' ? 'contained' : 'outlined'}
                      onClick={() => onSwitchJavTab('studio')}
                    >
                      {zh('片商', 'Studio')}
                    </Button>
                    <Button
                      variant={javTab === 'series' ? 'contained' : 'outlined'}
                      onClick={() => onSwitchJavTab('series')}
                    >
                      {zh('系列', 'Series')}
                    </Button>
                    <form
                      onSubmit={onSubmitJavSearch}
                      className="flex items-center overflow-hidden rounded-full border border-gray-200 bg-white shadow-sm"
                    >
                      <input
                        value={javSearchInput}
                        onChange={(e) => onJavSearchInputChange(e.target.value)}
                        placeholder={
                          javTab === 'idol'
                            ? zh('搜索女优名称', 'Search idol name')
                            : javTab === 'studio'
                              ? zh('搜索片商名称', 'Search studio name')
                              : javTab === 'series'
                                ? zh('搜索系列名称', 'Search series name')
                                : zh('搜索番号或标题', 'Search code or title')
                        }
                        className="h-10 flex-1 border-0 bg-white px-4 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                        aria-label={zh('搜索JAV', 'Search JAV')}
                      />
                      <Button
                        component="a"
                        href={javSearchHref}
                        aria-label={zh('搜索JAV', 'Search JAV')}
                        variant="contained"
                        size="medium"
                        onClick={(e) => {
                          if (isModifiedClick(e)) return
                          onSubmitJavSearch(e)
                        }}
                        sx={{
                          borderTopLeftRadius: 0,
                          borderBottomLeftRadius: 0,
                          minHeight: '40px',
                          height: '40px',
                          px: 2.5,
                        }}
                      >
                        <SearchIcon fontSize="small" />
                      </Button>
                    </form>
                    <Button
                      component="a"
                      href={javRandomHref}
                      startIcon={<ShuffleOutlinedIcon fontSize="small" />}
                      variant="outlined"
                      onClick={(e) => {
                        if (isModifiedClick(e)) return
                        e.preventDefault()
                        onJavRandomClick?.()
                      }}
                    >
                      {zh('随机', 'Random')}
                    </Button>
                    <Tooltip title={zh('标签管理', 'Tag management')} arrow>
                      <Button
                        variant="outlined"
                        onClick={onOpenJavTagModal}
                        aria-label={zh('标签管理', 'Tag management')}
                        sx={{
                          minWidth: 36,
                          width: 36,
                          height: 36,
                          p: 0,
                        }}
                      >
                        <LocalOfferOutlinedIcon fontSize="small" />
                      </Button>
                    </Tooltip>
                  </div>
                ) : (
                  <>
                    <form
                      onSubmit={onSubmitVideoSearch}
                      className="flex items-center overflow-hidden rounded-full border border-gray-200 bg-white shadow-sm"
                    >
                      <input
                        value={videoSearchInput}
                        onChange={(e) => onVideoSearchInputChange(e.target.value)}
                        placeholder={zh('搜索文件名', 'Search filename')}
                        className="h-10 flex-1 border-0 bg-white px-4 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                        aria-label={zh('搜索视频', 'Search videos')}
                      />
                      <Button
                        component="a"
                        href={videoSearchHref}
                        aria-label={zh('搜索视频', 'Search videos')}
                        variant="contained"
                        size="medium"
                        onClick={(e) => {
                          if (isModifiedClick(e)) return
                          onSubmitVideoSearch(e)
                        }}
                        sx={{
                          borderTopLeftRadius: 0,
                          borderBottomLeftRadius: 0,
                          minHeight: '40px',
                          height: '40px',
                          px: 2.5,
                        }}
                      >
                        <SearchIcon fontSize="small" />
                      </Button>
                    </form>
                    <div className="flex items-center gap-2">
                      <Button
                        component="a"
                        href={randomHref}
                        startIcon={<ShuffleOutlinedIcon fontSize="small" />}
                        variant="outlined"
                        onClick={(e) => {
                          if (isModifiedClick(e)) return
                          e.preventDefault()
                          onRandomClick()
                        }}
                      >
                        {zh('随机', 'Random')}
                      </Button>
                    </div>
                    <Button
                      startIcon={<LocalOfferOutlinedIcon fontSize="small" />}
                      variant="outlined"
                      onClick={onOpenTagModal}
                    >
                      {zh('标签', 'Tag')}
                    </Button>
                  </>
                )}

                {isJavMode ? (
                  <Tooltip title={zh('显示设置', 'Display settings')} arrow>
                    <Button
                      variant="outlined"
                      onClick={handleSettingsClick}
                      aria-label={zh('显示设置', 'Display settings')}
                      sx={{
                        minWidth: 36,
                        width: 36,
                        height: 36,
                        p: 0,
                      }}
                    >
                      <DisplaySettingsIcon fontSize="small" />
                    </Button>
                  </Tooltip>
                ) : (
                  <Button
                    startIcon={<DisplaySettingsIcon fontSize="small" />}
                    variant="outlined"
                    onClick={handleSettingsClick}
                    aria-label={zh('设置', 'Settings')}
                  >
                    {zh('设置', 'Settings')}
                  </Button>
                )}
              </div>

              {isJavMode && javTab === 'list' ? (
                <div className="flex min-w-0 flex-1 items-center gap-1">
                  {filterSummary ? (
                    <Tooltip title={`${filterLabelPrefix}${filterSummary}`} arrow>
                      <span className="min-w-0 truncate whitespace-nowrap text-xs text-gray-500">
                        {filterLabelPrefix}
                        <span className="font-semibold text-gray-700">{filterSummary}</span>
                      </span>
                    </Tooltip>
                  ) : null}
                  <Tooltip title={zh('编辑 JAV 查询条件', 'Edit JAV filters')} arrow>
                    <button
                      type="button"
                      onClick={onOpenJavQueryEditor}
                      className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-sm text-gray-400 hover:bg-gray-100 hover:text-gray-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                      aria-label={zh('编辑 JAV 查询条件', 'Edit JAV filters')}
                    >
                      <TuneOutlinedIcon fontSize="inherit" className="text-[16px]" />
                    </button>
                  </Tooltip>
                  {showJavFilterRandomButton ? (
                    <span className="inline-flex shrink-0 items-center">
                      <Tooltip
                        title={zh('基于当前筛选条件随机', 'Random within current filters')}
                        arrow
                      >
                        <button
                          type="button"
                          onClick={onJavFilterRandomClick}
                          className={`inline-flex h-5 w-5 items-center justify-center rounded-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 ${
                            javRandomMode
                              ? 'bg-amber-50 text-amber-600 hover:bg-amber-100'
                              : 'text-gray-400 hover:bg-gray-100 hover:text-gray-600'
                          }`}
                          aria-label={zh('基于当前筛选条件随机', 'Random within current filters')}
                        >
                          <ShuffleOutlinedIcon fontSize="inherit" className="text-[16px]" />
                        </button>
                      </Tooltip>
                      {javRandomMode ? (
                        <Tooltip title={zh('取消筛选随机', 'Cancel filter random')} arrow>
                          <button
                            type="button"
                            onClick={onCancelJavFilterRandom}
                            className="-ml-0.5 inline-flex h-4 w-4 items-center justify-center rounded-sm text-amber-500 hover:bg-amber-100 hover:text-amber-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                            aria-label={zh('取消筛选随机', 'Cancel filter random')}
                          >
                            <CloseRoundedIcon fontSize="inherit" className="text-[14px]" />
                          </button>
                        </Tooltip>
                      ) : null}
                    </span>
                  ) : null}
                </div>
              ) : filterSummary ? (
                <div className="min-w-0 flex-1">
                  <Tooltip title={`${filterLabelPrefix}${filterSummary}`} arrow>
                    <span className="inline-block max-w-full truncate whitespace-nowrap text-xs text-gray-500">
                      {filterLabelPrefix}
                      <span className="font-semibold text-gray-700">{filterSummary}</span>
                    </span>
                  </Tooltip>
                </div>
              ) : null}
            </div>
          </div>

          <div className="mt-0.5 flex flex-shrink-0 flex-wrap items-center justify-end gap-2">
            {!showDirectorySetupHint ? (
              <>
                <Button
                  type="button"
                  variant="outlined"
                  onClick={onBrowserBack}
                  disabled={!canGoBack}
                  title={zh('浏览器后退', 'Browser back')}
                  aria-label={zh('浏览器后退', 'Browser back')}
                  sx={{
                    minWidth: 36,
                    width: 36,
                    height: 36,
                    p: 0,
                  }}
                >
                  <ArrowBackRoundedIcon fontSize="small" />
                </Button>
                <Button
                  type="button"
                  variant="outlined"
                  onClick={onBrowserForward}
                  disabled={!canGoForward}
                  title={zh('浏览器前进', 'Browser forward')}
                  aria-label={zh('浏览器前进', 'Browser forward')}
                  sx={{
                    minWidth: 36,
                    width: 36,
                    height: 36,
                    p: 0,
                  }}
                >
                  <ArrowForwardRoundedIcon fontSize="small" />
                </Button>
              </>
            ) : null}
            {showDirectorySetupHint ? (
              <div
                className="directory-setup-hint flex max-w-full items-center gap-2 rounded-full border border-amber-200 bg-amber-50 px-3 py-1.5 text-xs font-medium text-amber-900 shadow-sm"
                role="status"
              >
                <span className="min-w-0 truncate">
                  {zh(
                    '您还没有添加目录，点击此处在目录管理内添加',
                    'No directories yet. Click here to add one in Directory Management'
                  )}
                </span>
                <ArrowForwardRoundedIcon
                  className="directory-setup-hint__arrow shrink-0"
                  fontSize="small"
                  aria-hidden="true"
                />
              </div>
            ) : null}
            <div ref={directoryMenuRef} className="relative inline-flex gap-2">
              <Tooltip title={zh('全局设置', 'Global settings')} arrow>
                <Button
                  variant="outlined"
                  onClick={onOpenGlobalSettings}
                  aria-label={zh('全局设置', 'Global settings')}
                  sx={{
                    minWidth: 36,
                    width: 36,
                    height: 36,
                    p: 0,
                  }}
                >
                  <SettingsOutlinedIcon fontSize="small" />
                </Button>
              </Tooltip>
              <Tooltip title={zh('选择启用目录', 'Choose enabled directories')} arrow>
                <Button
                  type="button"
                  onClick={() => setDirectoryMenuOpen((open) => !open)}
                  aria-label={zh('选择启用目录', 'Choose enabled directories')}
                  aria-haspopup="menu"
                  aria-expanded={directoryMenuOpen}
                  variant="outlined"
                  sx={{
                    minWidth: 54,
                    width: 54,
                    height: 36,
                    p: 0,
                    gap: 0.25,
                  }}
                >
                  <FolderOpenOutlinedIcon fontSize="small" />
                  <KeyboardArrowDownRoundedIcon
                    fontSize="small"
                    className={
                      directoryMenuOpen ? 'rotate-180 transition-transform' : 'transition-transform'
                    }
                  />
                </Button>
              </Tooltip>

              {directoryMenuOpen ? (
                <div
                  role="menu"
                  className="absolute right-0 top-full z-50 mt-2 w-80 overflow-hidden rounded border border-gray-200 bg-white text-left shadow-lg"
                >
                  <div className="flex items-center justify-between gap-2 border-b bg-gray-50 px-3 py-2">
                    <div className="min-w-0">
                      <div className="text-xs font-semibold text-gray-700">
                        {zh('启用目录', 'Enabled directories')}
                      </div>
                      <div className="truncate text-xs text-gray-500">{directorySummary}</div>
                    </div>
                    {activeDirectories.length > 0 ? (
                      <div className="flex shrink-0 items-center gap-1">
                        <button
                          type="button"
                          onClick={() => onEnabledDirectoryIdsChange?.(activeDirectoryIds)}
                          className="rounded border border-gray-200 bg-white px-2 py-1 text-xs text-gray-600 hover:bg-gray-100"
                        >
                          {zh('全选', 'All')}
                        </button>
                        <button
                          type="button"
                          onClick={() => onEnabledDirectoryIdsChange?.([])}
                          className="rounded border border-gray-200 bg-white px-2 py-1 text-xs text-gray-600 hover:bg-gray-100"
                        >
                          {zh('清空', 'None')}
                        </button>
                      </div>
                    ) : null}
                  </div>
                  <div className="max-h-[60vh] overflow-y-auto py-1">
                    {activeDirectories.length === 0 ? (
                      <div className="px-3 py-3 text-sm text-gray-500">
                        {zh('还没有添加目录', 'No directories yet')}
                      </div>
                    ) : (
                      activeDirectories.map((directory) => {
                        const id = Number(directory.id)
                        const checked = enabledDirectorySet.has(id)
                        return (
                          <label
                            key={directory.id}
                            className="flex cursor-pointer items-start gap-2 px-3 py-2 text-sm hover:bg-gray-50"
                          >
                            <input
                              type="checkbox"
                              checked={checked}
                              onChange={(event) => setDirectoryEnabled(id, event.target.checked)}
                              className="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-blue-600"
                              aria-label={zh(
                                `启用目录 ${directory.path}`,
                                `Enable directory ${directory.path}`
                              )}
                            />
                            <span className="min-w-0 flex-1 break-all text-gray-700">
                              {directory.path}
                            </span>
                          </label>
                        )
                      })
                    )}
                  </div>
                </div>
              ) : null}
            </div>
            <Button
              variant="contained"
              color={isJavMode ? 'secondary' : 'primary'}
              startIcon={<SwapHorizOutlinedIcon fontSize="small" />}
              onClick={onToggleMode}
            >
              {isJavMode ? zh('切换到视频', 'To Video') : zh('切换到 JAV', 'To JAV')}
            </Button>
          </div>
        </div>
      </div>
    </header>
  )
}
