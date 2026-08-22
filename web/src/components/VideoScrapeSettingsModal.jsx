import { useEffect, useRef, useState } from 'react'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'
import SearchIcon from '@mui/icons-material/Search'
import { Tooltip } from '@mui/material'
import AppModal from '@/components/AppModal'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'

const SKIP_OVERRIDE = ':skip'
const MANUAL_OVERRIDE_PREFIX = ':manual:'
const AUTO_SOURCE_FILENAME = 'filename'
const AUTO_SOURCE_CODE = 'code'
const CODE_PATTERN = /^[A-Z0-9_-]+$/
const CODE_INPUT_PATTERN = '[A-Z0-9_\\-]+'
const JAVBUS_ORIGIN = 'https://www.javbus.com'
const JAVLIBRARY_ORIGIN = 'https://www.javlibrary.com'
const JAVDB_ORIGIN = 'https://javdb.com'
const AVSOX_ORIGIN = 'https://avsox.click'
const BROWSER_SCRAPE_PROVIDERS = {
  javbus: { name: 'JavBus', requiresCode: false },
  javlibrary: { name: 'JavLibrary', requiresCode: false },
  javdb: { name: 'JavDB', requiresCode: false },
  avsox: { name: 'AVSOX', requiresCode: false },
}
const JAVBOSS_EXTENSION_ID = 'iikdjhkpjihfkehccfmkpkdmenmbaacn'
const JAVBOSS_EXTENSION_ORIGIN = `chrome-extension://${JAVBOSS_EXTENSION_ID}`
const JAVBOSS_EXTENSION_BRIDGE_URL = `${JAVBOSS_EXTENSION_ORIGIN}/bridge.html`
const JAVBUS_MESSAGE_CONNECT = 'JAVBOSS_EXTENSION_CONNECT'
const JAVBUS_MESSAGE_READY = 'JAVBOSS_EXTENSION_READY'
const JAVBUS_MESSAGE_METADATA = 'JAVBOSS_JAVBUS_METADATA'
const JAVBUS_MESSAGE_OPEN = 'JAVBOSS_JAVBUS_OPEN'
const JAVBUS_MESSAGE_OPEN_STATUS = 'JAVBOSS_JAVBUS_OPEN_STATUS'

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
    return { mode: 'skip', code: '', autoSource: AUTO_SOURCE_FILENAME }
  }
  if (override.toLowerCase().startsWith(MANUAL_OVERRIDE_PREFIX)) {
    return {
      mode: 'manual',
      code: override.slice(MANUAL_OVERRIDE_PREFIX.length).trim(),
      autoSource: AUTO_SOURCE_FILENAME,
    }
  }
  if (override) {
    return { mode: 'auto', code: override, autoSource: AUTO_SOURCE_CODE }
  }
  return { mode: 'auto', code: '', autoSource: AUTO_SOURCE_FILENAME }
}

function listToText(values) {
  if (!Array.isArray(values)) return ''
  return values
    .map((item) => String(item?.name || item || '').trim())
    .filter(Boolean)
    .join('\n')
}

function textToList(value) {
  const seen = new Set()
  return String(value || '')
    .split(/[\n,，;；]+/)
    .map((item) => item.trim())
    .filter((item) => {
      if (!item || seen.has(item)) return false
      seen.add(item)
      return true
    })
}

function ManualListEditor({
  values,
  input,
  onInputChange,
  onAdd,
  onRemove,
  disabled,
  placeholder,
}) {
  return (
    <div
      className={`min-h-24 rounded border px-2 py-2 focus-within:border-blue-500 focus-within:ring-1 focus-within:ring-blue-500 ${
        disabled ? 'bg-gray-50' : 'bg-white'
      }`}
    >
      <div className="flex flex-wrap items-center gap-2">
        {values.map((value) => (
          <span
            key={value}
            className="inline-flex min-w-0 items-center gap-1 rounded-full bg-gray-100 px-2 py-1 text-sm text-gray-800"
          >
            <span className="max-w-48 truncate">{value}</span>
            <button
              type="button"
              className="inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-gray-500 hover:bg-gray-200 hover:text-gray-900 disabled:cursor-not-allowed disabled:opacity-50"
              onClick={() => onRemove(value)}
              disabled={disabled}
              aria-label={zh(`移除 ${value}`, `Remove ${value}`)}
            >
              <CloseOutlinedIcon sx={{ fontSize: 13 }} />
            </button>
          </span>
        ))}
        <input
          type="text"
          value={input}
          onChange={(event) => onInputChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key !== 'Enter' || event.nativeEvent.isComposing) return
            event.preventDefault()
            onAdd()
          }}
          disabled={disabled}
          placeholder={values.length === 0 ? placeholder : ''}
          className="min-w-40 flex-1 bg-transparent px-1 py-1 text-sm outline-none disabled:cursor-not-allowed"
        />
      </div>
    </div>
  )
}

