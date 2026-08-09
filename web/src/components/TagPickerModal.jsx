import AppModal from '@/components/AppModal'
import { zh } from '@/utils/i18n'

export default function TagPickerModal({
  open,
  tags,
  selectedIds,
  onToggleChoice,
  onClose,
  onSave,
  saveDisabled,
}) {
  if (!open) return null

  const list = Array.isArray(tags) ? tags : []
  const selected = Array.isArray(selectedIds) ? selectedIds : []

  return (
    <AppModal
      ariaLabel={zh('选择标签', 'Choose Tags')}
      className="px-4"
      contentClassName="w-full max-w-xs rounded-lg bg-white p-4 shadow-xl"
      onClose={onClose}
    >
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-base font-semibold">{zh('选择标签', 'Choose Tags')}</h2>
        <button
          onClick={onClose}
          className="rounded px-2 py-1 text-gray-500 hover:bg-gray-100"
          aria-label={zh('关闭标签选择', 'Close Tag Picker')}
        >
          ✕
        </button>
      </div>
      <div className="max-h-64 space-y-1 overflow-y-auto rounded border p-2">
        {list.map((tag) => {
          const checked = selected.includes(String(tag.id))
          return (
            <label
              key={`${tag.id}-${tag.provider || 0}`}
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
        })}
      </div>
      <div className="mt-3 flex justify-end gap-2">
        <button onClick={onClose} className="rounded border px-3 py-1.5 text-sm hover:bg-gray-50">
          {zh('取消', 'Cancel')}
        </button>
        <button
          onClick={onSave}
          className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
          disabled={saveDisabled}
        >
          {zh('保存', 'Save')}
        </button>
      </div>
    </AppModal>
  )
}
