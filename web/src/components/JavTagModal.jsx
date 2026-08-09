import { useEffect, useMemo, useState } from 'react'
import { Button, IconButton, TextField } from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
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

export default function JavTagModal({
  open,
  onClose,
  tags,
  onApplyTagFilter,
  onCreateTag,
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

  const userTags = useMemo(() => displayTags.filter((tag) => isUserJavTag(tag)), [displayTags])
  const scrapedTags = useMemo(() => displayTags.filter((tag) => !isUserJavTag(tag)), [displayTags])

  const selectedIds = useMemo(() => {
    if (selectedTagIds.length === 0) return []
    const set = new Set(selectedTagIds)
    return displayTags.filter((t) => set.has(t.id)).map((t) => t.id)
  }, [displayTags, selectedTagIds])

  const renderTagGroup = (group) => {
    if (multiSelect) {
      return (
        <TagBar
          tags={group}
          onToggle={handleTagClick}
          multiSelect={multiSelect}
          selectedIds={selectedTagIds}
          variant="neumorphic"
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
          const baseTagClass = 'skeuo-tag--scraped'
          const interactiveTagClass = editMode
            ? showRenameHint
              ? 'skeuo-tag--active'
              : 'skeuo-tag--editing'
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
      contentClassName="mx-4 w-full max-w-4xl overflow-hidden rounded-3xl bg-white shadow-2xl ring-1 ring-slate-200/70"
      onClose={onClose}
    >
      <div className="flex items-center justify-between border-b border-slate-200/70 bg-slate-50/80 px-6 py-4">
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
      <div className="space-y-6 p-6">
        <section className="space-y-4">
          <div className="max-h-[65vh] overflow-y-auto pr-1">
            <div className="space-y-6">
              <div className="space-y-2">
                <div className="text-xs font-semibold uppercase tracking-[0.18em] text-slate-400">
                  {zh('我创建的标签', 'My tags')}
                </div>
                {userTags.length > 0 ? (
                  renderTagGroup(userTags)
                ) : (
                  <div className="text-xs text-slate-400">{zh('暂无', 'None')}</div>
                )}
                {!multiSelect && (
                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      size="small"
                      variant="outlined"
                      startIcon={editMode ? null : <EditOutlinedIcon fontSize="small" />}
                      onClick={handleToggleEditMode}
                      sx={compactButtonSx}
                    >
                      {editMode ? zh('退出编辑', 'Exit edit') : zh('编辑', 'Edit')}
                    </Button>
                    {!editMode && (
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
                  </div>
                )}
              </div>
              <div className="space-y-2">
                <div className="text-xs font-semibold uppercase tracking-[0.18em] text-slate-400">
                  {zh('抓取标签', 'Scraped tags')}
                </div>
                {scrapedTags.length > 0 ? (
                  renderTagGroup(scrapedTags)
                ) : (
                  <div className="text-xs text-slate-400">{zh('暂无', 'None')}</div>
                )}
              </div>
            </div>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              {!editMode && (
                <Button
                  size="small"
                  variant="outlined"
                  startIcon={multiSelect ? null : <CheckBoxOutlinedIcon fontSize="small" />}
                  onClick={() => {
                    setMultiSelect((prev) => !prev)
                    setSelectedTagIds([])
                    setEditMode(false)
                    setHoverTagId(null)
                  }}
                  sx={compactButtonSx}
                >
                  {multiSelect ? zh('退出多选', 'Exit multi-select') : zh('多选', 'Multi-select')}
                </Button>
              )}
              {multiSelect && (
                <Button
                  size="small"
                  variant="contained"
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
              )}
            </div>
          </div>
          {batchError && <div className="text-sm text-rose-600">{batchError}</div>}
        </section>
      </div>
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
