import JavIdolGrid from '@/components/JavIdolGrid'
import Pagination from '@/components/Pagination'
import WaterfallLoader from '@/components/WaterfallLoader'
import { zh } from '@/utils/i18n'

export default function JavIdolView({
  page,
  lastPage,
  hasPrev,
  hasNext,
  loading,
  buildPageUrl,
  buildIdolUrl,
  javMetadataLanguage,
  onFirst,
  onPrev,
  onGoToPage,
  onNext,
  onLast,
  items,
  onSelectIdol,
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
        <JavIdolGrid
          items={items}
          onSelectIdol={onSelectIdol}
          buildIdolUrl={buildIdolUrl}
          javMetadataLanguage={javMetadataLanguage}
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
