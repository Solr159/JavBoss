import SwapVertIcon from '@mui/icons-material/SwapVert'
import { Popover } from '@mui/material'
import { useState } from 'react'
import JavIdolGrid from '@/components/JavIdolGrid'
import Pagination from '@/components/Pagination'
import WaterfallLoader from '@/components/WaterfallLoader'
import {
  IDOL_FAVORITE_ORDER_SORT,
  IDOL_SORT_OPTIONS,
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

export default function JavIdolView({
  page,
  lastPage,
  totalItems,
  hasPrev,
  hasNext,
  loading,
  idolTempSort,
  idolGlobalSort,
  buildPageUrl,
  buildIdolUrl,
  directoryIds = [],
  preferChineseName = false,
  onFirst,
  onPrev,
  onGoToPage,
  onNext,
  onLast,
  items,
  onSelectIdol,
  onOpenFavorites,
  onMerged,
  waterfallMode,
  onWaterfallModeChange,
  setIdolTempSort,
  onLoadMore,
  loadingMore,
  hasMore,
}) {
  const [sortAnchorEl, setSortAnchorEl] = useState(null)
  const effectiveSort = idolTempSort || idolGlobalSort
  const currentOption = findSortOption(IDOL_SORT_OPTIONS, effectiveSort) || IDOL_SORT_OPTIONS[1]
  const isFavoriteOrder = effectiveSort === IDOL_FAVORITE_ORDER_SORT

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
      <div className="sticky-pagination pagination-toolbar-grid relative mb-2.5 grid md:grid-cols-[1fr_auto_1fr] md:items-center">
        <div />
        <div className="flex justify-center overflow-x-auto">
          <Pagination
            page={page}
            lastPage={lastPage}
            totalItems={totalItems}
            hasPrev={hasPrev}
            hasNext={hasNext}
            loading={loading}
            buildPageUrl={buildPageUrl}
            onFirst={onFirst}
            onPrev={onPrev}
            onGoToPage={onGoToPage}
            onNext={onNext}
            onLast={onLast}
            waterfallMode={waterfallMode}
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
              aria-label={zh('修改当前女优排序方式', 'Change current idol sort')}
              className="pagination-sort-button"
            >
              {isFavoriteOrder ? (
                <span className="font-semibold">{zh('自定义顺序', 'Custom order')}</span>
              ) : (
                <SortText option={currentOption} value={effectiveSort} />
              )}
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
              {idolGlobalSort === IDOL_FAVORITE_ORDER_SORT ? (
                <button
                  type="button"
                  onClick={() => {
                    closeSortMenu()
                    setIdolTempSort?.('')
                  }}
                  className={`pagination-sort-option rounded font-semibold ${
                    isFavoriteOrder ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-50'
                  }`}
                >
                  {zh('自定义顺序', 'Custom order')}
                </button>
              ) : null}
              {IDOL_SORT_OPTIONS.map((option) => {
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
                        setIdolTempSort?.(displayValue)
                      }}
                      className="pagination-sort-option"
                    >
                      <SortText option={option} value={displayValue} />
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        closeSortMenu()
                        setIdolTempSort?.(
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
      {loading ? (
        <div className="mt-4 flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
          {zh('加载中…', 'Loading...')}
        </div>
      ) : (
        <JavIdolGrid
          items={items}
          onSelectIdol={onSelectIdol}
          onOpenFavorites={onOpenFavorites}
          onMerged={onMerged}
          buildIdolUrl={buildIdolUrl}
          directoryIds={directoryIds}
          preferChineseName={preferChineseName}
        />
      )}
      <WaterfallLoader
        enabled={waterfallMode && !loading}
        hasMore={hasMore}
        loading={loadingMore}
        onLoadMore={onLoadMore}
      />
    </>
  )
}
