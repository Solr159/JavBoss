import { Popover, Switch } from '@mui/material'
import { useState } from 'react'
import { zh } from '@/utils/i18n'

export default function Pagination({
  page,
  lastPage,
  hasPrev,
  hasNext,
  buildPageUrl,
  onFirst,
  onPrev,
  onGoToPage,
  onNext,
  onLast,
  waterfallMode = false,
  onWaterfallModeChange,
  totalItems = null,
  totalItemsAction = null,
}) {
  const isModifiedClick = (e) =>
    e && (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0)
  const windowSize = 11
  const offset = Math.floor(windowSize / 2)
  const totalPages = lastPage && lastPage > 0 ? lastPage : page + offset
  const start = Math.max(1, Math.min(page - offset, totalPages - windowSize + 1))
  const end = Math.min(totalPages, start + windowSize - 1)
  const canJump = totalPages > 1
  const prevTenPage = Math.max(1, page - 10)
  const nextTenPage = Math.min(totalPages, page + 10)
  const hasPrevTen = page > 1
  const hasNextTen = page < totalPages
  const paginationDisabled = Boolean(waterfallMode)
  const [jumpAnchorEl, setJumpAnchorEl] = useState(null)
  const jumpColumnCount = Math.min(6, totalPages)
  const jumpPanelWidth = Math.min(504, Math.max(162, jumpColumnCount * 50.4 + 21.6))
  const normalizedTotalItems = Number(totalItems)
  const totalItemsLabel =
    Number.isFinite(normalizedTotalItems) && normalizedTotalItems >= 0
      ? zh(`${normalizedTotalItems} 项`, `${normalizedTotalItems} items`)
      : ''
  const pages = []
  for (let p = start; p <= end; p++) pages.push(p)

  const jumpOptions = []
  for (let p = 1; p <= totalPages; p++) jumpOptions.push(p)

  const openJumpPicker = (event) => {
    setJumpAnchorEl(event.currentTarget)
  }

  const closeJumpPicker = () => {
    setJumpAnchorEl(null)
  }

  const ignoreClick = (e, enabled = true) => {
    if (paginationDisabled || !enabled) {
      e.preventDefault()
      return true
    }
    return isModifiedClick(e)
  }

  return (
    <div className="pagination-root relative flex w-full flex-col items-center">
      {onWaterfallModeChange ? (
        <label className="pagination-waterfall fixed z-30 inline-flex shrink-0 -translate-y-1/2 items-center text-gray-600">
          <span>{zh('瀑布流', 'Waterfall')}</span>
          <Switch
            className="pagination-waterfall-switch"
            size="small"
            checked={waterfallMode}
            onChange={(event) => onWaterfallModeChange(event.target.checked)}
            inputProps={{ 'aria-label': zh('切换瀑布流模式', 'Toggle waterfall mode') }}
          />
          {totalItemsLabel ? (
            <span className="pagination-waterfall-total whitespace-nowrap text-gray-500">
              {totalItemsLabel}
            </span>
          ) : null}
          {totalItemsAction ? (
            <span className="pagination-waterfall-action inline-flex">{totalItemsAction}</span>
          ) : null}
        </label>
      ) : null}
      <div className="pagination-controls flex flex-wrap items-center justify-center">
        {!waterfallMode ? (
          <>
            <a
              href={buildPageUrl ? buildPageUrl({ page: 1 }) : '#'}
              onClick={(e) => {
                if (ignoreClick(e, hasPrev)) return
                e.preventDefault()
                onFirst()
              }}
              className={`pagination-button border ${
                paginationDisabled || !hasPrev ? 'pointer-events-none opacity-50' : ''
              }`}
              aria-disabled={paginationDisabled || !hasPrev}
              aria-label={zh('首页', 'First page')}
            >
              {zh('« 首页', '« First')}
            </a>
            <a
              href={buildPageUrl ? buildPageUrl({ page: prevTenPage }) : '#'}
              onClick={(e) => {
                if (ignoreClick(e, hasPrevTen)) return
                e.preventDefault()
                onGoToPage(prevTenPage)
              }}
              className={`pagination-button border ${
                paginationDisabled || !hasPrevTen ? 'pointer-events-none opacity-50' : ''
              }`}
              aria-disabled={paginationDisabled || !hasPrevTen}
              aria-label={zh('上十页', 'Previous 10 pages')}
            >
              {zh('‹ 上十页', '‹ -10')}
            </a>
            <a
              href={buildPageUrl ? buildPageUrl({ page: page - 1 }) : '#'}
              onClick={(e) => {
                if (ignoreClick(e, hasPrev)) return
                e.preventDefault()
                onPrev()
              }}
              className={`pagination-button border ${
                paginationDisabled || !hasPrev ? 'pointer-events-none opacity-50' : ''
              }`}
              aria-disabled={paginationDisabled || !hasPrev}
              aria-label={zh('上一页', 'Previous page')}
            >
              {zh('‹ 上一页', '‹ Prev')}
            </a>

            {pages.map((p) => (
              <a
                key={p}
                href={buildPageUrl ? buildPageUrl({ page: p }) : '#'}
                onClick={(e) => {
                  if (ignoreClick(e)) return
                  e.preventDefault()
                  onGoToPage(p)
                }}
                className={`pagination-button pagination-page-button border ${
                  paginationDisabled
                    ? 'pointer-events-none opacity-50'
                    : p === page
                      ? 'border-blue-600 bg-blue-600 text-white'
                      : 'bg-white'
                }`}
                aria-disabled={paginationDisabled}
                aria-current={p === page ? 'page' : undefined}
              >
                {p}
              </a>
            ))}

            <a
              href={buildPageUrl ? buildPageUrl({ page: page + 1 }) : '#'}
              onClick={(e) => {
                if (ignoreClick(e, hasNext)) return
                e.preventDefault()
                onNext()
              }}
              className={`pagination-button border ${
                paginationDisabled || !hasNext ? 'pointer-events-none opacity-50' : ''
              }`}
              aria-disabled={paginationDisabled || !hasNext}
              aria-label={zh('下一页', 'Next page')}
            >
              {zh('下一页 ›', 'Next ›')}
            </a>
            <a
              href={buildPageUrl ? buildPageUrl({ page: nextTenPage }) : '#'}
              onClick={(e) => {
                if (ignoreClick(e, hasNextTen)) return
                e.preventDefault()
                onGoToPage(nextTenPage)
              }}
              className={`pagination-button border ${
                paginationDisabled || !hasNextTen ? 'pointer-events-none opacity-50' : ''
              }`}
              aria-disabled={paginationDisabled || !hasNextTen}
              aria-label={zh('下十页', 'Next 10 pages')}
            >
              {zh('下十页 ›', '+10 ›')}
            </a>
            <a
              href={buildPageUrl ? buildPageUrl({ page: lastPage }) : '#'}
              onClick={(e) => {
                if (ignoreClick(e, hasNext)) return
                e.preventDefault()
                onLast()
              }}
              className={`pagination-button border ${
                paginationDisabled || !hasNext ? 'pointer-events-none opacity-50' : ''
              }`}
              aria-disabled={paginationDisabled || !hasNext}
              aria-label={zh('末页', 'Last page')}
            >
              {zh('末页 »', 'Last »')}
            </a>
            <button
              type="button"
              onClick={openJumpPicker}
              className={`pagination-button border ${
                paginationDisabled || !canJump ? 'cursor-not-allowed opacity-50' : 'bg-white'
              }`}
              disabled={paginationDisabled || !canJump}
              aria-haspopup="dialog"
              aria-expanded={Boolean(jumpAnchorEl)}
              aria-label={zh('跳转到指定页码', 'Jump to page')}
            >
              {zh('跳转', 'Jump')}
            </button>
            <Popover
              open={Boolean(jumpAnchorEl)}
              anchorEl={jumpAnchorEl}
              onClose={closeJumpPicker}
              disableScrollLock
              anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
              transformOrigin={{ vertical: 'top', horizontal: 'right' }}
            >
              <div
                className="pagination-jump-panel flex flex-col"
                style={{ width: jumpPanelWidth }}
              >
                <div className="pagination-jump-title text-gray-500">
                  {zh('选择页码', 'Select page')}
                </div>
                <div
                  className="pagination-jump-grid grid overflow-y-auto"
                  style={{ gridTemplateColumns: `repeat(${jumpColumnCount}, minmax(0, 1fr))` }}
                >
                  {jumpOptions.map((optionPage) => (
                    <button
                      key={optionPage}
                      type="button"
                      onClick={() => {
                        closeJumpPicker()
                        if (optionPage !== page) onGoToPage(optionPage)
                      }}
                      className={`pagination-jump-button border text-center ${
                        optionPage === page
                          ? 'border-blue-600 bg-blue-600 text-white'
                          : 'bg-white hover:border-blue-300 hover:text-blue-600'
                      }`}
                    >
                      {optionPage}
                    </button>
                  ))}
                </div>
              </div>
            </Popover>
          </>
        ) : null}
      </div>
    </div>
  )
}
