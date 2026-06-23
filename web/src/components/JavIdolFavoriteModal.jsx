import { useEffect, useMemo, useState } from 'react'
import AddRoundedIcon from '@mui/icons-material/AddRounded'
import CloseRoundedIcon from '@mui/icons-material/CloseRounded'
import { Button, IconButton } from '@mui/material'
import { zh } from '@/utils/i18n'

export default function JavIdolFavoriteModal({
  open,
  idol,
  entityType = 'idol',
  groups,
  selectedIds,
  loading,
  saving,
  error,
  onClose,
  onCreateGroup,
  onSave,
}) {
  const [localSelectedIds, setLocalSelectedIds] = useState([])
  const [newGroupName, setNewGroupName] = useState('')
  const [createError, setCreateError] = useState('')
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    if (!open) {
      setLocalSelectedIds([])
      setNewGroupName('')
      setCreateError('')
      setCreating(false)
      return
    }
    setLocalSelectedIds(cleanIds(selectedIds))
  }, [open, selectedIds])

  const groupList = useMemo(() => {
    return [...(Array.isArray(groups) ? groups : [])].sort((a, b) =>
      String(a?.name || '').localeCompare(String(b?.name || ''))
    )
  }, [groups])

  if (!open) return null

  const selectedSet = new Set(localSelectedIds)
  const entityLabel = favoriteEntityLabel(entityType)
  const itemName = favoriteItemName(entityType, idol)

  const toggleGroup = (id, checked) => {
    const parsed = Number(id)
    if (!Number.isFinite(parsed) || parsed <= 0) return
    setLocalSelectedIds((current) => {
      const next = new Set(current)
      if (checked) {
        next.add(parsed)
      } else {
        next.delete(parsed)
      }
      return Array.from(next).sort((a, b) => a - b)
    })
  }

  const handleCreate = async (event) => {
    event.preventDefault()
    const name = newGroupName.trim()
    if (!name || creating) return
    setCreating(true)
    setCreateError('')
    try {
      const group = await onCreateGroup?.(name)
      const groupID = Number(group?.id)
      if (Number.isFinite(groupID) && groupID > 0) {
        toggleGroup(groupID, true)
      }
      setNewGroupName('')
    } catch (err) {
      setCreateError(err.message || zh('创建收藏夹失败', 'Failed to create favorite group'))
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/35 px-4">
      <div className="flex max-h-[82vh] w-full max-w-md flex-col rounded-lg bg-white shadow-xl">
        <div className="flex items-start justify-between gap-3 border-b px-4 py-3">
          <div className="min-w-0">
            <h2 className="truncate text-base font-semibold text-gray-950">
              {zh('选择收藏夹', 'Choose favorite groups')}
            </h2>
            <div className="mt-0.5 truncate text-sm text-gray-500">{itemName}</div>
          </div>
          <IconButton
            type="button"
            size="small"
            onClick={onClose}
            disabled={saving}
            aria-label={zh('关闭收藏夹选择', 'Close favorite group picker')}
          >
            <CloseRoundedIcon fontSize="small" />
          </IconButton>
        </div>

        <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-4">
          {error ? (
            <div className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
              {error}
            </div>
          ) : null}

          <form onSubmit={handleCreate} className="flex gap-2">
            <input
              value={newGroupName}
              onChange={(event) => setNewGroupName(event.target.value)}
              placeholder={zh(`新建${entityLabel}收藏夹`, `New ${entityLabel} favorite group`)}
              className="min-w-0 flex-1 rounded border border-gray-200 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
              disabled={creating || saving}
            />
            <Button
              type="submit"
              variant="outlined"
              startIcon={<AddRoundedIcon fontSize="small" />}
              disabled={!newGroupName.trim() || creating || saving}
            >
              {zh('新建', 'Create')}
            </Button>
          </form>
          {createError ? <div className="text-sm text-red-600">{createError}</div> : null}

          <div className="rounded border border-gray-200">
            {loading ? (
              <div className="px-3 py-8 text-center text-sm text-gray-500">
                {zh('加载中…', 'Loading...')}
              </div>
            ) : groupList.length === 0 ? (
              <div className="px-3 py-8 text-center text-sm text-gray-500">
                {zh('暂无收藏夹', 'No favorite groups')}
              </div>
            ) : (
              <div className="max-h-72 overflow-y-auto p-1">
                {groupList.map((group) => {
                  const id = Number(group?.id)
                  const checked = selectedSet.has(id)
                  const count = Number.isFinite(group?.count) ? group.count : 0
                  return (
                    <label
                      key={id}
                      className="flex cursor-pointer items-center gap-3 rounded px-3 py-2 hover:bg-gray-50"
                    >
                      <input
                        type="checkbox"
                        checked={checked}
                        disabled={saving}
                        onChange={(event) => toggleGroup(id, event.target.checked)}
                      />
                      <span className="min-w-0 flex-1 truncate text-sm text-gray-900">
                        {group?.name || zh('未命名收藏夹', 'Untitled favorite group')}
                      </span>
                      <span className="shrink-0 text-xs text-gray-500">{count}</span>
                    </label>
                  )
                })}
              </div>
            )}
          </div>
        </div>

        <div className="flex justify-end gap-2 border-t px-4 py-3">
          <Button variant="outlined" onClick={onClose} disabled={saving}>
            {zh('取消', 'Cancel')}
          </Button>
          <Button
            variant="contained"
            onClick={() => onSave?.(localSelectedIds)}
            disabled={loading || saving}
          >
            {saving ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
          </Button>
        </div>
      </div>
    </div>
  )
}

function cleanIds(ids) {
  return Array.from(
    new Set((ids || []).map((id) => Number(id)).filter((id) => Number.isFinite(id) && id > 0))
  ).sort((a, b) => a - b)
}

function favoriteEntityLabel(entityType) {
  switch (entityType) {
    case 'jav':
      return zh('作品', 'JAV')
    case 'studio':
      return zh('片商', 'studio')
    case 'series':
      return zh('系列', 'series')
    case 'idol':
    default:
      return zh('女优', 'idol')
  }
}

function favoriteItemName(entityType, item) {
  const name = String(item?.name || '').trim()
  if (name) return name
  if (entityType === 'jav') {
    return (
      [item?.code, item?.title || item?.title_en].filter(Boolean).join(' ') ||
      zh('未知作品', 'Unknown JAV')
    )
  }
  if (entityType === 'studio') return zh('未知片商', 'Unknown studio')
  if (entityType === 'series') return zh('未知系列', 'Unknown series')
  return zh('未知女优', 'Unknown idol')
}
