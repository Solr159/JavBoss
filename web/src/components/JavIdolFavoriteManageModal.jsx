import { useEffect, useRef, useState } from 'react'
import CloseRoundedIcon from '@mui/icons-material/CloseRounded'
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded'
import EditRoundedIcon from '@mui/icons-material/EditRounded'
import MenuRoundedIcon from '@mui/icons-material/MenuRounded'
import { Button, IconButton } from '@mui/material'
import { zh } from '@/utils/i18n'

export default function JavIdolFavoriteManageModal({
  open,
  entityType = 'idol',
  groups,
  selectedGroupId,
  initialEditGroupId = null,
  loading,
  onClose,
  onCreateGroup,
  onReorderGroups,
  onRenameGroup,
  onDeleteGroup,
  onLoadGroupIdols,
  onReorderGroupIdols,
  onRemoveGroupIdols,
}) {
  const [localGroups, setLocalGroups] = useState([])
  const [editingGroup, setEditingGroup] = useState(null)
  const [creatingOpen, setCreatingOpen] = useState(false)
  const [newGroupName, setNewGroupName] = useState('')
  const [creating, setCreating] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const directEditMode = (() => {
    const id = Number(initialEditGroupId)
    return Number.isFinite(id) && id > 0
  })()
  const labels = favoriteManageLabels(entityType)

  useEffect(() => {
    if (!open) {
      setLocalGroups([])
      setEditingGroup(null)
      setCreatingOpen(false)
      setNewGroupName('')
      setCreating(false)
      setSaving(false)
      setError('')
      return
    }
    const nextGroups = normalizeGroups(groups)
    setLocalGroups(nextGroups)
    const editId = Number(initialEditGroupId)
    if (Number.isFinite(editId) && editId > 0) {
      setEditingGroup(nextGroups.find((group) => Number(group?.id) === editId) || null)
    }
  }, [groups, initialEditGroupId, open])

  if (!open) return null

  const commitGroupOrder = async (nextGroups) => {
    if (!Array.isArray(nextGroups) || nextGroups.length === 0) return
    setSaving(true)
    setError('')
    try {
      await onReorderGroups?.(nextGroups.map((group) => Number(group.id)))
    } catch (err) {
      setError(err.message || zh('保存收藏夹顺序失败', 'Failed to save favorite order'))
    } finally {
      setSaving(false)
    }
  }

  const handleRename = async (groupId, name) => {
    await onRenameGroup?.(groupId, name)
    setLocalGroups((current) =>
      current.map((group) => (Number(group.id) === Number(groupId) ? { ...group, name } : group))
    )
  }

  const handleDelete = async (groupId) => {
    await onDeleteGroup?.(groupId)
    setLocalGroups((current) => current.filter((group) => Number(group.id) !== Number(groupId)))
    setEditingGroup(null)
    if (directEditMode) onClose?.()
  }

  const handleCreate = async (event) => {
    event.preventDefault()
    const name = newGroupName.trim()
    if (!name || creating) return
    setCreating(true)
    setError('')
    try {
      const group = await onCreateGroup?.(name)
      if (group?.id) {
        setLocalGroups((current) => {
          const exists = current.some((item) => Number(item.id) === Number(group.id))
          return exists ? current : [...current, { ...group, count: group.count || 0 }]
        })
      }
      setNewGroupName('')
      setCreatingOpen(false)
    } catch (err) {
      setError(err.message || zh('创建收藏夹失败', 'Failed to create favorite'))
    } finally {
      setCreating(false)
    }
  }

  return (
    <>
      {!directEditMode ? (
        <div className="pointer-events-none fixed inset-0 z-50 flex items-center justify-center px-4">
          <div className="pointer-events-auto flex max-h-[82vh] w-full max-w-lg flex-col rounded-lg bg-white shadow-xl">
            <div className="flex items-center justify-between border-b px-4 py-3">
              <h2 className="text-base font-semibold text-gray-950">{labels.manageTitle}</h2>
              <IconButton
                type="button"
                size="small"
                onClick={onClose}
                disabled={saving}
                aria-label={zh('关闭收藏夹管理', 'Close favorite manager')}
              >
                <CloseRoundedIcon fontSize="small" />
              </IconButton>
            </div>

            <div className="min-h-0 flex-1 overflow-y-auto p-4">
              {error ? (
                <div className="mb-3 rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                  {error}
                </div>
              ) : null}

              <GroupOrderList
                groups={localGroups}
                selectedGroupId={selectedGroupId}
                emptyText={loading ? zh('加载中…', 'Loading...') : zh('暂无收藏夹', 'No favorites')}
                labels={labels}
                onReorder={setLocalGroups}
                onReorderCommit={commitGroupOrder}
                onEdit={(group) => setEditingGroup(group)}
              />
            </div>

            <div className="flex justify-end gap-2 border-t px-4 py-3">
              <Button variant="outlined" onClick={() => setCreatingOpen(true)} disabled={saving}>
                {zh('新增收藏夹', 'Add favorite')}
              </Button>
              <Button variant="outlined" onClick={onClose} disabled={saving}>
                {zh('关闭', 'Close')}
              </Button>
            </div>
          </div>
        </div>
      ) : null}

      <FavoriteGroupEditModal
        group={editingGroup}
        directMode={directEditMode}
        onClose={directEditMode ? onClose : () => setEditingGroup(null)}
        onRename={handleRename}
        onDelete={handleDelete}
        onLoadGroupIdols={onLoadGroupIdols}
        onReorderGroupIdols={onReorderGroupIdols}
        onRemoveGroupIdols={onRemoveGroupIdols}
        labels={labels}
      />

      <CreateGroupModal
        open={creatingOpen}
        name={newGroupName}
        creating={creating}
        onNameChange={setNewGroupName}
        onClose={() => {
          if (creating) return
          setCreatingOpen(false)
          setNewGroupName('')
        }}
        onSubmit={handleCreate}
      />
    </>
  )
}

