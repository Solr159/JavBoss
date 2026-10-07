import { useShallow } from 'zustand/react/shallow'
import { useStore, videoSelectionKey } from '@/store'
import { useState, useEffect, useCallback } from 'react'
import useApplicationData from '@/query/useApplicationData'
import useLibraryRoute from '@/navigation/useLibraryRoute'
import useListDisplay from '@/query/useListDisplay'
import useWebHotkeys from '@/navigation/useWebHotkeys'
import useLibraryFilters from '@/navigation/useLibraryFilters'
import useNotifications from '@/features/notifications/hooks/useNotifications'
import useVideoTagEditor from '@/features/tags/hooks/useVideoTagEditor'
import useVideoOperations from '@/features/video/hooks/useVideoOperations'
import usePlayback from '@/features/playback/hooks/usePlayback'
import useVideoSelection from '@/features/video/hooks/useVideoSelection'
import useFavoriteGroups from '@/features/favorites/hooks/useFavoriteGroups'
import useContentRestoration from '@/navigation/useContentRestoration'
import useSectionNavigation from '@/navigation/useSectionNavigation'
import useDocumentTitle from '@/shared/hooks/useDocumentTitle'
import { buildLibraryPageTitle, formatPageTitle } from '@/navigation/pageTitle'
import useJavSelection from '@/features/jav/hooks/useJavSelection'
import SideTabs from '@/app/layout/SideTabs'
import TopBar from '@/app/layout/TopBar'
import {
  createDefaultIdolProfileFilters,
  normalizeIdolProfileFilters,
  isUserJavTag,
} from '@/constants/jav'
import { StudioDetailNavigationContext } from '@/navigation/studioDetailNavigation'
import { JavDetailNavigationContext } from '@/navigation/javDetailNavigation'
import JavRoute from '@/routes/JavRoute'
import VideoRoute from '@/routes/VideoRoute'
import JavDetailRoute from '@/features/jav/components/JavDetailRoute'
import JavStudioDetailModal from '@/features/jav/components/JavStudioDetailModal'
import DownloadView from '@/features/downloads/components/DownloadView'
import JavQueryEditorModal from '@/features/jav/components/JavQueryEditorModal'
import { configFlag } from '@/utils/config'
import VideoSettings from '@/features/settings/components/VideoSettings'
import VideoScreenshotsModal from '@/features/video/components/VideoScreenshotsModal'
import PlayerModal from '@/features/playback/components/PlayerModal'
import VideoScrapeSettingsModal from '@/features/video/components/VideoScrapeSettingsModal'
import JavSettings from '@/features/settings/components/JavSettings'
import JavVideoPickerModal from '@/features/jav/components/JavVideoPickerModal'
import { buildVideoFullPath } from '@/utils/display'
import JavFavoriteModal from '@/features/favorites/components/JavFavoriteModal'
import JavFavoriteManageModal from '@/features/favorites/components/JavFavoriteManageModal'
import { zh } from '@/utils/i18n'
import JavSelectionOpsModal from '@/features/jav/components/JavSelectionOpsModal'
import JavSelectionFavoritesModal from '@/features/favorites/components/JavSelectionFavoritesModal'
import JavSelectionTagsModal from '@/features/tags/components/JavSelectionTagsModal'
import SelectionOpsModal from '@/features/video/components/SelectionOpsModal'
import SelectionTagsModal from '@/features/tags/components/SelectionTagsModal'
import SelectionJavTagsModal from '@/features/tags/components/SelectionJavTagsModal'
import { createJavTag } from '@/features/jav/api'
import TagPickerModal from '@/features/tags/components/TagPickerModal'
import VideoTagManager from '@/features/tags/components/VideoTagManager'
import JavTagManager from '@/features/tags/components/JavTagManager'
import GlobalSettings from '@/features/settings/components/GlobalSettings'
import Toast from '@/shared/ui/Toast'

