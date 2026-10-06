import { useCallback, useEffect, useRef, useState } from 'react'
import CloseRoundedIcon from '@mui/icons-material/CloseRounded'
import SortRoundedIcon from '@mui/icons-material/SortRounded'
import { fetchJavStudioPreview } from '@/features/jav/api'
import AppModal from '@/shared/ui/AppModal'
import { SeriesCard } from '@/features/jav/components/JavSeriesView'
import { getErrorMessage } from '@/utils/errors'
import { getStudioCodePrefixes, getStudioSeries } from '@/utils/javStudio'
import { zh } from '@/utils/i18n'
import useDocumentTitle from '@/shared/hooks/useDocumentTitle'
import { formatPageTitle } from '@/navigation/pageTitle'

export default function JavStudioDetailModal({
  studioId,
  initialItem,
  onLoaded,
  initialState,
  onStateChange,
  onClose,
  onSelectStudio,
  onSelectSeries,
  onSelectPrefix,
  onOpenSeriesFavorites,
  buildJavUrl,
}) {
  const seedItem = Number(initialItem?.id) === studioId ? initialItem : null
  const [item, setItem] = useState(seedItem)
  const [loading, setLoading] = useState(!seedItem)
  const [error, setError] = useState('')
  const [prefixSort, setPrefixSort] = useState(initialState.prefixSort)
  const [seriesSort, setSeriesSort] = useState(initialState.seriesSort)
  const initialScrollRef = useRef(initialState.scrollTop)
  const scrollRestoredRef = useRef(false)

  useEffect(() => {
    let cancelled = false
    fetchJavStudioPreview(studioId)
      .then((loaded) => {
        if (!cancelled) {
          setItem(loaded)
          onLoaded?.(loaded)
        }
      })
      .catch((err) => {
        if (!cancelled) setError(getErrorMessage(err))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [onLoaded, studioId])

  const restoreScroll = useCallback(
    (node) => {
      // The modal portal may attach after the parent's layout effects have run.
      if (node && item && !loading && !scrollRestoredRef.current) {
        node.scrollTop = initialScrollRef.current
        scrollRestoredRef.current = true
      }
    },
    [item, loading]
  )

  const name = item?.name || zh('片商详情', 'Studio details')
  useDocumentTitle(formatPageTitle(name), { priority: 20 })
  const aliases = Array.isArray(item?.aliases) ? item.aliases.filter(Boolean) : []
  const workCount = Number(item?.work_count) || 0
  const showWorkCount = workCount > 0
  const codePrefixes = getStudioCodePrefixes(item)
  const seriesItems = getStudioSeries(item)
  const searchOptions = {
    page: 1,
    search: '',
    tab: 'list',
    idolIds: [],
    tagIds: [],
    prefix: '',
    favoriteRatingEnabled: false,
    tempSort: '',
    random: false,
    soloOnly: false,
  }
  const href = buildJavUrl({
    ...searchOptions,
    studioId,
    studioName: item?.name || '',
    seriesId: null,
  })
  const buildSeriesUrl = (series) =>
    buildJavUrl({
      ...searchOptions,
      studioId: null,
      seriesId: series.id,
      seriesName: series.name,
    })
  const handleStudioClick = (event) => {
    event.stopPropagation()
    if (!item || String(window.getSelection?.() || '').trim()) {
      event.preventDefault()
      return
    }
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0)
      return
    event.preventDefault()
    onSelectStudio(item)
  }
  const handlePrefixClick = (prefixItem, event) => {
    event.preventDefault()
    event.stopPropagation()
    onSelectPrefix({
      ...prefixItem,
      include_studio_filter: true,
      studio_id: studioId,
      studio_name: name,
    })
  }

  return (
    <AppModal
      ariaLabel={zh('片商详情', 'Studio details')}
      className="p-4"
      contentClassName="flex h-[92vh] w-[min(84rem,calc(100vw-2rem))] flex-col overflow-hidden rounded-lg border border-gray-200 bg-white shadow-xl"
      contentProps={{ onClick: (event) => event.stopPropagation() }}
      onClose={(event) => {
        event?.stopPropagation()
        onClose()
      }}
      zIndex={1600}
    >
      <div className="flex shrink-0 items-start justify-between gap-3 border-b border-gray-100 px-5 py-2">
        <div className="min-w-0">
          <div className="flex items-baseline gap-3">
            <a
              href={href || '#'}
              onClick={handleStudioClick}
              className="min-w-0 break-words text-lg font-semibold text-gray-900 hover:text-blue-700 hover:underline"
            >
              {name}
            </a>
            {showWorkCount ? (
              <span className="shrink-0 text-xs text-gray-500">
                {zh(`作品 ${workCount}`, `${workCount} works`)}
              </span>
            ) : null}
          </div>
          {aliases.length > 0 ? (
            <p className="mt-1 break-words text-sm text-gray-500">
              {zh(aliases.join('、'), aliases.join(', '))}
            </p>
          ) : null}
        </div>
        <button
          type="button"
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-gray-500 hover:bg-gray-100 hover:text-gray-900"
          aria-label={zh('关闭片商详情', 'Close studio details')}
          onClick={() => onClose()}
        >
          <CloseRoundedIcon sx={{ fontSize: 20 }} />
        </button>
      </div>
      <div
        ref={restoreScroll}
        onScroll={(event) => onStateChange({ scrollTop: event.currentTarget.scrollTop })}
        className="min-h-0 flex-1 space-y-5 overflow-y-auto p-5"
      >
        {loading ? (
          <p className="py-8 text-center text-sm text-gray-500">{zh('加载中…', 'Loading...')}</p>
        ) : error && !item ? (
          <div role="alert" className="py-8 text-center text-sm text-red-600">
            {error}
          </div>
        ) : (
          <>
            <section>
              <StudioDetailSectionHeader
                title={zh('番号', 'Codes')}
                count={codePrefixes.length}
                sort={prefixSort}
                onSortChange={(sort) => {
                  setPrefixSort(sort)
                  onStateChange({ prefixSort: sort })
                }}
              />
              {codePrefixes.length > 0 ? (
                <div className="flex flex-wrap gap-2">
                  {sortStudioDetailItems(codePrefixes, prefixSort, 'prefix').map((prefixItem) => (
                    <button
                      type="button"
                      key={prefixItem.prefix}
                      className="rounded border border-gray-200 bg-gray-50 px-2 py-1 text-sm text-gray-700 hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700"
                      onClick={(event) => handlePrefixClick(prefixItem, event)}
                      title={zh(
                        `查看 ${prefixItem.prefix} 的全部作品`,
                        `Show all ${prefixItem.prefix} works`
                      )}
                    >
                      {prefixItem.prefix}
                      {prefixItem.work_count ? (
                        <span className="ml-2 text-xs text-gray-500">{prefixItem.work_count}</span>
                      ) : null}
                    </button>
                  ))}
                </div>
              ) : (
                <p className="text-sm text-gray-400">{zh('暂无番号', 'No codes')}</p>
              )}
            </section>
            <section>
              <StudioDetailSectionHeader
                title={zh('系列', 'Series')}
                count={seriesItems.length}
                sort={seriesSort}
                onSortChange={(sort) => {
                  setSeriesSort(sort)
                  onStateChange({ seriesSort: sort })
                }}
              />
              {seriesItems.length > 0 ? (
                <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-5">
                  {sortStudioDetailItems(seriesItems, seriesSort, 'name').map((series) => (
                    <SeriesCard
                      key={series.id}
                      item={series}
                      href={buildSeriesUrl?.(series)}
                      onSelectSeries={(selectedSeries) => {
                        onSelectSeries?.(selectedSeries)
                      }}
                      onSelectStudio={(studio) => {
                        onSelectStudio?.(studio)
                      }}
                      onOpenFavorites={onOpenSeriesFavorites}
                    />
                  ))}
                </div>
              ) : (
                <p className="text-sm text-gray-400">{zh('暂无系列', 'No series')}</p>
              )}
            </section>
          </>
        )}
      </div>
    </AppModal>
  )
}

function StudioDetailSectionHeader({ title, count, sort, onSortChange }) {
  return (
    <div className="mb-2 flex flex-wrap items-center gap-1.5">
      <h3 className="text-sm font-semibold text-gray-700">
        {title} ({count})
      </h3>
      <button
        type="button"
        onClick={() => onSortChange(sort === 'name_asc' ? 'work_count_desc' : 'name_asc')}
        title={
          sort === 'name_asc'
            ? zh('切换为作品数从多到少', 'Switch to most works first')
            : zh('切换为名称升序', 'Switch to name ascending')
        }
        aria-label={zh(
          `${title}排序：${sort === 'name_asc' ? '名称升序，点击切换为作品数降序' : '作品数降序，点击切换为名称升序'}`,
          `Sort ${title.toLowerCase()}: ${sort === 'name_asc' ? 'name ascending; click for most works first' : 'most works first; click for name ascending'}`
        )}
        className="inline-flex items-center gap-0.5 rounded-full bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium leading-3 text-blue-700 ring-1 ring-inset ring-blue-100 hover:bg-blue-100"
      >
        <SortRoundedIcon sx={{ fontSize: 13 }} />
        {sort === 'name_asc' ? zh('名称', 'Name') : zh('作品数', 'Works')}
      </button>
    </div>
  )
}

function sortStudioDetailItems(items, sort, nameKey) {
  return [...items].sort((a, b) => {
    const nameDiff = String(a[nameKey] || '').localeCompare(String(b[nameKey] || ''), undefined, {
      numeric: true,
      sensitivity: 'base',
    })
    if (sort === 'name_asc') return nameDiff
    const countDiff = (Number(b.work_count) || 0) - (Number(a.work_count) || 0)
    return countDiff || nameDiff
  })
}
