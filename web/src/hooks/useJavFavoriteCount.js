import { useEffect } from 'react'
import { fetchJavFavoriteSelection } from '@/features/favorites/api'
import { useStore } from '@/store'

// Invalidations also reach mounted previews and details outside the active list.
export default function useJavFavoriteCount(type, item) {
  const id = Number(item?.id)
  const saved = useStore((state) => state.favoriteCountsByType[type]?.[id])
  const revision = useStore((state) => state.favoriteCountsRevision[type] || 0)
  useEffect(() => {
    if (!id || !revision || saved !== undefined) return undefined
    let cancelled = false
    fetchJavFavoriteSelection(type, id)
      .then((groups) => {
        if (cancelled) return
        useStore.setState((state) => {
          // A later mutation/save takes precedence over an older request.
          if (
            state.favoriteCountsRevision[type] !== revision ||
            state.favoriteCountsByType[type]?.[id] !== undefined
          )
            return state
          return {
            favoriteCountsByType: {
              ...state.favoriteCountsByType,
              [type]: { ...state.favoriteCountsByType[type], [id]: groups.length },
            },
          }
        })
      })
      .catch((error) => console.warn('refresh favorite count failed', error))
    return () => {
      cancelled = true
    }
  }, [id, revision, saved, type])
  return saved ?? (Number(item?.favorite_count) || 0)
}
