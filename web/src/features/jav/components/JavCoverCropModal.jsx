import { useEffect, useMemo, useRef, useState } from 'react'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'
import RestartAltRoundedIcon from '@mui/icons-material/RestartAltRounded'
import SaveRoundedIcon from '@mui/icons-material/SaveRounded'
import AppModal from '@/shared/ui/AppModal'
import { updateJavItem } from '@/features/jav/api'
import { getJavDisplayTitle } from '@/utils/jav'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'
import {
  JAV_COVER_DEFAULT_CROP_LEFT,
  javCoverVisibleRatio,
  normalizeJavCoverCropLeft,
} from '@/utils/javCover'

function clampCropLeft(value, maxCropLeft) {
  const max = Number(maxCropLeft)
  const upper = Number.isFinite(max) && max > 0 ? max : 0
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return 0
  return Math.min(Math.max(parsed, 0), upper)
}

// Same portrait crop editors as the idol page, but for a JAV's own cover.
export default function JavCoverCropModal({ open, item, onClose, onSaved }) {
  const previewRef = useRef(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [cropLeft, setCropLeft] = useState(JAV_COVER_DEFAULT_CROP_LEFT)
  const [dragging, setDragging] = useState(false)
  const [imageFailed, setImageFailed] = useState(false)
  const [imageSize, setImageSize] = useState(null)

  const javId = Number(item?.id)
  const code = String(item?.code || '').trim()
  const coverSrc = code ? `/jav/${encodeURIComponent(code)}/cover` : ''

  useEffect(() => {
    if (!open) return
    setError('')
    setSaving(false)
    setDragging(false)
    setImageFailed(false)
    setImageSize(null)
    setCropLeft(normalizeJavCoverCropLeft(item?.cover_crop_left ?? JAV_COVER_DEFAULT_CROP_LEFT))
  }, [item?.cover_crop_left, item?.id, open])

  const title = useMemo(() => getJavDisplayTitle(item), [item])
  const visibleRatio = javCoverVisibleRatio(imageSize ? imageSize.width / imageSize.height : 0)
  const maxCropLeft = Math.max(0, 1 - visibleRatio)
  const displayCropLeft = Math.min(cropLeft, maxCropLeft)

  const setCropFromClientX = (clientX) => {
    const rect = previewRef.current?.getBoundingClientRect()
    if (!rect || rect.width <= 0) return
    const ratio = (clientX - rect.left) / rect.width
    setCropLeft(clampCropLeft(ratio - visibleRatio / 2, maxCropLeft))
  }

  const handlePointerDown = (event) => {
    if (!coverSrc) return
    event.preventDefault()
    event.currentTarget.setPointerCapture?.(event.pointerId)
    setDragging(true)
    setCropFromClientX(event.clientX)
  }

  const handlePointerMove = (event) => {
    if (!dragging) return
    setCropFromClientX(event.clientX)
  }

  const handlePointerUp = () => {
    setDragging(false)
  }

  const handleSave = async () => {
    if (!Number.isFinite(javId) || javId <= 0 || saving) return
    setSaving(true)
    setError('')
    try {
      const updated = await updateJavItem(javId, { cover_crop_left: displayCropLeft })
      onSaved?.(updated)
      onClose?.()
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  if (!open) return null

  return (
    <AppModal
      ariaLabel={zh('封面起点', 'Cover start')}
      className="px-4 py-6"
      closeDisabled={saving}
      contentClassName="flex max-h-[92vh] w-full max-w-4xl flex-col overflow-hidden rounded-lg bg-white shadow-2xl"
      onClose={onClose}
    >
      <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
        <div className="min-w-0">
          <div className="truncate text-base font-semibold text-slate-950">
            {code || zh('JAV 封面', 'JAV cover')}
          </div>
          <div className="text-xs text-slate-500">
            {zh('简洁模式封面起点', 'Compact cover start')}
          </div>
        </div>
        <button
          type="button"
          className="flex h-8 w-8 items-center justify-center rounded-full text-slate-500 hover:bg-slate-100"
          onClick={onClose}
          aria-label={zh('关闭', 'Close')}
        >
          <CloseOutlinedIcon sx={{ fontSize: 18 }} />
        </button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-4">
        <div className="mx-auto max-w-3xl">
          <div className="mb-2 flex items-center justify-between gap-3">
            <div className="min-w-0">
              {title && title !== code ? (
                <div className="truncate text-sm text-slate-700">{title}</div>
              ) : null}
            </div>
            <div className="text-xs tabular-nums text-slate-500">
              {Math.round(displayCropLeft * 100)}%
            </div>
          </div>

          <div
            ref={previewRef}
            className="relative w-full select-none overflow-hidden rounded bg-slate-100"
            onPointerDown={handlePointerDown}
            onPointerMove={handlePointerMove}
            onPointerUp={handlePointerUp}
            onPointerCancel={handlePointerUp}
          >
            {coverSrc && !imageFailed ? (
              <img
                src={coverSrc}
                alt={code}
                className="block w-full"
                draggable={false}
                onLoad={(event) => {
                  const img = event.currentTarget
                  setImageSize({ width: img.naturalWidth, height: img.naturalHeight })
                }}
                onError={() => {
                  setImageFailed(true)
                  setImageSize(null)
                }}
              />
            ) : (
              <div className="flex aspect-[800/538] items-center justify-center text-sm text-slate-500">
                {zh('封面待下载', 'Cover pending')}
              </div>
            )}
            {coverSrc && !imageFailed ? (
              <div
                className="absolute inset-y-0 border-2 border-white bg-white/10 shadow-[0_0_0_999px_rgba(15,23,42,0.75)]"
                style={{
                  left: `${displayCropLeft * 100}%`,
                  width: `${visibleRatio * 100}%`,
                }}
              >
                <div className="absolute inset-y-0 left-0 w-1 bg-white/80" />
                <div className="absolute inset-y-0 right-0 w-1 bg-white/80" />
              </div>
            ) : null}
          </div>

          <label className="mt-4 flex items-center gap-3 text-sm text-slate-700">
            <span className="w-10 shrink-0">{zh('起点', 'Start')}</span>
            <input
              type="range"
              min="0"
              max={maxCropLeft}
              step="0.001"
              value={displayCropLeft}
              onChange={(event) => setCropLeft(clampCropLeft(event.target.value, maxCropLeft))}
              className="min-w-0 flex-1 accent-slate-900"
            />
          </label>

          {error ? <div className="mt-3 text-sm text-rose-600">{error}</div> : null}
        </div>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2 border-t border-slate-200 px-4 py-3">
        <button
          type="button"
          className="inline-flex items-center gap-1.5 rounded border border-slate-300 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-50"
          onClick={() => {
            setCropLeft(JAV_COVER_DEFAULT_CROP_LEFT)
            setImageFailed(false)
          }}
        >
          <RestartAltRoundedIcon sx={{ fontSize: 17 }} />
          {zh('重置', 'Reset')}
        </button>
        <div className="flex items-center gap-2">
          <button
            type="button"
            className="rounded border border-slate-300 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-50"
            onClick={onClose}
          >
            {zh('取消', 'Cancel')}
          </button>
          <button
            type="button"
            className="inline-flex items-center gap-1.5 rounded bg-slate-900 px-3 py-1.5 text-sm text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
            disabled={saving || !code}
            onClick={handleSave}
          >
            <SaveRoundedIcon sx={{ fontSize: 17 }} />
            {saving ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
          </button>
        </div>
      </div>
    </AppModal>
  )
}