function CreateGroupModal({ open, name, creating, onNameChange, onClose, onSubmit }) {
  if (!open) return null

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 px-4">
      <form onSubmit={onSubmit} className="w-full max-w-sm rounded-lg bg-white shadow-xl">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <h2 className="text-base font-semibold text-gray-950">
            {zh('新增收藏夹', 'Add favorite')}
          </h2>
          <IconButton
            type="button"
            size="small"
            onClick={onClose}
            disabled={creating}
            aria-label={zh('关闭新增收藏夹', 'Close add favorite')}
          >
            <CloseRoundedIcon fontSize="small" />
          </IconButton>
        </div>
        <div className="p-4">
          <input
            value={name}
            onChange={(event) => onNameChange(event.target.value)}
            placeholder={zh('收藏夹名称', 'Favorite name')}
            className="h-9 w-full rounded border border-gray-200 px-3 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
            disabled={creating}
          />
        </div>
        <div className="flex justify-end gap-2 border-t px-4 py-3">
          <Button variant="outlined" onClick={onClose} disabled={creating}>
            {zh('取消', 'Cancel')}
          </Button>
          <Button type="submit" variant="contained" disabled={!name.trim() || creating}>
            {creating ? zh('添加中…', 'Adding...') : zh('添加', 'Add')}
          </Button>
        </div>
      </form>
    </div>
  )
}

function GroupOrderList({
  groups,
  selectedGroupId,
  emptyText,
  onReorder,
  onReorderCommit,
  onEdit,
  labels,
}) {
  if (!groups.length) {
    return (
      <div className="rounded border border-dashed border-gray-200 px-3 py-8 text-center text-sm text-gray-500">
        {emptyText}
      </div>
    )
  }

  return (
    <SortableList
      items={groups}
      onReorder={onReorder}
      onReorderCommit={onReorderCommit}
      getLabel={(group) => group.name}
      getMeta={(group) => labels.groupMeta(group.count || 0)}
      isActive={(group) => Number(group.id) === Number(selectedGroupId)}
      renderLeading={(group) => (
        <IconButton
          type="button"
          size="small"
          onClick={() => onEdit(group)}
          aria-label={zh('编辑收藏夹', 'Edit favorite')}
          sx={{ width: 28, height: 28 }}
        >
          <EditRoundedIcon sx={{ fontSize: 16 }} />
        </IconButton>
      )}
    />
  )
}

