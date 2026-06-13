import { useEffect, useState } from 'react'
import { zh } from '@/utils/i18n'

const SKIP_OVERRIDE = ':skip'

function initialState(video) {
  const override = String(video?.jav_scrape_override || '').trim()
  if (override === SKIP_OVERRIDE) {
    return { mode: 'skip', code: '' }
  }
  if (override) {
    return { mode: 'code', code: override }
  }
  return { mode: 'auto', code: '' }
}

export default function VideoScrapeSettingsModal({ open, video, saving = false, onClose, onSave }) {
  const [mode, setMode] = useState('auto')
  const [code, setCode] = useState('')

  useEffect(() => {
    if (!open) return
    const next = initialState(video)
    setMode(next.mode)
    setCode(next.code)
  }, [open, video])

  if (!open) return null

  const displayName = String(video?.filename || video?.path || `#${video?.id || ''}`).trim()
  const normalizedCode = code.trim().toUpperCase()
  const canSave = !saving && (mode !== 'code' || normalizedCode.length > 0)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 px-4">
      <div className="w-full max-w-sm rounded-lg bg-white p-3 shadow-xl">
        <div className="mb-2 flex items-center justify-between gap-3">
          <h2 className="min-w-0 truncate text-base font-semibold">
            {zh('刮削设置', 'Scrape Settings')}
          </h2>
          <button
            type="button"
            onClick={onClose}
            disabled={saving}
            className="rounded px-2 py-1 text-gray-500 hover:bg-gray-100 disabled:opacity-50"
            aria-label={zh('关闭设置', 'Close settings')}
          >
            ✕
          </button>
        </div>
        {displayName ? (
          <div className="mb-3 truncate text-xs text-gray-500" title={displayName}>
            {displayName}
          </div>
        ) : null}
        <div className="space-y-2">
          <label className="flex cursor-pointer items-center gap-2 rounded border px-3 py-2 text-sm font-medium text-gray-700 hover:border-blue-500">
            <input
              type="radio"
              name="video-scrape-mode"
              value="auto"
              checked={mode === 'auto'}
              onChange={() => setMode('auto')}
              disabled={saving}
            />
            <span>{zh('自动刮削（根据文件名）', 'Automatic scraping (by filename)')}</span>
          </label>
          <label className="flex cursor-pointer items-center gap-2 rounded border px-3 py-2 text-sm font-medium text-gray-700 hover:border-blue-500">
            <input
              type="radio"
              name="video-scrape-mode"
              value="skip"
              checked={mode === 'skip'}
              onChange={() => setMode('skip')}
              disabled={saving}
            />
            <span>{zh('不刮削此视频', 'Do not scrape this video')}</span>
          </label>
          <div className="flex flex-col gap-2 rounded border px-3 py-2 text-sm font-medium text-gray-700 hover:border-blue-500">
            <label className="flex cursor-pointer items-center gap-2">
              <input
                type="radio"
                name="video-scrape-mode"
                value="code"
                checked={mode === 'code'}
                onChange={() => setMode('code')}
                disabled={saving}
              />
              <span>{zh('指定刮削番号', 'Force scrape code')}</span>
            </label>
            <input
              type="text"
              value={code}
              onFocus={() => setMode('code')}
              onChange={(event) => setCode(event.target.value.toUpperCase())}
              disabled={saving || mode !== 'code'}
              placeholder="ABC-001"
              aria-label={zh('指定刮削番号', 'Force scrape code')}
              className="w-full rounded border px-3 py-1.5 text-sm uppercase focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:bg-gray-50"
            />
          </div>
        </div>
        <div className="mt-3 flex justify-end">
          <button
            type="button"
            onClick={onClose}
            disabled={saving}
            className="rounded border px-3 py-1 text-sm hover:bg-gray-50 disabled:opacity-50"
          >
            {zh('取消', 'Cancel')}
          </button>
          <button
            type="button"
            onClick={() => canSave && onSave?.({ mode, code: normalizedCode })}
            disabled={!canSave}
            className="ml-2 rounded bg-blue-600 px-3 py-1 text-sm text-white hover:bg-blue-700 disabled:bg-gray-300"
          >
            {saving ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
          </button>
        </div>
      </div>
    </div>
  )
}
