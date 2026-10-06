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

export { videoSelectionKey } from '@/state/model'

export function createAppState(set, get) {
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
