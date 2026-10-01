import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { useState, useCallback, useEffect } from 'react'
import {
  fetchTagCategories,
  assignTagsCategory,
  createTagCategory,
  reorderTagCategories,
  renameTagCategory,
  deleteTagCategory,
} from '@/features/tags/api'
import VideoTagModal from '@/features/tags/components/VideoTagModal'

export default function VideoTagManager({ open, onClose, tagModalApplyMode }) {
  const {
    tags,
    toggleTagFilter,
    createTag,
    loadTags,
    renameTag,
    deleteTag,
    setSelectedTags,
    selectedTags,
    setSearchTerm,
  } = useStore(
    useShallow((s) => ({
      tags: s.tags,
      toggleTagFilter: s.toggleTagFilter,
      createTag: s.createTag,
      loadTags: s.loadTags,
      renameTag: s.renameTag,
      deleteTag: s.deleteTag,
      setSelectedTags: s.setSelectedTags,
      selectedTags: s.selectedTags,
      setSearchTerm: s.setSearchTerm,
    }))
  )
  const [tagCategories, setTagCategories] = useState([])
  const loadTagCategories = useCallback(async () => {
    const categories = await fetchTagCategories()
    setTagCategories(Array.isArray(categories) ? categories : [])
    return categories
  }, [])
  useEffect(() => {
    loadTagCategories().catch(() => setTagCategories([]))
  }, [loadTagCategories])
  return (
    <VideoTagModal
      open={open}
      onClose={onClose}
      tags={tags}
      categories={tagCategories}
      onToggleFilter={(name) => toggleTagFilter(name)}
      onCreateTag={async (name, categoryId) => {
        const tag = await createTag(name)
        await assignTagsCategory([tag.id], categoryId)
        await loadTags()
        return tag
      }}
      onCreateCategory={async (name) => {
        const category = await createTagCategory(name)
        await loadTagCategories()
        return category
      }}
      onReorderCategories={async (categoryIds) => {
        await reorderTagCategories(categoryIds)
        await loadTagCategories()
      }}
      onRenameCategory={async (id, name) => {
        await renameTagCategory(id, name)
        await Promise.all([loadTags(), loadTagCategories()])
      }}
      onDeleteCategory={async (id) => {
        await deleteTagCategory(id)
        await Promise.all([loadTags(), loadTagCategories()])
      }}
      onAssignCategory={async (tagIds, categoryId) => {
        await assignTagsCategory(tagIds, categoryId)
        await Promise.all([loadTags(), loadTagCategories()])
      }}
      onRenameTag={async (id, name) => {
        const oldName = tags.find((t) => t.id === id)?.name || ''
        await renameTag(id, name)
        useStore.setState((state) => {
          const nextTags = Array.isArray(state.tags)
            ? state.tags.map((tag) => (tag.id === id ? { ...tag, name } : tag))
            : state.tags
          const nextVideos = Array.isArray(state.videos)
            ? state.videos.map((video) => {
                if (!Array.isArray(video.tags)) return video
                const nextVideoTags = video.tags.map((tag) =>
                  tag.id === id ? { ...tag, name } : tag
                )
                return nextVideoTags === video.tags ? video : { ...video, tags: nextVideoTags }
              })
            : state.videos
          const nextSelectedTags =
            oldName && Array.isArray(state.selectedTags)
              ? state.selectedTags.map((tagName) => (tagName === oldName ? name : tagName))
              : state.selectedTags
          return { tags: nextTags, videos: nextVideos, selectedTags: nextSelectedTags }
        })
        await loadTags()
      }}
      onDeleteTag={async (tag) => {
        const id = typeof tag === 'object' ? tag?.id : tag
        if (!id) return
        const name =
          typeof tag === 'object' ? tag?.name : tags.find((item) => item.id === id)?.name || ''
        await deleteTag(id)
        if (name) {
          useStore.setState((state) => ({
            selectedTags: state.selectedTags.filter((tagName) => tagName !== name),
          }))
        }
        await loadTags()
      }}
      onApplyTagFilter={(names) => {
        if (tagModalApplyMode === 'append') {
          setSelectedTags([...selectedTags, ...names])
          return
        }
        setSearchTerm('', { resetPage: false, triggerLoad: false })
        setSelectedTags(names)
      }}
    />
  )
}
