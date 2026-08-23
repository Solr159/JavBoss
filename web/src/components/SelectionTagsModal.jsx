import { Button } from '@mui/material'
import AppModal from '@/components/AppModal'
import { zh } from '@/utils/i18n'

export default function SelectionTagsModal({
  open,
  onClose,
  tags,
  action = 'add',
  selectedChoices,
  onToggleChoice,
  onConfirm,
  confirmDisabled,
}) {
  if (!open) return null

  const list = Array.isArray(tags) ? tags : []
  const selected = Array.isArray(selectedChoices) ? selectedChoices : []
  const isRemove = action === 'remove'
  const title = isRemove ? zh('移除标签', 'Remove Tags') : zh('添加标签', 'Add Tags')
  const confirmLabel = isRemove ? zh('移除', 'Remove') : zh('添加', 'Add')

  return (
    <AppModal
      ariaLabel={title}
      className="px-4"
      contentClassName="w-full max-w-xs rounded-lg bg-white p-4 shadow-xl"
      onClose={onClose}
      zIndex={1600}
    >
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-base font-semibold">{title}</h2>
        <button
          onClick={onClose}
          className="rounded px-2 py-1 text-gray-500 hover:bg-gray-100"
          aria-label={zh('关闭标签选择', 'Close Tag Picker')}
        >
          ✕
        </button>
      </div>
      <div className="max-h-64 space-y-1 overflow-y-auto rounded border p-2">
        {list.length === 0 ? (
          <div className="px-2 py-1 text-sm text-gray-500">{zh('暂无标签', 'No tags')}</div>
        ) : (
          list.map((tag) => {
            const checked = selected.includes(String(tag.id))
            return (
              <label
                key={tag.id}
                className="flex cursor-pointer items-center gap-2 rounded px-2 py-1 hover:bg-gray-50"
              >
                <input
                  type="checkbox"
                  checked={checked}
                  onChange={(e) => onToggleChoice?.(tag.id, e.target.checked)}
                />
                <span className="text-sm text-gray-800">{tag.name}</span>
              </label>
            )
          })
        )}
      </div>
      <div className="mt-3 flex justify-end gap-2">
        <Button variant="outlined" size="small" onClick={onClose}>
          {zh('取消', 'Cancel')}
        </Button>
        <Button variant="contained" size="small" onClick={onConfirm} disabled={confirmDisabled}>
          {confirmLabel}
        </Button>
      </div>
    </AppModal>
  )
}
