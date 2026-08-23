import { useEffect, useMemo, useState } from 'react'
import AddRoundedIcon from '@mui/icons-material/AddRounded'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'
import { Button } from '@mui/material'
import AppModal from '@/components/AppModal'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

function SelectedTag({ tag, disabled, onRemove }) {
  return (
    <span className="inline-flex min-w-0 items-center gap-1 rounded-full bg-gray-100 px-2 py-1 text-sm text-gray-800">
      <span className="truncate">{tag.name}</span>
      <button
        type="button"
        className="inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-gray-500 hover:bg-gray-200 hover:text-gray-900 disabled:cursor-not-allowed disabled:opacity-50"
        onClick={onRemove}
        disabled={disabled}
        aria-label={zh(`移除 ${tag.name}`, `Remove ${tag.name}`)}
      >
        <CloseOutlinedIcon sx={{ fontSize: 13 }} />
      </button>
    </span>
  )
}

export default function SelectionJavTagsModal({
  open,
  onClose,
  items,
  tags,
  selectedIds,
  onToggleChoice,
  onCreateTag,
  onConfirm,
  saving = false,
}) {
  const [pickerOpen, setPickerOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const selected = useMemo(() => new Set((selectedIds || []).map(String)), [selectedIds])
  const selectedTags = useMemo(
    () => (tags || []).filter((tag) => selected.has(String(tag.id))),
    [selected, tags]
  )
  const availableTags = useMemo(() => {
    const query = search.trim().toLocaleLowerCase()
    return (tags || []).filter((tag) => {
      if (selected.has(String(tag.id))) return false
      return (
        !query ||
        String(tag?.name || '')
          .toLocaleLowerCase()
          .includes(query)
      )
    })
  }, [search, selected, tags])
  const exactMatch = useMemo(() => {
    const name = search.trim()
    if (!name) return null
    return (tags || []).find((tag) => String(tag?.name || '').trim() === name) || null
  }, [search, tags])

  useEffect(() => {
    if (open) return
    setPickerOpen(false)
    setSearch('')
    setCreating(false)
    setError('')
  }, [open])

  if (!open) return null

  const handleAddTag = async () => {
    const name = search.trim()
    if (!name || creating || saving) return
    if (exactMatch?.id) {
      onToggleChoice?.(exactMatch.id, true)
      setSearch('')
      return
    }

    setCreating(true)
    setError('')
    try {
      const created = await onCreateTag?.(name)
      if (!created?.id) throw new Error(zh('创建自定义标签失败', 'Failed to create custom tag'))
      onToggleChoice?.(created.id, true)
      setSearch('')
    } catch (createError) {
      setError(getErrorMessage(createError))
    } finally {
      setCreating(false)
    }
  }

  const disabled = saving || creating
  const list = (Array.isArray(items) ? items : []).filter((item) => Number(item?.jav_id) > 0)
  const javCount = new Set(list.map((item) => Number(item.jav_id))).size

  return (
    <AppModal
      ariaLabel={zh(
        '给选中视频对应的所有 JAV 添加自定义标签',
        'Add custom tags to all JAV items linked to the selected videos'
      )}
      className="p-4"
      closeDisabled={disabled}
      contentClassName="flex max-h-[85vh] w-full max-w-lg flex-col rounded-lg bg-white shadow-2xl"
      onClose={onClose}
      zIndex={1600}
    >
      <div className="flex items-start justify-between gap-3 p-5 pb-3">
        <div>
          <div className="text-base font-semibold text-gray-900">
            {zh(
              '给选中视频对应的所有 JAV 添加自定义标签',
              'Add custom tags to all JAV items linked to the selected videos'
            )}
          </div>
          <div className="mt-1 text-xs text-gray-500">
            {zh(`已选中 ${javCount} 部 JAV`, `${javCount} JAV items selected`)}
          </div>
        </div>
        <button
          type="button"
          className="rounded px-2 py-1 text-xl leading-none text-gray-500 hover:bg-gray-100 hover:text-gray-900"
          onClick={onClose}
          disabled={disabled}
          aria-label={zh('关闭', 'Close')}
        >
          ×
        </button>
      </div>

      <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-5 pb-5">
        <div>
          <div className="mb-2 grid grid-cols-[minmax(0,1fr)_auto] gap-3 px-2 text-xs font-medium text-gray-500">
            <span>{zh('文件名', 'Filename')}</span>
            <span>{zh('番号', 'Code')}</span>
          </div>
          <ul className="max-h-48 divide-y overflow-y-auto rounded-md border border-gray-200 bg-gray-50">
            {list.map((item) => (
              <li
                key={item.id}
                className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 px-3 py-2 text-sm"
              >
                <span className="truncate text-gray-800" title={item.label}>
                  {item.label}
                </span>
                <span className="font-medium text-gray-700">{item.jav_code || '-'}</span>
              </li>
            ))}
          </ul>
        </div>

        <div>
          <div className="text-sm font-medium text-gray-700">{zh('自定义标签', 'Custom tags')}</div>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            {selectedTags.map((tag) => (
              <SelectedTag
                key={`${tag.id}-${tag.provider || 0}`}
                tag={tag}
                disabled={disabled}
                onRemove={() => onToggleChoice?.(tag.id, false)}
              />
            ))}
            <button
              type="button"
              className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-gray-300 text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
              onClick={() => setPickerOpen((current) => !current)}
              disabled={disabled}
              title={zh('新增自定义标签', 'Add custom tag')}
              aria-label={zh('新增自定义标签', 'Add custom tag')}
              aria-expanded={pickerOpen}
            >
              <AddRoundedIcon sx={{ fontSize: 15 }} />
            </button>
          </div>

          {pickerOpen ? (
            <div className="mt-2 rounded-md border border-gray-200 p-2">
              <div className="mb-2 flex items-center gap-2">
                <input
                  type="text"
                  value={search}
                  onChange={(event) => {
                    setSearch(event.target.value)
                    if (error) setError('')
                  }}
                  onKeyDown={(event) => {
                    if (event.key !== 'Enter' || event.nativeEvent.isComposing) return
                    event.preventDefault()
                    void handleAddTag()
                  }}
                  placeholder={zh('搜索或输入自定义标签', 'Search or enter a custom tag')}
                  className="min-w-0 flex-1 rounded border border-gray-300 px-2 py-1.5 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                  disabled={disabled}
                />
                <button
                  type="button"
                  className="inline-flex shrink-0 items-center gap-1 rounded-md border border-gray-300 px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50"
                  onClick={() => {
                    setSearch('')
                    setPickerOpen(false)
                  }}
                  disabled={disabled}
                >
                  <CloseOutlinedIcon sx={{ fontSize: 14 }} />
                  {zh('完成', 'Done')}
                </button>
              </div>
              <div className="max-h-40 overflow-y-auto">
                {search.trim() ? (
                  <button
                    type="button"
                    className="mb-1 flex w-full items-center gap-1 rounded bg-blue-50 px-2 py-1.5 text-left text-sm text-blue-700 hover:bg-blue-100 disabled:cursor-wait disabled:opacity-60"
                    onClick={() => void handleAddTag()}
                    disabled={disabled}
                  >
                    <AddRoundedIcon sx={{ fontSize: 15 }} />
                    {creating
                      ? zh('创建中...', 'Creating...')
                      : exactMatch
                        ? zh(`选择“${search.trim()}”`, `Select “${search.trim()}”`)
                        : zh(`添加“${search.trim()}”`, `Add “${search.trim()}”`)}
                  </button>
                ) : null}
                {availableTags.length === 0 && !search.trim() ? (
                  <div className="px-2 py-1 text-sm text-gray-500">
                    {zh('暂无可添加标签', 'No tags to add')}
                  </div>
                ) : (
                  availableTags.map((tag) => (
                    <button
                      key={`${tag.id}-${tag.provider || 0}`}
                      type="button"
                      className="flex w-full items-center gap-3 rounded px-2 py-1.5 text-left text-sm text-gray-800 hover:bg-gray-50"
                      onClick={() => onToggleChoice?.(tag.id, true)}
                      disabled={disabled}
                    >
                      <span className="min-w-0 flex-1 truncate">{tag.name}</span>
                      <span className="shrink-0 text-xs tabular-nums text-gray-400">
                        {Math.max(0, Number(tag?.count) || 0)}
                      </span>
                    </button>
                  ))
                )}
              </div>
            </div>
          ) : null}
        </div>
        {error ? <div className="text-sm text-red-600">{error}</div> : null}
      </div>

      <div className="flex justify-end gap-2 border-t border-gray-200 p-5">
        <Button variant="outlined" size="small" onClick={onClose} disabled={disabled}>
          {zh('取消', 'Cancel')}
        </Button>
        <Button
          variant="contained"
          size="small"
          onClick={onConfirm}
          disabled={disabled || selected.size === 0}
        >
          {saving ? zh('添加中...', 'Adding...') : zh('添加', 'Add')}
        </Button>
      </div>
    </AppModal>
  )
}