function FavoriteGroupEditModal({
  group,
  directMode = false,
  onClose,
  onRename,
  onDelete,
  onLoadGroupIdols,
  onReorderGroupIdols,
  onRemoveGroupIdols,
  labels = favoriteManageLabels('idol'),
}) {
  const [groupName, setGroupName] = useState('')
  const [idols, setIdols] = useState([])
  const [selectedIds, setSelectedIds] = useState([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const groupId = Number(group?.id) || null

  useEffect(() => {
    if (!groupId) {
      setGroupName('')
      setIdols([])
      setSelectedIds([])
      setLoading(false)
      setSaving(false)
      setError('')
      return
    }
    setGroupName(String(group?.name || ''))
    setIdols([])
    setSelectedIds([])
    setLoading(true)
    setError('')
    let cancelled = false
    onLoadGroupIdols?.(groupId)
      .then((items) => {
        if (!cancelled) setIdols(Array.isArray(items) ? items : [])
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err.message || labels.loadItemsError)
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [group, groupId, labels.loadItemsError, onLoadGroupIdols])

  if (!groupId) return null

  const saveRename = async () => {
    const name = groupName.trim()
    if (!name) return
    setSaving(true)
    setError('')
    try {
      await onRename?.(groupId, name)
    } catch (err) {
      setError(err.message || zh('重命名收藏夹失败', 'Failed to rename favorite'))
    } finally {
      setSaving(false)
    }
  }

  const deleteGroup = async () => {
    if (!window.confirm(zh(`删除收藏夹“${group.name}”？`, `Delete favorite "${group.name}"?`))) {
      return
    }
    setSaving(true)
    setError('')
    try {
      await onDelete?.(groupId)
    } catch (err) {
      setError(err.message || zh('删除收藏夹失败', 'Failed to delete favorite'))
    } finally {
      setSaving(false)
    }
  }

  const commitIdolOrder = async (nextIdols) => {
    if (!Array.isArray(nextIdols) || nextIdols.length === 0) return
    setSaving(true)
    setError('')
    try {
      await onReorderGroupIdols?.(
        groupId,
        nextIdols.map((idol) => Number(idol.id))
      )
    } catch (err) {
      setError(err.message || labels.saveOrderError)
    } finally {
      setSaving(false)
    }
  }

  const toggleSelected = (idolId, checked) => {
    const id = Number(idolId)
    if (!Number.isFinite(id) || id <= 0) return
    setSelectedIds((current) => {
      const next = new Set(current)
      if (checked) next.add(id)
      else next.delete(id)
      return Array.from(next)
    })
  }

  const removeSelected = async () => {
    if (selectedIds.length === 0) return
    if (
      !window.confirm(
        zh(
          `将选中的 ${selectedIds.length} 个${labels.itemNameZh}移出收藏夹？`,
          `Remove ${selectedIds.length} selected ${labels.itemNameEn} from favorite?`
        )
      )
    ) {
      return
    }
    setSaving(true)
    setError('')
    try {
      await onRemoveGroupIdols?.(groupId, selectedIds)
      const removed = new Set(selectedIds)
      setIdols((current) => current.filter((idol) => !removed.has(Number(idol.id))))
      setSelectedIds([])
    } catch (err) {
      setError(err.message || labels.removeItemsError)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      className={`fixed inset-0 z-[60] flex items-center justify-center px-4 ${
        directMode ? 'pointer-events-none' : 'bg-black/40'
      }`}
    >
      <div className="pointer-events-auto flex max-h-[86vh] w-full max-w-xl flex-col rounded-lg bg-white shadow-xl">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <h2 className="min-w-0 truncate text-base font-semibold text-gray-950">
            {zh('编辑收藏夹', 'Edit favorite')}
          </h2>
          <IconButton
            type="button"
            size="small"
            onClick={onClose}
            disabled={saving}
            aria-label={zh('关闭编辑收藏夹', 'Close favorite editor')}
          >
            <CloseRoundedIcon fontSize="small" />
          </IconButton>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          {error ? (
            <div className="mb-3 rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
              {error}
            </div>
          ) : null}

          <div className="mb-4 flex flex-wrap items-center gap-2">
            <input
              value={groupName}
              onChange={(event) => setGroupName(event.target.value)}
              className="h-8 min-w-0 flex-1 rounded border border-gray-200 px-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
              disabled={saving}
            />
            <Button
              variant="outlined"
              size="small"
              onClick={saveRename}
              disabled={saving || !groupName.trim()}
            >
              {zh('重命名', 'Rename')}
            </Button>
            <IconButton
              type="button"
              size="small"
              onClick={deleteGroup}
              disabled={saving}
              aria-label={zh('删除收藏夹', 'Delete favorite')}
            >
              <DeleteOutlineRoundedIcon fontSize="small" />
            </IconButton>
          </div>

          <IdolOrderList
            idols={idols}
            loading={loading}
            selectedIds={selectedIds}
            onToggleSelected={toggleSelected}
            onReorder={setIdols}
            onReorderCommit={commitIdolOrder}
            labels={labels}
          />
        </div>

        <div className="flex justify-end gap-2 border-t px-4 py-3">
          <Button
            variant="outlined"
            color="error"
            onClick={removeSelected}
            disabled={saving || loading || selectedIds.length === 0}
          >
            {zh('移除', 'Remove')}
          </Button>
          <Button variant="outlined" onClick={onClose} disabled={saving}>
            {zh('关闭', 'Close')}
          </Button>
        </div>
      </div>
    </div>
  )
}

function IdolOrderList({
  idols,
  loading,
  selectedIds = [],
  onToggleSelected,
  onReorder,
  onReorderCommit,
  labels = favoriteManageLabels('idol'),
}) {
  if (!idols.length) {
    return (
      <div className="rounded border border-dashed border-gray-200 px-3 py-8 text-center text-sm text-gray-500">
        {loading ? zh('加载中…', 'Loading...') : labels.emptyItemsText}
      </div>
    )
  }
  return (
    <SortableList
      items={idols}
      onReorder={onReorder}
      onReorderCommit={onReorderCommit}
      getLabel={(idol) => favoriteItemLabel(labels.entityType, idol)}
      getMeta={(idol) => labels.itemMeta(idol)}
      renderLeading={(idol) => (
        <input
          type="checkbox"
          checked={selectedIds.includes(Number(idol.id))}
          onChange={(event) => onToggleSelected?.(idol.id, event.target.checked)}
          className="h-4 w-4 shrink-0 accent-blue-600"
          aria-label={labels.selectAria}
        />
      )}
    />
  )
}

function SortableList({
  items,
  onReorder,
  onReorderCommit,
  getLabel,
  getMeta,
  isActive = () => false,
  renderLeading = null,
}) {
  const containerRef = useRef(null)
  const rowRefs = useRef(new Map())
  const draftItemsRef = useRef(null)
  const [drag, setDrag] = useState(null)

  useEffect(() => {
    if (!drag) return undefined

    const handlePointerMove = (event) => {
      event.preventDefault()
      setDrag((current) =>
        current
          ? {
              ...current,
              pointerX: event.clientX,
              pointerY: event.clientY,
            }
          : current
      )

      const nextIndex = calculateDropIndex(items, rowRefs.current, drag.id, event.clientY)
      const currentIndex = items.findIndex((item) => String(item.id) === drag.id)
      if (nextIndex >= 0 && currentIndex >= 0 && nextIndex !== currentIndex) {
        const nextItems = moveItemToIndex(items, drag.id, nextIndex)
        draftItemsRef.current = nextItems
        onReorder?.(nextItems)
      }
    }

    const handlePointerUp = () => {
      const nextItems = draftItemsRef.current
      draftItemsRef.current = null
      setDrag(null)
      if (nextItems) onReorderCommit?.(nextItems)
    }

    window.addEventListener('pointermove', handlePointerMove, { passive: false })
    window.addEventListener('pointerup', handlePointerUp)
    window.addEventListener('pointercancel', handlePointerUp)
    return () => {
      window.removeEventListener('pointermove', handlePointerMove)
      window.removeEventListener('pointerup', handlePointerUp)
      window.removeEventListener('pointercancel', handlePointerUp)
    }
  }, [drag, items, onReorder, onReorderCommit])

  const startDrag = (event, item) => {
    if (event.button !== 0) return
    const id = String(item.id)
    const row = rowRefs.current.get(id)
    if (!row) return
    const rect = row.getBoundingClientRect()
    event.preventDefault()
    setDrag({
      id,
      pointerX: event.clientX,
      pointerY: event.clientY,
      offsetX: event.clientX - rect.left,
      offsetY: event.clientY - rect.top,
      left: rect.left,
      top: rect.top,
      width: rect.width,
      height: rect.height,
    })
  }

  return (
    <div
      ref={containerRef}
      className={`rounded border border-gray-200 p-1 ${drag ? 'select-none' : ''}`}
    >
      {items.map((item) => {
        const id = String(item.id)
        const isDragging = drag?.id === id
        return (
          <SortableRow
            key={id}
            refCallback={(node) => {
              if (node) rowRefs.current.set(id, node)
              else rowRefs.current.delete(id)
            }}
            item={item}
            label={getLabel(item)}
            meta={getMeta?.(item)}
            leading={renderLeading?.(item)}
            active={isActive(item)}
            dragging={isDragging}
            onHandlePointerDown={(event) => startDrag(event, item)}
          />
        )
      })}
      {drag ? (
        <div
          className="pointer-events-none fixed z-[80]"
          style={{
            left: drag.pointerX - drag.offsetX,
            top: drag.pointerY - drag.offsetY,
            width: drag.width,
            height: drag.height,
          }}
        >
          {(() => {
            const item = items.find((candidate) => String(candidate.id) === drag.id)
            if (!item) return null
            return (
              <SortableRow
                item={item}
                label={getLabel(item)}
                meta={getMeta?.(item)}
                leading={renderLeading?.(item)}
                active={isActive(item)}
                dragging={false}
                floating
              />
            )
          })()}
        </div>
      ) : null}
    </div>
  )
}

function SortableRow({
  item,
  label,
  meta,
  leading = null,
  active = false,
  dragging = false,
  floating = false,
  refCallback,
  onHandlePointerDown,
}) {
  return (
    <div
      ref={refCallback}
      className={`mb-1 flex items-center gap-2 rounded border px-2 py-1.5 last:mb-0 ${
        floating ? 'shadow-lg ring-1 ring-blue-200' : 'transition-[background-color,opacity]'
      } ${dragging ? 'opacity-0' : 'opacity-100'} ${
        active ? 'border-blue-200 bg-blue-50' : 'border-transparent bg-gray-50'
      }`}
      data-sortable-id={item.id}
    >
      {leading}
      <span className="min-w-0 flex-1 truncate text-sm text-gray-900">{label}</span>
      <span className="shrink-0 text-xs text-gray-500">{meta}</span>
      <button
        type="button"
        onPointerDown={onHandlePointerDown}
        className="inline-flex h-7 w-7 shrink-0 cursor-grab touch-none items-center justify-center rounded border border-gray-200 bg-white text-gray-500 active:cursor-grabbing"
        aria-label={zh('拖动排序', 'Drag to reorder')}
        title={zh('拖动排序', 'Drag to reorder')}
      >
        <MenuRoundedIcon sx={{ fontSize: 17 }} />
      </button>
    </div>
  )
}

function calculateDropIndex(items, refs, draggingId, pointerY) {
  let nextIndex = 0
  for (const item of items) {
    const id = String(item.id)
    if (id === draggingId) continue
    const row = refs.get(id)
    if (!row) continue
    const rect = row.getBoundingClientRect()
    if (pointerY > rect.top + rect.height / 2) {
      nextIndex += 1
    }
  }
  return Math.min(nextIndex, items.length - 1)
}

function moveItemToIndex(items, sourceId, targetIndex) {
  const next = [...items]
  const from = next.findIndex((item) => String(item.id) === String(sourceId))
  if (from < 0) return next
  const [item] = next.splice(from, 1)
  const safeIndex = Math.max(0, Math.min(targetIndex, next.length))
  next.splice(safeIndex, 0, item)
  return next
}

function normalizeGroups(groups) {
  return [...(Array.isArray(groups) ? groups : [])].sort((a, b) => {
    const orderA = Number(a?.sort_order) || 0
    const orderB = Number(b?.sort_order) || 0
    if (orderA !== orderB) return orderA - orderB
    return String(a?.name || '').localeCompare(String(b?.name || ''))
  })
}

function favoriteManageLabels(entityType) {
  switch (entityType) {
    case 'jav':
      return {
        entityType,
        itemNameZh: '作品',
        itemNameEn: 'JAV items',
        manageTitle: zh('管理作品收藏夹', 'Manage JAV favorites'),
        loadItemsError: zh('加载收藏夹作品失败', 'Failed to load favorite group JAV'),
        saveOrderError: zh('保存作品顺序失败', 'Failed to save JAV order'),
        removeItemsError: zh('批量移除作品失败', 'Failed to remove selected JAV'),
        emptyItemsText: zh('该收藏夹暂无作品', 'No JAV in this favorite'),
        selectAria: zh('选择作品', 'Select JAV'),
        groupMeta: (count) => zh(`${count} 部`, `${count} JAV`),
        itemMeta: () => '',
      }
    case 'studio':
      return {
        entityType,
        itemNameZh: '片商',
        itemNameEn: 'studios',
        manageTitle: zh('管理片商收藏夹', 'Manage studio favorites'),
        loadItemsError: zh('加载收藏夹片商失败', 'Failed to load favorite group studios'),
        saveOrderError: zh('保存片商顺序失败', 'Failed to save studio order'),
        removeItemsError: zh('批量移除片商失败', 'Failed to remove selected studios'),
        emptyItemsText: zh('该收藏夹暂无片商', 'No studios in this favorite'),
        selectAria: zh('选择片商', 'Select studio'),
        groupMeta: (count) => zh(`${count} 个`, `${count} studios`),
        itemMeta: (item) =>
          zh(`${Number(item?.work_count) || 0} 部`, `${Number(item?.work_count) || 0} works`),
      }
    case 'series':
      return {
        entityType,
        itemNameZh: '系列',
        itemNameEn: 'series',
        manageTitle: zh('管理系列收藏夹', 'Manage series favorites'),
        loadItemsError: zh('加载收藏夹系列失败', 'Failed to load favorite group series'),
        saveOrderError: zh('保存系列顺序失败', 'Failed to save series order'),
        removeItemsError: zh('批量移除系列失败', 'Failed to remove selected series'),
        emptyItemsText: zh('该收藏夹暂无系列', 'No series in this favorite'),
        selectAria: zh('选择系列', 'Select series'),
        groupMeta: (count) => zh(`${count} 个`, `${count} series`),
        itemMeta: (item) =>
          zh(`${Number(item?.work_count) || 0} 部`, `${Number(item?.work_count) || 0} works`),
      }
    case 'idol':
    default:
      return {
        entityType: 'idol',
        itemNameZh: '女优',
        itemNameEn: 'idols',
        manageTitle: zh('管理女优收藏夹', 'Manage idol favorites'),
        loadItemsError: zh('加载收藏夹女优失败', 'Failed to load favorite group idols'),
        saveOrderError: zh('保存女优顺序失败', 'Failed to save idol order'),
        removeItemsError: zh('批量移除女优失败', 'Failed to remove selected idols'),
        emptyItemsText: zh('该收藏夹暂无女优', 'No idols in this favorite'),
        selectAria: zh('选择女优', 'Select idol'),
        groupMeta: (count) => zh(`${count} 位`, `${count} idols`),
        itemMeta: (item) =>
          zh(`${Number(item?.work_count) || 0} 部`, `${Number(item?.work_count) || 0} works`),
      }
  }
}

function favoriteItemLabel(entityType, item) {
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
