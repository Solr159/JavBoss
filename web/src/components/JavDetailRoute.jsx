import useJavFavoriteCount from '@/hooks/useJavFavoriteCount'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { fetchJavItem } from '@/api'
import AppModal from '@/components/AppModal'
import JavGrid from '@/components/JavGrid'
import { useStore } from '@/store'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

// Reuse the card's actions and editors while mounting details independently of the list.
export default function JavDetailRoute({
  itemId,
  initialItem,
  onLoaded,
  initialState,
  onStateChange,
  onClose,
  ...actions
}) {
  const [item, setItem] = useState(initialItem)
  const [error, setError] = useState('')
  const listedItem = useStore((state) =>
    state.javItems?.find((entry) => Number(entry.id) === itemId)
  )
  const updateItem = useCallback((updated) => {
    if (!updated?.id) return
    setItem(updated)
  }, [])

  const favoriteCount = useJavFavoriteCount('jav', item)
  const detailItem = useMemo(
    () => (item ? { ...item, favorite_count: favoriteCount } : null),
    [item, favoriteCount]
  )
  useEffect(() => {
    if (detailItem) onLoaded(detailItem)
  }, [detailItem, onLoaded])

  useEffect(() => {
    if (listedItem) updateItem(listedItem)
  }, [listedItem, updateItem])

  useEffect(() => {
    // The card already contains full details. Only direct links/reloads need a request.
    if (initialItem || listedItem) return undefined
    let cancelled = false
    fetchJavItem(itemId)
      .then((loaded) => {
        if (!cancelled) updateItem(loaded)
      })
      .catch((err) => {
        if (!cancelled) setError(getErrorMessage(err))
      })
    return () => {
      cancelled = true
    }
  }, [initialItem, itemId, listedItem, updateItem])

  if (!item)
    return (
      <AppModal
        ariaLabel={zh('JAV 详情', 'JAV details')}
        onClose={onClose}
        zIndex={1300}
        contentClassName="w-full max-w-xl rounded-lg bg-white p-6 shadow-xl"
      >
        <p role={error ? 'alert' : 'status'}>{error || zh('加载中…', 'Loading…')}</p>
        <button type="button" onClick={onClose} className="mt-4 rounded border px-3 py-1">
          {zh('关闭', 'Close')}
        </button>
      </AppModal>
    )
  return (
    <JavGrid
      {...actions}
      items={[detailItem]}
      detailView={{
        onClose,
        scrollTop: initialState.scrollTop,
        onScrollChange: (scrollTop) => onStateChange({ scrollTop }),
        onItemUpdated: updateItem,
      }}
    />
  )
}
