import { useStore } from '@/store'
import { useRef } from 'react'
import {
  fetchJavIdolPreview,
  fetchJavStudioPreview,
  fetchJavSeriesPreview,
} from '@/features/jav/api'

export default function useJavPreviews() {
  const directoryVisibilityKey = useStore((state) =>
    (state.directories || [])
      .map((directory) => `${directory.id}:${directory.enabled !== false ? '1' : '0'}`)
      .join(',')
  )
  const idolPreviewCacheRef = useRef(new Map())
  const idolPreviewInflightRef = useRef(new Map())
  const studioPreviewCacheRef = useRef(new Map())
  const studioPreviewInflightRef = useRef(new Map())
  const seriesPreviewCacheRef = useRef(new Map())
  const seriesPreviewInflightRef = useRef(new Map())
  const loadIdolPreview = async (idol) => {
    const idolId = Number(idol?.id)
    if (!Number.isFinite(idolId) || idolId <= 0) {
      return idol || null
    }

    const cacheKey = `${idolId}|${directoryVisibilityKey}`
    const cached = idolPreviewCacheRef.current.get(cacheKey)
    if (cached) {
      return cached
    }

    const inflight = idolPreviewInflightRef.current.get(cacheKey)
    if (inflight) {
      return inflight
    }

    const request = fetchJavIdolPreview(idolId)
      .then((preview) => {
        idolPreviewCacheRef.current.set(cacheKey, preview)
        return preview
      })
      .finally(() => {
        idolPreviewInflightRef.current.delete(cacheKey)
      })
    idolPreviewInflightRef.current.set(cacheKey, request)
    return request
  }

  const loadStudioPreview = async (studio) => {
    const studioId = Number(studio?.id)
    if (!Number.isFinite(studioId) || studioId <= 0) {
      return studio || null
    }

    const cacheKey = `${studioId}|${directoryVisibilityKey}`
    const cached = studioPreviewCacheRef.current.get(cacheKey)
    if (cached) {
      return cached
    }

    const inflight = studioPreviewInflightRef.current.get(cacheKey)
    if (inflight) {
      return inflight
    }

    const request = fetchJavStudioPreview(studioId)
      .then((preview) => {
        studioPreviewCacheRef.current.set(cacheKey, preview)
        return preview
      })
      .finally(() => {
        studioPreviewInflightRef.current.delete(cacheKey)
      })
    studioPreviewInflightRef.current.set(cacheKey, request)
    return request
  }

  const loadSeriesPreview = async (series) => {
    const seriesId = Number(series?.id)
    if (!Number.isFinite(seriesId) || seriesId <= 0) {
      return series || null
    }

    const cacheKey = `${seriesId}|${directoryVisibilityKey}`
    const cached = seriesPreviewCacheRef.current.get(cacheKey)
    if (cached) {
      return cached
    }

    const inflight = seriesPreviewInflightRef.current.get(cacheKey)
    if (inflight) {
      return inflight
    }

    const request = fetchJavSeriesPreview(seriesId)
      .then((preview) => {
        seriesPreviewCacheRef.current.set(cacheKey, preview)
        return preview
      })
      .finally(() => {
        seriesPreviewInflightRef.current.delete(cacheKey)
      })
    seriesPreviewInflightRef.current.set(cacheKey, request)
    return request
  }

  const handleIdolPreviewUpdated = (updated) => {
    const idolId = Number(updated?.id)
    if (!Number.isFinite(idolId) || idolId <= 0) return
    for (const [key, cached] of idolPreviewCacheRef.current.entries()) {
      if (String(key).startsWith(`${idolId}|`)) {
        idolPreviewCacheRef.current.set(key, { ...cached, ...updated })
      }
    }
  }

  return { loadIdolPreview, loadStudioPreview, loadSeriesPreview, handleIdolPreviewUpdated }
}
