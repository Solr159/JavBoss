import { createListResources } from '@/query/createListResources'
import { createVideoSlice } from '@/state/createVideoSlice'
import { createJavSlice } from '@/state/createJavSlice'
import { createNavigationSlice } from '@/state/createNavigationSlice'
import { createFavoriteSlice } from '@/state/createFavoriteSlice'
import { createCatalogSlice } from '@/state/createCatalogSlice'
import { createConfigSlice } from '@/state/createConfigSlice'
import { createTagSlice } from '@/state/createTagSlice'
import { createDirectorySlice } from '@/state/createDirectorySlice'
import { create } from 'zustand'
import { createWatchedTimeSlice } from '@/state/createWatchedTimeSlice'
import { applyWatchedTimeTotals } from '@/features/playback/watchedTimeState'

export { videoSelectionKey } from '@/state/model'

export function createAppState(rawSet, get) {
  // Apply the latest committed counters even when an older list/edit request
  // finishes after a live update. No list invalidation or reordering is needed.
  const set = (update) =>
    rawSet((state) => {
      const patch = typeof update === 'function' ? update(state) : update
      if (patch === state) return state
      return applyWatchedTimeTotals(patch, patch.watchedTimes || state.watchedTimes)
    })
  const lists = createListResources({ set, get })
  const invalidateDirectoryScopedRequests = () => {
    Object.values(lists).forEach((list) => list.invalidate())
    get().invalidateTagRequests()
    get().invalidateFavoriteRequests()
  }
  return {
    ...createWatchedTimeSlice({ set }),
    ...createVideoSlice({ set, get, lists }),
    ...createJavSlice({ set, get, lists }),
    ...createNavigationSlice({ set }),
    ...createFavoriteSlice({ set, get }),
    ...createCatalogSlice({ set, lists }),
    ...createConfigSlice({ get, set }),
    ...createTagSlice({ get, set }),
    ...createDirectorySlice({ set, get, invalidateDirectoryScopedRequests }),
  }
}
export const useStore = create(createAppState)
