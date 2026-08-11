import SwapVertIcon from '@mui/icons-material/SwapVert'
import { Popover } from '@mui/material'
import { useState } from 'react'
import JavGrid from '@/components/JavGrid'
import Pagination from '@/components/Pagination'
import WaterfallLoader from '@/components/WaterfallLoader'
import {
  JAV_SORT_OPTIONS,
  JAV_SORT_RULE_FILTERS,
  findSortOption,
  reverseSortValue,
  sortLabelParts,
} from '@/constants/jav'
import { zh } from '@/utils/i18n'

function SortText({ option, value, className = '' }) {
  const parts = sortLabelParts(option, value, zh)

  return (
    <span className={`truncate font-semibold ${className}`}>
      <span>{parts.label}</span>
      <span className="font-normal text-gray-500">{parts.separator}</span>
      <span className="font-normal text-gray-500">{parts.direction}</span>
    </span>
  )
}

function matchedRuleLabel(rule) {
  if (!rule) return ''
  const labels = (rule.active || []).map((key) => {
    const filter = JAV_SORT_RULE_FILTERS.find((item) => item.key === key)
    return filter ? zh(filter.label[0], filter.label[1]) : key
  })
  if (labels.length === 0) return zh('匹配全部情况', 'Matches all contexts')
  if (rule.mode === 'any') {
    return zh(`包含任意一个：${labels.join('、')}`, `Includes any: ${labels.join(', ')}`)
  }
  return zh(`全部包含：${labels.join('、')}`, `Includes all: ${labels.join(', ')}`)
}

