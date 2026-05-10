import { Popover } from '@mui/material'
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
  const [jumpAnchorEl, setJumpAnchorEl] = useState(null)
  const jumpColumnCount = Math.min(6, totalPages)
  const jumpPanelWidth = Math.min(560, Math.max(180, jumpColumnCount * 56 + 24))
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

  return (
    <div className="flex flex-col items-center gap-1 py-1 text-sm">
      <div className="flex flex-wrap items-center justify-center gap-1.5">
        <a
          href={buildPageUrl ? buildPageUrl({ page: 1 }) : '#'}
          onClick={(e) => {
            if (isModifiedClick(e) || !hasPrev) return
            e.preventDefault()
            onFirst()
          }}
          className={`rounded border px-2 py-0.5 ${!hasPrev ? 'pointer-events-none opacity-50' : ''}`}
          aria-disabled={!hasPrev}
          aria-label={zh('首页', 'First page')}
        >
          {zh('« 首页', '« First')}
        </a>
        <a
          href={buildPageUrl ? buildPageUrl({ page: prevTenPage }) : '#'}
          onClick={(e) => {
            if (isModifiedClick(e) || !hasPrevTen) return
            e.preventDefault()
            onGoToPage(prevTenPage)
          }}
          className={`rounded border px-2 py-0.5 ${!hasPrevTen ? 'pointer-events-none opacity-50' : ''}`}
          aria-disabled={!hasPrevTen}
          aria-label={zh('上十页', 'Previous 10 pages')}
        >
          {zh('‹ 上十页', '‹ -10')}
        </a>
        <a
          href={buildPageUrl ? buildPageUrl({ page: page - 1 }) : '#'}
          onClick={(e) => {
            if (isModifiedClick(e) || !hasPrev) return
            e.preventDefault()
            onPrev()
          }}
          className={`rounded border px-2 py-0.5 ${!hasPrev ? 'pointer-events-none opacity-50' : ''}`}
          aria-disabled={!hasPrev}
          aria-label={zh('上一页', 'Previous page')}
        >
          {zh('‹ 上一页', '‹ Prev')}
        </a>

        {pages.map((p) => (
          <a
            key={p}
            href={buildPageUrl ? buildPageUrl({ page: p }) : '#'}
            onClick={(e) => {
              if (isModifiedClick(e)) return
              e.preventDefault()
              onGoToPage(p)
            }}
            className={`rounded border px-2.5 py-0.5 leading-tight ${p === page ? 'border-blue-600 bg-blue-600 text-white' : 'bg-white'}`}
            aria-current={p === page ? 'page' : undefined}
          >
            {p}
          </a>
        ))}

        <a
          href={buildPageUrl ? buildPageUrl({ page: page + 1 }) : '#'}
          onClick={(e) => {
            if (isModifiedClick(e) || !hasNext) return
            e.preventDefault()
            onNext()
          }}
          className={`rounded border px-2 py-0.5 ${!hasNext ? 'pointer-events-none opacity-50' : ''}`}
          aria-disabled={!hasNext}
          aria-label={zh('下一页', 'Next page')}
        >
          {zh('下一页 ›', 'Next ›')}
        </a>
        <a
          href={buildPageUrl ? buildPageUrl({ page: nextTenPage }) : '#'}
          onClick={(e) => {
            if (isModifiedClick(e) || !hasNextTen) return
            e.preventDefault()
            onGoToPage(nextTenPage)
          }}
          className={`rounded border px-2 py-0.5 ${!hasNextTen ? 'pointer-events-none opacity-50' : ''}`}
          aria-disabled={!hasNextTen}
          aria-label={zh('下十页', 'Next 10 pages')}
        >
          {zh('下十页 ›', '+10 ›')}
        </a>
        <a
          href={buildPageUrl ? buildPageUrl({ page: lastPage }) : '#'}
          onClick={(e) => {
            if (isModifiedClick(e) || !hasNext) return
            e.preventDefault()
            onLast()
          }}
          className={`rounded border px-2 py-0.5 ${!hasNext ? 'pointer-events-none opacity-50' : ''}`}
          aria-disabled={!hasNext}
          aria-label={zh('末页', 'Last page')}
        >
          {zh('末页 »', 'Last »')}
        </a>
        <button
          type="button"
          onClick={openJumpPicker}
          className={`rounded border px-2 py-0.5 ${!canJump ? 'cursor-not-allowed opacity-50' : 'bg-white'}`}
          disabled={!canJump}
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
          <div className="flex max-w-[560px] flex-col gap-3 p-3" style={{ width: jumpPanelWidth }}>
            <div className="text-xs text-gray-500">{zh('选择页码', 'Select page')}</div>
            <div
              className="grid max-h-72 gap-2 overflow-y-auto pr-2"
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
                  className={`rounded border px-2 py-1 text-center text-xs leading-tight ${
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
      </div>
    </div>
  )
}
