import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { useState, useCallback, useMemo } from 'react'
import { replaceTagsForVideos } from '@/features/tags/api'
import { getErrorMessage } from '@/utils/errors'

export default function useVideoTagEditor({ showCenterToast }) {
  const { videos, tags } = useStore(
    useShallow((state) => ({ videos: state.videos, tags: state.tags }))
  )
  const [tagPickerFor, setTagPickerFor] = useState(null)

  const [tagPickerSelected, setTagPickerSelected] = useState([])

  const openTagEditor = useCallback(
    (videoId) => {
      setTagPickerFor(videoId)
      const target = videos.find((v) => v.id === videoId)
      const initial = Array.isArray(target?.tags) ? target.tags.map((t) => String(t.id)) : []
      setTagPickerSelected(initial)
    },
    [videos]
  )

  const tagPickerExisting = useMemo(() => {
    if (!tagPickerFor) return []
    const target = videos.find((v) => v.id === tagPickerFor)
    return Array.isArray(target?.tags) ? target.tags.map((t) => String(t.id)) : []
  }, [tagPickerFor, videos])

  const tagPickerDirty = useMemo(() => {
    if (!tagPickerFor) return false
    const current = new Set(tagPickerExisting)
    const selected = new Set(tagPickerSelected)
    if (current.size !== selected.size) return true
    for (const id of current) {
      if (!selected.has(id)) return true
    }
    return false
  }, [tagPickerExisting, tagPickerFor, tagPickerSelected])

  const handleApplyTags = async () => {
    if (!tagPickerFor) {
      setTagPickerFor(null)
      setTagPickerSelected([])
      return
    }
    const selectedIds = tagPickerSelected.map((t) => Number(t)).filter(Boolean)
    if (!tagPickerDirty) {
      setTagPickerFor(null)
      setTagPickerSelected([])
      return
    }
    try {
      await replaceTagsForVideos([tagPickerFor], selectedIds)
      useStore.setState((state) => {
        if (!Array.isArray(state.videos)) return {}
        const tagLookup = new Map((tags || []).map((tag) => [tag.id, tag]))
        const nextVideos = state.videos.map((video) => {
          if (video.id !== tagPickerFor) return video
          const nextTags = selectedIds.map((id) => tagLookup.get(id)).filter(Boolean)
          return { ...video, tags: nextTags }
        })
        return { videos: nextVideos }
      })
    } catch (err) {
      console.error('update tags failed', err)
      showCenterToast(getErrorMessage(err))
    } finally {
      setTagPickerFor(null)
      setTagPickerSelected([])
    }
  }

  const handleTagPickerClose = () => {
    setTagPickerFor(null)
    setTagPickerSelected([])
  }

  const handleTagPickerToggle = (tagId, checked) => {
    setTagPickerSelected((prev) => {
      const set = new Set(prev)
      if (checked) set.add(String(tagId))
      else set.delete(String(tagId))
      return Array.from(set)
    })
  }
  return {
    tagPickerFor,
    setTagPickerFor,
    tagPickerSelected,
    openTagEditor,
    tagPickerDirty,
    handleApplyTags,
    handleTagPickerClose,
    handleTagPickerToggle,
  }
}
