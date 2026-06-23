import { useState } from 'react'
import { Tooltip } from '@mui/material'
import StarBorderRoundedIcon from '@mui/icons-material/StarBorderRounded'
import StarRoundedIcon from '@mui/icons-material/StarRounded'
import VideocamOutlinedIcon from '@mui/icons-material/VideocamOutlined'

import { fetchJavSeriesJavDBURL } from '@/api'
import Pagination from '@/components/Pagination'
import WaterfallLoader from '@/components/WaterfallLoader'
import { zh } from '@/utils/i18n'

export default function JavSeriesView({
  page,
  lastPage,
  totalItems,
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
  onOpenFavorites,
  waterfallMode,
  onWaterfallModeChange,
  onLoadMore,
  loadingMore,
  hasMore,
}) {
  return (
    <>
      <div className="sticky-pagination mb-4 flex justify-center">
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
      {loading ? (
        <div className="mt-4 flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
          {zh('加载中…', 'Loading...')}
        </div>
      ) : (
        <JavSeriesGrid
          items={items}
          onSelectSeries={onSelectSeries}
          onSelectStudio={onSelectStudio}
          onOpenFavorites={onOpenFavorites}
          buildSeriesUrl={buildSeriesUrl}
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

function JavSeriesGrid({ items, onSelectSeries, onSelectStudio, onOpenFavorites, buildSeriesUrl }) {
  const hasItems = Array.isArray(items) && items.length > 0
  if (!hasItems) {
    return (
      <div className="flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
        {zh('暂无系列数据', 'No series data')}
      </div>
    )
  }

  return (
    <div
      className="grid gap-4 bg-white"
      style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(16rem, 1fr))' }}
    >
      {items.map((item) => (
        <SeriesCard
          key={item.id || item.name}
          item={item}
          href={buildSeriesUrl?.(item)}
          onSelectSeries={onSelectSeries}
          onSelectStudio={onSelectStudio}
          onOpenFavorites={onOpenFavorites}
        />
      ))}
    </div>
  )
}

export function SeriesCard({ item, href, onSelectSeries, onSelectStudio, onOpenFavorites }) {
  const sampleCode = String(item?.sample_code || '').trim()
  const cover = sampleCode ? `/jav/${encodeURIComponent(sampleCode)}/cover` : null
  const name = item?.name || zh('未知系列', 'Unknown series')
  const studioName = String(item?.studio_name || '').trim()
  const seriesId = Number(item?.id)
  const studioId = Number(item?.studio_id)
  const canFilterStudio =
    studioName && Number.isFinite(studioId) && studioId > 0 && typeof onSelectStudio === 'function'
  const workCount = Number(item?.work_count)
  const showWorkCount = Number.isFinite(workCount) && workCount > 0
  const favoriteCount = Number(item?.favorite_count) || 0
  const [javdbURL, setJavdbURL] = useState(String(item?.javdb_url || '').trim())
  const [javdbOpening, setJavdbOpening] = useState(false)
  const canOpenJavDB = Boolean(javdbURL || (Number.isFinite(seriesId) && seriesId > 0))

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

  const handleOpenJavDB = async (event) => {
    event.preventDefault()
    event.stopPropagation()
    if (!canOpenJavDB || javdbOpening) return

    const popup = window.open('about:blank', '_blank')
    if (popup) {
      popup.opener = null
    }

    try {
      setJavdbOpening(true)
      let targetURL = javdbURL
      if (!targetURL) {
        targetURL = await fetchJavSeriesJavDBURL({ seriesId })
        setJavdbURL(targetURL)
      }
      if (!targetURL) {
        popup?.close()
        return
      }
      if (popup) {
        popup.location.replace(targetURL)
      } else {
        window.open(targetURL, '_blank', 'noopener,noreferrer')
      }
    } catch (error) {
      popup?.close()
      console.warn('open javdb series failed', error)
    } finally {
      setJavdbOpening(false)
    }
  }

  const handleOpenFavorites = (event) => {
    event.preventDefault()
    event.stopPropagation()
    onOpenFavorites?.(item)
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
        {showWorkCount ? (
          <div className="absolute left-2 top-2 rounded bg-black/70 px-2 py-1 text-xs text-white">
            {zh(`作品 ${workCount}`, `${workCount} works`)}
          </div>
        ) : null}
        <button
          type="button"
          className={`absolute right-2 top-2 flex h-8 w-8 items-center justify-center rounded-full shadow-lg shadow-black/40 transition ${
            favoriteCount > 0
              ? 'bg-amber-400 text-amber-950 hover:bg-amber-300'
              : 'bg-black/65 text-white opacity-0 hover:bg-black/80 group-focus-within:opacity-100 group-hover:opacity-100'
          }`}
          title={zh('加入系列收藏夹', 'Add to series favorite groups')}
          aria-label={zh('加入系列收藏夹', 'Add to series favorite groups')}
          onClick={handleOpenFavorites}
        >
          {favoriteCount > 0 ? (
            <StarRoundedIcon sx={{ fontSize: 18 }} />
          ) : (
            <StarBorderRoundedIcon sx={{ fontSize: 18 }} />
          )}
        </button>
        <button
          type="button"
          className={`absolute bottom-2 left-2 flex h-7 w-7 items-center justify-center rounded-full text-white opacity-0 shadow-lg shadow-black/60 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100 ${
            canOpenJavDB ? 'bg-black/70 hover:bg-black/85' : 'cursor-not-allowed bg-black/30'
          }`}
          title={zh('在 JavDB 中打开系列详情', 'Open series profile in JavDB')}
          aria-label={zh('在 JavDB 中打开系列详情', 'Open series profile in JavDB')}
          disabled={!canOpenJavDB || javdbOpening}
          onClick={handleOpenJavDB}
        >
          <img
            src="/ico/javdb.png"
            alt="JavDB"
            className={`h-4 w-4 ${javdbOpening ? 'animate-pulse' : ''}`}
            loading="lazy"
          />
        </button>
      </div>
      <div className="flex flex-1 flex-col gap-1 p-3">
        <div className="line-clamp-2 text-sm font-semibold leading-tight">{name}</div>
        <div className="flex min-w-0 items-center gap-2 text-xs text-gray-500">
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
