import { useEffect, useState } from 'react'
import { zh } from '@/utils/i18n'

const SKIP_OVERRIDE = ':skip'
const MANUAL_OVERRIDE_PREFIX = ':manual:'
const CODE_SCRAPE_AUTO = 'auto'
const CODE_SCRAPE_MANUAL = 'manual'

const emptyManualInfo = {
  code: '',
  title: '',
  studio: '',
  series: '',
  release_date: '',
  duration_min: '',
  tags_text: '',
  actors_text: '',
  cover_url: '',
  is_uncensored: '',
}

function initialState(video) {
  const override = String(video?.jav_scrape_override || '').trim()
  if (override === SKIP_OVERRIDE) {
    return { mode: 'skip', code: '', codeScrapeMode: CODE_SCRAPE_AUTO }
  }
  if (override.toLowerCase().startsWith(MANUAL_OVERRIDE_PREFIX)) {
    return {
      mode: 'code',
      code: override.slice(MANUAL_OVERRIDE_PREFIX.length).trim(),
      codeScrapeMode: CODE_SCRAPE_MANUAL,
    }
  }
  if (override) {
    return { mode: 'code', code: override, codeScrapeMode: CODE_SCRAPE_AUTO }
  }
  return { mode: 'auto', code: '', codeScrapeMode: CODE_SCRAPE_AUTO }
}

function dateFromUnix(value) {
  const unix = Number(value || 0)
  if (!Number.isFinite(unix) || unix <= 0) return ''
  return new Date(unix * 1000).toISOString().slice(0, 10)
}

function listToText(values) {
  if (!Array.isArray(values)) return ''
  return values
    .map((item) => String(item?.name || item || '').trim())
    .filter(Boolean)
    .join('\n')
}

