import { useEffect, useMemo, useRef, useState } from 'react'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'
import ImageRoundedIcon from '@mui/icons-material/ImageRounded'
import RestartAltRoundedIcon from '@mui/icons-material/RestartAltRounded'
import SaveRoundedIcon from '@mui/icons-material/SaveRounded'
import { fetchJavIdolCoverOptions, updateJavIdolCover } from '@/api'
import { getJavDisplayTitle } from '@/utils/jav'
import { zh } from '@/utils/i18n'

export const IDOL_COVER_VISIBLE_RATIO = 0.47
export const IDOL_COVER_DEFAULT_CROP_LEFT = 1 - IDOL_COVER_VISIBLE_RATIO

export function normalizeIdolCoverCropLeft(value) {
  const max = IDOL_COVER_DEFAULT_CROP_LEFT
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return max
  return Math.min(Math.max(parsed, 0), max)
}

export function idolCoverBackgroundPosition(cropLeft) {
  const max = IDOL_COVER_DEFAULT_CROP_LEFT
  if (max <= 0) return '50% 50%'
  const percent = (normalizeIdolCoverCropLeft(cropLeft) / max) * 100
  return `${percent}% 50%`
}

export default function JavIdolCoverModal({
  open,
  item,
  directoryIds = [],
  javMetadataLanguage = 'zh',
  onClose,
  onSaved,
}) {
  const previewRef = useRef(null)
  const [options, setOptions] = useState([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [selectedJavId, setSelectedJavId] = useState(0)
  const [cropLeft, setCropLeft] = useState(IDOL_COVER_DEFAULT_CROP_LEFT)
  const [dragging, setDragging] = useState(false)
  const [imageFailed, setImageFailed] = useState(false)

  const idolId = Number(item?.id)
  const directoryKey = (directoryIds || []).join(',')

  useEffect(() => {
    if (!open || !Number.isFinite(idolId) || idolId <= 0) return undefined
    let cancelled = false
    setLoading(true)
    setError('')
    setOptions([])
    setSelectedJavId(Number(item?.cover_jav_id) || 0)
    setCropLeft(normalizeIdolCoverCropLeft(item?.cover_crop_left ?? IDOL_COVER_DEFAULT_CROP_LEFT))
    fetchJavIdolCoverOptions(idolId, { directoryIds })
      .then((items) => {
        if (cancelled) return
        setOptions(items)
        setSelectedJavId((current) => {
          if (current && items.some((option) => Number(option.id) === current)) return current
          const coverCode = String(item?.cover_code || item?.sample_code || '').trim()
          const matched = items.find((option) => String(option?.code || '').trim() === coverCode)
          return matched ? Number(matched.id) : 0
        })
      })
      .catch((err) => {
        if (!cancelled)
          setError(err.message || zh('加载封面作品失败', 'Failed to load cover works'))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [
    directoryIds,
    directoryKey,
    idolId,
    item?.cover_code,
    item?.cover_crop_left,
    item?.cover_jav_id,
    item?.sample_code,
    open,
  ])

  useEffect(() => {
    if (!open) {
      setDragging(false)
      setImageFailed(false)
    }
  }, [open])

  const selectedOption = useMemo(
    () => options.find((option) => Number(option?.id) === Number(selectedJavId)) || null,
    [options, selectedJavId]
  )
  const previewCode = String(selectedOption?.code || item?.sample_code || '').trim()
  const coverSrc = previewCode ? `/jav/${encodeURIComponent(previewCode)}/cover` : ''
  const title = selectedOption ? getJavDisplayTitle(selectedOption, javMetadataLanguage) : ''

  const setCropFromClientX = (clientX) => {
    const rect = previewRef.current?.getBoundingClientRect()
    if (!rect || rect.width <= 0) return
    const ratio = (clientX - rect.left) / rect.width
    setCropLeft(normalizeIdolCoverCropLeft(ratio - IDOL_COVER_VISIBLE_RATIO / 2))
  }

  const handlePointerDown = (event) => {
    if (!previewCode) return
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
    if (!Number.isFinite(idolId) || idolId <= 0 || saving) return
    setSaving(true)
    setError('')
    try {
      const updated = await updateJavIdolCover(idolId, {
        javId: Number(selectedJavId) || 0,
        cropLeft,
        directoryIds,
      })
      onSaved?.(updated)
      onClose?.()
    } catch (err) {
      setError(err.message || zh('保存女优封面失败', 'Failed to save idol cover'))
    } finally {
      setSaving(false)
    }
  }

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 px-4 py-6 backdrop-blur-sm">
      <div className="flex max-h-[92vh] w-full max-w-5xl flex-col overflow-hidden rounded-lg bg-white shadow-2xl">
        <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
          <div className="min-w-0">
            <div className="truncate text-base font-semibold text-slate-950">
              {item?.name || zh('女优封面', 'Idol cover')}
            </div>
            <div className="text-xs text-slate-500">{zh('封面', 'Cover')}</div>
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

        <div className="grid min-h-0 flex-1 gap-0 overflow-hidden md:grid-cols-[18rem_minmax(0,1fr)]">
          <div className="min-h-[12rem] overflow-y-auto border-b border-slate-200 md:border-b-0 md:border-r">
            <button
              type="button"
              className={`flex w-full items-center gap-2 border-b px-3 py-2 text-left text-sm ${
                selectedJavId === 0 ? 'bg-slate-900 text-white' : 'hover:bg-slate-50'
              }`}
              onClick={() => {
                setSelectedJavId(0)
                setImageFailed(false)
              }}
            >
              <ImageRoundedIcon sx={{ fontSize: 17 }} />
              <span className="min-w-0 flex-1 truncate">{zh('自动选择', 'Auto')}</span>
            </button>
            {loading ? (
              <div className="px-3 py-4 text-sm text-slate-500">{zh('加载中…', 'Loading...')}</div>
            ) : options.length > 0 ? (
              options.map((option) => {
                const active = Number(option.id) === Number(selectedJavId)
                const optionTitle = getJavDisplayTitle(option, javMetadataLanguage)
                return (
                  <button
                    key={option.id}
                    type="button"
                    className={`flex w-full flex-col gap-0.5 border-b px-3 py-2 text-left text-sm ${
                      active ? 'bg-slate-900 text-white' : 'hover:bg-slate-50'
                    }`}
                    onClick={() => {
                      setSelectedJavId(Number(option.id))
                      setImageFailed(false)
                    }}
                  >
                    <span className="flex w-full items-center gap-2">
                      <span className="font-semibold">{option.code}</span>
                      {option.solo ? (
                        <span
                          className={`rounded px-1.5 py-0.5 text-[10px] ${
                            active ? 'bg-white/20 text-white' : 'bg-emerald-50 text-emerald-700'
                          }`}
                        >
                          {zh('单人', 'Solo')}
                        </span>
                      ) : null}
                    </span>
                    {optionTitle && optionTitle !== option.code ? (
                      <span className="line-clamp-2 text-xs opacity-75">{optionTitle}</span>
                    ) : null}
                  </button>
                )
              })
            ) : (
              <div className="px-3 py-4 text-sm text-slate-500">
                {zh('暂无可用作品', 'No available works')}
              </div>
            )}
          </div>

          <div className="min-h-0 overflow-y-auto p-4">
            <div className="mx-auto max-w-3xl">
              <div className="mb-2 flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <div className="truncate text-sm font-semibold text-slate-900">
                    {previewCode || zh('无封面', 'No cover')}
                  </div>
                  {title && title !== previewCode ? (
                    <div className="truncate text-xs text-slate-500">{title}</div>
                  ) : null}
                </div>
                <div className="text-xs tabular-nums text-slate-500">
                  {Math.round(cropLeft * 100)}%
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
                    alt={previewCode}
                    className="block w-full"
                    draggable={false}
                    onError={() => setImageFailed(true)}
                  />
                ) : (
                  <div className="flex aspect-[800/538] items-center justify-center text-sm text-slate-500">
                    {zh('封面待下载', 'Cover pending')}
                  </div>
                )}
                {previewCode && !imageFailed ? (
                  <div
                    className="absolute inset-y-0 border-2 border-white bg-white/10 shadow-[0_0_0_999px_rgba(15,23,42,0.45)]"
                    style={{
                      left: `${cropLeft * 100}%`,
                      width: `${IDOL_COVER_VISIBLE_RATIO * 100}%`,
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
                  max={IDOL_COVER_DEFAULT_CROP_LEFT}
                  step="0.001"
                  value={cropLeft}
                  onChange={(event) => setCropLeft(normalizeIdolCoverCropLeft(event.target.value))}
                  className="min-w-0 flex-1 accent-slate-900"
                />
              </label>

              {error ? <div className="mt-3 text-sm text-rose-600">{error}</div> : null}
            </div>
          </div>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-2 border-t border-slate-200 px-4 py-3">
          <button
            type="button"
            className="inline-flex items-center gap-1.5 rounded border border-slate-300 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-50"
            onClick={() => {
              setSelectedJavId(0)
              setCropLeft(IDOL_COVER_DEFAULT_CROP_LEFT)
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
              disabled={saving || loading}
              onClick={handleSave}
            >
              <SaveRoundedIcon sx={{ fontSize: 17 }} />
              {saving ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
