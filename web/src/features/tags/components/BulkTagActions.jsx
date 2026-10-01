import { useState } from 'react'
import { zh } from '@/utils/i18n'

export default function BulkTagActions({ tags, disabled, onAdd, onRemove, onClear }) {
  const [tagId, setTagId] = useState('')
  const parsed = Number(tagId) || 0

  return (
    <div className="flex items-center gap-2">
      <select
        value={tagId}
        onChange={(e) => setTagId(e.target.value)}
        className="rounded border px-2 py-1"
        aria-label={zh('选择标签', 'Choose Tag')}
      >
        <option value="">{zh('选择标签…', 'Choose a tag...')}</option>
        {tags.map((t) => (
          <option key={t.id} value={t.id}>
            {t.name}
          </option>
        ))}
      </select>
      <button
        disabled={disabled || !parsed}
        onClick={() => parsed && onAdd(parsed)}
        className="rounded bg-emerald-600 px-2 py-1 text-white disabled:opacity-50"
      >
        {zh('为选中添加', 'Add to selected')}
      </button>
      <button
        disabled={disabled || !parsed}
        onClick={() => parsed && onRemove(parsed)}
        className="rounded bg-amber-600 px-2 py-1 text-white disabled:opacity-50"
      >
        {zh('从选中移除', 'Remove from selected')}
      </button>
      <button
        disabled={disabled}
        onClick={onClear}
        className="rounded border px-2 py-1 disabled:opacity-50"
      >
        {zh('清除选择', 'Clear selection')}
      </button>
    </div>
  )
}
