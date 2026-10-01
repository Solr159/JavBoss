import {
  fetchTags,
  createTag,
  deleteTag,
  renameTag,
  addTagToVideos,
  removeTagFromVideos,
} from '@/features/tags/api'
import { fetchJavTags } from '@/features/jav/api'
import { getErrorMessage } from '@/utils/errors'
import { selectedVideoContentIds } from '@/state/model'

export function createTagSlice({ get, set }) {
  let lastTagFetchKey = null
  let lastJavTagFetchKey = null
  let tagFetchInFlight = null
  let tagFetchInFlightKey = null
  let javTagFetchInFlight = null
  let javTagFetchInFlightKey = null
  return {
    tags: [],
    javTagOptions: [],
    loadTags: async (options = {}) => {
      const { videoHideJav } = get()
      const key = `tags|${videoHideJav ? 'hide-jav' : 'show-jav'}`
      if (tagFetchInFlight && tagFetchInFlightKey === key) {
        return tagFetchInFlight
      }
      if (!options.force && options.skipUnchanged && key === lastTagFetchKey) {
        return null
      }
      tagFetchInFlightKey = key
      tagFetchInFlight = (async () => {
        try {
          const tags = await fetchTags({ hideJav: videoHideJav })
          set({ tags })
          lastTagFetchKey = key
          return tags
        } catch (e) {
          set({ error: e.message })
          return null
        } finally {
          if (tagFetchInFlightKey === key) {
            tagFetchInFlight = null
            tagFetchInFlightKey = null
          }
        }
      })()
      return tagFetchInFlight
    },
    loadJavTags: async (options = {}) => {
      const key = 'jav-tags'
      if (javTagFetchInFlight && javTagFetchInFlightKey === key) {
        const pending = javTagFetchInFlight
        if (!options.force) return pending

        // A forced refresh must observe mutations completed before this call. The
        // existing request may already contain a pre-mutation snapshot, so wait
        // for it and then ensure a newer request is used.
        await pending
        if (javTagFetchInFlight && javTagFetchInFlightKey === key) {
          return javTagFetchInFlight
        }
      }
      if (!options.force && options.skipUnchanged && key === lastJavTagFetchKey) {
        return null
      }
      javTagFetchInFlightKey = key
      javTagFetchInFlight = (async () => {
        try {
          const tags = await fetchJavTags()
          set({ javTagOptions: tags })
          lastJavTagFetchKey = key
          return tags
        } catch (e) {
          set({ javError: getErrorMessage(e) })
          return null
        } finally {
          if (javTagFetchInFlightKey === key) {
            javTagFetchInFlight = null
            javTagFetchInFlightKey = null
          }
        }
      })()
      return javTagFetchInFlight
    },
    createTag: async (name) => {
      const tag = await createTag(name)
      set({ tags: [...get().tags, tag] })
      return tag
    },
    deleteTag: async (id) => {
      await deleteTag(id)
      set({ tags: get().tags.filter((t) => t.id !== id) })
    },
    renameTag: async (id, name) => {
      await renameTag(id, name)
      set({ tags: get().tags.map((t) => (t.id === id ? { ...t, name } : t)) })
    },
    addTagToSelection: async (tagId) => {
      const ids = selectedVideoContentIds(get())
      if (ids.length === 0) return
      await addTagToVideos(tagId, ids)
      await get().loadVideos({ force: true })
    },
    removeTagFromSelection: async (tagId) => {
      const ids = selectedVideoContentIds(get())
      if (ids.length === 0) return
      await removeTagFromVideos(tagId, ids)
      await get().loadVideos({ force: true })
    },
    invalidateTagRequests: () => {
      lastTagFetchKey = null
      lastJavTagFetchKey = null
    },
  }
}
