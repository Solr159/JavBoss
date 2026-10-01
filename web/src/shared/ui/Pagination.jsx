import { Popover, Switch } from '@mui/material'
import FirstPageRoundedIcon from '@mui/icons-material/FirstPageRounded'
import KeyboardArrowLeftRoundedIcon from '@mui/icons-material/KeyboardArrowLeftRounded'
import KeyboardArrowRightRoundedIcon from '@mui/icons-material/KeyboardArrowRightRounded'
import KeyboardDoubleArrowLeftRoundedIcon from '@mui/icons-material/KeyboardDoubleArrowLeftRounded'
import KeyboardDoubleArrowRightRoundedIcon from '@mui/icons-material/KeyboardDoubleArrowRightRounded'
import LastPageRoundedIcon from '@mui/icons-material/LastPageRounded'
import ShortcutRoundedIcon from '@mui/icons-material/ShortcutRounded'
import { useEffect, useRef, useState } from 'react'
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
  const jumpGridRef = useRef(null)
  const currentJumpPageRef = useRef(null)
  const jumpColumnCount = Math.min(8, totalPages)
  const jumpPanelWidth = Math.min(672, Math.max(162, jumpColumnCount * 50.4 + 21.6))
  const normalizedTotalItems = Number(totalItems)
  const totalItemsLabel =
    Number.isFinite(normalizedTotalItems) && normalizedTotalItems >= 0
      ? zh(`${normalizedTotalItems} 项`, `${normalizedTotalItems} items`)
      : ''
  const pages = []
  for (let p = start; p <= end; p++) pages.push(p)

  const jumpOptions = []
  for (let p = 1; p <= totalPages; p++) jumpOptions.push(p)

  const toggleJumpPicker = (event) => {
    const anchor = event.currentTarget
    setJumpAnchorEl((current) => (current ? null : anchor))
  }

  const closeJumpPicker = () => {
    setJumpAnchorEl(null)
  }

  useEffect(() => {
    if (!jumpAnchorEl) return undefined
    const frame = window.requestAnimationFrame(() => {
      const grid = jumpGridRef.current
      const currentButton = currentJumpPageRef.current
      if (!grid || !currentButton) return
      grid.scrollTop = Math.max(
        0,
        currentButton.offsetTop -
          grid.offsetTop -
          (grid.clientHeight - currentButton.offsetHeight) / 2
      )
    })
    return () => window.cancelAnimationFrame(frame)
  }, [jumpAnchorEl, page])

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
              <FirstPageRoundedIcon className="pagination-button__icon" aria-hidden="true" />
              <span>{zh('首页', 'First')}</span>
            </a>
            <a
              href={buildPageUrl ? buildPageUrl({ page: prevTenPage }) : '#'}
              onClick={(e) => {
                if (ignoreClick(e, hasPrevTen)) return
                e.preventDefault()
                onGoToPage(prevTenPage)
              }}
              className={`pagination-button pagination-step-button pagination-step-button--back border ${
                paginationDisabled || !hasPrevTen ? 'pointer-events-none opacity-50' : ''
              }`}
              aria-disabled={paginationDisabled || !hasPrevTen}
              aria-label={zh('上十页', 'Previous 10 pages')}
            >
              <KeyboardDoubleArrowLeftRoundedIcon
                className="pagination-button__icon"
                aria-hidden="true"
              />
              <span>{zh('上十页', '-10')}</span>
            </a>
            <a
              href={buildPageUrl ? buildPageUrl({ page: page - 1 }) : '#'}
              onClick={(e) => {
                if (ignoreClick(e, hasPrev)) return
                e.preventDefault()
                onPrev()
              }}
              className={`pagination-button pagination-step-button pagination-step-button--back border ${
                paginationDisabled || !hasPrev ? 'pointer-events-none opacity-50' : ''
              }`}
              aria-disabled={paginationDisabled || !hasPrev}
              aria-label={zh('上一页', 'Previous page')}
            >
              <KeyboardArrowLeftRoundedIcon
                className="pagination-button__icon"
                aria-hidden="true"
              />
              <span>{zh('上一页', 'Prev')}</span>
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
              className={`pagination-button pagination-step-button pagination-step-button--forward border ${
                paginationDisabled || !hasNext ? 'pointer-events-none opacity-50' : ''
              }`}
              aria-disabled={paginationDisabled || !hasNext}
              aria-label={zh('下一页', 'Next page')}
            >
              <span>{zh('下一页', 'Next')}</span>
              <KeyboardArrowRightRoundedIcon
                className="pagination-button__icon"
                aria-hidden="true"
              />
            </a>
            <a
              href={buildPageUrl ? buildPageUrl({ page: nextTenPage }) : '#'}
              onClick={(e) => {
                if (ignoreClick(e, hasNextTen)) return
                e.preventDefault()
                onGoToPage(nextTenPage)
              }}
              className={`pagination-button pagination-step-button pagination-step-button--forward border ${
                paginationDisabled || !hasNextTen ? 'pointer-events-none opacity-50' : ''
              }`}
              aria-disabled={paginationDisabled || !hasNextTen}
              aria-label={zh('下十页', 'Next 10 pages')}
            >
              <span>{zh('下十页', '+10')}</span>
              <KeyboardDoubleArrowRightRoundedIcon
                className="pagination-button__icon"
                aria-hidden="true"
              />
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
              <span>{zh('末页', 'Last')}</span>
              <LastPageRoundedIcon className="pagination-button__icon" aria-hidden="true" />
            </a>
            <button
              type="button"
              data-page-jump-trigger="true"
              onClick={toggleJumpPicker}
              className={`pagination-button border ${
                paginationDisabled || !canJump ? 'cursor-not-allowed opacity-50' : 'bg-white'
              }`}
              disabled={paginationDisabled || !canJump}
              aria-haspopup="dialog"
              aria-expanded={Boolean(jumpAnchorEl)}
              aria-label={zh('跳转到指定页码', 'Jump to page')}
            >
              <ShortcutRoundedIcon className="pagination-button__icon" aria-hidden="true" />
              <span>{zh('跳转', 'Jump')}</span>
            </button>
            <Popover
              open={Boolean(jumpAnchorEl)}
              anchorEl={jumpAnchorEl}
              onClose={closeJumpPicker}
              disableScrollLock
              slotProps={{ paper: { className: 'pagination-jump-popover' } }}
              anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
              transformOrigin={{ vertical: 'top', horizontal: 'center' }}
            >
              <div
                className="pagination-jump-panel flex flex-col"
                style={{ width: jumpPanelWidth }}
              >
                <div className="pagination-jump-title text-gray-500">
                  {zh('选择页码', 'Select page')}
                </div>
                <div
                  ref={jumpGridRef}
                  className="pagination-jump-grid grid overflow-y-auto"
                  style={{ gridTemplateColumns: `repeat(${jumpColumnCount}, minmax(0, 1fr))` }}
                >
                  {jumpOptions.map((optionPage) => (
                    <button
                      key={optionPage}
                      ref={optionPage === page ? currentJumpPageRef : null}
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
                      aria-current={optionPage === page ? 'page' : undefined}
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
