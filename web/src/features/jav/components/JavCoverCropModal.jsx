import { useState } from 'react'
import AppModal from '@/shared/ui/AppModal'
import { updateJavItem } from '@/features/jav/api'
import { getCoverCropRect, normalizeCoverCropPosition } from '@/features/jav/coverCrop'
import { useStore } from '@/store'
import { zh } from '@/utils/i18n'

export default function JavCoverCropModal({ item, src, onClose }) {
  const [position, setPosition] = useState(() =>
    normalizeCoverCropPosition(item.cover_crop_position)
  )
  const [imageSize, setImageSize] = useState(null)
  const [imageFailed, setImageFailed] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const crop = getCoverCropRect(imageSize, position)
  const canMove = crop && crop.width < 1 && !saving

  const moveCrop = (event) => {
    if (!canMove) return
    const bounds = event.currentTarget.getBoundingClientRect()
    const x = (event.clientX - bounds.left) / bounds.width
    setPosition(normalizeCoverCropPosition((x - crop.width / 2) / (1 - crop.width)))
  }

  const save = async () => {
    setSaving(true)
    setError('')
    try {
      const updated = await updateJavItem(item.id, { cover_crop_position: position })
      useStore.getState().patchJavItem(updated)
      onClose()
    } catch (err) {
      setError(err?.message || zh('保存失败', 'Save failed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <AppModal
      ariaLabel={zh('调整封面', 'Adjust cover')}
      onClose={onClose}
      closeDisabled={saving}
      contentClassName="mx-4 flex max-h-[90vh] w-full max-w-2xl flex-col overflow-hidden rounded-xl bg-white shadow-xl"
    >
      <div className="border-b px-4 py-3 text-base font-semibold">
        {zh('调整封面', 'Adjust cover')} · {item.code}
      </div>
      <div className="min-h-0 overflow-y-auto p-4">
        <p className="mb-3 text-sm text-slate-500">
          {zh(
            '拖动裁切区域或使用滑块调整竖版封面。',
            'Drag the crop area or use the slider to adjust the portrait cover.'
          )}
        </p>
        <div
          className="relative touch-none select-none overflow-hidden rounded bg-slate-100"
          onPointerDown={(event) => {
            if (!canMove || event.button !== 0) return
            event.preventDefault()
            event.currentTarget.setPointerCapture(event.pointerId)
            moveCrop(event)
          }}
          onPointerMove={(event) => {
            if (event.currentTarget.hasPointerCapture(event.pointerId)) moveCrop(event)
          }}
        >
          {imageFailed ? (
            <div className="flex aspect-[800/538] items-center justify-center text-sm text-slate-500">
              {zh('封面加载失败', 'Cover failed to load')}
            </div>
          ) : (
            <img
              src={src}
              alt={item.code}
              draggable={false}
              className="block w-full"
              onLoad={(event) =>
                setImageSize({
                  width: event.currentTarget.naturalWidth,
                  height: event.currentTarget.naturalHeight,
                })
              }
              onError={() => {
                setImageFailed(true)
                setImageSize(null)
              }}
            />
          )}
          {crop ? (
            <div
              className="pointer-events-none absolute border-2 border-white bg-white/10 shadow-[0_0_0_999px_rgba(15,23,42,0.75)]"
              style={{
                left: `${crop.left * 100}%`,
                top: `${crop.top * 100}%`,
                width: `${crop.width * 100}%`,
                height: `${crop.height * 100}%`,
              }}
            />
          ) : null}
        </div>
        <label className="mt-4 flex items-center gap-3 text-sm text-slate-700">
          <span>{zh('位置', 'Position')}</span>
          <input
            type="range"
            min="0"
            max="1"
            step="0.001"
            value={position}
            disabled={!canMove}
            onChange={(event) => setPosition(Number(event.target.value))}
            className="min-w-0 flex-1 accent-slate-900"
          />
          <span className="w-10 text-right tabular-nums">{Math.round(position * 100)}%</span>
        </label>
        {error ? (
          <div role="alert" className="mt-3 text-sm text-rose-600">
            {error}
          </div>
        ) : null}
      </div>
      <div className="flex items-center justify-between gap-2 border-t px-4 py-3">
        <button
          type="button"
          disabled={saving}
          onClick={() => setPosition(1)}
          className="rounded border px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-50 disabled:opacity-50"
        >
          {zh('重置', 'Reset')}
        </button>
        <div className="flex gap-2">
          <button
            type="button"
            disabled={saving}
            onClick={onClose}
            className="rounded border px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-50 disabled:opacity-50"
          >
            {zh('取消', 'Cancel')}
          </button>
          <button
            type="button"
            disabled={saving || !crop}
            onClick={save}
            className="rounded bg-slate-900 px-3 py-1.5 text-sm text-white hover:bg-slate-800 disabled:opacity-50"
          >
            {saving ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
          </button>
        </div>
      </div>
    </AppModal>
  )
}
