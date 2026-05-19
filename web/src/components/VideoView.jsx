import SwapVertIcon from '@mui/icons-material/SwapVert'
import { Button, Popover } from '@mui/material'
import { useState } from 'react'
import Pagination from '@/components/Pagination'
import VideoGrid from '@/components/VideoGrid'
import WaterfallLoader from '@/components/WaterfallLoader'
import {
  VIDEO_SORT_OPTIONS,
  findVideoSortOption,
  reverseVideoSortValue,
  videoSortLabelParts,
} from '@/constants/video'
import { videoSelectionKey } from '@/store'
import { zh } from '@/utils/i18n'

function SortText({ option, value, className = '' }) {
  const parts = videoSortLabelParts(option, value, zh)

  return (
    <span className={`truncate font-semibold ${className}`}>
      <span>{parts.label}</span>
      <span className="font-normal text-gray-500">{parts.separator}</span>
      <span className="font-normal text-gray-500">{parts.direction}</span>
    </span>
  )
}

export default function VideoView({
  selectedCount,
  clearSelection,
  setSelectionOpsOpen,
  page,
  lastPage,
  canPrev,
  canNext,
  loading,
  randomMode,
  videoTempSort,
  videoGlobalSort,
  buildVideoUrl,
  setPage,
  setVideoTempSort,
  goToLastPage,
  videos,
  selectedVideoIds,
  toggleSelectVideo,
  onToggleSelectPage,
  openPlayer,
  openAlternatePlayer,
  revealFile,
  alternatePlayerLabel,
  setTagPickerFor,
  onOpenScreenshots,
  onTagClick,
  waterfallMode,
  onWaterfallModeChange,
  onLoadMore,
  loadingMore,
  hasMore,
}) {
  const [sortAnchorEl, setSortAnchorEl] = useState(null)
  const pageIds = videos.map((video) => videoSelectionKey(video)).filter(Boolean)
  const pageSelectable = pageIds.length > 0
  const pageAllSelected = pageSelectable && pageIds.every((id) => selectedVideoIds.has(id))
  const hasSelection = selectedCount > 0
  const effectiveSort = videoTempSort || videoGlobalSort
  const currentOption = findVideoSortOption(effectiveSort) || VIDEO_SORT_OPTIONS[0]
  const activeWaterfallMode = waterfallMode && !randomMode

  const isOptionActive = (option) => {
    return findVideoSortOption(effectiveSort)?.base === option.base
  }

  const openSortMenu = (event) => {
    setSortAnchorEl(event.currentTarget)
  }

  const closeSortMenu = () => {
    setSortAnchorEl(null)
  }

  return (
    <>
      <div className="sticky-pagination mb-4 grid gap-3 md:grid-cols-[1fr_auto_1fr] md:items-center">
        <div className="flex items-center gap-1 overflow-x-auto overflow-y-hidden whitespace-nowrap">
          {hasSelection && (
            <>
              <span className="rounded-full bg-sky-50 px-2 py-0.5 text-xs font-medium leading-5 text-sky-700">
                {zh(`已选 ${selectedCount} 项`, `${selectedCount} selected`)}
              </span>
              <Button
                variant="outlined"
                size="small"
                onClick={onToggleSelectPage}
                disabled={!pageSelectable}
                className="!min-h-0 !min-w-0 !px-2 !py-0.5 !text-xs !leading-5"
              >
                {pageAllSelected ? zh('取消本页', 'Unselect page') : zh('全选本页', 'Select page')}
              </Button>
              <Button
                variant="outlined"
                size="small"
                onClick={() => setSelectionOpsOpen(true)}
                disabled={selectedCount === 0}
                className="!min-h-0 !min-w-0 !px-2 !py-0.5 !text-xs !leading-5"
              >
                {zh('操作', 'Actions')}
              </Button>
              <Button
                variant="text"
                size="small"
                onClick={clearSelection}
                className="!min-h-0 !min-w-0 !px-2 !py-0.5 !text-xs !leading-5"
              >
                {zh('清空', 'Clear')}
              </Button>
            </>
          )}
        </div>
        <div className="flex justify-center">
          {!randomMode && (
            <Pagination
              page={page}
              lastPage={lastPage}
              hasPrev={canPrev}
              hasNext={canNext}
              loading={loading}
              buildPageUrl={({ page: targetPage }) =>
                buildVideoUrl({ page: targetPage, random: false })
              }
              onFirst={() => setPage(1)}
              onPrev={() => {
                if (canPrev) setPage(page - 1)
              }}
              onGoToPage={(p) => setPage(p)}
              onNext={() => {
                if (canNext) setPage(page + 1)
              }}
              onLast={() => {
                goToLastPage()
              }}
              waterfallMode={activeWaterfallMode}
              onWaterfallModeChange={onWaterfallModeChange}
            />
          )}
        </div>
        <div className="flex justify-end">
          {!randomMode && (
            <div className="flex items-center gap-2">
              <span className="text-xs text-gray-500">{zh('排序', 'Sort')}</span>
              <button
                type="button"
                onClick={openSortMenu}
                aria-haspopup="dialog"
                aria-expanded={Boolean(sortAnchorEl)}
                aria-label={zh('修改当前视频排序方式', 'Change current video sort')}
                className="inline-flex items-center gap-1.5 rounded-md border border-gray-300 bg-white px-2 py-1 text-xs text-gray-700 shadow-sm hover:border-gray-400"
              >
                <SortText option={currentOption} value={effectiveSort} />
                <span
                  aria-hidden="true"
                  className="block h-1.5 w-1.5 rotate-45 border-b border-r border-gray-400"
                />
              </button>
            </div>
          )}
          <Popover
            open={Boolean(sortAnchorEl)}
            anchorEl={sortAnchorEl}
            onClose={closeSortMenu}
            disableScrollLock
            anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
            transformOrigin={{ vertical: 'top', horizontal: 'right' }}
          >
            <div className="flex min-w-[180px] flex-col p-1">
              {VIDEO_SORT_OPTIONS.map((option) => {
                const active = isOptionActive(option)
                const displayValue = active ? effectiveSort : option.defaultValue
                return (
                  <div
                    key={option.base}
                    className={`flex items-center gap-1 rounded ${
                      active ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-50'
                    }`}
                  >
                    <button
                      type="button"
                      onClick={() => {
                        closeSortMenu()
                        setVideoTempSort?.(displayValue)
                      }}
                      className="min-w-0 flex-1 px-2 py-1 text-left text-xs"
                    >
                      <SortText option={option} value={displayValue} />
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        closeSortMenu()
                        setVideoTempSort?.(reverseVideoSortValue(displayValue, option.defaultValue))
                      }}
                      className="mr-1 inline-flex h-6 w-6 shrink-0 items-center justify-center rounded text-gray-500 hover:bg-white hover:text-blue-700"
                      title={zh('反转排序', 'Reverse sort')}
                      aria-label={zh(
                        `反转${option.label[0]}排序`,
                        `Reverse ${option.label[1]} sort`
                      )}
                    >
                      <SwapVertIcon fontSize="inherit" />
                    </button>
                  </div>
                )
              })}
            </div>
          </Popover>
        </div>
      </div>
      {loading ? (
        <div className="mt-4 flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
          {zh('加载中…', 'Loading...')}
        </div>
      ) : (
        <VideoGrid
          videos={videos}
          selectedIds={selectedVideoIds}
          onToggleSelect={toggleSelectVideo}
          onPlay={(video) => openPlayer(video)}
          onOpenFile={(video) => openAlternatePlayer?.(video)}
          onRevealFile={(video) => revealFile?.(video)}
          openFileLabel={alternatePlayerLabel}
          onOpenTagPicker={(vid) => setTagPickerFor(vid)}
          onOpenScreenshots={onOpenScreenshots}
          onTagClick={onTagClick}
        />
      )}
      <WaterfallLoader
        enabled={activeWaterfallMode && !loading}
        hasMore={hasMore}
        loading={loadingMore}
        onLoadMore={onLoadMore}
      />
    </>
  )
}
