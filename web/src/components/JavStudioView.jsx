import { getIdolCardLayoutProps } from '@/components/JavIdolGrid'
import Pagination from '@/components/Pagination'
import WaterfallLoader from '@/components/WaterfallLoader'
import { zh } from '@/utils/i18n'

export default function JavStudioView({
  page,
  lastPage,
  hasPrev,
  hasNext,
  loading,
  buildPageUrl,
  buildStudioUrl,
  onFirst,
  onPrev,
  onGoToPage,
  onNext,
  onLast,
  items,
  onSelectStudio,
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
          buildStudioUrl={buildStudioUrl}
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

function JavStudioGrid({ items, onSelectStudio, buildStudioUrl }) {
  const hasItems = Array.isArray(items) && items.length > 0
  if (!hasItems) {
    return (
      <div className="flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
        {zh('暂无片商数据', 'No studio data')}
      </div>
    )
  }

  return (
    <div className="grid gap-3 bg-white sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8">
      {items.map((item) => (
        <StudioCard
          key={item.id || item.name}
          item={item}
          href={buildStudioUrl?.(item)}
          onSelectStudio={onSelectStudio}
        />
      ))}
    </div>
  )
}

function StudioCard({ item, href, onSelectStudio }) {
  const { bgWidthPercent, coverAspectPercent } = getIdolCardLayoutProps()
  const cover = item?.sample_code ? `/jav/${encodeURIComponent(item.sample_code)}/cover` : null
  const name = item?.name || zh('未知片商', 'Unknown studio')
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
    onSelectStudio?.(item)
  }

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
      <div
        className="relative w-full overflow-hidden bg-gray-100"
        style={{ paddingTop: `${coverAspectPercent}%` }}
      >
        {cover ? (
          <div
            className="absolute inset-0 transition duration-200 group-hover:scale-[1.03]"
            style={{
              backgroundImage: `url(${cover})`,
              backgroundSize: `${bgWidthPercent}% 100%`,
              backgroundPosition: '100% 50%',
              backgroundRepeat: 'no-repeat',
            }}
            role="img"
            aria-label={name}
          />
        ) : (
          <div className="absolute inset-0 flex h-full w-full items-center justify-center bg-gradient-to-br from-gray-100 to-gray-200 p-4 text-center text-lg font-semibold text-gray-600">
            {name}
          </div>
        )}
      </div>
      <div className="flex flex-1 flex-col gap-1 p-3">
        <div className="line-clamp-2 text-sm font-semibold leading-tight">{name}</div>
        <div className="text-xs text-gray-500">
          {zh(`${workCount} 部作品`, `${workCount} works`)}
        </div>
      </div>
    </a>
  )
}