function initialManualInfo(video) {
  const state = initialState(video)
  const code = String(state.code || '')
    .trim()
    .toUpperCase()
  return {
    ...emptyManualInfo,
    code,
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

function infoFromProvider(data, fallbackCode = '') {
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

function browserScrapeURL(provider, code) {
  const normalizedCode = String(code || '')
    .trim()
    .toUpperCase()
  const validCode = normalizedCode && CODE_PATTERN.test(normalizedCode)
  if (provider === 'javlibrary') {
    if (!validCode) return `${JAVLIBRARY_ORIGIN}/tw/`
    const url = new URL('/tw/vl_searchbyid.php', JAVLIBRARY_ORIGIN)
    url.searchParams.set('keyword', normalizedCode)
    return url.href
  }
  if (provider === 'javdb') {
    if (!validCode) return `${JAVDB_ORIGIN}/`
    const url = new URL('/search', JAVDB_ORIGIN)
    url.searchParams.set('q', normalizedCode)
    url.searchParams.set('f', 'all')
    return url.href
  }
  if (provider === 'avsox') {
    return validCode
      ? `${AVSOX_ORIGIN}/tw/search/${encodeURIComponent(normalizedCode)}`
      : `${AVSOX_ORIGIN}/tw`
  }
  return validCode ? `${JAVBUS_ORIGIN}/${encodeURIComponent(normalizedCode)}` : JAVBUS_ORIGIN
}

function newBrowserScrapeSessionId() {
  if (typeof crypto?.randomUUID === 'function') return crypto.randomUUID()
  return `javboss-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

function limitedText(value, maxLength) {
  return String(value || '')
    .trim()
    .slice(0, maxLength)
}

function limitedTextList(value, maxItems = 200) {
  if (!Array.isArray(value)) return []
  return value.slice(0, maxItems).map((item) => limitedText(item?.name || item, 200))
}

function safeExternalURL(value) {
  const candidate = limitedText(value, 2048)
  if (!candidate) return ''
  try {
    const parsed = new URL(candidate)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? parsed.href : ''
  } catch {
    return ''
  }
}

function infoFromBrowserExtension(data, fallbackCode = '') {
  if (!data || typeof data !== 'object') return null
  const code = limitedText(data.code || fallbackCode, 64).toUpperCase()
  if (!code || !CODE_PATTERN.test(code)) return null

  const rawDuration = Number.parseInt(data.duration_min, 10)
  const releaseDate = limitedText(data.release_date, 10)
  return infoFromProvider(
    {
      code,
      title: limitedText(data.title, 5000),
      studio: limitedText(data.studio, 500),
      series: limitedText(data.series, 500),
      release_date: /^\d{4}-\d{2}-\d{2}$/.test(releaseDate) ? releaseDate : '',
      duration_min:
        Number.isFinite(rawDuration) && rawDuration >= 0 && rawDuration <= 10000
          ? rawDuration
          : null,
      tags: limitedTextList(data.tags),
      actors: limitedTextList(data.actors, 100),
      cover_url: safeExternalURL(data.cover_url),
      is_uncensored: typeof data.is_uncensored === 'boolean' ? data.is_uncensored : undefined,
    },
    fallbackCode
  )
}

export default function VideoScrapeSettingsModal({
  open,
  video,
  saving = false,
  onClose,
  onSave,
  onFetchPossibleCodes,
  onManualScrape,
  onLinkExistingJav,
}) {
  const [mode, setMode] = useState('auto')
  const [autoSource, setAutoSource] = useState(AUTO_SOURCE_FILENAME)
  const [code, setCode] = useState('')
  const [manualInfo, setManualInfo] = useState(emptyManualInfo)
  const [manualCensorError, setManualCensorError] = useState(false)
  const [tagInput, setTagInput] = useState('')
  const [actorInput, setActorInput] = useState('')
  const [linkLoading, setLinkLoading] = useState(false)
  const [linkError, setLinkError] = useState('')
  const [possibleCodesOpen, setPossibleCodesOpen] = useState(false)
  const [possibleCodesLoading, setPossibleCodesLoading] = useState(false)
  const [possibleCodesError, setPossibleCodesError] = useState('')
  const [possibleCodesResult, setPossibleCodesResult] = useState(null)
  const javBusBridgeRef = useRef(null)
  const browserScrapeProviderRef = useRef('')
  const [javBusSessionId, setJavBusSessionId] = useState('')
  const [javBusExtensionReady, setJavBusExtensionReady] = useState(false)
  const [javBusOpening, setJavBusOpening] = useState(false)
  const [javBusStatus, setJavBusStatus] = useState('')
  const [javBusSourceURL, setJavBusSourceURL] = useState('')

  useEffect(() => {
    if (!open) return
    const next = initialState(video)
    setMode(next.mode)
    setAutoSource(next.autoSource)
    setCode(next.code)
    setManualInfo(initialManualInfo(video))
    setManualCensorError(false)
    setTagInput('')
    setActorInput('')
    setLinkLoading(false)
    setLinkError('')
    setPossibleCodesOpen(false)
    setPossibleCodesLoading(false)
    setPossibleCodesError('')
    setPossibleCodesResult(null)
    setJavBusSessionId(newBrowserScrapeSessionId())
    setJavBusExtensionReady(false)
    setJavBusOpening(false)
    setJavBusStatus('')
    setJavBusSourceURL('')
    browserScrapeProviderRef.current = ''
  }, [open, video])

  useEffect(() => {
    if (!open) return undefined

    const receiveJavBusMessage = (event) => {
      if (
        event.origin !== JAVBOSS_EXTENSION_ORIGIN ||
        event.source !== javBusBridgeRef.current?.contentWindow
      ) {
        return
      }
      const message = event.data
      if (!message || message.version !== 1 || message.sessionId !== javBusSessionId) return

      if (message.type === JAVBUS_MESSAGE_READY) {
        setJavBusExtensionReady(true)
        setJavBusStatus(zh('JavBoss 助手已连接', 'JavBoss Assistant connected'))
        return
      }
      if (message.type === JAVBUS_MESSAGE_OPEN_STATUS) {
        setJavBusOpening(false)
        setJavBusStatus(
          message.ok
            ? zh(
                `已打开 ${browserScrapeProviderRef.current || '元数据网站'} 新标签页。`,
                `Opened a new ${browserScrapeProviderRef.current || 'metadata site'} tab.`
              )
            : zh(
                `打开新标签页失败：${limitedText(message.error, 300)}`,
                `Failed to open a new tab: ${limitedText(message.error, 300)}`
              )
        )
        return
      }
      if (message.type !== JAVBUS_MESSAGE_METADATA) return

      const nextInfo = infoFromBrowserExtension(message.payload, code)
      if (!nextInfo) {
        setJavBusStatus(
          zh('扩展返回的数据无效，请确认当前是作品详情页。', 'The extension returned invalid data.')
        )
        return
      }
      setCode(nextInfo.code)
      setManualInfo((current) => ({ ...current, ...nextInfo }))
      setManualCensorError(false)
      setTagInput('')
      setActorInput('')
      setJavBusSourceURL(safeExternalURL(message.payload?.source_url))
      const sourceName = limitedText(message.payload?.source_name, 50) || '元数据网站'
      setJavBusStatus(
        zh(
          `已从 ${sourceName} 回填 ${nextInfo.code}，请检查后保存。`,
          `Filled ${nextInfo.code} from ${sourceName}. Review it before saving.`
        )
      )
    }

    window.addEventListener('message', receiveJavBusMessage)
    return () => window.removeEventListener('message', receiveJavBusMessage)
  }, [code, javBusSessionId, open])

  useEffect(() => {
    if (!open || !javBusSessionId) return undefined
    const connect = () => {
      javBusBridgeRef.current?.contentWindow?.postMessage(
        { type: JAVBUS_MESSAGE_CONNECT, sessionId: javBusSessionId },
        JAVBOSS_EXTENSION_ORIGIN
      )
    }
    const timers = [0, 300, 1000].map((delay) => window.setTimeout(connect, delay))
    return () => timers.forEach((timer) => window.clearTimeout(timer))
  }, [javBusSessionId, open])

  useEffect(() => {
    if (!open || !javBusSessionId || javBusExtensionReady) return undefined
    const timer = window.setTimeout(() => {
      setJavBusStatus(
        zh(
          '尚未检测到扩展。请重新加载 browser-extension 目录并刷新 JavBoss。',
          'Extension not detected. Reload the browser-extension directory, then reload JavBoss.'
        )
      )
    }, 5000)
    return () => window.clearTimeout(timer)
  }, [javBusExtensionReady, javBusSessionId, open])

  useEffect(() => {
    if (!javBusOpening) return undefined
    const timer = window.setTimeout(() => {
      setJavBusOpening(false)
      setJavBusStatus(
        zh(
          '打开元数据网站超时，请重新加载扩展后重试。',
          'Opening the metadata site timed out. Reload the extension and try again.'
        )
      )
    }, 10000)
    return () => window.clearTimeout(timer)
  }, [javBusOpening])

  if (!open) return null

  const displayName = String(video?.filename || video?.path || `#${video?.id || ''}`).trim()
  const rawCode = code.toUpperCase()
  const normalizedCode = rawCode.trim()
  const codeInvalid =
    rawCode.length > 0 && (rawCode !== normalizedCode || !CODE_PATTERN.test(rawCode))
  const codeValid = normalizedCode.length > 0 && !codeInvalid
  const manualDuration = String(manualInfo.duration_min || '').trim()
  const manualDurationValid =
    manualDuration === '' ||
    (Number.isFinite(Number.parseInt(manualDuration, 10)) &&
      Number.parseInt(manualDuration, 10) >= 0)
  const manualCensorStateValid = ['true', 'false'].includes(manualInfo.is_uncensored)
  const manualTags = textToList(manualInfo.tags_text)
  const manualActors = textToList(manualInfo.actors_text)
  const canSave =
    !saving &&
    !linkLoading &&
    (mode === 'skip' ||
      (mode === 'auto' && autoSource === AUTO_SOURCE_FILENAME) ||
      (codeValid && (mode !== 'manual' || manualDurationValid)))

  const updateManual = (patch) => setManualInfo((current) => ({ ...current, ...patch }))

  const updateCode = (value) => {
    const nextCode = value.toUpperCase()
    setCode(nextCode)
    setManualInfo((current) => ({ ...current, code: nextCode }))
    if (linkError) setLinkError('')
  }

  const openBrowserScrapeProvider = (provider) => {
    if (javBusOpening) return
    const providerConfig = BROWSER_SCRAPE_PROVIDERS[provider]
    if (!providerConfig) return
    if (!javBusSessionId || !javBusExtensionReady) {
      setJavBusStatus(
        zh(
          '未连接到扩展，请确认已重新加载扩展并刷新 JavBoss。',
          'Extension is not connected. Reload the extension and the JavBoss page.'
        )
      )
      return
    }
    setJavBusSourceURL('')
    setJavBusOpening(true)
    browserScrapeProviderRef.current = providerConfig.name
    setJavBusStatus(
      zh(`正在打开 ${providerConfig.name} 新标签页…`, `Opening a new ${providerConfig.name} tab...`)
    )
    javBusBridgeRef.current?.contentWindow?.postMessage(
      {
        type: JAVBUS_MESSAGE_OPEN,
        sessionId: javBusSessionId,
        url: browserScrapeURL(provider, normalizedCode),
      },
      JAVBOSS_EXTENSION_ORIGIN
    )
  }

  const testPossibleCodes = async () => {
    if (possibleCodesLoading || saving) return
    setPossibleCodesOpen(true)
    setPossibleCodesLoading(true)
    setPossibleCodesError('')
    setPossibleCodesResult(null)
    try {
      const data = await onFetchPossibleCodes?.()
      setPossibleCodesResult(data || {})
    } catch (err) {
      setPossibleCodesError(getErrorMessage(err))
    } finally {
      setPossibleCodesLoading(false)
    }
  }

  const linkExistingJav = async () => {
    if (!codeValid || saving || linkLoading || !onLinkExistingJav) return
    setLinkLoading(true)
    setLinkError('')
    try {
      await onLinkExistingJav(normalizedCode)
    } catch (err) {
      setLinkError(getErrorMessage(err))
    } finally {
      setLinkLoading(false)
    }
  }

  const addManualListValues = (field, value, clearInput) => {
    const additions = textToList(value)
    if (additions.length === 0) return
    setManualInfo((current) => ({
      ...current,
      [field]: listToText([...textToList(current[field]), ...additions]),
    }))
    clearInput('')
  }

  const removeManualListValue = (field, value) => {
    setManualInfo((current) => ({
      ...current,
      [field]: listToText(textToList(current[field]).filter((item) => item !== value)),
    }))
  }

  const submit = () => {
    if (mode === 'manual' && !manualCensorStateValid) {
      setManualCensorError(true)
      return
    }
    if (!canSave) return
    setManualCensorError(false)
    if (mode === 'manual') {
      onManualScrape?.(
        manualPayload({
          ...manualInfo,
          code: normalizedCode,
          tags_text: listToText([...manualTags, ...textToList(tagInput)]),
          actors_text: listToText([...manualActors, ...textToList(actorInput)]),
        })
      )
      return
    }
    onSave?.({
      mode: mode === 'auto' && autoSource === AUTO_SOURCE_CODE ? 'code' : mode,
      code: normalizedCode,
    })
  }

  return (
    <AppModal
      ariaLabel={zh('刮削设置', 'Scrape Settings')}
      className="px-4"
      closeDisabled={saving}
      contentClassName="flex max-h-[90vh] w-full max-w-2xl flex-col rounded-lg bg-white shadow-xl"
      onClose={onClose}
    >
      <div className="shrink-0 p-3 pb-0">
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
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-3">
        <div className="space-y-2">
          <section
            className={`overflow-hidden rounded-lg border ${
              mode === 'auto' ? 'border-blue-500 bg-blue-50/30' : 'border-gray-200'
            }`}
          >
            <label className="flex cursor-pointer items-center gap-2 px-3 py-2.5 text-sm font-semibold text-gray-800">
              <input
                type="radio"
                name="video-scrape-mode"
                value="auto"
                checked={mode === 'auto'}
                onChange={() => setMode('auto')}
                disabled={saving}
              />
              <span className="shrink-0">{zh('自动刮削', 'Automatic Scrape')}</span>
              <span className="min-w-0 text-xs font-normal text-gray-500">
                {zh(
                  '根据文件名或指定番号在扫描过程中自动获取影片信息',
                  'Automatically fetch metadata during scans by filename or a specified code'
                )}
              </span>
            </label>
            {mode === 'auto' ? (
              <div className="space-y-2 border-t border-blue-100 px-3 py-3">
                <div className="flex items-center gap-2 rounded border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700">
                  <label className="flex min-w-0 flex-1 cursor-pointer items-center gap-2 font-medium">
                    <input
                      type="radio"
                      name="video-auto-scrape-source"
                      value={AUTO_SOURCE_FILENAME}
                      checked={autoSource === AUTO_SOURCE_FILENAME}
                      onChange={() => setAutoSource(AUTO_SOURCE_FILENAME)}
                      disabled={saving}
                    />
                    <span>{zh('根据文件名', 'By filename')}</span>
                  </label>
                  <button
                    type="button"
                    onClick={testPossibleCodes}
                    disabled={saving || possibleCodesLoading}
                    className="inline-flex shrink-0 items-center gap-1 rounded border px-2 py-1 text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
                    title={zh('提取番号测试', 'Test code extraction')}
                  >
                    <SearchIcon fontSize="inherit" />
                    <span>
                      {possibleCodesLoading
                        ? zh('提取中…', 'Extracting...')
                        : zh('提取番号测试', 'Test')}
                    </span>
                  </button>
                </div>
                <div className="rounded border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700">
                  <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
                    <label className="flex flex-1 cursor-pointer items-center gap-2 font-medium">
                      <input
                        type="radio"
                        name="video-auto-scrape-source"
                        value={AUTO_SOURCE_CODE}
                        checked={autoSource === AUTO_SOURCE_CODE}
                        onChange={() => setAutoSource(AUTO_SOURCE_CODE)}
                        disabled={saving}
                      />
                      <span>{zh('指定番号', 'Specified code')}</span>
                    </label>
                    <input
                      type="text"
                      value={code}
                      onFocus={() => setAutoSource(AUTO_SOURCE_CODE)}
                      onChange={(event) => updateCode(event.target.value)}
                      disabled={saving || autoSource !== AUTO_SOURCE_CODE}
                      placeholder="IPX-001"
                      pattern={CODE_INPUT_PATTERN}
                      aria-label={zh('指定番号', 'Specified code')}
                      aria-invalid={autoSource === AUTO_SOURCE_CODE && codeInvalid}
                      className={`w-full rounded border px-3 py-1.5 text-sm uppercase focus:outline-none focus:ring-1 disabled:bg-gray-50 sm:w-44 ${
                        autoSource === AUTO_SOURCE_CODE && codeInvalid
                          ? 'border-red-500 focus:border-red-500 focus:ring-red-500'
                          : 'focus:border-blue-500 focus:ring-blue-500'
                      }`}
                    />
                  </div>
                  {autoSource === AUTO_SOURCE_CODE && codeInvalid ? (
                    <div className="mt-2 text-xs text-red-600">
                      {zh(
                        '番号只能包含大写字母、数字、_、-',
                        'Code can only contain uppercase letters, numbers, _, and -'
                      )}
                    </div>
                  ) : null}
                </div>
              </div>
            ) : null}
          </section>

          <section
            className={`overflow-hidden rounded-lg border ${
              mode === 'manual' ? 'border-blue-500 bg-blue-50/30' : 'border-gray-200'
            }`}
          >
            <label className="flex cursor-pointer items-center gap-2 px-3 py-2.5 text-sm font-semibold text-gray-800">
              <input
                type="radio"
                name="video-scrape-mode"
                value="manual"
                checked={mode === 'manual'}
                onChange={() => setMode('manual')}
                disabled={saving}
              />
              <span className="shrink-0">{zh('手动刮削', 'Manual Scrape')}</span>
              <span className="min-w-0 text-xs font-normal text-gray-500">
                {zh(
                  '自行编辑影片信息，可使用浏览器扩展辅助回填。',
                  'Edit metadata manually; you can use the browser extension to fill it.'
                )}
              </span>
            </label>
            {mode === 'manual' ? (
              <div className="grid gap-3 border-t border-blue-100 px-3 py-3 md:grid-cols-2">
                <div className="md:col-span-2">
                  <label className="mb-1 block text-xs font-medium text-gray-500">
                    {zh('番号', 'Code')}
                  </label>
                  <input
                    type="text"
                    value={code}
                    onChange={(event) => updateCode(event.target.value)}
                    disabled={saving}
                    placeholder="IPX-001"
                    pattern={CODE_INPUT_PATTERN}
                    aria-invalid={codeInvalid}
                    className={`w-full rounded border px-3 py-1.5 text-sm uppercase focus:outline-none focus:ring-1 disabled:bg-gray-50 ${
                      codeInvalid
                        ? 'border-red-500 focus:border-red-500 focus:ring-red-500'
                        : 'focus:border-blue-500 focus:ring-blue-500'
                    }`}
                  />
                  <div className="mt-2">
                    <Tooltip
                      arrow
                      title={zh(
                        '如果该番号在 JAV 库中已存在，可直接关联，无需手动填入信息。',
                        'If this code already exists in the JAV library, link it directly without entering metadata manually.'
                      )}
                    >
                      <span className="inline-flex">
                        <button
                          type="button"
                          onClick={() => void linkExistingJav()}
                          disabled={!codeValid || saving || linkLoading || !onLinkExistingJav}
                          className="rounded border border-blue-300 bg-white px-3 py-1 text-xs font-medium text-blue-700 hover:border-blue-500 hover:bg-blue-50 disabled:opacity-50"
                        >
                          {linkLoading
                            ? zh('关联中…', 'Linking...')
                            : zh('直接关联已有番号', 'Link existing JAV directly')}
                        </button>
                      </span>
                    </Tooltip>
                  </div>
                  {linkError ? (
                    <div role="alert" className="mt-1 text-xs text-red-600">
                      {linkError}
                    </div>
                  ) : null}
                  {codeInvalid ? (
                    <div className="mt-1 text-xs text-red-600">
                      {zh(
                        '番号只能包含大写字母、数字、_、-',
                        'Code can only contain uppercase letters, numbers, _, and -'
                      )}
                    </div>
                  ) : null}
                  <div className="mt-3 rounded border border-dashed border-blue-200 bg-blue-50/60 p-3">
                    <div className="text-xs font-medium text-gray-700">
                      {zh('浏览器扩展辅助刮削', 'Browser extension-assisted scrape')}
                    </div>
                    <div className="mt-1 text-[11px] leading-4 text-gray-500">
                      {zh(
                        '安装并启用“JavBoss 助手”扩展后，点击下方按钮打开对应网站，在影片详情页右下角点击“回填到 JavBoss”即可自动填充影片信息',
                        'Install and enable the “JavBoss 助手” extension, click a button below to open the corresponding site, then click “Fill JavBoss” in the lower-right corner of a movie detail page to fill its metadata automatically.'
                      )}
                    </div>
                    <div className="mt-2 flex flex-wrap gap-2">
                      {Object.entries(BROWSER_SCRAPE_PROVIDERS).map(
                        ([provider, providerConfig]) => (
                          <button
                            key={provider}
                            type="button"
                            onClick={() => openBrowserScrapeProvider(provider)}
                            disabled={
                              saving || javBusOpening || (providerConfig.requiresCode && !codeValid)
                            }
                            className="rounded border border-blue-300 bg-white px-3 py-1 text-xs font-medium text-blue-700 hover:border-blue-500 hover:bg-blue-50 disabled:opacity-50"
                          >
                            {javBusOpening &&
                            browserScrapeProviderRef.current === providerConfig.name
                              ? zh('正在打开…', 'Opening...')
                              : zh(`打开 ${providerConfig.name}`, `Open ${providerConfig.name}`)}
                          </button>
                        )
                      )}
                    </div>
                    {javBusStatus ? (
                      <div
                        className={`mt-2 text-xs leading-5 ${
                          javBusExtensionReady ? 'text-blue-700' : 'text-amber-700'
                        }`}
                      >
                        {javBusStatus}
                      </div>
                    ) : null}
                    {javBusSourceURL ? (
                      <div className="mt-1 truncate text-xs text-gray-400" title={javBusSourceURL}>
                        {javBusSourceURL}
                      </div>
                    ) : null}
                  </div>
                </div>
                <div className="md:col-span-2">
                  <label className="mb-1 block text-xs font-medium text-gray-500">
                    {zh('标题', 'Title')}
                  </label>
                  <input
                    type="text"
                    value={manualInfo.title}
                    onChange={(event) => updateManual({ title: event.target.value })}
                    disabled={saving}
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
                    disabled={saving}
                    placeholder={zh('优先填写英文名称', 'English name preferred')}
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
                    disabled={saving}
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
                    disabled={saving}
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
                    disabled={saving}
                    className="w-full rounded border px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:bg-gray-50"
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-gray-500">
                    {zh('标签', 'Tags')}
                  </label>
                  <ManualListEditor
                    values={manualTags}
                    input={tagInput}
                    onInputChange={setTagInput}
                    onAdd={() => addManualListValues('tags_text', tagInput, setTagInput)}
                    onRemove={(value) => removeManualListValue('tags_text', value)}
                    disabled={saving}
                    placeholder={zh('输入标签后按回车添加', 'Type a tag and press Enter')}
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-gray-500">
                    {zh('女优', 'Actors')}
                  </label>
                  <ManualListEditor
                    values={manualActors}
                    input={actorInput}
                    onInputChange={setActorInput}
                    onAdd={() => addManualListValues('actors_text', actorInput, setActorInput)}
                    onRemove={(value) => removeManualListValue('actors_text', value)}
                    disabled={saving}
                    placeholder={zh(
                      '输入女优名称后按回车添加',
                      'Type an actor name and press Enter'
                    )}
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
                    disabled={saving}
                    className="w-full rounded border px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:bg-gray-50"
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-gray-500">
                    {zh('有码状态', 'Censor State')}
                  </label>
                  <select
                    value={manualInfo.is_uncensored}
                    onChange={(event) => {
                      updateManual({ is_uncensored: event.target.value })
                      setManualCensorError(false)
                    }}
                    disabled={saving}
                    required
                    aria-invalid={manualCensorError}
                    className={`w-full rounded border px-3 py-1.5 text-sm focus:outline-none focus:ring-1 disabled:bg-gray-50 ${
                      manualCensorError
                        ? 'border-red-500 focus:border-red-500 focus:ring-red-500'
                        : 'focus:border-blue-500 focus:ring-blue-500'
                    }`}
                  >
                    <option value="" disabled>
                      {zh('请选择', 'Select')}
                    </option>
                    <option value="false">{zh('有码', 'Censored')}</option>
                    <option value="true">{zh('无码', 'Uncensored')}</option>
                  </select>
                  {manualCensorError ? (
                    <div role="alert" className="mt-1 text-xs text-red-600">
                      {zh('请选择有码状态', 'Select a censor state')}
                    </div>
                  ) : null}
                </div>
              </div>
            ) : null}
          </section>

          <label
            className={`flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2.5 text-sm font-semibold text-gray-800 ${
              mode === 'skip' ? 'border-blue-500 bg-blue-50/30' : 'border-gray-200'
            }`}
          >
            <input
              type="radio"
              name="video-scrape-mode"
              value="skip"
              checked={mode === 'skip'}
              onChange={() => setMode('skip')}
              disabled={saving}
            />
            <span className="shrink-0">{zh('不刮削', 'Do Not Scrape')}</span>
            <span className="min-w-0 text-xs font-normal text-gray-500">
              {zh(
                '扫描过程中跳过此视频，不获取影片信息',
                'Skip this video during scans without fetching metadata'
              )}
            </span>
          </label>
        </div>
      </div>
      <div className="shrink-0 p-3">
        <div className="flex justify-end">
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
              : mode === 'manual'
                ? zh('手动刮削', 'Manual Scrape')
                : zh('保存', 'Save')}
          </button>
        </div>
      </div>
      {possibleCodesOpen ? (
        <AppModal
          ariaLabel={zh('提取番号测试', 'Code Extraction Test')}
          className="px-4"
          contentClassName="w-full max-w-md rounded-lg bg-white p-4 shadow-xl"
          onClose={() => setPossibleCodesOpen(false)}
          zIndex={1400}
        >
          <div className="mb-3 flex items-center justify-between gap-3">
            <h3 className="min-w-0 truncate text-base font-semibold">
              {zh('提取番号测试', 'Code Extraction Test')}
            </h3>
            <button
              type="button"
              onClick={() => setPossibleCodesOpen(false)}
              className="rounded px-2 py-1 text-gray-500 hover:bg-gray-100"
              aria-label={zh('关闭', 'Close')}
            >
              ✕
            </button>
          </div>
          <p className="mb-3 text-sm leading-6 text-gray-600">
            {zh(
              '自动刮削会依次尝试以下番号进行刮削。如果一直未刮削或者刮削错误，可以修改文件名或修改刮削设置为：不刮削/指定番号刮削。',
              'Automatic scraping will try the following codes in order. If scraping keeps failing or matches the wrong item, rename the file or change the scrape setting to: Do not scrape / Force scrape code.'
            )}
          </p>
          {possibleCodesResult?.filename ? (
            <div
              className="mb-3 truncate rounded bg-gray-50 px-2 py-1 text-xs text-gray-500"
              title={possibleCodesResult.filename}
            >
              {possibleCodesResult.filename}
            </div>
          ) : null}
          {possibleCodesLoading ? (
            <div className="rounded border border-dashed px-3 py-6 text-center text-sm text-gray-500">
              {zh('正在提取…', 'Extracting...')}
            </div>
          ) : possibleCodesError ? (
            <div className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
              {possibleCodesError}
            </div>
          ) : Array.isArray(possibleCodesResult?.possible_codes) &&
            possibleCodesResult.possible_codes.length > 0 ? (
            <ol className="max-h-64 list-decimal space-y-1 overflow-y-auto rounded border bg-gray-50 px-8 py-3 text-sm text-gray-800">
              {possibleCodesResult.possible_codes.map((item) => (
                <li key={item} className="font-mono">
                  {item}
                </li>
              ))}
            </ol>
          ) : (
            <div className="rounded border border-dashed px-3 py-6 text-center text-sm text-gray-500">
              {zh('没有提取到番号', 'No codes extracted')}
            </div>
          )}
          <div className="mt-4 flex justify-end">
            <button
              type="button"
              onClick={() => setPossibleCodesOpen(false)}
              className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700"
            >
              {zh('知道了', 'OK')}
            </button>
          </div>
        </AppModal>
      ) : null}
      <iframe
        ref={javBusBridgeRef}
        src={JAVBOSS_EXTENSION_BRIDGE_URL}
        title={zh('JavBoss 扩展通信桥', 'JavBoss extension bridge')}
        className="hidden"
        tabIndex={-1}
        aria-hidden="true"
        onLoad={() => {
          if (!javBusSessionId) return
          javBusBridgeRef.current?.contentWindow?.postMessage(
            { type: JAVBUS_MESSAGE_CONNECT, sessionId: javBusSessionId },
            JAVBOSS_EXTENSION_ORIGIN
          )
        }}
      />
    </AppModal>
  )
}
