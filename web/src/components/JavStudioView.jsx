import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Popper } from '@mui/material'
import StarBorderRoundedIcon from '@mui/icons-material/StarBorderRounded'
import StarRoundedIcon from '@mui/icons-material/StarRounded'

import { fetchJavSeriesPreview, fetchJavStudioJavDBURL } from '@/api'
import Pagination from '@/components/Pagination'
import { SeriesCard } from '@/components/JavSeriesView'
import WaterfallLoader from '@/components/WaterfallLoader'
import { zh } from '@/utils/i18n'

export default function JavStudioView({
  page,
  lastPage,
  totalItems,
  hasPrev,
  hasNext,
  loading,
  buildPageUrl,
  buildStudioUrl,
  buildSeriesUrl,
  onFirst,
  onPrev,
  onGoToPage,
  onNext,
  onLast,
  items,
  onSelectStudio,
  onSelectSeries,
  onSelectPrefix,
  onOpenFavorites,
  directoryIds = [],
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
        <JavStudioGrid
          items={items}
          onSelectStudio={onSelectStudio}
          onSelectPrefix={onSelectPrefix}
          onOpenFavorites={onOpenFavorites}
          buildStudioUrl={buildStudioUrl}
          buildSeriesUrl={buildSeriesUrl}
          onSelectSeries={onSelectSeries}
          directoryIds={directoryIds}
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

function JavStudioGrid({
  items,
  onSelectStudio,
  onSelectSeries,
  onSelectPrefix,
  onOpenFavorites,
  buildStudioUrl,
  buildSeriesUrl,
  directoryIds,
}) {
  const hasItems = Array.isArray(items) && items.length > 0
  if (!hasItems) {
    return (
      <div className="flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
        {zh('暂无片商数据', 'No studio data')}
      </div>
    )
  }

  return (
    <div
      className="grid gap-4 bg-white"
      style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(16rem, 1fr))' }}
    >
      {items.map((item) => (
        <StudioCard
          key={item.id || item.name}
          item={item}
          href={buildStudioUrl?.(item)}
          onSelectStudio={onSelectStudio}
          onSelectSeries={onSelectSeries}
          onSelectPrefix={onSelectPrefix}
          onOpenFavorites={onOpenFavorites}
          buildSeriesUrl={buildSeriesUrl}
          directoryIds={directoryIds}
        />
      ))}
    </div>
  )
}

export function StudioCard({
  item,
  href,
  onSelectStudio,
  onSelectSeries,
  onSelectPrefix,
  onOpenFavorites,
  buildSeriesUrl,
  directoryIds = [],
}) {
  const cover = item?.sample_code ? `/jav/${encodeURIComponent(item.sample_code)}/cover` : null
  const name = item?.name || zh('未知片商', 'Unknown studio')
  const studioId = Number(item?.id)
  const workCount = Number(item?.work_count)
  const showWorkCount = Number.isFinite(workCount) && workCount > 0
  const codePrefixes = Array.isArray(item?.code_prefixes)
    ? item.code_prefixes
        .map((prefixItem) => {
          if (typeof prefixItem === 'string') {
            const prefix = prefixItem.trim()
            return prefix ? { prefix, work_count: null } : null
          }
          const prefix = String(prefixItem?.prefix || '').trim()
          if (!prefix) return null
          const prefixWorkCount = Number(prefixItem?.work_count)
          return {
            prefix,
            work_count:
              Number.isFinite(prefixWorkCount) && prefixWorkCount > 0 ? prefixWorkCount : null,
          }
        })
        .filter(Boolean)
    : []
  const seriesItems = useMemo(
    () =>
      Array.isArray(item?.series)
        ? item.series.filter(
            (series) => Number(series?.id) > 0 && String(series?.name || '').trim()
          )
        : [],
    [item?.series]
  )
  const favoriteCount = Number(item?.favorite_count) || 0
  const [javdbURL, setJavdbURL] = useState(String(item?.javdb_url || '').trim())
  const [javdbOpening, setJavdbOpening] = useState(false)
  const [previewSeries, setPreviewSeries] = useState(null)
  const [seriesHoverAnchorEl, setSeriesHoverAnchorEl] = useState(null)
  const [seriesListOpen, setSeriesListOpen] = useState(false)
  const [visibleSeriesCount, setVisibleSeriesCount] = useState(null)
  const seriesPreviewCacheRef = useRef(new Map())
  const seriesListRef = useRef(null)
  const seriesMeasureRef = useRef(null)
  const closeTimerRef = useRef(null)
  const activeSeriesHoverIdRef = useRef(null)
  const canOpenJavDB = Boolean(javdbURL || (Number.isFinite(studioId) && studioId > 0))
  const displayedSeriesItems =
    visibleSeriesCount == null ? seriesItems : seriesItems.slice(0, visibleSeriesCount)
  const hasHiddenSeries = displayedSeriesItems.length < seriesItems.length

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
    onSelectStudio?.(item)
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
        targetURL = await fetchJavStudioJavDBURL({ studioId })
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
      console.warn('open javdb studio failed', error)
    } finally {
      setJavdbOpening(false)
    }
  }

  const handleOpenFavorites = (event) => {
    event.preventDefault()
    event.stopPropagation()
    onOpenFavorites?.(item)
  }

  const clearSeriesHoverTimer = () => {
    if (closeTimerRef.current) {
      window.clearTimeout(closeTimerRef.current)
      closeTimerRef.current = null
    }
  }

  const closeSeriesPreview = () => {
    activeSeriesHoverIdRef.current = null
    setPreviewSeries(null)
    setSeriesHoverAnchorEl(null)
  }

  const closeSeriesList = () => {
    setSeriesListOpen(false)
  }

  const scheduleSeriesPreviewClose = () => {
    clearSeriesHoverTimer()
    closeTimerRef.current = window.setTimeout(() => {
      closeSeriesPreview()
      closeTimerRef.current = null
    }, 120)
  }

  const handleSeriesHoverStart = (series, event) => {
    clearSeriesHoverTimer()
    const seriesId = Number(series?.id)
    if (!Number.isFinite(seriesId) || seriesId <= 0) return
    activeSeriesHoverIdRef.current = seriesId
    setPreviewSeries(series)
    setSeriesHoverAnchorEl(event.currentTarget)

    const cacheKey = `${seriesId}|${(directoryIds || []).join(',')}`
    const cached = seriesPreviewCacheRef.current.get(cacheKey)
    if (cached) {
      setPreviewSeries(cached)
      return
    }
    fetchJavSeriesPreview(seriesId, { directoryIds })
      .then((loadedSeries) => {
        seriesPreviewCacheRef.current.set(cacheKey, loadedSeries)
        if (activeSeriesHoverIdRef.current === Number(loadedSeries?.id)) {
          setPreviewSeries(loadedSeries)
        }
      })
      .catch((error) => {
        console.warn('load studio series preview failed', error)
      })
  }

  const handleSeriesClick = (series, event) => {
    event.preventDefault()
    event.stopPropagation()
    onSelectSeries?.(series)
  }

  const handlePrefixClick = (prefixItem, event) => {
    event.preventDefault()
    event.stopPropagation()
    const prefix = String(prefixItem?.prefix || '').trim()
    if (!prefix) return
    onSelectPrefix?.({
      prefix,
      work_count: prefixItem?.work_count || 0,
      include_studio_filter: true,
      studio_id: Number.isFinite(studioId) && studioId > 0 ? studioId : null,
      studio_name: name,
    })
  }

  const openSeriesList = (event) => {
    event.preventDefault()
    event.stopPropagation()
    setSeriesListOpen(true)
  }

  useEffect(() => {
    if (!seriesListOpen) return undefined
    const handleKeyDown = (event) => {
      if (event.key === 'Escape') closeSeriesList()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [seriesListOpen])

  useLayoutEffect(() => {
    const el = seriesListRef.current
    const measureEl = seriesMeasureRef.current
    if (!el || !measureEl) {
      setVisibleSeriesCount(null)
      return undefined
    }
    const updateVisibleCount = () => {
      const countChip = measureEl.querySelector('[data-series-count-chip="true"]')
      const chips = Array.from(measureEl.querySelectorAll('[data-series-measure-chip="true"]'))
      const moreButton = measureEl.querySelector('[data-series-more-button="true"]')
      const containerWidth = el.getBoundingClientRect().width
      if (!countChip || chips.length === 0 || !moreButton || containerWidth <= 0) {
        setVisibleSeriesCount(0)
        return
      }

      const styles = window.getComputedStyle(measureEl)
      const gap = Number.parseFloat(styles.columnGap || styles.gap || '0') || 0
      const countWidth = countChip.getBoundingClientRect().width
      const chipWidths = chips.map((chip) => chip.getBoundingClientRect().width)
      const moreWidth = moreButton.getBoundingClientRect().width

      const measureRows = (reserveMoreButton) => {
        let row = 0
        let rowWidth = 0
        let visibleCount = 0
        const itemWidths = [countWidth, ...chipWidths]

        for (let index = 0; index < itemWidths.length; index += 1) {
          const itemWidth = itemWidths[index]
          const rowLimit =
            row === 1 && reserveMoreButton
              ? Math.max(0, containerWidth - moreWidth - gap)
              : containerWidth
          const nextWidth = rowWidth === 0 ? itemWidth : rowWidth + gap + itemWidth
          if (nextWidth <= rowLimit || rowWidth === 0) {
            rowWidth = nextWidth <= rowLimit ? nextWidth : itemWidth
            if (index > 0) visibleCount += 1
            continue
          }

          row += 1
          if (row > 1) break
          rowWidth = itemWidth
          if (index > 0) visibleCount += 1
        }

        return visibleCount
      }

      const fullWidthCount = measureRows(false)
      if (fullWidthCount >= seriesItems.length) {
        setVisibleSeriesCount(seriesItems.length)
        return
      }

      let reservedCount = measureRows(true)
      while (reservedCount > 0) {
        let row = 0
        let rowWidth = 0
        const itemWidths = [countWidth, ...chipWidths.slice(0, reservedCount)]

        for (let index = 0; index < itemWidths.length; index += 1) {
          const itemWidth = itemWidths[index]
          const rowLimit =
            row === 1 ? Math.max(0, containerWidth - moreWidth - gap) : containerWidth
          const nextWidth = rowWidth === 0 ? itemWidth : rowWidth + gap + itemWidth
          if (nextWidth <= rowLimit || rowWidth === 0) {
            rowWidth = nextWidth <= rowLimit ? nextWidth : itemWidth
            continue
          }
          row += 1
          rowWidth = itemWidth
          if (row > 1) break
        }
        if (row <= 1) break
        reservedCount -= 1
      }
      setVisibleSeriesCount(reservedCount)
    }
    updateVisibleCount()
    if (typeof ResizeObserver === 'undefined') {
      window.addEventListener('resize', updateVisibleCount)
      return () => window.removeEventListener('resize', updateVisibleCount)
    }
    const observer = new ResizeObserver(updateVisibleCount)
    observer.observe(el)
    return () => observer.disconnect()
  }, [seriesItems])

  useEffect(() => {
    return () => {
      if (closeTimerRef.current) {
        window.clearTimeout(closeTimerRef.current)
      }
    }
  }, [])

  return (
    <a
      href={href || '#'}
      className="group flex cursor-pointer flex-col overflow-hidden rounded-lg border bg-white shadow-sm transition hover:shadow-lg"
      onClick={handleClick}
      onKeyDown={(e) => {
        if (e.key === ' ') {
          e.preventDefault()
          onSelectStudio?.(item)
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
          title={zh('加入片商收藏夹', 'Add to studio favorite groups')}
          aria-label={zh('加入片商收藏夹', 'Add to studio favorite groups')}
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
          title={zh('在 JavDB 中打开片商详情', 'Open studio profile in JavDB')}
          aria-label={zh('在 JavDB 中打开片商详情', 'Open studio profile in JavDB')}
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
        {codePrefixes.length > 0 ? (
          <div
            className="mt-1 flex max-h-20 flex-wrap gap-0.5 overflow-y-auto"
            title={codePrefixes
              .map((prefixItem) => {
                const count = Number(prefixItem.work_count)
                return count > 0 ? `${prefixItem.prefix} (${count})` : prefixItem.prefix
              })
              .join(', ')}
          >
            <span className="rounded bg-gray-900 px-1 py-0.5 text-[9px] font-semibold leading-3 text-white">
              {zh(`${codePrefixes.length}个番号`, `${codePrefixes.length} codes`)}
            </span>
            {codePrefixes.map((prefixItem) => (
              <button
                type="button"
                key={prefixItem.prefix}
                className="rounded border border-gray-200 bg-gray-50 px-1 py-0.5 text-[9px] font-medium leading-3 text-gray-700 hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700"
                title={zh(
                  `查看 ${prefixItem.prefix} 的全部作品`,
                  `Show all ${prefixItem.prefix} works`
                )}
                onClick={(event) => handlePrefixClick(prefixItem, event)}
              >
                <span>{prefixItem.prefix}</span>
                {prefixItem.work_count ? (
                  <span className="ml-1 text-[8px] font-semibold leading-3 text-gray-500">
                    {prefixItem.work_count}
                  </span>
                ) : null}
              </button>
            ))}
          </div>
        ) : null}
        {seriesItems.length > 0 ? (
          <div className="relative mt-1">
            <div
              ref={seriesMeasureRef}
              aria-hidden="true"
              className="pointer-events-none invisible absolute left-0 top-0 -z-10 flex w-full flex-wrap gap-0.5 overflow-visible"
            >
              <span
                data-series-count-chip="true"
                className="rounded bg-emerald-700 px-1 py-0.5 text-[9px] font-semibold leading-3 text-white"
              >
                {zh(`${seriesItems.length}个系列`, `${seriesItems.length} series`)}
              </span>
              {seriesItems.map((series) => {
                const seriesName = String(series?.name || '').trim()
                return (
                  <span
                    key={series.id}
                    data-series-measure-chip="true"
                    className="max-w-[7rem] truncate rounded border border-emerald-100 bg-emerald-50 px-1 py-0.5 text-[9px] font-medium leading-3 text-emerald-700"
                  >
                    {seriesName}
                  </span>
                )
              })}
              <span
                data-series-more-button="true"
                className="rounded border border-emerald-200 bg-white px-1 py-0.5 text-[9px] font-semibold leading-3 text-emerald-700 shadow-sm"
              >
                {zh('展开全部', 'All')}
              </span>
            </div>
            <div
              ref={seriesListRef}
              className="relative flex max-h-[2.625rem] flex-wrap gap-0.5 overflow-hidden"
            >
              <span className="rounded bg-emerald-700 px-1 py-0.5 text-[9px] font-semibold leading-3 text-white">
                {zh(`${seriesItems.length}个系列`, `${seriesItems.length} series`)}
              </span>
              {displayedSeriesItems.map((series) => {
                const seriesName = String(series?.name || '').trim()
                return (
                  <button
                    type="button"
                    key={series.id}
                    data-series-chip="true"
                    className="max-w-[7rem] truncate rounded border border-emerald-100 bg-emerald-50 px-1 py-0.5 text-[9px] font-medium leading-3 text-emerald-700 hover:border-emerald-300 hover:bg-emerald-100"
                    title={seriesName}
                    onClick={(event) => handleSeriesClick(series, event)}
                    onMouseEnter={(event) => handleSeriesHoverStart(series, event)}
                    onMouseLeave={scheduleSeriesPreviewClose}
                    onFocus={(event) => handleSeriesHoverStart(series, event)}
                    onBlur={scheduleSeriesPreviewClose}
                  >
                    {seriesName}
                  </button>
                )
              })}
              {hasHiddenSeries ? (
                <button
                  type="button"
                  className="absolute bottom-0 right-0 rounded border border-emerald-200 bg-white px-1 py-0.5 text-[9px] font-semibold leading-3 text-emerald-700 shadow-sm hover:bg-emerald-50"
                  onClick={openSeriesList}
                >
                  {zh('展开全部', 'All')}
                </button>
              ) : null}
            </div>
            <Popper
              open={Boolean(previewSeries && seriesHoverAnchorEl)}
              anchorEl={seriesHoverAnchorEl}
              placement="right-start"
              className="z-[1400]"
              modifiers={[
                {
                  name: 'offset',
                  options: {
                    offset: [10, 0],
                  },
                },
              ]}
            >
              <div
                className="w-[260px]"
                onMouseEnter={clearSeriesHoverTimer}
                onMouseLeave={scheduleSeriesPreviewClose}
              >
                {previewSeries ? (
                  <SeriesCard
                    item={previewSeries}
                    href={buildSeriesUrl?.(previewSeries)}
                    onSelectSeries={(series) => onSelectSeries?.(series)}
                    onSelectStudio={(studio) => onSelectStudio?.(studio)}
                  />
                ) : null}
              </div>
            </Popper>
            {seriesListOpen
              ? createPortal(
                  <div className="fixed inset-0 z-[1500] flex items-center justify-center bg-black/40 p-4">
                    <button
                      type="button"
                      className="absolute inset-0 h-full w-full cursor-default bg-transparent"
                      aria-label={zh('关闭', 'Close')}
                      onClick={(event) => {
                        event.preventDefault()
                        event.stopPropagation()
                        closeSeriesList()
                      }}
                    />
                    {/* eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-noninteractive-element-interactions */}
                    <div
                      className="relative z-10 max-h-[84vh] w-[min(72rem,calc(100vw-2rem))] overflow-y-auto rounded-lg border border-gray-200 bg-white shadow-xl"
                      role="dialog"
                      aria-modal="true"
                      aria-label={zh('片商系列', 'Studio series')}
                      tabIndex={-1}
                      onClick={(event) => event.stopPropagation()}
                    >
                      <div className="sticky top-0 z-10 mb-2 flex items-center justify-between gap-3 border-b border-gray-100 bg-white px-3 py-2">
                        <div className="text-xs font-semibold text-gray-700">
                          {zh(
                            `片商：${name}，共${seriesItems.length}个系列`,
                            `Studio: ${name}, ${seriesItems.length} series`
                          )}
                        </div>
                        <button
                          type="button"
                          className="rounded px-2 py-1 text-xs text-gray-500 hover:bg-gray-100 hover:text-gray-800"
                          onClick={(event) => {
                            event.preventDefault()
                            event.stopPropagation()
                            closeSeriesList()
                          }}
                        >
                          {zh('关闭', 'Close')}
                        </button>
                      </div>
                      <div className="grid grid-cols-3 gap-3 p-3 md:grid-cols-4 xl:grid-cols-5">
                        {seriesItems.map((series) => {
                          return (
                            <SeriesCard
                              key={series.id}
                              item={series}
                              href={buildSeriesUrl?.(series)}
                              onSelectSeries={(selectedSeries) => {
                                closeSeriesList()
                                onSelectSeries?.(selectedSeries)
                              }}
                              onSelectStudio={(studio) => onSelectStudio?.(studio)}
                            />
                          )
                        })}
                      </div>
                    </div>
                  </div>,
                  document.body
                )
              : null}
          </div>
        ) : null}
      </div>
    </a>
  )
}