export default function App() {
  const {
    config,
    selectionPageAvailable,
    selectionAllAvailable,
    selectionLoading,
    tags,
    selectedVideoIds,
    loadTags,
    createTag,
    clearSelection,
    viewMode,
    javTab,
    javSearchTerm,
    javIdolIds,
    javTags,
    javStudioId,
    javStudioName,
    javSeriesId,
    javSeriesName,
    javPrefix,
    javSoloOnly,
    javFavoriteRatingEnabled,
    javFavoriteRatingMin,
    javFavoriteRatingMax,
    javFavoriteGroupId,
    javItems,
    loadJavTags,
    idolProfileFilters,
    favoriteGroupsByType,
    favoriteGroupsLoadingByType,
    favoriteGroupsErrorByType,
    loadJavFavoriteGroups,
  } = useStore(
    useShallow((s) => ({
      config: s.config,
      selectionPageAvailable:
        s.viewMode === 'jav'
          ? s.javItems.some((item) => Number(item?.id) > 0)
          : s.videos.some((item) => videoSelectionKey(item)),
      selectionAllAvailable:
        Number(
          s.viewMode === 'jav'
            ? s.javRandomMode
              ? s.javItems.length
              : s.javTotal
            : s.randomMode
              ? s.videos.length
              : s.total
        ) > 0,
      selectionLoading: s.viewMode === 'jav' ? s.javLoading : s.loading,
      tags: s.tags,
      selectedVideoIds: s.selectedVideoIds,
      loadTags: s.loadTags,
      createTag: s.createTag,
      clearSelection: s.clearSelection,
      viewMode: s.viewMode,
      javTab: s.javTab,
      javSearchTerm: s.javSearchTerm,
      javIdolIds: s.javIdolIds,
      javTags: s.javTags,
      javStudioId: s.javStudioId,
      javStudioName: s.javStudioName,
      javSeriesId: s.javSeriesId,
      javSeriesName: s.javSeriesName,
      javPrefix: s.javPrefix,
      javSoloOnly: s.javSoloOnly,
      javFavoriteRatingEnabled: s.javFavoriteRatingEnabled,
      javFavoriteRatingMin: s.javFavoriteRatingMin,
      javFavoriteRatingMax: s.javFavoriteRatingMax,
      javFavoriteGroupId: s.javFavoriteGroupId,
      javItems: s.javItems,
      loadJavTags: s.loadJavTags,
      idolProfileFilters: s.idolProfileFilters,
      favoriteGroupsByType: s.favoriteGroupsByType,
      favoriteGroupsLoadingByType: s.favoriteGroupsLoadingByType,
      favoriteGroupsErrorByType: s.favoriteGroupsErrorByType,
      loadJavFavoriteGroups: s.loadJavFavoriteGroups,
    }))
  )

  const [tagModalOpen, setTagModalOpen] = useState(false)
  const [tagModalApplyMode, setTagModalApplyMode] = useState('replace')

  const [videoSettingsOpen, setVideoSettingsOpen] = useState(false)
  const [javSettingsOpen, setJavSettingsOpen] = useState(false)
  const [globalSettingsOpen, setGlobalSettingsOpen] = useState(false)
  const [downloadOpen, setDownloadOpen] = useState(false)
  const [javTagModalOpen, setJavTagModalOpen] = useState(false)

  const [javQueryEditorOpen, setJavQueryEditorOpen] = useState(false)

  const [searchInput, setSearchInput] = useState('')
  const [javSearchInput, setJavSearchInput] = useState('')

  const { configLoaded, directoryStateKey } = useApplicationData()

  const {
    hydrated,
    javDetailId,
    javDetailItem,
    javDetailState,
    cacheJavDetail,
    openJavDetail,
    closeJavDetail,
    saveJavDetailState,
    navigateFromJavDetail,
    studioDetailId,
    studioDetailKey,
    studioDetailState,
    studioDetailItem,
    cacheStudioDetail,
    openStudioDetail,
    closeStudioDetail,
    saveStudioDetailState,
    navigateFromStudioDetail,
    browserNavigation,
    handleBrowserBack,
    handleBrowserForward,
    pendingScrollRestoreRef,
    saveScrollBeforeUrlStateChange,
    schedulePendingScrollRestore,
    buildVideoUrl,
    buildJavUrl,
  } = useLibraryRoute({ setJavSearchInput, setSearchInput, configLoaded })
  const { waterfallModes, forceReloadVideos, forceReloadJavByTab, setWaterfallMode } =
    useListDisplay({ configLoaded, hydrated })

  const isJavMode = viewMode === 'jav'
  useWebHotkeys({ isJavMode, waterfallModes, setJavQueryEditorOpen })

  useEffect(() => {
    if (javTab !== 'download') return
    setDownloadOpen(true)
    useStore.getState().clearLegacyDownloadTab()
  }, [javTab])

  const {
    handleVideoTagClick,
    applyJavTagFilter,
    displayJavTagOptions,
    javIdolFilterOptions,
    searchHref,
    javSearchHref,
    handleJavRandomClick,
    handleVideoRandomClick,
    handleFavoriteRatingEnabledChange,
    handleFavoriteRatingRangeChange,
    handleIdolProfileFilterChange,
    activeFilterItems,
    handleClearActiveFilters,
    submitSearch,
    submitJavSearch,
    handleApplyJavQuery,
  } = useLibraryFilters({
    saveScrollBeforeUrlStateChange,
    setSearchInput,
    setJavSearchInput,
    directoryStateKey,
    isJavMode,
    buildVideoUrl,
    searchInput,
    buildJavUrl,
    javSearchInput,
    setJavQueryEditorOpen,
  })

  const {
    toastMessage,
    toastDuration,
    toastId,
    centerToastMessage,
    showToast,
    closeToast,
    showCenterToast,
    closeCenterToast,
  } = useNotifications()
  const {
    tagPickerFor,
    tagPickerSelected,
    openTagEditor,
    tagPickerDirty,
    handleApplyTags,
    handleTagPickerClose,
    handleTagPickerToggle,
  } = useVideoTagEditor({ showCenterToast })

  const {
    scrapeSettingsVideo,
    setScrapeSettingsVideo,
    scrapeSettingsSaving,
    handleRenameVideo,
    handleDeleteVideo,
    handleOpenScrapeSettings,
    handleSaveScrapeSettings,
    handleFetchScrapePossibleCodes,
    handleManualScrape,
    handleLinkExistingJav,
  } = useVideoOperations({ showCenterToast, showToast })

  const {
    javVideoPickerOpen,
    javVideoPickerItem,
    javVideoPickerAction,
    locationPickerOpen,
    locationPickerChoices,
    locationPickerAction,
    playerVideo,
    playerPlaylist,
    playerIndex,
    selectPlayerVideo,
    closePlayer,
    playerStartTime,
    screenshotsVideo,
    setScreenshotsVideo,
    screenshotsAllowSetCover,
    javVideoChoices,
    locationPickerItem,
    containerMode,
    desktopIntegrationEnabled,
    bulkPlaybackEnabled,
    defaultPlayer,
    alternatePlayer,
    alternatePlayerLabel,
    ensurePlaylistAvailable,
    isVideoOpenable,
    closeLocationPicker,
    playVideoFromTime,
    handleOpenPlayer,
    handleOpenAlternatePlayer,
    handleRevealVideoFile,
    closeJavVideoPicker,
    playVideos,
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
  } = usePlayback({ showCenterToast, showToast })
  const {
    selectionOpsOpen,
    setSelectionOpsOpen,
    selectionTagsOpen,
    setSelectionTagsOpen,
    selectionTagAction,
    setSelectionTagAction,
    selectionTagChoices,
    setSelectionTagChoices,
    selectionJavTagsOpen,
    setSelectionJavTagsOpen,
    selectionJavTagChoices,
    setSelectionJavTagChoices,
    selectionJavTagSaving,
    selectionPlaying,
    selectionDeleting,
    videoBulkActionBusy,
    selectedCount,
    selectedList,
    selectedJavIds,
    handleRemoveSelectedVideo,
    handlePlaySelection,
    handleDeleteSelection,
    handleSelectionTagsClose,
    handleSelectionTagChoiceToggle,
    handleApplySelectionTags,
    handleSelectionJavTagsClose,
    handleSelectionJavTagChoiceToggle,
    handleApplySelectionJavTags,
    handleSelectVideoPage,
    handleDeselectVideoPage,
    handleDeselectAllVideos,
    handleSelectAllVideos,
    handlePlayVideoPage,
    handlePlayAllVideos,
  } = useVideoSelection({
    bulkPlaybackEnabled,
    ensurePlaylistAvailable,
    showCenterToast,
    showToast,
    playVideos,
  })

  useEffect(() => {
    const updateScrolledState = () => {
      document.body.classList.toggle('has-page-scroll', window.scrollY > 0)
    }

    updateScrolledState()
    window.addEventListener('scroll', updateScrolledState, { passive: true })
    return () => {
      window.removeEventListener('scroll', updateScrolledState)
      document.body.classList.remove('has-page-scroll')
    }
  }, [])

  const handleOpenTagModal = useCallback(() => {
    loadTags()
    setTagModalApplyMode('replace')
    setTagModalOpen(true)
  }, [loadTags])
  const handleOpenTagFilterEditor = useCallback(() => {
    loadTags()
    setTagModalApplyMode('append')
    setTagModalOpen(true)
  }, [loadTags])

  const {
    idolFavoriteModalOpen,
    idolFavoriteModalItem,
    favoriteModalEntityType,
    idolFavoriteSelectedIds,
    idolFavoriteModalLoading,
    idolFavoriteModalSaving,
    idolFavoriteModalError,
    idolFavoriteManageOpen,
    setIdolFavoriteManageOpen,
    idolFavoriteManageEditGroupId,
    setIdolFavoriteManageEditGroupId,
    favoriteManageEntityType,
    setFavoriteManageEntityType,
    handleOpenFavoriteModal,
    handleOpenIdolFavoriteModal,
    handleCloseIdolFavoriteModal,
    activeFavoriteGroupId,
    handleFavoriteGroupSelect,
    handleCreateFavoriteGroup,
    handleSaveIdolFavoriteGroups,
    handleReorderIdolFavoriteGroups,
    handleRenameIdolFavoriteGroup,
    handleDeleteIdolFavoriteGroup,
    handleLoadIdolFavoriteGroupIdols,
    handleReorderIdolFavoriteGroupIdols,
    handleRemoveIdolFavoriteGroupIdols,
    activeFavoriteEntityType,
    activeFavoriteGroups,
    activeFavoriteGroupsLoading,
    activeFavoriteGroupsError,
    activeSelectedFavoriteGroupId,
  } = useFavoriteGroups({ isJavMode, saveScrollBeforeUrlStateChange, setJavSearchInput })

  useDocumentTitle(
    buildLibraryPageTitle({
      isJavMode,
      javTab,
      filterItems: activeFilterItems,
      javSearchTerm,
      favoriteGroupName: activeFavoriteGroups.find(
        (group) => Number(group.id) === Number(activeSelectedFavoriteGroupId)
      )?.name,
    })
  )
  useDocumentTitle(
    formatPageTitle(globalSettingsOpen ? zh('全局设置', 'Settings') : zh('下载', 'Downloads')),
    { priority: 30, enabled: globalSettingsOpen || downloadOpen }
  )

  const { activeError, showDirectorySetupHint } = useContentRestoration({
    isJavMode,
    hydrated,
    configLoaded,
    pendingScrollRestoreRef,
    schedulePendingScrollRestore,
    waterfallModes,
  })

  const openVideoSettings = useCallback(() => setVideoSettingsOpen(true), [])

  const openJavSettings = useCallback(() => setJavSettingsOpen(true), [])

  const {
    handleHomeClick,
    handleSelectSideTab,
    handleSelectIdol,
    handleJavIdolClick,
    handleSelectStudio,
    handleSelectSeries,
    handleSelectJavPrefix,
    handleJavTagClick,
  } = useSectionNavigation({
    setTagModalOpen,
    setVideoSettingsOpen,
    setJavSettingsOpen,
    setGlobalSettingsOpen,
    isJavMode,
    setJavSearchInput,
    forceReloadJavByTab,
    setSearchInput,
    forceReloadVideos,
    saveScrollBeforeUrlStateChange,
    applyJavTagFilter,
  })

  const handleOpenJavTagModal = useCallback(() => {
    loadJavTags()
    setJavTagModalOpen(true)
  }, [loadJavTags])

  const javSelection = useJavSelection({
    items: javItems,
    bulkPlaybackEnabled,
    ensurePlayAvailable: ensurePlaylistAvailable,
    playVideos: playVideos,
    showToast,
    showError: showCenterToast,
  })

  const javCardActions = {
    onPlay: handleJavPlay,
    onOpenFile: handleJavOpenFile,
    alternatePlayerLabel,
    onRevealFile: handleJavRevealFile,
    onOpenScreenshots: handleJavOpenScreenshots,
    onManageVideoPlay: handleOpenPlayer,
    onManageVideoPlayAtTime: playVideoFromTime,
    onManageVideoCoverChanged: handleVideoCoverChanged,
    onManageVideoOpenFile: handleOpenAlternatePlayer,
    onManageVideoRevealFile: handleRevealVideoFile,
    onManageVideoOpenTagPicker: openTagEditor,
    onManageVideoOpenScreenshots: openJavScreenshots,
    onManageVideoOpenScrapeSettings: handleOpenScrapeSettings,
    onManageVideoRename: handleRenameVideo,
    onManageVideoDelete: handleDeleteVideo,
    onManageVideoTagClick: handleVideoTagClick,
    onIdolClick: handleJavIdolClick,
    onOpenFavorites: handleOpenIdolFavoriteModal,
    onOpenJavFavorites: (item) => handleOpenFavoriteModal('jav', item),
    onOpenStudioFavorites: (studio) => handleOpenFavoriteModal('studio', studio),
    onOpenSeriesFavorites: (series) => handleOpenFavoriteModal('series', series),
    onStudioClick: handleSelectStudio,
    onSeriesClick: handleSelectSeries,
    onPrefixClick: handleSelectJavPrefix,
    onTagClick: handleJavTagClick,
  }

  return (
    <div className="app-shell min-h-screen">
      <SideTabs
        activeTab={isJavMode ? javTab : 'video'}
        buildJavPrefixUrl={(item) =>
          buildJavUrl({
            page: 1,
            tab: 'list',
            search: '',
            idolIds: [],
            tagIds: [],
            studioId: item?.include_studio_filter ? item?.studio_id || 0 : null,
            studioName: item?.include_studio_filter ? item?.studio_name || '' : '',
            seriesId: null,
            prefix: item?.prefix || '',
            soloOnly: false,
            favoriteRatingEnabled: false,
            favoriteGroupId: null,
            random: false,
            tempSort: '',
          })
        }
        canGoBack={browserNavigation.canGoBack}
        canGoForward={browserNavigation.canGoForward}
        isJavMode={isJavMode}
        javPrefix={javPrefix}
        onBrowserBack={handleBrowserBack}
        onBrowserForward={handleBrowserForward}
        onOpenDownload={() => setDownloadOpen(true)}
        onOpenGlobalSettings={() => setGlobalSettingsOpen(true)}
        onOpenJavSettings={openJavSettings}
        onOpenJavTagModal={handleOpenJavTagModal}
        onJavPrefixClick={handleSelectJavPrefix}
        onOpenTagModal={handleOpenTagModal}
        onOpenVideoSettings={openVideoSettings}
        onSelectTab={handleSelectSideTab}
        showDirectorySetupHint={showDirectorySetupHint}
      />
      <TopBar
        buildFavoriteGroupUrl={(groupId) => {
          const parsedGroupId = Number(groupId)
          const targetGroupId =
            Number.isFinite(parsedGroupId) && parsedGroupId > 0 ? parsedGroupId : null
          const parsedCurrentGroupId = Number(activeSelectedFavoriteGroupId)
          const currentGroupId =
            Number.isFinite(parsedCurrentGroupId) && parsedCurrentGroupId > 0
              ? parsedCurrentGroupId
              : null
          if (targetGroupId === currentGroupId) {
            return buildJavUrl({
              page: 1,
              tab: javTab,
              favoriteGroupId: targetGroupId,
            })
          }
          return buildJavUrl({
            page: 1,
            tab: javTab,
            search: '',
            idolIds: [],
            tagIds: [],
            studioId: null,
            studioName: '',
            seriesId: null,
            seriesName: '',
            prefix: '',
            soloOnly: false,
            favoriteRatingEnabled: false,
            idolProfileFilters: createDefaultIdolProfileFilters(),
            favoriteGroupId: targetGroupId,
            random: false,
            tempSort: '',
          })
        }}
        favoriteEntityType={activeFavoriteEntityType}
        favoriteGroups={activeFavoriteGroups}
        favoriteGroupsLoading={activeFavoriteGroupsLoading}
        favoriteGroupsError={activeFavoriteGroupsError}
        favoriteManagerOpen={idolFavoriteManageOpen}
        favoriteRatingEnabled={javFavoriteRatingEnabled}
        favoriteRatingMin={javFavoriteRatingMin}
        favoriteRatingMax={javFavoriteRatingMax}
        idolProfileFilters={idolProfileFilters}
        filterItems={activeFilterItems}
        hasActiveControlFilter={
          isJavMode &&
          ((javTab === 'list' && javFavoriteRatingEnabled) ||
            (javTab === 'idol' &&
              (Boolean(String(javSearchTerm || '').trim()) ||
                Object.values(normalizeIdolProfileFilters(idolProfileFilters)).some(
                  (value) => value.enabled
                ))))
        }
        isJavMode={isJavMode}
        javSearchHref={javSearchHref}
        javSearchInput={javSearchInput}
        javTab={javTab}
        onClearFilters={handleClearActiveFilters}
        onFavoriteGroupSelect={(groupId) =>
          handleFavoriteGroupSelect(activeFavoriteEntityType, groupId)
        }
        onFavoriteRatingEnabledChange={handleFavoriteRatingEnabledChange}
        onFavoriteRatingRangeChange={handleFavoriteRatingRangeChange}
        onIdolProfileFilterChange={handleIdolProfileFilterChange}
        onHome={handleHomeClick}
        onOpenFavoriteGroups={() =>
          loadJavFavoriteGroups(activeFavoriteEntityType, { force: true })
        }
        onOpenFavoriteManager={(group) => {
          const groupId = Number(group?.id)
          setFavoriteManageEntityType(activeFavoriteEntityType)
          setIdolFavoriteManageEditGroupId(Number.isFinite(groupId) && groupId > 0 ? groupId : null)
          setIdolFavoriteManageOpen(true)
        }}
        onOpenFilterEditor={
          isJavMode
            ? javTab === 'list'
              ? () => {
                  setJavQueryEditorOpen(true)
                  loadJavTags()
                }
              : null
            : handleOpenTagFilterEditor
        }
        onOpenSelectionOps={isJavMode ? javSelection.openOps : () => setSelectionOpsOpen(true)}
        onClearSelection={isJavMode ? javSelection.clear : clearSelection}
        onSelectPage={isJavMode ? javSelection.selectPage : handleSelectVideoPage}
        onDeselectPage={isJavMode ? javSelection.deselectPage : handleDeselectVideoPage}
        onSelectAll={isJavMode ? javSelection.selectAll : handleSelectAllVideos}
        onDeselectAll={isJavMode ? javSelection.deselectAll : handleDeselectAllVideos}
        selectionPageAvailable={selectionPageAvailable}
        selectionAllAvailable={selectionAllAvailable}
        selectionBusy={
          selectionLoading ||
          (isJavMode
            ? javSelection.busy
            : videoBulkActionBusy || selectionPlaying || selectionDeleting)
        }
        onRandomClick={
          !isJavMode ? handleVideoRandomClick : javTab === 'list' ? handleJavRandomClick : null
        }
        onSearchInputChange={isJavMode ? setJavSearchInput : setSearchInput}
        onSubmitSearch={isJavMode ? submitJavSearch : submitSearch}
        searchHref={searchHref}
        searchInput={searchInput}
        selectedCount={isJavMode ? javSelection.count : selectedCount}
        selectedFavoriteGroupId={activeSelectedFavoriteGroupId}
      />

      <main className="page-main w-full pb-6 pt-0">
        <StudioDetailNavigationContext.Provider value={openStudioDetail}>
          <JavDetailNavigationContext.Provider value={openJavDetail}>
            {activeError && (
              <div
                role="alert"
                className="mb-4 rounded border border-red-200 bg-red-50 p-3 text-red-700"
              >
                {String(activeError)}
              </div>
            )}

            {isJavMode ? (
              <JavRoute
                hydrated={hydrated}
                configLoaded={configLoaded}
                buildJavUrl={buildJavUrl}
                handleSelectStudio={handleSelectStudio}
                handleSelectIdol={handleSelectIdol}
                handleOpenIdolFavoriteModal={handleOpenIdolFavoriteModal}
                waterfallModes={waterfallModes}
                setWaterfallMode={setWaterfallMode}
                handleSelectSeries={handleSelectSeries}
                handleSelectJavPrefix={handleSelectJavPrefix}
                handleOpenFavoriteModal={handleOpenFavoriteModal}
                javSelection={javSelection}
                bulkPlaybackEnabled={bulkPlaybackEnabled}
                javCardActions={javCardActions}
              />
            ) : (
              <VideoRoute
                hydrated={hydrated}
                configLoaded={configLoaded}
                buildVideoUrl={buildVideoUrl}
                handlePlayVideoPage={handlePlayVideoPage}
                handlePlayAllVideos={handlePlayAllVideos}
                videoBulkActionBusy={videoBulkActionBusy}
                selectionPlaying={selectionPlaying}
                bulkPlaybackEnabled={bulkPlaybackEnabled}
                handleOpenPlayer={handleOpenPlayer}
                containerMode={containerMode}
                alternatePlayer={alternatePlayer}
                handleOpenAlternatePlayer={handleOpenAlternatePlayer}
                desktopIntegrationEnabled={desktopIntegrationEnabled}
                handleRevealVideoFile={handleRevealVideoFile}
                alternatePlayerLabel={alternatePlayerLabel}
                openTagEditor={openTagEditor}
                openVideoScreenshots={openVideoScreenshots}
                handleOpenScrapeSettings={handleOpenScrapeSettings}
                handleRenameVideo={handleRenameVideo}
                handleDeleteVideo={handleDeleteVideo}
                handleVideoTagClick={handleVideoTagClick}
                waterfallModes={waterfallModes}
                setWaterfallMode={setWaterfallMode}
              />
            )}
          </JavDetailNavigationContext.Provider>
        </StudioDetailNavigationContext.Provider>
      </main>
      {configLoaded && hydrated && javDetailId ? (
        <StudioDetailNavigationContext.Provider value={openStudioDetail}>
          <JavDetailRoute
            key={javDetailId}
            {...javCardActions}
            openFileLabel={alternatePlayerLabel}
            buildJavUrl={buildJavUrl}
            itemId={javDetailId}
            initialItem={javDetailItem}
            onLoaded={cacheJavDetail}
            initialState={javDetailState}
            onStateChange={saveJavDetailState}
            onClose={closeJavDetail}
            onStudioClick={(item) => navigateFromJavDetail(() => handleSelectStudio(item))}
            onSeriesClick={(item) => navigateFromJavDetail(() => handleSelectSeries(item))}
            onPrefixClick={(item) => navigateFromJavDetail(() => handleSelectJavPrefix(item))}
            onIdolClick={(item) => navigateFromJavDetail(() => handleJavIdolClick(item))}
            onTagClick={(item) => navigateFromJavDetail(() => handleJavTagClick(item))}
            onManageVideoTagClick={(item) => navigateFromJavDetail(() => handleVideoTagClick(item))}
          />
        </StudioDetailNavigationContext.Provider>
      ) : null}
      {configLoaded && hydrated && studioDetailId ? (
        <JavStudioDetailModal
          key={studioDetailKey}
          studioId={studioDetailId}
          initialItem={studioDetailItem}
          onLoaded={cacheStudioDetail}
          initialState={studioDetailState}
          onStateChange={saveStudioDetailState}
          onClose={closeStudioDetail}
          onSelectStudio={(studio) => navigateFromStudioDetail(() => handleSelectStudio(studio))}
          onSelectSeries={(series) => navigateFromStudioDetail(() => handleSelectSeries(series))}
          onSelectPrefix={(prefix) => navigateFromStudioDetail(() => handleSelectJavPrefix(prefix))}
          onOpenSeriesFavorites={(series) => handleOpenFavoriteModal('series', series)}
          buildJavUrl={buildJavUrl}
        />
      ) : null}

      <DownloadView
        open={downloadOpen}
        onClose={() => setDownloadOpen(false)}
        onToast={showToast}
      />

      <JavQueryEditorModal
        open={javQueryEditorOpen}
        onClose={() => setJavQueryEditorOpen(false)}
        onApply={handleApplyJavQuery}
        search={javSearchTerm}
        idolIds={javIdolIds}
        idolOptions={javIdolFilterOptions}
        tagIds={javTags}
        tagOptions={displayJavTagOptions}
        studioId={javStudioId}
        studioName={javStudioName}
        seriesId={javSeriesId}
        seriesName={javSeriesName}
        prefix={javPrefix}
        soloOnly={javSoloOnly}
        preferChineseName={configFlag(config?.jav_idol_prefer_chinese_name)}
        showSimplifiedTags={configFlag(config?.jav_tag_show_simplified)}
        favoriteGroupId={javFavoriteGroupId}
        favoriteRatingEnabled={javFavoriteRatingEnabled}
        favoriteRatingMin={javFavoriteRatingMin}
        favoriteRatingMax={javFavoriteRatingMax}
      />

      {videoSettingsOpen && (
        <VideoSettings
          onClose={() => setVideoSettingsOpen(false)}
          onError={showCenterToast}
          onWaterfallChange={setWaterfallMode}
        />
      )}

      <VideoScreenshotsModal
        video={screenshotsVideo}
        playerHotkeys={config?.player_hotkeys}
        allowSetCover={screenshotsAllowSetCover}
        onClose={() => setScreenshotsVideo(null)}
        onPlayAtTime={playVideoFromTime}
        onCoverChanged={handleVideoCoverChanged}
      />

      <PlayerModal
        video={playerVideo}
        playlist={playerPlaylist}
        currentIndex={playerIndex}
        onSelectVideo={selectPlayerVideo}
        startTime={playerStartTime}
        resumePlayback={configFlag(config?.browser_player_resume_playback, true)}
        hotkeys={config?.player_hotkeys}
        showHotkeyHint={configFlag(config?.browser_player_show_hotkey_hint, true)}
        onClose={closePlayer}
      />

      <VideoScrapeSettingsModal
        open={Boolean(scrapeSettingsVideo)}
        video={scrapeSettingsVideo}
        saving={scrapeSettingsSaving}
        onClose={() => {
          if (!scrapeSettingsSaving) setScrapeSettingsVideo(null)
        }}
        onSave={handleSaveScrapeSettings}
        onFetchPossibleCodes={handleFetchScrapePossibleCodes}
        onManualScrape={handleManualScrape}
        onLinkExistingJav={handleLinkExistingJav}
      />

      {javSettingsOpen && (
        <JavSettings
          onClose={() => setJavSettingsOpen(false)}
          onError={showCenterToast}
          onWaterfallChange={setWaterfallMode}
          initialTab={javTab === 'list' ? 'jav' : javTab}
        />
      )}

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
      />

      <JavFavoriteModal
        open={idolFavoriteModalOpen}
        entityType={favoriteModalEntityType}
        idol={idolFavoriteModalItem}
        groups={favoriteGroupsByType?.[favoriteModalEntityType] || []}
        selectedIds={idolFavoriteSelectedIds}
        loading={
          idolFavoriteModalLoading ||
          Boolean(favoriteGroupsLoadingByType?.[favoriteModalEntityType])
        }
        saving={idolFavoriteModalSaving}
        error={idolFavoriteModalError || favoriteGroupsErrorByType?.[favoriteModalEntityType] || ''}
        onClose={handleCloseIdolFavoriteModal}
        onCreateGroup={(name) => handleCreateFavoriteGroup(name, favoriteModalEntityType)}
        onSave={handleSaveIdolFavoriteGroups}
        preferChineseName={configFlag(config?.jav_idol_prefer_chinese_name)}
      />

      <JavFavoriteManageModal
        open={idolFavoriteManageOpen}
        entityType={favoriteManageEntityType}
        groups={favoriteGroupsByType?.[favoriteManageEntityType] || []}
        selectedGroupId={activeFavoriteGroupId(favoriteManageEntityType)}
        initialEditGroupId={idolFavoriteManageEditGroupId}
        loading={Boolean(favoriteGroupsLoadingByType?.[favoriteManageEntityType])}
        onClose={() => {
          setIdolFavoriteManageOpen(false)
          setIdolFavoriteManageEditGroupId(null)
        }}
        onCreateGroup={(name) => handleCreateFavoriteGroup(name, favoriteManageEntityType)}
        onReorderGroups={handleReorderIdolFavoriteGroups}
        onRenameGroup={handleRenameIdolFavoriteGroup}
        onDeleteGroup={handleDeleteIdolFavoriteGroup}
        onLoadGroupIdols={handleLoadIdolFavoriteGroupIdols}
        onReorderGroupIdols={handleReorderIdolFavoriteGroupIdols}
        onRemoveGroupIdols={handleRemoveIdolFavoriteGroupIdols}
        preferChineseName={configFlag(config?.jav_idol_prefer_chinese_name)}
      />

      <JavVideoPickerModal
        open={locationPickerOpen}
        title={
          locationPickerAction === 'reveal'
            ? zh('选择定位文件', 'Choose a file to reveal')
            : locationPickerAction === 'open'
              ? alternatePlayer === 'mpv'
                ? zh('选择使用MPV播放器播放的文件', 'Choose a file to play with MPV player')
                : alternatePlayer === 'system'
                  ? zh('选择使用系统播放器播放的文件', 'Choose a file to play with system player')
                  : zh('选择使用网页播放器播放的文件', 'Choose a file to play in the web player')
              : defaultPlayer === 'system'
                ? zh('选择使用系统播放器播放的文件', 'Choose a file to play with system player')
                : defaultPlayer === 'browser'
                  ? zh('选择使用网页播放器播放的文件', 'Choose a file to play in the web player')
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

      <JavSelectionOpsModal
        open={javSelection.opsOpen}
        busy={javSelection.busy}
        onClose={javSelection.closeOps}
        items={javSelection.selectedList}
        bulkPlaybackEnabled={bulkPlaybackEnabled}
        playing={javSelection.playing}
        onRemoveSelected={javSelection.remove}
        onPlaySelected={javSelection.playSelected}
        onOpenTags={javSelection.openTags}
        onOpenFavorites={javSelection.openFavorites}
      />

      <JavSelectionFavoritesModal
        open={javSelection.favoritesOpen}
        selectedCount={javSelection.count}
        groups={favoriteGroupsByType?.jav || []}
        selectedIds={javSelection.favoriteChoices}
        onToggleChoice={javSelection.toggleFavorite}
        onCreateGroup={(name) => handleCreateFavoriteGroup(name, 'jav')}
        onClose={javSelection.closeFavorites}
        onConfirm={javSelection.applyFavorites}
        onReload={() => loadJavFavoriteGroups('jav', { force: true })}
        loading={Boolean(favoriteGroupsLoadingByType?.jav)}
        saving={javSelection.favoritesSaving}
        loadError={favoriteGroupsErrorByType?.jav}
        error={javSelection.favoriteError}
      />

      <JavSelectionTagsModal
        open={javSelection.tagsOpen}
        selectedCount={javSelection.count}
        tags={displayJavTagOptions.filter(isUserJavTag)}
        selectedIds={javSelection.tagChoices}
        onToggleChoice={javSelection.toggleTag}
        onClose={javSelection.closeTags}
        onConfirm={javSelection.applyTags}
        saving={javSelection.saving}
      />

      <SelectionOpsModal
        open={selectionOpsOpen}
        onClose={() => setSelectionOpsOpen(false)}
        selectedList={selectedList}
        selectedCount={selectedCount}
        selectedJavCount={selectedJavIds.length}
        bulkPlaybackEnabled={bulkPlaybackEnabled}
        playing={selectionPlaying}
        deleting={selectionDeleting}
        onRemoveSelected={handleRemoveSelectedVideo}
        onPlaySelected={handlePlaySelection}
        onOpenTags={() => {
          loadTags()
          setSelectionTagAction('add')
          setSelectionTagChoices([])
          setSelectionTagsOpen(true)
        }}
        onOpenJavTags={() => {
          loadJavTags()
          setSelectionJavTagChoices([])
          setSelectionJavTagsOpen(true)
        }}
        onOpenRemoveTags={() => {
          loadTags()
          setSelectionTagAction('remove')
          setSelectionTagChoices([])
          setSelectionTagsOpen(true)
        }}
        onDeleteSelected={handleDeleteSelection}
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

      <SelectionJavTagsModal
        open={selectionJavTagsOpen}
        items={selectedList}
        tags={displayJavTagOptions.filter((tag) => isUserJavTag(tag))}
        selectedIds={selectionJavTagChoices}
        onToggleChoice={handleSelectionJavTagChoiceToggle}
        onCreateTag={async (name) => {
          const tag = await createJavTag(name)
          await loadJavTags({ force: true })
          return tag
        }}
        onClose={handleSelectionJavTagsClose}
        onConfirm={handleApplySelectionJavTags}
        saving={selectionJavTagSaving}
      />

      <TagPickerModal
        open={Boolean(tagPickerFor)}
        tags={tags}
        selectedIds={tagPickerSelected}
        onToggleChoice={handleTagPickerToggle}
        onCreateTag={createTag}
        onClose={handleTagPickerClose}
        onSave={handleApplyTags}
        saveDisabled={!tagPickerDirty}
      />

      <VideoTagManager
        open={tagModalOpen}
        onClose={() => setTagModalOpen(false)}
        tagModalApplyMode={tagModalApplyMode}
      />
      <JavTagManager
        open={javTagModalOpen}
        onClose={() => setJavTagModalOpen(false)}
        displayJavTagOptions={displayJavTagOptions}
        applyJavTagFilter={applyJavTagFilter}
      />
      <GlobalSettings
        onToast={showToast}
        open={globalSettingsOpen}
        onClose={() => setGlobalSettingsOpen(false)}
      />
      <Toast
        key={toastId}
        open={Boolean(toastMessage)}
        message={toastMessage}
        duration={toastDuration}
        onClose={closeToast}
      />
      <Toast
        centered
        open={Boolean(centerToastMessage)}
        message={centerToastMessage}
        onClose={closeCenterToast}
      />
    </div>
  )
}
