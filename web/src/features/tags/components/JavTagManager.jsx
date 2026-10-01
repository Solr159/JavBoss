import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { useState, useCallback, useEffect } from 'react'
import {
  fetchJavTagCategories,
  createJavTag,
  assignJavTagsCategory,
  organizeJavTags,
  createJavTagCategory,
  reorderJavTagCategories,
  renameJavTagCategory,
  deleteJavTagCategory,
  renameJavTag,
  deleteJavTag,
} from '@/features/jav/api'
import JavTagModal from '@/features/tags/components/JavTagModal'

export default function JavTagManager({ open, onClose, displayJavTagOptions, applyJavTagFilter }) {
  const { loadJavTags } = useStore(useShallow((s) => ({ loadJavTags: s.loadJavTags })))
  const [javTagCategories, setJavTagCategories] = useState([])
  const loadJavTagCategories = useCallback(async () => {
    const categories = await fetchJavTagCategories()
    setJavTagCategories(Array.isArray(categories) ? categories : [])
    return categories
  }, [])
  useEffect(() => {
    loadJavTagCategories().catch(() => setJavTagCategories([]))
  }, [loadJavTagCategories])
  return (
    <JavTagModal
      open={open}
      onClose={onClose}
      tags={displayJavTagOptions}
      categories={javTagCategories}
      onApplyTagFilter={applyJavTagFilter}
      onCreateTag={async (name, categoryId) => {
        const tag = await createJavTag(name)
        await assignJavTagsCategory([tag.id], categoryId)
        await loadJavTags({ force: true })
        return tag
      }}
      onOrganizeTags={async () => {
        const result = await organizeJavTags()
        await Promise.all([loadJavTags({ force: true }), loadJavTagCategories()])
        return result
      }}
      onCreateCategory={async (name) => {
        const category = await createJavTagCategory(name)
        await loadJavTagCategories()
        return category
      }}
      onReorderCategories={async (categoryIds) => {
        await reorderJavTagCategories(categoryIds)
        await loadJavTagCategories()
      }}
      onRenameCategory={async (id, name) => {
        await renameJavTagCategory(id, name)
        await Promise.all([loadJavTags({ force: true }), loadJavTagCategories()])
      }}
      onDeleteCategory={async (id) => {
        await deleteJavTagCategory(id)
        await Promise.all([loadJavTags({ force: true }), loadJavTagCategories()])
      }}
      onAssignCategory={async (tagIds, categoryId) => {
        await assignJavTagsCategory(tagIds, categoryId)
        await Promise.all([loadJavTags({ force: true }), loadJavTagCategories()])
      }}
      onRenameTag={async (id, name) => {
        await renameJavTag(id, name)
        useStore.setState((state) => {
          const options = Array.isArray(state.javTagOptions) ? state.javTagOptions : []
          const items = Array.isArray(state.javItems) ? state.javItems : []
          const nextOptions = options.map((tag) => (tag.id === id ? { ...tag, name } : tag))
          const nextItems = items.map((item) => {
            if (!Array.isArray(item.tags)) return item
            const nextTags = item.tags.map((tag) => (tag.id === id ? { ...tag, name } : tag))
            return nextTags === item.tags ? item : { ...item, tags: nextTags }
          })
          return { javTagOptions: nextOptions, javItems: nextItems }
        })
        await loadJavTags()
      }}
      onDeleteTag={async (tag) => {
        const id = typeof tag === 'object' ? tag?.id : tag
        if (!id) return
        await deleteJavTag(id)
        useStore.setState((state) => {
          const nextOptions = Array.isArray(state.javTagOptions)
            ? state.javTagOptions.filter((item) => item.id !== id)
            : state.javTagOptions
          const nextItems = Array.isArray(state.javItems)
            ? state.javItems.map((item) => {
                if (!Array.isArray(item.tags)) return item
                const nextTags = item.tags.filter((tagItem) => tagItem.id !== id)
                return nextTags === item.tags ? item : { ...item, tags: nextTags }
              })
            : state.javItems
          const nextFilters = Array.isArray(state.javTags)
            ? state.javTags.filter((tagId) => tagId !== id)
            : state.javTags
          return { javTagOptions: nextOptions, javItems: nextItems, javTags: nextFilters }
        })
        await loadJavTags()
      }}
    />
  )
}
