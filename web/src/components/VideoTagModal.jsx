import { useEffect, useMemo, useState } from 'react'
import { Button, IconButton, TextField } from '@mui/material'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'

import AppModal from '@/components/AppModal'
import TagBar from '@/components/TagBar'
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

const footerButtonSx = {
  ...compactButtonSx,
  color: '#475569',
  backgroundColor: '#fff',
  borderColor: '#cbd5e1',
  '&:hover': {
    color: '#1e293b',
    backgroundColor: '#f8fafc',
    borderColor: '#94a3b8',
  },
}

export default function VideoTagModal({
  open,
  onClose,
  tags,
  onToggleFilter,
  onCreateTag,
  onDeleteTag,
  onRenameTag,
  onApplyTagFilter,
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

  const handleTagClick = (name) => {
    if (typeof onApplyTagFilter === 'function') {
      onApplyTagFilter([name])
      onClose()
      return
    }
    onToggleFilter?.(name)
    onClose()
  }

  useEffect(() => {
    if (!open) {
      setCreateOpen(false)
      setNewTagName('')
      setCreateError('')
      setCreating(false)
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

  const selectedTags = useMemo(() => {
    if (selectedTagIds.length === 0) return []
    const set = new Set(selectedTagIds)
    return tags.filter((t) => set.has(t.id))
  }, [selectedTagIds, tags])

  const selectedNames = useMemo(() => selectedTags.map((t) => t.name), [selectedTags])
  const handleStartRename = (tag) => {
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
      <div className="tag-management-modal-list min-h-0 flex-1 overflow-y-auto px-6 py-5">
        {multiSelect ? (
          <TagBar
            tags={tags}
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
        ) : tags.length > 0 ? (
          <div className="flex flex-wrap gap-2">
            {tags.map((t) => {
              const count = Number.isFinite(t.count) ? t.count : null
              const showRenameHint = editMode && hoverTagId === t.id
              const showDelete = editMode && hoverTagId === t.id
              const interactiveClass = editMode
                ? showRenameHint
                  ? 'skeuo-tag--active'
                  : 'skeuo-tag--editing'
                : 'skeuo-tag--button'
              return (
                <div
                  key={t.id}
                  className={`skeuo-tag skeuo-tag--main-button ${interactiveClass} ${
                    editMode ? 'skeuo-tag--edit-mode' : ''
                  }`}
                  onMouseEnter={() => {
                    if (editMode) setHoverTagId(t.id)
                  }}
                  onMouseLeave={() => {
                    if (editMode) setHoverTagId((prev) => (prev === t.id ? null : prev))
                  }}
                >
                  <button
                    type="button"
                    className="skeuo-tag-main flex min-w-0 items-center gap-2 text-left"
                    onClick={() => {
                      if (editMode) {
                        handleStartRename(t)
                        return
                      }
                      handleTagClick(t.name)
                    }}
                    title={t.name}
                  >
                    <span className="skeuo-tag-label">{t.name}</span>
                    {!editMode && count !== null && (
                      <span className="skeuo-tag-count">{count}</span>
                    )}
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
                          !window.confirm(
                            zh(`确定删除标签“${t.name}”吗？`, `Delete tag "${t.name}"?`)
                          )
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
        ) : (
          <div className="text-sm text-slate-400">{zh('暂无标签', 'No tags')}</div>
        )}
      </div>
      <div className="shrink-0 border-t border-slate-200/70 bg-slate-50/80 px-6 py-4">
        {batchError && <div className="mb-3 text-sm text-rose-600">{batchError}</div>}
        <div className="flex flex-wrap items-center gap-2">
          {!editMode && !multiSelect && (
            <Button
              size="small"
              variant="outlined"
              onClick={() => {
                setCreateError('')
                setNewTagName('')
                setCreateOpen(true)
              }}
              sx={footerButtonSx}
            >
              {zh('新增标签', 'New tag')}
            </Button>
          )}
          {!multiSelect && (
            <Button
              size="small"
              variant="outlined"
              onClick={handleToggleEditMode}
              sx={footerButtonSx}
            >
              {editMode ? zh('退出编辑', 'Exit edit') : zh('编辑', 'Edit')}
            </Button>
          )}
          {!editMode && (
            <Button
              size="small"
              variant="outlined"
              onClick={() => {
                setBatchError('')
                setMultiSelect((prev) => !prev)
                setSelectedTagIds([])
                setEditMode(false)
                setHoverTagId(null)
              }}
              sx={footerButtonSx}
            >
              {multiSelect ? zh('退出多选', 'Exit multi-select') : zh('多选', 'Multi-select')}
            </Button>
          )}
          {multiSelect && (
            <Button
              size="small"
              variant="outlined"
              onClick={() => {
                if (selectedNames.length === 0) return
                onApplyTagFilter(selectedNames)
                onClose()
              }}
              disabled={selectedNames.length === 0}
              sx={footerButtonSx}
            >
              {zh('查找视频', 'Find videos')}
            </Button>
          )}
        </div>
      </div>
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
                  await onCreateTag(trimmed)
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
    </AppModal>
  )
}
