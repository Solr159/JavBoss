import { Tooltip } from '@mui/material'
import VideocamOutlinedIcon from '@mui/icons-material/VideocamOutlined'

import Pagination from '@/components/Pagination'
import { zh } from '@/utils/i18n'

export default function JavSeriesView({
  page,
  lastPage,
  hasPrev,
  hasNext,
  loading,
  buildPageUrl,
  buildSeriesUrl,
  onFirst,
  onPrev,
  onGoToPage,
  onNext,
  onLast,
  items,
  onSelectSeries,
  onSelectStudio,
}) {
  return (
    <>
      <div className="sticky-pagination mb-4 flex justify-center">
        <Pagination
          page={page}
          lastPage={lastPage}
          hasPrev={hasPrev}
          hasNext={hasNext}
          loading={loading}
          buildPageUrl={buildPageUrl}
          onFirst={onFirst}
          onPrev={onPrev}
          onGoToPage={onGoToPage}
          onNext={onNext}
          onLast={onLast}
        />
      </div>
      {loading ? (
        <div className="mt-4 flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
          {zh('加载中…', 'Loading...')}
        </div>
      ) : (
        <JavSeriesGrid
          items={items}
          onSelectSeries={onSelectSeries}
          onSelectStudio={onSelectStudio}
          buildSeriesUrl={buildSeriesUrl}
        />
      )}
    </>
  )
}

function JavSeriesGrid({ items, onSelectSeries, onSelectStudio, buildSeriesUrl }) {
  const hasItems = Array.isArray(items) && items.length > 0
  if (!hasItems) {
    return (
      <div className="flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
        {zh('暂无系列数据', 'No series data')}
      </div>
    )
  }

  return (
    <div className="grid gap-4 bg-white sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6">
      {items.map((item) => (
        <SeriesCard
          key={item.id || item.name}
          item={item}
          href={buildSeriesUrl?.(item)}
          onSelectSeries={onSelectSeries}
          onSelectStudio={onSelectStudio}
        />
      ))}
    </div>
  )
}

function SeriesCard({ item, href, onSelectSeries, onSelectStudio }) {
  const cover = item?.sample_code ? `/jav/${encodeURIComponent(item.sample_code)}/cover` : null
  const name = item?.name || zh('未知系列', 'Unknown series')
  const studioName = String(item?.studio_name || '').trim()
  const studioId = Number(item?.studio_id)
  const canFilterStudio =
    studioName && Number.isFinite(studioId) && studioId > 0 && typeof onSelectStudio === 'function'
  const workCount = item?.work_count || 0

  const handleClick = (e) => {
    const selection = window.getSelection?.()
    if (selection && String(selection).trim() !== '') {
      e.preventDefault()
      return
    }
    const isModified = e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0
    if (isModified) {
      return
    }
    e.preventDefault()
    onSelectSeries?.(item)
  }

  const handleStudioClick = (e) => {
    e.stopPropagation()
    e.preventDefault()
    if (!canFilterStudio) return
    onSelectStudio?.({ id: studioId, name: studioName })
  }

  return (
    <a
      href={href || '#'}
      className="group flex cursor-pointer flex-col overflow-hidden rounded-lg border bg-white shadow-sm transition hover:shadow-lg"
      onClick={handleClick}
      onKeyDown={(e) => {
        if (e.key === ' ') {
          e.preventDefault()
          onSelectSeries?.(item)
        }
      }}
    >
      <div className="relative aspect-[800/538] w-full overflow-hidden bg-gray-100">
        {cover ? (
          <img
            src={cover}
            alt={name}
            className="h-full w-full object-cover transition duration-200 group-hover:scale-[1.03]"
            loading="lazy"
          />
        ) : (
          <div className="absolute inset-0 flex h-full w-full items-center justify-center bg-gradient-to-br from-gray-100 to-gray-200 p-4 text-center text-lg font-semibold text-gray-600">
            {name}
          </div>
        )}
      </div>
      <div className="flex flex-1 flex-col gap-1 p-3">
        <div className="line-clamp-2 text-sm font-semibold leading-tight">{name}</div>
        <div className="flex min-w-0 items-center gap-2 text-xs text-gray-500">
          <span className="shrink-0">{zh(`${workCount} 部作品`, `${workCount} works`)}</span>
          {studioName ? (
            <span className="inline-flex min-w-0 items-center gap-1">
              <Tooltip title={zh('片商', 'Studio')} arrow>
                <span className="inline-flex">
                  <VideocamOutlinedIcon sx={{ fontSize: 16 }} className="shrink-0 text-sky-600" />
                </span>
              </Tooltip>
              <button
                type="button"
                className={`min-w-0 truncate text-left ${
                  canFilterStudio ? 'cursor-pointer hover:text-blue-700 hover:underline' : ''
                }`}
                onClick={handleStudioClick}
                disabled={!canFilterStudio}
              >
                {studioName}
              </button>
            </span>
          ) : null}
        </div>
      </div>
    </a>
  )
}
