import { useEffect, useMemo, useState } from 'react'
import { Button, IconButton, MenuItem, TextField } from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import AutoFixHighOutlinedIcon from '@mui/icons-material/AutoFixHighOutlined'
import CategoryOutlinedIcon from '@mui/icons-material/CategoryOutlined'
import CheckBoxOutlinedIcon from '@mui/icons-material/CheckBoxOutlined'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'
import EditOutlinedIcon from '@mui/icons-material/EditOutlined'
import SearchOutlinedIcon from '@mui/icons-material/SearchOutlined'

import AppModal from '@/components/AppModal'
import TagBar from '@/components/TagBar'
import { isUserJavTag } from '@/constants/jav'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'

const compactButtonSx = {
  minHeight: 28,
  px: 1.25,
  py: 0.25,
  fontSize: '0.75rem',
  lineHeight: 1.25,
  '& .MuiButton-startIcon': {
    marginRight: 0.5,
    '& svg': {
      fontSize: 16,
    },
  },
}

const categoryEnglishLabels = {
  主题: 'Theme',
  角色: 'Role',
  服装: 'Clothing',
  体型: 'Body type',
  行为: 'Activity',
  玩法: 'Play',
  类别: 'Type',
  场景: 'Scene',
  其他: 'Other',
}

export default function JavTagModal({
  open,
  onClose,
  tags,
  categories = [],
  onApplyTagFilter,
  onCreateTag,
  onOrganizeTags,
  onCreateCategory,
  onRenameCategory,
  onDeleteCategory,
  onAssignCategory,
  onRenameTag,
  onDeleteTag,
}) {
  const [createOpen, setCreateOpen] = useState(false)
  const [newTagName, setNewTagName] = useState('')
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState('')
  const [renameOpen, setRenameOpen] = useState(false)
  const [renameTagId, setRenameTagId] = useState(null)
  const [renameOriginalName, setRenameOriginalName] = useState('')
  const [renameTagName, setRenameTagName] = useState('')
  const [renaming, setRenaming] = useState(false)
  const [renameError, setRenameError] = useState('')
  const [editMode, setEditMode] = useState(false)
  const [hoverTagId, setHoverTagId] = useState(null)
  const [multiSelect, setMultiSelect] = useState(false)
  const [selectedTagIds, setSelectedTagIds] = useState([])
  const [batchError, setBatchError] = useState('')
  const [deletingId, setDeletingId] = useState(null)
  const [organizing, setOrganizing] = useState(false)
  const [actionMessage, setActionMessage] = useState('')
  const [categoryManageOpen, setCategoryManageOpen] = useState(false)
  const [newCategoryName, setNewCategoryName] = useState('')
  const [categoryEditingId, setCategoryEditingId] = useState(null)
  const [categoryEditingName, setCategoryEditingName] = useState('')
  const [categoryBusyId, setCategoryBusyId] = useState(null)
  const [categoryError, setCategoryError] = useState('')
  const [batchCategoryValue, setBatchCategoryValue] = useState('')
  const [batchCategoryOpen, setBatchCategoryOpen] = useState(false)
  const [assigningCategory, setAssigningCategory] = useState(false)

  const handleTagClick = (tagId) => {
    onApplyTagFilter?.([tagId])
    onClose()
  }

  useEffect(() => {
    if (!open) {
      setCreateOpen(false)
      setNewTagName('')
      setCreating(false)
      setCreateError('')
      setRenameOpen(false)
      setRenameTagId(null)
      setRenameOriginalName('')
      setRenameTagName('')
      setRenaming(false)
      setRenameError('')
      setEditMode(false)
      setHoverTagId(null)
      setMultiSelect(false)
      setSelectedTagIds([])
      setBatchError('')
      setDeletingId(null)
      setOrganizing(false)
      setActionMessage('')
      setCategoryManageOpen(false)
      setNewCategoryName('')
      setCategoryEditingId(null)
      setCategoryEditingName('')
      setCategoryBusyId(null)
      setCategoryError('')
      setBatchCategoryValue('')
      setBatchCategoryOpen(false)
      setAssigningCategory(false)
    }
  }, [open])

  const handleStartRename = (tag) => {
    if (!isUserJavTag(tag)) return
    setRenameOpen(true)
    setRenameTagId(tag.id)
    setRenameOriginalName(tag.name || '')
    setRenameTagName(tag.name || '')
    setRenameError('')
    setRenaming(false)
  }

  const handleCloseRename = () => {
    setRenameOpen(false)
    setRenameTagId(null)
    setRenameOriginalName('')
    setRenameTagName('')
    setRenameError('')
    setRenaming(false)
  }

  const handleToggleEditMode = () => {
    setEditMode((prev) => !prev)
    setHoverTagId(null)
  }

  const displayTags = useMemo(() => {
    return [...tags].sort((a, b) => {
      const countA = Number.isFinite(a?.count) ? a.count : 0
      const countB = Number.isFinite(b?.count) ? b.count : 0
      if (countB !== countA) return countB - countA
      return String(a?.name || '').localeCompare(String(b?.name || ''))
    })
  }, [tags])

  const categoryGroups = useMemo(() => {
    const groups = new Map()
    for (const tag of displayTags) {
      const category = String(tag?.category || '').trim()
      if (!groups.has(category)) groups.set(category, [])
      groups.get(category).push(tag)
    }
    const categoryOrder = new Map(
      ['主题', '角色', '服装', '体型', '行为', '玩法', '类别', '场景', '其他'].map(
        (name, index) => [name, index]
      )
    )
    return Array.from(groups.entries())
      .map(([category, groupTags]) => ({ category, tags: groupTags }))
      .sort((a, b) => {
        if (!a.category) return 1
        if (!b.category) return -1
        const orderA = categoryOrder.get(a.category) ?? categoryOrder.size
        const orderB = categoryOrder.get(b.category) ?? categoryOrder.size
        if (orderA !== orderB) return orderA - orderB
        return a.category.localeCompare(b.category)
      })
  }, [displayTags])

  const selectedIds = useMemo(() => {
    if (selectedTagIds.length === 0) return []
    const set = new Set(selectedTagIds)
    return displayTags.filter((t) => set.has(t.id)).map((t) => t.id)
  }, [displayTags, selectedTagIds])

  const categoryUsageCounts = useMemo(() => {
    const counts = new Map()
    for (const tag of displayTags) {
      const categoryId = Number(tag?.category_id)
      if (!Number.isFinite(categoryId) || categoryId <= 0) continue
      counts.set(categoryId, (counts.get(categoryId) || 0) + 1)
    }
    return counts
  }, [displayTags])

  const handleOrganizeTags = async () => {
    if (organizing) return
    setOrganizing(true)
    setBatchError('')
    setActionMessage('')
    try {
      const result = await onOrganizeTags?.()
      setActionMessage(
        zh(
          `整理完成：从 JavBus 读取 ${result?.remote_tag_count || 0} 个标签，匹配 ${result?.matched_tag_count || 0} 个，更新 ${result?.updated_tag_count || 0} 个，未匹配 ${result?.unmatched_tag_count || 0} 个`,
          `Organized: ${result?.remote_tag_count || 0} read from JavBus, ${result?.matched_tag_count || 0} matched, ${result?.updated_tag_count || 0} updated, ${result?.unmatched_tag_count || 0} unmatched`
        )
      )
    } catch (err) {
      setBatchError(getErrorMessage(err))
    } finally {
      setOrganizing(false)
    }
  }

  const handleAssignCategory = async () => {
    if (assigningCategory || selectedIds.length === 0 || !batchCategoryValue) return
    const categoryId = batchCategoryValue === '__uncategorized' ? null : Number(batchCategoryValue)
    setAssigningCategory(true)
    setBatchError('')
    setActionMessage('')
    try {
      await onAssignCategory?.(selectedIds, categoryId)
      setActionMessage(
        zh(
          `已调整 ${selectedIds.length} 个标签的分类`,
          `Updated the category for ${selectedIds.length} tags`
        )
      )
      setSelectedTagIds([])
      setBatchCategoryValue('')
      setBatchCategoryOpen(false)
    } catch (err) {
      setBatchError(getErrorMessage(err))
    } finally {
      setAssigningCategory(false)
    }
  }

  const renderTagGroup = (group) => {
    if (multiSelect) {
      return (
        <TagBar
          tags={group}
          onToggle={handleTagClick}
          multiSelect={multiSelect}
          selectedIds={selectedTagIds}
          variant="neumorphic"
          tagClassName={(tag) => (isUserJavTag(tag) ? 'skeuo-tag--user' : 'skeuo-tag--scraped')}
          onSelect={(id) => {
            setSelectedTagIds((prev) => {
              const next = new Set(prev)
              if (next.has(id)) {
                next.delete(id)
              } else {
                next.add(id)
              }
              return Array.from(next)
            })
          }}
        />
      )
    }

    return (
      <div className="flex flex-wrap gap-2">
        {group.map((t) => {
          const count = Number.isFinite(t.count) ? t.count : null
          const canRename = isUserJavTag(t)
          const showRenameHint = editMode && hoverTagId === t.id && canRename
          const showDelete = editMode && hoverTagId === t.id && canRename
          const baseTagClass = isUserJavTag(t) ? 'skeuo-tag--user' : 'skeuo-tag--scraped'
          const interactiveTagClass = editMode
            ? canRename
              ? showRenameHint
                ? 'skeuo-tag--active'
                : 'skeuo-tag--editing'
              : ''
            : 'skeuo-tag--button'
          return (
            <div
              key={`${t.id}-${t.provider || 0}`}
              className={`skeuo-tag ${baseTagClass} ${interactiveTagClass}`}
              onMouseEnter={() => {
                if (editMode) setHoverTagId(t.id)
              }}
              onMouseLeave={() => {
                if (editMode) setHoverTagId((prev) => (prev === t.id ? null : prev))
              }}
            >
              <button
                type="button"
                className="flex min-w-0 items-center gap-2 text-left"
                onClick={() => {
                  if (editMode) {
                    if (canRename) handleStartRename(t)
                    return
                  }
                  handleTagClick(t.id)
                }}
                title={t.name}
              >
                <span className="skeuo-tag-label">{t.name}</span>
                {!editMode && count !== null && <span className="skeuo-tag-count">{count}</span>}
                {showRenameHint && (
                  <span className="skeuo-tag-hint">{zh('单击重命名', 'Click to rename')}</span>
                )}
              </button>
              {showDelete && (
                <IconButton
                  size="small"
                  type="button"
                  aria-label={zh('删除标签', 'Delete tag')}
                  disabled={deletingId === t.id}
                  className="skeuo-tag-delete"
                  sx={{
                    borderRadius: 0,
                    padding: 0,
                    width: '1.5rem',
                    height: '1.5rem',
                  }}
                  onClick={async (event) => {
                    event.preventDefault()
                    event.stopPropagation()
                    if (deletingId === t.id) return
                    if (
                      !window.confirm(zh(`确定删除标签“${t.name}”吗？`, `Delete tag "${t.name}"?`))
                    )
                      return
                    setDeletingId(t.id)
                    setBatchError('')
                    try {
                      await onDeleteTag?.(t)
                    } catch (err) {
                      setBatchError(getErrorMessage(err))
                    } finally {
                      setDeletingId(null)
                    }
                  }}
                >
                  <CloseOutlinedIcon fontSize="inherit" className="h-3.5 w-3.5" />
                </IconButton>
              )}
            </div>
          )
        })}
      </div>
    )
  }

  if (!open) return null
  return (
    <AppModal
      ariaLabel={zh('标签管理', 'Tag Management')}
      contentClassName="mx-4 flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden rounded-3xl bg-white shadow-2xl ring-1 ring-slate-200/70"
      onClose={onClose}
    >
      <div className="flex shrink-0 items-center justify-between border-b border-slate-200/70 bg-slate-50/80 px-6 py-4">
        <h2 className="text-lg font-semibold text-slate-900">{zh('标签管理', 'Tag Management')}</h2>
        <Button
          size="small"
          variant="text"
          onClick={onClose}
          aria-label={zh('关闭', 'Close')}
          sx={compactButtonSx}
        >
          {zh('关闭', 'Close')}
        </Button>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-6 py-5">
        <div className="mb-5 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-slate-500">
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full border border-orange-300 bg-orange-50" />
            {zh('我创建的标签', 'My tags')}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full border border-blue-300 bg-blue-50" />
            {zh('抓取标签', 'Scraped tags')}
          </span>
        </div>
        {categoryGroups.length > 0 ? (
          <div className="space-y-6">
            {categoryGroups.map((group) => (
              <div key={group.category || '__uncategorized'} className="space-y-2">
                <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-slate-400">
                  <span>
                    {group.category
                      ? zh(group.category, categoryEnglishLabels[group.category] || group.category)
                      : zh('未分类', 'Uncategorized')}
                  </span>
                  <span className="font-normal tracking-normal text-slate-300">
                    {group.tags.length}
                  </span>
                </div>
                {renderTagGroup(group.tags)}
              </div>
            ))}
          </div>
        ) : (
          <div className="text-sm text-slate-400">{zh('暂无标签', 'No tags')}</div>
        )}
      </div>
      <div className="shrink-0 border-t border-slate-200/70 bg-slate-50/80 px-6 py-4">
        {(actionMessage || batchError) && (
          <div className="mb-3 space-y-1">
            {actionMessage && <div className="text-sm text-emerald-700">{actionMessage}</div>}
            {batchError && <div className="text-sm text-rose-600">{batchError}</div>}
          </div>
        )}
        <div className="flex flex-wrap items-center gap-2">
          <Button
            size="small"
            variant="contained"
            startIcon={<AutoFixHighOutlinedIcon fontSize="small" />}
            onClick={handleOrganizeTags}
            disabled={organizing || editMode || multiSelect}
            sx={compactButtonSx}
          >
            {organizing ? zh('整理中…', 'Organizing...') : zh('自动整理', 'Auto organize')}
          </Button>
          {!editMode && !multiSelect && (
            <Button
              size="small"
              variant="outlined"
              startIcon={<CategoryOutlinedIcon fontSize="small" />}
              onClick={() => {
                setCategoryError('')
                setCategoryManageOpen(true)
              }}
              sx={compactButtonSx}
            >
              {zh('分类管理', 'Manage categories')}
            </Button>
          )}
          {!editMode && !multiSelect && (
            <Button
              size="small"
              variant="outlined"
              startIcon={<AddIcon fontSize="small" />}
              onClick={() => {
                setCreateError('')
                setNewTagName('')
                setCreateOpen(true)
              }}
              sx={compactButtonSx}
            >
              {zh('新增标签', 'New tag')}
            </Button>
          )}
          {!multiSelect && (
            <Button
              size="small"
              variant="outlined"
              startIcon={editMode ? null : <EditOutlinedIcon fontSize="small" />}
              onClick={handleToggleEditMode}
              sx={compactButtonSx}
            >
              {editMode ? zh('退出编辑', 'Exit edit') : zh('编辑', 'Edit')}
            </Button>
          )}
          {!editMode && (
            <Button
              size="small"
              variant="outlined"
              startIcon={multiSelect ? null : <CheckBoxOutlinedIcon fontSize="small" />}
              onClick={() => {
                setMultiSelect((prev) => !prev)
                setSelectedTagIds([])
                setBatchCategoryValue('')
                setEditMode(false)
                setHoverTagId(null)
              }}
              sx={compactButtonSx}
            >
              {multiSelect ? zh('退出多选', 'Exit multi-select') : zh('多选', 'Multi-select')}
            </Button>
          )}
          {multiSelect && (
            <>
              <Button
                size="small"
                variant="contained"
                onClick={() => {
                  setBatchCategoryValue('')
                  setBatchError('')
                  setBatchCategoryOpen(true)
                }}
                disabled={selectedIds.length === 0}
                sx={compactButtonSx}
              >
                {zh('调整分类', 'Move tags')}
              </Button>
              <Button
                size="small"
                variant="outlined"
                startIcon={<SearchOutlinedIcon fontSize="small" />}
                onClick={() => {
                  if (selectedIds.length === 0) return
                  onApplyTagFilter(selectedIds)
                  onClose()
                }}
                disabled={selectedIds.length === 0}
                sx={compactButtonSx}
              >
                {zh('查找视频', 'Find videos')}
              </Button>
            </>
          )}
        </div>
      </div>
      {batchCategoryOpen && (
        <AppModal
          ariaLabel={zh('调整标签分类', 'Move tags to category')}
          className="px-4"
          closeDisabled={assigningCategory}
          contentClassName="w-full max-w-sm rounded-2xl bg-white p-5 shadow-xl"
          onClose={() => {
            setBatchCategoryOpen(false)
            setBatchError('')
          }}
          zIndex={1400}
        >
          <div className="mb-4 flex items-center justify-between">
            <div>
              <h3 className="text-base font-semibold text-slate-900">
                {zh('调整标签分类', 'Move tags to category')}
              </h3>
              <div className="mt-1 text-xs text-slate-400">
                {zh(`已选择 ${selectedIds.length} 个标签`, `${selectedIds.length} tags selected`)}
              </div>
            </div>
            <IconButton
              size="small"
              disabled={assigningCategory}
              onClick={() => {
                setBatchCategoryOpen(false)
                setBatchError('')
              }}
              aria-label={zh('关闭调整分类', 'Close category assignment')}
            >
              <CloseOutlinedIcon fontSize="small" />
            </IconButton>
          </div>
          <TextField
            select
            fullWidth
            size="small"
            value={batchCategoryValue}
            onChange={(event) => setBatchCategoryValue(event.target.value)}
            label={zh('目标分类', 'Target category')}
          >
            <MenuItem value="__uncategorized">{zh('未分类', 'Uncategorized')}</MenuItem>
            {categories.map((category) => (
              <MenuItem key={category.id} value={String(category.id)}>
                {category.name}
              </MenuItem>
            ))}
          </TextField>
          {batchError && <div className="mt-2 text-sm text-rose-600">{batchError}</div>}
          <div className="mt-5 flex justify-end gap-2">
            <Button
              size="small"
              variant="outlined"
              disabled={assigningCategory}
              onClick={() => {
                setBatchCategoryOpen(false)
                setBatchError('')
              }}
              sx={compactButtonSx}
            >
              {zh('取消', 'Cancel')}
            </Button>
            <Button
              size="small"
              variant="contained"
              disabled={assigningCategory || !batchCategoryValue}
              onClick={handleAssignCategory}
              sx={compactButtonSx}
            >
              {assigningCategory ? zh('调整中…', 'Updating...') : zh('确认调整', 'Confirm')}
            </Button>
          </div>
        </AppModal>
      )}
      {categoryManageOpen && (
        <AppModal
          ariaLabel={zh('分类管理', 'Category management')}
          className="px-4"
          closeDisabled={categoryBusyId !== null}
          contentClassName="flex max-h-[75vh] w-full max-w-md flex-col overflow-hidden rounded-2xl bg-white shadow-xl"
          onClose={() => setCategoryManageOpen(false)}
          zIndex={1400}
        >
          <div className="flex shrink-0 items-center justify-between border-b border-slate-200 px-5 py-4">
            <h3 className="text-base font-semibold text-slate-900">
              {zh('分类管理', 'Category management')}
            </h3>
            <IconButton
              size="small"
              disabled={categoryBusyId !== null}
              onClick={() => setCategoryManageOpen(false)}
              aria-label={zh('关闭分类管理', 'Close category management')}
            >
              <CloseOutlinedIcon fontSize="small" />
            </IconButton>
          </div>
          <div className="shrink-0 border-b border-slate-100 p-4">
            <div className="flex gap-2">
              <TextField
                size="small"
                fullWidth
                value={newCategoryName}
                onChange={(event) => setNewCategoryName(event.target.value)}
                placeholder={zh('新分类名称', 'New category name')}
              />
              <Button
                size="small"
                variant="contained"
                disabled={categoryBusyId !== null || !newCategoryName.trim()}
                onClick={async () => {
                  const name = newCategoryName.trim()
                  if (!name) return
                  setCategoryBusyId('__new')
                  setCategoryError('')
                  try {
                    await onCreateCategory?.(name)
                    setNewCategoryName('')
                  } catch (err) {
                    setCategoryError(getErrorMessage(err))
                  } finally {
                    setCategoryBusyId(null)
                  }
                }}
                sx={compactButtonSx}
              >
                {zh('新增', 'Add')}
              </Button>
            </div>
            {categoryError && <div className="mt-2 text-sm text-rose-600">{categoryError}</div>}
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto p-4">
            {categories.length > 0 ? (
              <div className="divide-y divide-slate-100">
                {categories.map((category) => {
                  const editing = categoryEditingId === category.id
                  const busy = categoryBusyId === category.id
                  return (
                    <div key={category.id} className="flex min-h-12 items-center gap-2 py-2">
                      {editing ? (
                        <TextField
                          size="small"
                          fullWidth
                          value={categoryEditingName}
                          onChange={(event) => setCategoryEditingName(event.target.value)}
                        />
                      ) : (
                        <div className="min-w-0 flex-1">
                          <div className="truncate text-sm font-medium text-slate-700">
                            {category.name}
                          </div>
                          <div className="text-xs text-slate-400">
                            {zh(
                              `${categoryUsageCounts.get(category.id) || 0} 个标签`,
                              `${categoryUsageCounts.get(category.id) || 0} tags`
                            )}
                          </div>
                        </div>
                      )}
                      {editing ? (
                        <>
                          <Button
                            size="small"
                            variant="contained"
                            disabled={busy || !categoryEditingName.trim()}
                            onClick={async () => {
                              const name = categoryEditingName.trim()
                              if (!name) return
                              setCategoryBusyId(category.id)
                              setCategoryError('')
                              try {
                                await onRenameCategory?.(category.id, name)
                                setCategoryEditingId(null)
                                setCategoryEditingName('')
                              } catch (err) {
                                setCategoryError(getErrorMessage(err))
                              } finally {
                                setCategoryBusyId(null)
                              }
                            }}
                            sx={compactButtonSx}
                          >
                            {zh('保存', 'Save')}
                          </Button>
                          <Button
                            size="small"
                            disabled={busy}
                            onClick={() => {
                              setCategoryEditingId(null)
                              setCategoryEditingName('')
                            }}
                            sx={compactButtonSx}
                          >
                            {zh('取消', 'Cancel')}
                          </Button>
                        </>
                      ) : (
                        <>
                          <IconButton
                            size="small"
                            disabled={categoryBusyId !== null}
                            onClick={() => {
                              setCategoryEditingId(category.id)
                              setCategoryEditingName(category.name || '')
                              setCategoryError('')
                            }}
                            aria-label={zh('修改分类', 'Rename category')}
                          >
                            <EditOutlinedIcon fontSize="small" />
                          </IconButton>
                          <IconButton
                            size="small"
                            disabled={categoryBusyId !== null}
                            onClick={async () => {
                              if (
                                !window.confirm(
                                  zh(
                                    `确定删除分类“${category.name}”吗？该分类中的标签将变为未分类。`,
                                    `Delete category "${category.name}"? Its tags will become uncategorized.`
                                  )
                                )
                              )
                                return
                              setCategoryBusyId(category.id)
                              setCategoryError('')
                              try {
                                await onDeleteCategory?.(category.id)
                              } catch (err) {
                                setCategoryError(getErrorMessage(err))
                              } finally {
                                setCategoryBusyId(null)
                              }
                            }}
                            aria-label={zh('删除分类', 'Delete category')}
                          >
                            <CloseOutlinedIcon fontSize="small" />
                          </IconButton>
                        </>
                      )}
                    </div>
                  )
                })}
              </div>
            ) : (
              <div className="py-6 text-center text-sm text-slate-400">
                {zh('暂无分类', 'No categories')}
              </div>
            )}
          </div>
        </AppModal>
      )}
      {createOpen && (
        <AppModal
          ariaLabel={zh('新增标签', 'New tag')}
          className="px-4"
          closeDisabled={creating}
          contentClassName="w-full max-w-sm rounded-2xl bg-white p-5 shadow-xl"
          onClose={() => setCreateOpen(false)}
          zIndex={1400}
        >
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-base font-semibold text-slate-900">{zh('新增标签', 'New tag')}</h3>
            <IconButton
              size="small"
              onClick={() => setCreateOpen(false)}
              aria-label={zh('关闭新增标签', 'Close new tag')}
            >
              <CloseOutlinedIcon fontSize="small" />
            </IconButton>
          </div>
          <div className="space-y-3">
            <TextField
              size="small"
              fullWidth
              value={newTagName}
              onChange={(e) => setNewTagName(e.target.value)}
              placeholder={zh('请输入标签名', 'Enter tag name')}
            />
            {createError && <div className="text-sm text-red-600">{createError}</div>}
          </div>
          <div className="mt-4 flex justify-end gap-2">
            <Button
              size="small"
              variant="outlined"
              onClick={() => setCreateOpen(false)}
              sx={compactButtonSx}
            >
              {zh('取消', 'Cancel')}
            </Button>
            <Button
              size="small"
              variant="contained"
              onClick={async () => {
                const trimmed = newTagName.trim()
                if (!trimmed) {
                  setCreateError(zh('标签名不能为空', 'Tag name cannot be empty'))
                  return
                }
                setCreating(true)
                setCreateError('')
                try {
                  await onCreateTag?.(trimmed)
                  setCreateOpen(false)
                  setNewTagName('')
                } catch (err) {
                  setCreateError(getErrorMessage(err))
                } finally {
                  setCreating(false)
                }
              }}
              disabled={creating}
              sx={compactButtonSx}
            >
              {creating ? zh('创建中…', 'Creating...') : zh('创建', 'Create')}
            </Button>
          </div>
        </AppModal>
      )}
      {renameOpen && (
        <AppModal
          ariaLabel={zh('重命名标签', 'Rename tag')}
          className="px-4"
          closeDisabled={renaming}
          contentClassName="w-full max-w-sm rounded-2xl bg-white p-5 shadow-xl"
          onClose={handleCloseRename}
          zIndex={1400}
        >
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-base font-semibold text-slate-900">
              {zh('重命名标签', 'Rename tag')}
            </h3>
            <IconButton
              size="small"
              onClick={handleCloseRename}
              aria-label={zh('关闭重命名', 'Close rename')}
            >
              <CloseOutlinedIcon fontSize="small" />
            </IconButton>
          </div>
          <div className="space-y-3">
            <TextField
              size="small"
              fullWidth
              value={renameTagName}
              onChange={(e) => setRenameTagName(e.target.value)}
              placeholder={zh('请输入新的标签名', 'Enter a new tag name')}
              onKeyDown={(event) => {
                if (event.key === 'Escape') {
                  handleCloseRename()
                }
              }}
            />
            {renameError && <div className="text-sm text-red-600">{renameError}</div>}
          </div>
          <div className="mt-4 flex justify-end gap-2">
            <Button
              size="small"
              variant="outlined"
              onClick={handleCloseRename}
              sx={compactButtonSx}
            >
              {zh('取消', 'Cancel')}
            </Button>
            <Button
              size="small"
              variant="contained"
              onClick={async () => {
                const trimmed = renameTagName.trim()
                if (!trimmed) {
                  setRenameError(zh('标签名不能为空', 'Tag name cannot be empty'))
                  return
                }
                if (!renameTagId) {
                  setRenameError(zh('标签不存在', 'Tag not found'))
                  return
                }
                if (trimmed === renameOriginalName) {
                  handleCloseRename()
                  return
                }
                setRenaming(true)
                setRenameError('')
                try {
                  await onRenameTag?.(renameTagId, trimmed)
                  handleCloseRename()
                } catch (err) {
                  setRenameError(getErrorMessage(err))
                } finally {
                  setRenaming(false)
                }
              }}
              disabled={renaming}
              sx={compactButtonSx}
            >
              {renaming ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
            </Button>
          </div>
        </AppModal>
      )}
    </AppModal>
  )
}
