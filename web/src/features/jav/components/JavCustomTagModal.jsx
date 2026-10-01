import { useStore } from '@/store'
import { useState, useMemo, useEffect } from 'react'
import { isUserJavTag } from '@/constants/jav'
import {
  mergeOptionsById,
  filterOptionsByName,
  optionsByIds,
  SelectedChip,
} from '@/features/jav/components/JavEditorFields'
import { createJavTag, updateJavItem } from '@/features/jav/api'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'
import AppModal from '@/components/AppModal'

export function JavCustomTagModal({ open, item, onClose, onSaved }) {
  const tagOptions = useStore((state) => state.javTagOptions || [])
  const loadJavTags = useStore((state) => state.loadJavTags)
  const [selectedTagIds, setSelectedTagIds] = useState([])
  const [createdTags, setCreatedTags] = useState([])
  const [search, setSearch] = useState('')
  const [saving, setSaving] = useState(false)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const code = String(item?.code || '').trim()
  const currentUserTags = useMemo(
    () => (Array.isArray(item?.tags) ? item.tags.filter((tag) => isUserJavTag(tag)) : []),
    [item?.tags]
  )
  const userTagOptions = useMemo(() => tagOptions.filter((tag) => isUserJavTag(tag)), [tagOptions])
  const mergedTagOptions = useMemo(
    () => mergeOptionsById(userTagOptions, [...currentUserTags, ...createdTags]),
    [createdTags, currentUserTags, userTagOptions]
  )
  const visibleTagOptions = useMemo(
    () => filterOptionsByName(mergedTagOptions, search),
    [mergedTagOptions, search]
  )
  const selectedTags = useMemo(
    () => optionsByIds(mergedTagOptions, selectedTagIds),
    [mergedTagOptions, selectedTagIds]
  )
  const exactMatch = useMemo(() => {
    const name = search.trim()
    if (!name) return null
    return mergedTagOptions.find((tag) => String(tag?.name || '').trim() === name) || null
  }, [mergedTagOptions, search])

  useEffect(() => {
    if (!open) return
    setSelectedTagIds(currentUserTags.map((tag) => String(tag.id)))
    setCreatedTags([])
    setSearch('')
    setSaving(false)
    setCreating(false)
    setError('')
    void loadJavTags?.({ skipUnchanged: true })
  }, [currentUserTags, loadJavTags, open])

  if (!open) return null

  const toggleTag = (tagId) => {
    const id = String(tagId)
    setSelectedTagIds((current) =>
      current.includes(id) ? current.filter((currentId) => currentId !== id) : [...current, id]
    )
    if (error) setError('')
  }

  const addCustomTag = async () => {
    const name = search.trim()
    if (!name || creating || saving) return
    if (exactMatch?.id) {
      const id = String(exactMatch.id)
      setSelectedTagIds((current) => (current.includes(id) ? current : [...current, id]))
      setSearch('')
      return
    }

    setCreating(true)
    setError('')
    try {
      const created = await createJavTag(name)
      if (!created?.id) throw new Error(zh('创建自定义标签失败', 'Failed to create custom tag'))
      setCreatedTags((current) => mergeOptionsById(current, [created]))
      setSelectedTagIds((current) => [...new Set([...current, String(created.id)])])
      setSearch('')
      void loadJavTags?.({ force: true })
    } catch (createError) {
      setError(getErrorMessage(createError))
    } finally {
      setCreating(false)
    }
  }

  const handleSave = async () => {
    const javID = Number(item?.id)
    if (!Number.isFinite(javID) || javID <= 0) {
      setError(zh('缺少 JAV ID', 'Missing JAV ID'))
      return
    }

    setSaving(true)
    setError('')
    try {
      const updated = await updateJavItem(javID, {
        tag_ids: selectedTagIds.map((id) => Number(id)).filter((id) => id > 0),
      })
      void loadJavTags?.({ force: true })
      onSaved?.(updated)
    } catch (saveError) {
      setError(getErrorMessage(saveError))
    } finally {
      setSaving(false)
    }
  }

  return (
    <AppModal
      ariaLabel={zh('编辑自定义标签', 'Edit custom tags')}
      className="p-4"
      closeDisabled={saving || creating}
      contentClassName="flex max-h-[80vh] w-full max-w-lg flex-col rounded-lg bg-white shadow-2xl"
      onClose={onClose}
      zIndex={1600}
    >
      <div className="flex items-start justify-between gap-3 p-5 pb-3">
        <div className="min-w-0">
          <div className="text-base font-semibold text-gray-900">
            {zh('编辑自定义标签', 'Edit custom tags')}
          </div>
          {code ? <div className="mt-1 truncate text-xs text-gray-500">{code}</div> : null}
        </div>
        <button
          type="button"
          className="rounded px-2 py-1 text-xl leading-none text-gray-500 hover:bg-gray-100 hover:text-gray-900"
          onClick={onClose}
          disabled={saving || creating}
          aria-label={zh('关闭', 'Close')}
        >
          ×
        </button>
      </div>
      <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-5 pb-5">
        {selectedTags.length > 0 ? (
          <div className="flex flex-wrap gap-2">
            {selectedTags.map((tag) => (
              <SelectedChip
                key={tag.id}
                label={tag.name}
                disabled={saving || creating}
                onRemove={() => toggleTag(tag.id)}
              />
            ))}
          </div>
        ) : null}
        <div className="flex items-center gap-2">
          <input
            type="search"
            value={search}
            onChange={(event) => {
              setSearch(event.target.value)
              if (error) setError('')
            }}
            onKeyDown={(event) => {
              if (event.key !== 'Enter' || event.nativeEvent.isComposing) return
              event.preventDefault()
              void addCustomTag()
            }}
            placeholder={zh('搜索或输入新的自定义标签', 'Search or enter a new custom tag')}
            className="min-w-0 flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
            disabled={saving || creating}
          />
          {search.trim() ? (
            <button
              type="button"
              className="shrink-0 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm text-blue-700 hover:bg-blue-100 disabled:cursor-wait disabled:opacity-60"
              onClick={() => void addCustomTag()}
              disabled={saving || creating}
            >
              {creating
                ? zh('创建中...', 'Creating...')
                : exactMatch
                  ? zh('选择', 'Select')
                  : zh('创建', 'Create')}
            </button>
          ) : null}
        </div>
        <div className="max-h-64 overflow-y-auto rounded-md border border-gray-200 p-1">
          {visibleTagOptions.length > 0 ? (
            visibleTagOptions.map((tag) => {
              const checked = selectedTagIds.includes(String(tag.id))
              return (
                <button
                  key={tag.id}
                  type="button"
                  className={`flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm hover:bg-gray-50 ${
                    checked ? 'bg-blue-50 text-blue-800' : 'text-gray-800'
                  }`}
                  onClick={() => toggleTag(tag.id)}
                  disabled={saving || creating}
                  aria-pressed={checked}
                >
                  <span
                    aria-hidden="true"
                    className={`flex h-4 w-4 shrink-0 items-center justify-center rounded border text-[11px] ${
                      checked
                        ? 'border-blue-600 bg-blue-600 text-white'
                        : 'border-gray-300 bg-white text-transparent'
                    }`}
                  >
                    ✓
                  </span>
                  <span className="min-w-0 flex-1 truncate">{tag.name}</span>
                  <span className="shrink-0 text-xs tabular-nums text-gray-400">
                    {Math.max(0, Number(tag?.count) || 0)}
                  </span>
                </button>
              )
            })
          ) : (
            <div className="px-2 py-4 text-center text-sm text-gray-500">
              {search.trim()
                ? zh('没有匹配标签，可直接创建', 'No matching tags; create it directly')
                : zh('暂无自定义标签', 'No custom tags')}
            </div>
          )}
        </div>
        {error ? <div className="text-sm text-red-600">{error}</div> : null}
      </div>
      <div className="flex justify-end gap-2 border-t border-gray-200 p-5">
        <button
          type="button"
          className="rounded-md border border-gray-300 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
          onClick={onClose}
          disabled={saving || creating}
        >
          {zh('取消', 'Cancel')}
        </button>
        <button
          type="button"
          className={`rounded-md px-4 py-2 text-sm font-medium text-white ${
            saving ? 'cursor-wait bg-blue-400' : 'bg-blue-600 hover:bg-blue-700'
          }`}
          onClick={() => void handleSave()}
          disabled={saving || creating}
        >
          {saving ? zh('保存中...', 'Saving...') : zh('保存', 'Save')}
        </button>
      </div>
    </AppModal>
  )
}