export default function JavView({
  javPage,
  javLastPage,
  javTotal,
  javHasPrev,
  javHasNext,
  javLoading,
  javRandomMode,
  javResolvedSort,
  javSortSource,
  javMatchedSortRule,
  buildJavUrl,
  setJavPage,
  setJavTempSort,
  javItems,
  javGridColumns,
  javTitleMaxRows,
  javIdolTagMaxRows,
  javTagMaxRows,
  onPlay,
  onIdolClick,
  onOpenFavorites,
  onOpenJavFavorites,
  onOpenStudioFavorites,
  onOpenSeriesFavorites,
  onPrefixClick,
  onStudioClick,
  onSeriesClick,
  onTagClick,
  onOpenFile,
  openFileLabel,
  onRevealFile,
  onOpenScreenshots,
  onManageVideoPlay,
  onManageVideoPlayAtTime,
  onManageVideoCoverChanged,
  onManageVideoOpenFile,
  onManageVideoRevealFile,
  onManageVideoOpenTagPicker,
  onManageVideoOpenScreenshots,
  onManageVideoOpenScrapeSettings,
  onManageVideoRename,
  onManageVideoDelete,
  onManageVideoTagClick,
  waterfallMode,
  onWaterfallModeChange,
  onLoadMore,
  loadingMore,
  hasMore,
}) {
  const contentClass = javRandomMode ? 'mt-4' : ''
  const [sortAnchorEl, setSortAnchorEl] = useState(null)
  const effectiveSort = javResolvedSort
  const currentOption = findSortOption(JAV_SORT_OPTIONS, effectiveSort) || JAV_SORT_OPTIONS[0]
  const activeWaterfallMode = waterfallMode && !javRandomMode

  const isOptionActive = (option) => {
    return findSortOption([option], effectiveSort)
  }

  const openSortMenu = (event) => {
    setSortAnchorEl(event.currentTarget)
  }

  const closeSortMenu = () => {
    setSortAnchorEl(null)
  }

  return (
    <>
      {!javRandomMode && (
        <div className="sticky-pagination pagination-toolbar-grid mb-4 grid md:grid-cols-[1fr_auto_1fr] md:items-center">
          <div className="hidden md:block" />
          <div className="flex justify-center overflow-x-auto">
            <Pagination
              page={javPage}
              lastPage={javLastPage}
              totalItems={javTotal}
              hasPrev={javHasPrev}
              hasNext={javHasNext}
              loading={javLoading}
              buildPageUrl={({ page: targetPage }) => buildJavUrl({ page: targetPage })}
              onFirst={() => setJavPage(1)}
              onPrev={() => {
                if (javHasPrev) setJavPage(javPage - 1)
              }}
              onGoToPage={(p) => setJavPage(p)}
              onNext={() => {
                if (javHasNext) setJavPage(javPage + 1)
              }}
              onLast={() => setJavPage(javLastPage)}
              waterfallMode={activeWaterfallMode}
              onWaterfallModeChange={onWaterfallModeChange}
            />
          </div>
          <div className="flex justify-end">
            <div className="pagination-sort-group flex items-center">
              <span className="pagination-sort-label text-gray-500">{zh('排序', 'Sort')}</span>
              <button
                type="button"
                onClick={openSortMenu}
                aria-haspopup="dialog"
                aria-expanded={Boolean(sortAnchorEl)}
                aria-label={zh('修改当前 JAV 排序方式', 'Change current JAV sort')}
                className="pagination-sort-button"
              >
                <span className="flex min-w-0 flex-col items-start">
                  <SortText option={currentOption} value={effectiveSort} />
                  {javSortSource === 'rule' ? (
                    <span className="max-w-full truncate text-[11px] font-normal text-blue-600">
                      {zh('排序规则', 'Sort rule')}：{matchedRuleLabel(javMatchedSortRule)}
                    </span>
                  ) : javSortSource === 'temporary' ? (
                    <span className="text-[11px] font-normal text-amber-600">
                      {zh('临时排序', 'Temporary sort')}
                    </span>
                  ) : null}
                </span>
                <span aria-hidden="true" className="pagination-sort-caret" />
              </button>
            </div>
            <Popover
              open={Boolean(sortAnchorEl)}
              anchorEl={sortAnchorEl}
              onClose={closeSortMenu}
              disableScrollLock
              anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
              transformOrigin={{ vertical: 'top', horizontal: 'right' }}
            >
              <div className="pagination-sort-menu">
                {javSortSource === 'temporary' ? (
                  <button
                    type="button"
                    onClick={() => {
                      closeSortMenu()
                      setJavTempSort?.('')
                    }}
                    className="w-full border-b border-slate-100 px-3 py-2 text-left text-xs font-medium text-blue-700 hover:bg-blue-50"
                  >
                    {zh('恢复自动排序', 'Restore automatic sort')}
                  </button>
                ) : null}
                {JAV_SORT_OPTIONS.map((option) => {
                  const active = isOptionActive(option)
                  const displayValue = active ? effectiveSort : option.defaultValue
                  return (
                    <div
                      key={option.base}
                      className={`pagination-sort-row ${
                        active ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-50'
                      }`}
                    >
                      <button
                        type="button"
                        onClick={() => {
                          closeSortMenu()
                          setJavTempSort?.(displayValue)
                        }}
                        className="pagination-sort-option"
                      >
                        <SortText option={option} value={displayValue} />
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          closeSortMenu()
                          setJavTempSort?.(
                            reverseSortValue([option], displayValue, option.defaultValue)
                          )
                        }}
                        className="pagination-sort-reverse"
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
      )}
      {javLoading ? (
        <div
          className={`${contentClass} flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500`}
        >
          {zh('加载中…', 'Loading...')}
        </div>
      ) : (
        <div className={contentClass}>
          <JavGrid
            items={javItems}
            columns={javGridColumns}
            titleMaxRows={javTitleMaxRows}
            idolTagMaxRows={javIdolTagMaxRows}
            tagMaxRows={javTagMaxRows}
            buildJavUrl={buildJavUrl}
            onPlay={onPlay}
            onIdolClick={onIdolClick}
            onOpenFavorites={onOpenFavorites}
            onOpenJavFavorites={onOpenJavFavorites}
            onOpenStudioFavorites={onOpenStudioFavorites}
            onOpenSeriesFavorites={onOpenSeriesFavorites}
            onPrefixClick={onPrefixClick}
            onStudioClick={onStudioClick}
            onSeriesClick={onSeriesClick}
            onTagClick={onTagClick}
            onOpenFile={onOpenFile}
            openFileLabel={openFileLabel}
            onRevealFile={onRevealFile}
            onOpenScreenshots={onOpenScreenshots}
            onManageVideoPlay={onManageVideoPlay}
            onManageVideoPlayAtTime={onManageVideoPlayAtTime}
            onManageVideoCoverChanged={onManageVideoCoverChanged}
            onManageVideoOpenFile={onManageVideoOpenFile}
            onManageVideoRevealFile={onManageVideoRevealFile}
            onManageVideoOpenTagPicker={onManageVideoOpenTagPicker}
            onManageVideoOpenScreenshots={onManageVideoOpenScreenshots}
            onManageVideoOpenScrapeSettings={onManageVideoOpenScrapeSettings}
            onManageVideoRename={onManageVideoRename}
            onManageVideoDelete={onManageVideoDelete}
            onManageVideoTagClick={onManageVideoTagClick}
          />
        </div>
      )}
      <WaterfallLoader
        enabled={activeWaterfallMode && !javLoading}
        hasMore={hasMore}
        loading={loadingMore}
        onLoadMore={onLoadMore}
      />
    </>
  )
}
