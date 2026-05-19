import { useEffect, useRef } from 'react'
import { zh } from '@/utils/i18n'

export default function WaterfallLoader({ enabled, hasMore, loading, onLoadMore }) {
  const sentinelRef = useRef(null)

  useEffect(() => {
    if (!enabled || !hasMore || loading || typeof onLoadMore !== 'function') return undefined
    const node = sentinelRef.current
    if (!node) return undefined

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          onLoadMore()
        }
      },
      { rootMargin: '640px 0px' }
    )
    observer.observe(node)
    return () => observer.disconnect()
  }, [enabled, hasMore, loading, onLoadMore])

  if (!enabled) return null

  return (
    <div ref={sentinelRef} className="flex min-h-16 items-center justify-center py-4 text-sm">
      {loading ? (
        <span className="text-gray-500">{zh('加载更多…', 'Loading more...')}</span>
      ) : hasMore ? (
        <button
          type="button"
          onClick={onLoadMore}
          className="rounded border border-gray-300 bg-white px-3 py-1 text-gray-600 shadow-sm hover:border-gray-400"
        >
          {zh('加载更多', 'Load more')}
        </button>
      ) : (
        <span className="text-gray-400">{zh('没有更多了', 'No more items')}</span>
      )}
    </div>
  )
}