function textToList(value) {
  return String(value || '')
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function initialManualInfo(video) {
  const state = initialState(video)
  const item = video?.jav || video?.locations?.[0]?.jav || null
  const code = String(state.code || item?.code || '')
    .trim()
    .toUpperCase()
  return {
    ...emptyManualInfo,
    code,
    title: String(item?.title || item?.title_en || '').trim(),
    studio: String(item?.studio?.name || '').trim(),
    series: String(item?.series?.name || item?.series_en?.name || '').trim(),
    release_date: dateFromUnix(item?.release_unix),
    duration_min: item?.duration_min ? String(item.duration_min) : '',
    tags_text: listToText(item?.tags),
    actors_text: listToText(item?.idols),
    is_uncensored:
      typeof item?.is_uncensored === 'boolean' ? (item.is_uncensored ? 'true' : 'false') : '',
  }
}

function manualPayload(info) {
  const duration = String(info.duration_min || '').trim()
  const isUncensored = String(info.is_uncensored || '')
  const payload = {
    code: String(info.code || '')
      .trim()
      .toUpperCase(),
    title: String(info.title || '').trim(),
    studio: String(info.studio || '').trim(),
    series: String(info.series || '').trim(),
    release_date: String(info.release_date || '').trim(),
    duration_min: duration === '' ? null : Number.parseInt(duration, 10),
    tags: textToList(info.tags_text),
    actors: textToList(info.actors_text),
    cover_url: String(info.cover_url || '').trim(),
  }
  if (isUncensored === 'true') payload.is_uncensored = true
  if (isUncensored === 'false') payload.is_uncensored = false
  return payload
}

function infoFromJavDB(data, fallbackCode = '') {
  return {
    code: String(data?.code || fallbackCode || '')
      .trim()
      .toUpperCase(),
    title: String(data?.title || '').trim(),
    studio: String(data?.studio || '').trim(),
    series: String(data?.series || '').trim(),
    release_date: String(data?.release_date || '').trim(),
    duration_min: data?.duration_min ? String(data.duration_min) : '',
    tags_text: listToText(data?.tags),
    actors_text: listToText(data?.actors),
    cover_url: String(data?.cover_url || '').trim(),
    is_uncensored:
      typeof data?.is_uncensored === 'boolean' ? (data.is_uncensored ? 'true' : 'false') : '',
  }
}

export default function VideoScrapeSettingsModal({
  open,
  video,
  saving = false,
  onClose,
  onSave,
  onLookupJavDB,
  onManualScrape,
}) {
  const [mode, setMode] = useState('auto')
  const [codeScrapeMode, setCodeScrapeMode] = useState(CODE_SCRAPE_AUTO)
  const [code, setCode] = useState('')
  const [manualInfo, setManualInfo] = useState(emptyManualInfo)
  const [lookupLoading, setLookupLoading] = useState(false)
  const [lookupError, setLookupError] = useState('')

  useEffect(() => {
    if (!open) return
    const next = initialState(video)
    setMode(next.mode)
    setCodeScrapeMode(next.codeScrapeMode)
    setCode(next.code)
    setManualInfo(initialManualInfo(video))
    setLookupLoading(false)
    setLookupError('')
  }, [open, video])

  if (!open) return null

  const displayName = String(video?.filename || video?.path || `#${video?.id || ''}`).trim()
  const normalizedCode = code.trim().toUpperCase()
  const manualDuration = String(manualInfo.duration_min || '').trim()
  const manualDurationValid =
    manualDuration === '' ||
    (Number.isFinite(Number.parseInt(manualDuration, 10)) &&
      Number.parseInt(manualDuration, 10) >= 0)
  const canSave =
    !saving &&
    !lookupLoading &&
    (mode !== 'code' ||
      (normalizedCode.length > 0 && (codeScrapeMode !== CODE_SCRAPE_MANUAL || manualDurationValid)))

  const updateManual = (patch) => setManualInfo((current) => ({ ...current, ...patch }))

  const updateCode = (value) => {
    const nextCode = value.toUpperCase()
    setCode(nextCode)
    setManualInfo((current) => ({ ...current, code: nextCode }))
  }

  const lookupJavDB = async () => {
    if (!normalizedCode || lookupLoading || saving) return
    setLookupLoading(true)
    setLookupError('')
    try {
      const data = await onLookupJavDB?.(normalizedCode)
      const nextInfo = infoFromJavDB(data, normalizedCode)
      setCode(nextInfo.code)
      setManualInfo((current) => ({ ...current, ...nextInfo }))
    } catch (err) {
      setLookupError(
        err?.message || zh('从 JavDB 获取信息失败', 'Failed to fetch metadata from JavDB')
      )
    } finally {
      setLookupLoading(false)
    }
  }

  const submit = () => {
    if (!canSave) return
    if (mode === 'code' && codeScrapeMode === CODE_SCRAPE_MANUAL) {
      onManualScrape?.(manualPayload({ ...manualInfo, code: normalizedCode }))
      return
    }
    onSave?.({ mode, code: normalizedCode })
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 px-4">
      <div className="max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-lg bg-white p-3 shadow-xl">
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
              disabled={saving || lookupLoading}
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
              disabled={saving || lookupLoading}
            />
            <span>{zh('不刮削此视频', 'Do not scrape this video')}</span>
          </label>
          <div className="flex flex-col gap-2 rounded border px-3 py-2 text-sm text-gray-700 hover:border-blue-500">
            <label className="flex cursor-pointer items-center gap-2 font-medium">
              <input
                type="radio"
                name="video-scrape-mode"
                value="code"
                checked={mode === 'code'}
                onChange={() => setMode('code')}
                disabled={saving || lookupLoading}
              />
              <span>{zh('指定刮削番号', 'Force scrape code')}</span>
            </label>
            <input
              type="text"
              value={code}
              onFocus={() => setMode('code')}
              onChange={(event) => updateCode(event.target.value)}
              disabled={saving || lookupLoading || mode !== 'code'}
              placeholder="ABC-001"
              aria-label={zh('指定刮削番号', 'Force scrape code')}
              className="w-full rounded border px-3 py-1.5 text-sm uppercase focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:bg-gray-50"
            />
            {mode === 'code' ? (
              <div className="space-y-3">
                <div className="grid gap-2 md:grid-cols-2">
                  <label className="flex cursor-pointer items-center gap-2 rounded border px-3 py-2 text-sm font-medium text-gray-700 hover:border-blue-500">
                    <input
                      type="radio"
                      name="video-code-scrape-mode"
                      value={CODE_SCRAPE_AUTO}
                      checked={codeScrapeMode === CODE_SCRAPE_AUTO}
                      onChange={() => {
                        setMode('code')
                        setCodeScrapeMode(CODE_SCRAPE_AUTO)
                      }}
                      disabled={saving || lookupLoading}
                    />
                    <span>{zh('自动刮削', 'Automatic scrape')}</span>
                  </label>
                  <label className="flex cursor-pointer items-center gap-2 rounded border px-3 py-2 text-sm font-medium text-gray-700 hover:border-blue-500">
                    <input
                      type="radio"
                      name="video-code-scrape-mode"
                      value={CODE_SCRAPE_MANUAL}
                      checked={codeScrapeMode === CODE_SCRAPE_MANUAL}
                      onChange={() => {
                        setMode('code')
                        setCodeScrapeMode(CODE_SCRAPE_MANUAL)
                      }}
                      disabled={saving || lookupLoading}
                    />
                    <span>{zh('手动刮削', 'Manual scrape')}</span>
                  </label>
                </div>
                {codeScrapeMode === CODE_SCRAPE_MANUAL ? (
                  <div className="grid gap-3 md:grid-cols-2">
                    <div className="md:col-span-2">
                      <button
                        type="button"
                        onClick={lookupJavDB}
                        disabled={!normalizedCode || saving || lookupLoading}
                        className="rounded border px-3 py-1.5 text-sm hover:bg-gray-50 disabled:opacity-50"
                      >
                        {lookupLoading
                          ? zh('填充中…', 'Filling...')
                          : zh('JavDB填充', 'Fill from JavDB')}
                      </button>
                      {lookupError ? (
                        <div className="mt-1 text-xs text-red-600">{lookupError}</div>
                      ) : null}
                    </div>
                    <div className="md:col-span-2">
                      <label className="mb-1 block text-xs font-medium text-gray-500">
                        {zh('标题', 'Title')}
                      </label>
                      <input
                        type="text"
                        value={manualInfo.title}
                        onChange={(event) => updateManual({ title: event.target.value })}
                        disabled={saving || lookupLoading}
                        className="w-full rounded border px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:bg-gray-50"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-gray-500">
                        {zh('片商', 'Studio')}
                      </label>
                      <input
                        type="text"
                        value={manualInfo.studio}
                        onChange={(event) => updateManual({ studio: event.target.value })}
                        disabled={saving || lookupLoading}
                        className="w-full rounded border px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:bg-gray-50"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-gray-500">
                        {zh('系列', 'Series')}
                      </label>
                      <input
                        type="text"
                        value={manualInfo.series}
                        onChange={(event) => updateManual({ series: event.target.value })}
                        disabled={saving || lookupLoading}
                        className="w-full rounded border px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:bg-gray-50"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-gray-500">
                        {zh('发行日期', 'Release Date')}
                      </label>
                      <input
                        type="date"
                        value={manualInfo.release_date}
                        onChange={(event) => updateManual({ release_date: event.target.value })}
                        disabled={saving || lookupLoading}
                        className="w-full rounded border px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:bg-gray-50"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-gray-500">
                        {zh('时长（分钟）', 'Duration (min)')}
                      </label>
                      <input
                        type="number"
                        min="0"
                        value={manualInfo.duration_min}
                        onChange={(event) => updateManual({ duration_min: event.target.value })}
                        disabled={saving || lookupLoading}
                        className="w-full rounded border px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:bg-gray-50"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-gray-500">
                        {zh('类别', 'Tags')}
                      </label>
                      <textarea
                        rows={4}
                        value={manualInfo.tags_text}
                        onChange={(event) => updateManual({ tags_text: event.target.value })}
                        disabled={saving || lookupLoading}
                        className="w-full resize-y rounded border px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:bg-gray-50"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-gray-500">
                        {zh('女优', 'Actors')}
                      </label>
                      <textarea
                        rows={4}
                        value={manualInfo.actors_text}
                        onChange={(event) => updateManual({ actors_text: event.target.value })}
                        disabled={saving || lookupLoading}
                        className="w-full resize-y rounded border px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:bg-gray-50"
                      />
                    </div>
                    <div className="md:col-span-2">
                      <label className="mb-1 block text-xs font-medium text-gray-500">
                        {zh('封面链接', 'Cover URL')}
                      </label>
                      <input
                        type="url"
                        value={manualInfo.cover_url}
                        onChange={(event) => updateManual({ cover_url: event.target.value })}
                        disabled={saving || lookupLoading}
                        className="w-full rounded border px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:bg-gray-50"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-gray-500">
                        {zh('有码状态', 'Censor State')}
                      </label>
                      <select
                        value={manualInfo.is_uncensored}
                        onChange={(event) => updateManual({ is_uncensored: event.target.value })}
                        disabled={saving || lookupLoading}
                        className="w-full rounded border px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:bg-gray-50"
                      >
                        <option value="">{zh('未知', 'Unknown')}</option>
                        <option value="false">{zh('有码', 'Censored')}</option>
                        <option value="true">{zh('无码', 'Uncensored')}</option>
                      </select>
                    </div>
                  </div>
                ) : null}
              </div>
            ) : null}
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
            onClick={submit}
            disabled={!canSave}
            className="ml-2 rounded bg-blue-600 px-3 py-1 text-sm text-white hover:bg-blue-700 disabled:bg-gray-300"
          >
            {saving
              ? zh('保存中…', 'Saving...')
              : mode === 'code' && codeScrapeMode === CODE_SCRAPE_MANUAL
                ? zh('手动刮削', 'Manual Scrape')
                : zh('保存', 'Save')}
          </button>
        </div>
      </div>
    </div>
  )
}
