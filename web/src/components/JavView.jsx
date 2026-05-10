import SwapVertIcon from '@mui/icons-material/SwapVert'
import { Popover } from '@mui/material'
import { useState } from 'react'
import JavGrid from '@/components/JavGrid'
import Pagination from '@/components/Pagination'
import { JAV_SORT_OPTIONS, findSortOption, reverseSortValue, sortLabelParts } from '@/constants/jav'
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

export default function JavView({
  javPage,
  javLastPage,
  javHasPrev,
  javHasNext,
  javLoading,
  javRandomMode,
  javTempSort,
  javGlobalSort,
  buildJavUrl,
  setJavPage,
  setJavTempSort,
  javItems,
  javGridColumns,
  onPlay,
  onIdolClick,
  onStudioClick,
  onTagClick,
  onEditTags,
  onOpenFile,
  openFileLabel,
  onRevealFile,
  onOpenScreenshots,
}) {
  const contentClass = javRandomMode ? 'mt-4' : ''
  const [sortAnchorEl, setSortAnchorEl] = useState(null)
  const effectiveSort = javTempSort || javGlobalSort
  const currentOption = findSortOption(JAV_SORT_OPTIONS, effectiveSort) || JAV_SORT_OPTIONS[0]

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
        <div className="sticky-pagination mb-4 grid gap-3 md:grid-cols-[1fr_auto_1fr] md:items-center">
          <div className="hidden md:block" />
          <div className="flex justify-center overflow-x-auto">
            <Pagination
              page={javPage}
              lastPage={javLastPage}
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
            />
          </div>
          <div className="flex justify-end">
            <div className="flex items-center gap-2">
              <span className="text-xs text-gray-500">{zh('排序', 'Sort')}</span>
              <button
                type="button"
                onClick={openSortMenu}
                aria-haspopup="dialog"
                aria-expanded={Boolean(sortAnchorEl)}
                aria-label={zh('修改当前 JAV 排序方式', 'Change current JAV sort')}
                className="inline-flex items-center gap-1.5 rounded-md border border-gray-300 bg-white px-2 py-1 text-xs text-gray-700 shadow-sm hover:border-gray-400"
              >
                <SortText option={currentOption} value={effectiveSort} />
                <span
                  aria-hidden="true"
                  className="block h-1.5 w-1.5 rotate-45 border-b border-r border-gray-400"
                />
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
              <div className="flex min-w-[180px] flex-col p-1">
                {JAV_SORT_OPTIONS.map((option) => {
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
                          setJavTempSort?.(displayValue)
                        }}
                        className="min-w-0 flex-1 px-2 py-1 text-left text-xs"
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
            buildJavUrl={buildJavUrl}
            onPlay={onPlay}
            onIdolClick={onIdolClick}
            onStudioClick={onStudioClick}
            onTagClick={onTagClick}
            onEditTags={onEditTags}
            onOpenFile={onOpenFile}
            openFileLabel={openFileLabel}
            onRevealFile={onRevealFile}
            onOpenScreenshots={onOpenScreenshots}
          />
        </div>
      )}
    </>
  )
}
