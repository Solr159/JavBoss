import { useEffect, useMemo, useRef, useState } from 'react'
import CloseRoundedIcon from '@mui/icons-material/CloseRounded'
import EditRoundedIcon from '@mui/icons-material/EditRounded'
import PhotoCameraRoundedIcon from '@mui/icons-material/PhotoCameraRounded'
import SearchRoundedIcon from '@mui/icons-material/SearchRounded'
import StarBorderRoundedIcon from '@mui/icons-material/StarBorderRounded'
import StarRoundedIcon from '@mui/icons-material/StarRounded'
import { fetchJavIdolJavDBURL, fetchJavIdolOptions, mergeJavIdols, updateJavIdol } from '@/api'
import JavIdolCoverModal, {
  IDOL_COVER_DEFAULT_CROP_LEFT,
  IDOL_COVER_VISIBLE_RATIO,
  normalizeIdolCoverCropLeft,
} from '@/components/JavIdolCoverModal'
import { getIdolDisplayNames } from '@/utils/javIdol'
import { zh } from '@/utils/i18n'

export { getIdolDisplayName, getIdolDisplayNames } from '@/utils/javIdol'

const RIGHT_PORTION = IDOL_COVER_VISIBLE_RATIO

export function getIdolCardLayoutProps() {
  const visibleRatio = Math.min(Math.max(RIGHT_PORTION, 0.01), 1)
  const bgWidthPercent = (1 / visibleRatio) * 100
  const originalWidth = 800
  const originalHeight = 538
  const coverAspectPercent = (originalHeight / (originalWidth * visibleRatio)) * 100

  return { bgWidthPercent, coverAspectPercent }
}

export default function JavIdolGrid({
  items,
  onSelectIdol,
  onOpenFavorites,
  buildIdolUrl,
  javMetadataLanguage,
  preferChineseName = false,
  directoryIds = [],
  onMerged,
}) {
  const { coverAspectPercent } = getIdolCardLayoutProps()
  const [coverEditorItem, setCoverEditorItem] = useState(null)
  const [editItem, setEditItem] = useState(null)
  const [coverOverrides, setCoverOverrides] = useState(() => new Map())
  const displayItems = useMemo(() => {
    if (!Array.isArray(items)) return []
    return items.map((item) => {
      const id = Number(item?.id)
      const override = Number.isFinite(id) ? coverOverrides.get(id) : null
      return override ? { ...item, ...override } : item
    })
  }, [coverOverrides, items])

  const hasItems = displayItems.length > 0
  if (!hasItems) {
    return (
      <div className="flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
        {zh('暂无女优数据', 'No idol data')}
      </div>
    )
  }

  return (
    <>
      <div
        className="grid gap-3 bg-white"
        style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(11rem, 1fr))' }}
      >
        {displayItems.map((item) => (
          <IdolCard
            key={item.id || item.name}
            item={item}
            onSelectIdol={onSelectIdol}
            onOpenFavorites={onOpenFavorites}
            onOpenCoverEditor={setCoverEditorItem}
            onOpenEditor={setEditItem}
            href={buildIdolUrl?.(item)}
            coverAspectPercent={coverAspectPercent}
            javMetadataLanguage={javMetadataLanguage}
            preferChineseName={preferChineseName}
          />
        ))}
      </div>
      <JavIdolCoverModal
        key={coverEditorItem?.id || 'closed'}
        open={Boolean(coverEditorItem)}
        item={coverEditorItem}
        directoryIds={directoryIds}
        javMetadataLanguage={javMetadataLanguage}
        preferChineseName={preferChineseName}
        onClose={() => setCoverEditorItem(null)}
        onSaved={(updated) => {
          const id = Number(updated?.id)
          if (!Number.isFinite(id) || id <= 0) return
          setCoverOverrides((current) => {
            const next = new Map(current)
            next.set(id, updated)
            return next
          })
        }}
      />
      <JavIdolEditModal
        key={editItem?.id || 'closed'}
        open={Boolean(editItem)}
        item={editItem}
        directoryIds={directoryIds}
        javMetadataLanguage={javMetadataLanguage}
        preferChineseName={preferChineseName}
        onClose={() => setEditItem(null)}
        onSaved={(updated) => {
          const id = Number(updated?.id)
          if (!Number.isFinite(id) || id <= 0) return
          setCoverOverrides((current) => {
            const next = new Map(current)
            next.set(id, updated)
            return next
          })
          setEditItem(updated)
        }}
        onMerged={(updated) => {
          setEditItem(null)
          onMerged?.(updated)
        }}
      />
    </>
  )
}

export function IdolCard({
  item,
  onSelectIdol,
  onOpenFavorites,
  onOpenCoverEditor,
  onOpenEditor,
  href,
  coverAspectPercent,
  showWorkCount = true,
  javMetadataLanguage = 'zh',
  preferChineseName = false,
}) {
  const coverCode = String(item?.cover_code || '').trim()
  const cover = coverCode ? `/jav/${encodeURIComponent(coverCode)}/cover` : null
  const coverCropLeft = normalizeIdolCoverCropLeft(
    item?.cover_crop_left ?? IDOL_COVER_DEFAULT_CROP_LEFT
  )
  const coverFrameRef = useRef(null)
  const [coverFrame, setCoverFrame] = useState({ width: 0, height: 0 })
  const [coverImageSize, setCoverImageSize] = useState(null)
  const workCount = item?.work_count || 0
  const favoriteCount = Number(item?.favorite_count) || 0
  const name = item?.name || zh('未知女优', 'Unknown idol')
  const aliases = Array.isArray(item?.aliases) ? item.aliases : []
  const birthDate = formatBirthDateWithAge(item?.birth_date)
  const height = typeof item?.height_cm === 'number' ? `${item.height_cm}cm` : ''
  const bwh = formatBwh(item)
  const cup = formatCup(item?.cup)
  const lookupCode = coverCode
  const [javdbURL, setJavdbURL] = useState(String(item?.javdb_url || '').trim())
  const [javdbOpening, setJavdbOpening] = useState(false)
  const { primaryName, secondaryName } = getIdolDisplayNames(
    item,
    javMetadataLanguage,
    preferChineseName
  )
  const metaRows = buildMetaRows({ birthDate, height, bwh, cup, aliases })
  const canOpenJavDB = Boolean(javdbURL || (lookupCode && name))
  const renderedCoverWidth =
    coverImageSize?.height > 0 && coverFrame.height > 0
      ? coverFrame.height * (coverImageSize.width / coverImageSize.height)
      : 0
  const coverLeft = calculateCoverLeft({
    cropLeft: coverCropLeft,
    frameWidth: coverFrame.width,
    renderedWidth: renderedCoverWidth,
  })

  useEffect(() => {
    setCoverImageSize(null)
  }, [cover])

  useEffect(() => {
    const node = coverFrameRef.current
    if (!node) return undefined

    const updateFrame = () => {
      const rect = node.getBoundingClientRect()
      setCoverFrame({ width: rect.width, height: rect.height })
    }
    updateFrame()

    if (!window.ResizeObserver) {
      window.addEventListener('resize', updateFrame)
      return () => window.removeEventListener('resize', updateFrame)
    }
    const observer = new window.ResizeObserver(updateFrame)
    observer.observe(node)
    return () => observer.disconnect()
  }, [cover])

  const handleClick = (e) => {
    const selection = window.getSelection?.()
    if (selection && String(selection).trim() !== '') {
      e.preventDefault()
      return
    }
    const isModified = e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0
    if (isModified) {
      return
    }
    e.preventDefault()
    onSelectIdol?.(item)
  }

  const handleOpenJavDB = async (event) => {
    event.preventDefault()
    event.stopPropagation()
    if (!canOpenJavDB || javdbOpening) return

    const popup = window.open('about:blank', '_blank')
    if (popup) {
      popup.opener = null
    }

    try {
      setJavdbOpening(true)
      let targetURL = javdbURL
      if (!targetURL) {
        targetURL = await fetchJavIdolJavDBURL({ code: lookupCode, name })
        setJavdbURL(targetURL)
      }
      if (!targetURL) {
        popup?.close()
        return
      }
      if (popup) {
        popup.location.replace(targetURL)
      } else {
        window.open(targetURL, '_blank', 'noopener,noreferrer')
      }
    } catch (error) {
      popup?.close()
      console.warn('open javdb idol failed', error)
    } finally {
      setJavdbOpening(false)
    }
  }

  const handleOpenFavorites = (event) => {
    event.preventDefault()
    event.stopPropagation()
    onOpenFavorites?.(item)
  }

  const handleOpenCoverEditor = (event) => {
    event.preventDefault()
    event.stopPropagation()
    onOpenCoverEditor?.(item)
  }

  const handleOpenEditor = (event) => {
    event.preventDefault()
    event.stopPropagation()
    onOpenEditor?.(item)
  }

  return (
    <a
      href={href || '#'}
      className="group flex cursor-pointer flex-col overflow-hidden rounded-lg border bg-white shadow-sm transition hover:shadow-lg"
      draggable={false}
      onClick={handleClick}
      onKeyDown={(e) => {
        if (e.key === ' ') {
          e.preventDefault()
          onSelectIdol?.(item)
        }
      }}
    >
      <div
        ref={coverFrameRef}
        className="relative w-full overflow-hidden bg-gray-100"
        style={{ paddingTop: `${coverAspectPercent}%` }} // 维持可见区域的原始纵横比，避免压扁
      >
        {cover ? (
          <img
            src={cover}
            alt={primaryName}
            className="absolute top-0 h-full max-w-none select-none"
            style={{
              left: `${coverLeft}px`,
              width: 'auto',
            }}
            draggable={false}
            loading="lazy"
            onLoad={(event) => {
              const img = event.currentTarget
              setCoverImageSize({ width: img.naturalWidth, height: img.naturalHeight })
            }}
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center bg-gradient-to-br from-gray-100 to-gray-200 text-lg font-semibold text-gray-600">
            {primaryName}
          </div>
        )}
        {showWorkCount && (
          <div className="absolute left-2 top-2 rounded bg-black/70 px-2 py-1 text-xs text-white">
            {zh(`作品 ${workCount}`, `${workCount} javs`)}
          </div>
        )}
        <button
          type="button"
          className={`absolute right-2 top-2 flex h-8 w-8 items-center justify-center rounded-full shadow-lg shadow-black/40 transition ${
            favoriteCount > 0
              ? 'bg-amber-400 text-amber-950 hover:bg-amber-300'
              : 'bg-black/65 text-white opacity-0 hover:bg-black/80 group-focus-within:opacity-100 group-hover:opacity-100'
          }`}
          title={zh('加入女优收藏夹', 'Add to idol favorite groups')}
          aria-label={zh('加入女优收藏夹', 'Add to idol favorite groups')}
          onClick={handleOpenFavorites}
        >
          {favoriteCount > 0 ? (
            <StarRoundedIcon sx={{ fontSize: 18 }} />
          ) : (
            <StarBorderRoundedIcon sx={{ fontSize: 18 }} />
          )}
        </button>
        <button
          type="button"
          className={`absolute bottom-2 left-2 flex h-7 w-7 items-center justify-center rounded-full text-white opacity-0 shadow-lg shadow-black/60 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100 ${
            canOpenJavDB ? 'bg-black/70 hover:bg-black/85' : 'cursor-not-allowed bg-black/30'
          }`}
          title={zh('在 JavDB 中打开女优详情', 'Open idol profile in JavDB')}
          aria-label={zh('在 JavDB 中打开女优详情', 'Open idol profile in JavDB')}
          disabled={!canOpenJavDB || javdbOpening}
          onClick={handleOpenJavDB}
        >
          <img
            src="/ico/javdb.png"
            alt="JavDB"
            className={`h-4 w-4 ${javdbOpening ? 'animate-pulse' : ''}`}
            loading="lazy"
          />
        </button>
        <button
          type="button"
          className="absolute bottom-2 right-10 flex h-7 w-7 items-center justify-center rounded-full bg-black/70 text-white opacity-0 shadow-lg shadow-black/60 transition-opacity hover:bg-black/85 group-focus-within:opacity-100 group-hover:opacity-100"
          title={zh('编辑女优封面', 'Edit idol cover')}
          aria-label={zh('编辑女优封面', 'Edit idol cover')}
          onClick={handleOpenCoverEditor}
        >
          <PhotoCameraRoundedIcon sx={{ fontSize: 16 }} />
        </button>
        <button
          type="button"
          className="absolute bottom-2 right-2 flex h-7 w-7 items-center justify-center rounded-full bg-black/70 text-white opacity-0 shadow-lg shadow-black/60 transition-opacity hover:bg-black/85 group-focus-within:opacity-100 group-hover:opacity-100"
          title={zh('编辑女优信息', 'Edit idol info')}
          aria-label={zh('编辑女优信息', 'Edit idol info')}
          onClick={handleOpenEditor}
        >
          <EditRoundedIcon sx={{ fontSize: 16 }} />
        </button>
      </div>
      <div className="flex flex-1 select-text flex-col gap-2 p-3">
        <div className="flex min-w-0 items-baseline gap-1.5 leading-tight">
          <span
            className="min-w-0 max-w-[70%] truncate text-sm font-semibold text-gray-950"
            title={primaryName}
          >
            {primaryName}
          </span>
          {secondaryName ? (
            <span
              className="min-w-0 flex-1 truncate text-[11px] font-normal text-gray-500"
              title={secondaryName}
            >
              {secondaryName}
            </span>
          ) : null}
        </div>
        {metaRows.length > 0 ? (
          <div className="flex flex-col gap-1.5 text-[10px] text-gray-900">
            {metaRows.map((row) => (
              <div
                key={row.key}
                className={`flex gap-1.5 overflow-hidden ${row.wrap ? 'flex-wrap' : 'flex-nowrap'} ${row.className || ''}`}
              >
                {row.items.map((meta) => (
                  <span
                    key={meta.key}
                    className={`inline-flex items-center ${meta.wrap ? 'whitespace-normal break-words' : 'whitespace-nowrap'}`}
                  >
                    {meta.label}
                  </span>
                ))}
              </div>
            ))}
          </div>
        ) : (
          <div className="text-xs text-gray-400">{zh('信息待补充', 'More info coming')}</div>
        )}
      </div>
    </a>
  )
}

export function JavIdolEditModal({
  open,
  item,
  directoryIds = [],
  javMetadataLanguage = 'zh',
  preferChineseName = false,
  onClose,
  onSaved,
  onMerged,
}) {
  const [form, setForm] = useState(() => buildIdolEditForm(item))
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [mergeOpen, setMergeOpen] = useState(false)
  const idolId = Number(item?.id)
  const displayName = displayIdolOptionName(item, javMetadataLanguage, preferChineseName)
  const editingEnglishMetadata = javMetadataLanguage === 'en'

  useEffect(() => {
    if (open) {
      setForm(buildIdolEditForm(item))
      setError('')
      setSaving(false)
      setMergeOpen(false)
    }
  }, [item, open])

  if (!open || !item) return null

  const setField = (key, value) => {
    setForm((current) => ({ ...current, [key]: value }))
  }

  const addAliases = (value) => {
    const incoming = textToList(value)
    if (!incoming.length) return
    setForm((current) => {
      const aliases = mergeAliasLists(current.aliases, incoming)
      return { ...current, aliases, alias_input: '' }
    })
  }

  const removeAlias = (alias) => {
    setForm((current) => ({
      ...current,
      aliases: current.aliases.filter((item) => item !== alias),
    }))
  }

  const handleSubmit = async (event) => {
    event.preventDefault()
    if (!Number.isFinite(idolId) || idolId <= 0 || saving) return
    setSaving(true)
    setError('')
    try {
      const payload = buildIdolEditPayload(form)
      const updated = await updateJavIdol(idolId, payload, { directoryIds })
      const normalizedUpdated = {
        ...updated,
        aliases: Array.isArray(updated?.aliases) ? updated.aliases : [],
      }
      setForm(buildIdolEditForm(normalizedUpdated))
      onSaved?.(normalizedUpdated)
      onClose?.()
    } catch (err) {
      setError(err.message || zh('保存女优信息失败', 'Failed to save idol info'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <div className="fixed inset-0 z-[1600] flex items-center justify-center bg-black/45 p-4">
        <form
          className="flex max-h-[90vh] w-full max-w-2xl flex-col overflow-hidden rounded-lg bg-white shadow-2xl"
          onSubmit={handleSubmit}
        >
          <div className="flex items-center justify-between border-b px-4 py-3">
            <div className="min-w-0">
              <div className="text-base font-semibold text-gray-950">
                {zh('编辑女优信息', 'Edit idol info')}
              </div>
              <div className="truncate text-xs text-gray-500">{displayName}</div>
            </div>
            <button
              type="button"
              className="flex h-8 w-8 items-center justify-center rounded-full text-gray-500 hover:bg-gray-100 hover:text-gray-900"
              aria-label={zh('关闭', 'Close')}
              onClick={onClose}
            >
              <CloseRoundedIcon sx={{ fontSize: 20 }} />
            </button>
          </div>

          <div className="flex-1 overflow-y-auto p-4">
            <div className="grid gap-3 md:grid-cols-2">
              <TextField
                label={zh('名称', 'Name')}
                value={form.name}
                onChange={(value) => setField('name', value)}
                required
              />
              {editingEnglishMetadata ? (
                <TextField
                  label={zh('日文名', 'Japanese name')}
                  value={form.japanese_name}
                  onChange={(value) => setField('japanese_name', value)}
                />
              ) : (
                <TextField
                  label={zh('罗马名', 'Roman name')}
                  value={form.roman_name}
                  onChange={(value) => setField('roman_name', value)}
                />
              )}
              <TextField
                label={zh('中文名', 'Chinese name')}
                value={form.chinese_name}
                onChange={(value) => setField('chinese_name', value)}
              />
              <TextField
                label={zh('身高', 'Height')}
                value={form.height_cm}
                type="number"
                min="1"
                onChange={(value) => setField('height_cm', value)}
              />
              <TextField
                label={zh('生日', 'Birth date')}
                value={form.birth_date}
                type="date"
                onChange={(value) => setField('birth_date', value)}
              />
              <TextField
                label={zh('胸围', 'Bust')}
                value={form.bust}
                type="number"
                min="1"
                onChange={(value) => setField('bust', value)}
              />
              <TextField
                label={zh('腰围', 'Waist')}
                value={form.waist}
                type="number"
                min="1"
                onChange={(value) => setField('waist', value)}
              />
              <TextField
                label={zh('臀围', 'Hips')}
                value={form.hips}
                type="number"
                min="1"
                onChange={(value) => setField('hips', value)}
              />
              <label className="flex flex-col gap-1 text-sm font-medium text-gray-700">
                <span>{zh('罩杯', 'Cup')}</span>
                <select
                  value={form.cup}
                  onChange={(event) => setField('cup', event.target.value)}
                  className="rounded border border-gray-300 bg-white px-3 py-2 text-sm outline-none focus:border-gray-900"
                >
                  <option value="">{zh('未设置', 'Unset')}</option>
                  {Array.from({ length: 26 }, (_, index) => index + 1).map((value) => (
                    <option key={value} value={String(value)}>
                      {String.fromCharCode(64 + value)}
                    </option>
                  ))}
                </select>
              </label>
            </div>

            <AliasEditor
              aliases={form.aliases}
              inputValue={form.alias_input}
              onInputChange={(value) => setField('alias_input', value)}
              onAdd={addAliases}
              onRemove={removeAlias}
            />

            <div className="mt-4 flex flex-wrap gap-2 border-t pt-4">
              <button
                type="button"
                className="rounded border border-gray-300 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50"
                onClick={() => setMergeOpen(true)}
              >
                {zh('合并到其它女优', 'Merge into another idol')}
              </button>
            </div>

            {error ? <div className="mt-3 text-sm text-red-600">{error}</div> : null}
          </div>

          <div className="flex justify-end gap-2 border-t px-4 py-3">
            <button
              type="button"
              className="rounded border border-gray-300 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50"
              onClick={onClose}
              disabled={saving}
            >
              {zh('取消', 'Cancel')}
            </button>
            <button
              type="submit"
              className="rounded bg-gray-950 px-3 py-1.5 text-sm text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-300"
              disabled={saving || !String(form.name || '').trim()}
            >
              {saving ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
            </button>
          </div>
        </form>
      </div>
      <JavIdolMergeModal
        open={mergeOpen}
        item={item}
        directoryIds={directoryIds}
        javMetadataLanguage={javMetadataLanguage}
        preferChineseName={preferChineseName}
        onClose={() => setMergeOpen(false)}
        onMerged={onMerged}
      />
    </>
  )
}

function TextField({ label, value, onChange, type = 'text', required = false, min }) {
  return (
    <label className="flex flex-col gap-1 text-sm font-medium text-gray-700">
      <span>{label}</span>
      <input
        value={value}
        type={type}
        required={required}
        min={min}
        onChange={(event) => onChange?.(event.target.value)}
        className="rounded border border-gray-300 px-3 py-2 text-sm outline-none focus:border-gray-900"
      />
    </label>
  )
}

function AliasEditor({ aliases = [], inputValue = '', onInputChange, onAdd, onRemove }) {
  const commitInput = () => {
    const value = String(inputValue || '')
    if (!value.trim()) return
    onAdd?.(value)
  }

  return (
    <div className="mt-3 flex flex-col gap-1 text-sm font-medium text-gray-700">
      <div className="flex flex-wrap items-center gap-2">
        <span>{zh('别名：', 'Aliases:')}</span>
        <span className="text-xs font-normal text-gray-400">
          {zh('输入后按 Enter 添加', 'Press Enter to add')}
        </span>
      </div>
      <div className="flex min-h-[2.75rem] flex-wrap items-center gap-2 rounded border border-gray-300 bg-white px-2 py-2 focus-within:border-gray-900">
        {aliases.map((alias) => (
          <span
            key={alias}
            className="inline-flex max-w-full items-center gap-1 rounded-full border border-gray-200 bg-gray-100 px-2 py-1 text-xs font-medium text-gray-800"
          >
            <span className="max-w-[12rem] truncate">{alias}</span>
            <button
              type="button"
              className="flex h-4 w-4 items-center justify-center rounded-full text-gray-500 hover:bg-gray-300 hover:text-gray-900"
              aria-label={zh(`移除别名 ${alias}`, `Remove alias ${alias}`)}
              onClick={() => onRemove?.(alias)}
            >
              ×
            </button>
          </span>
        ))}
        <input
          value={inputValue}
          onChange={(event) => {
            const value = event.target.value
            if (/[,\n]/.test(value)) {
              onAdd?.(value)
              return
            }
            onInputChange?.(value)
          }}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault()
              commitInput()
            } else if (event.key === 'Backspace' && !inputValue && aliases.length > 0) {
              event.preventDefault()
              onRemove?.(aliases[aliases.length - 1])
            }
          }}
          onBlur={commitInput}
          className="min-w-[9rem] flex-1 border-0 bg-transparent px-1 py-1 text-sm outline-none"
        />
      </div>
    </div>
  )
}

function JavIdolMergeModal({
  open,
  item,
  directoryIds = [],
  javMetadataLanguage = 'zh',
  preferChineseName = false,
  onClose,
  onMerged,
}) {
  const [search, setSearch] = useState('')
  const [options, setOptions] = useState([])
  const [selectedId, setSelectedId] = useState(0)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const sourceId = Number(item?.id)
  const sourceName = rawIdolName(item)

  useEffect(() => {
    if (!open) {
      setSearch('')
      setOptions([])
      setSelectedId(0)
      setError('')
      setSaving(false)
    }
  }, [open])

  useEffect(() => {
    if (!open) return undefined
    let cancelled = false
    const timer = window.setTimeout(() => {
      setLoading(true)
      setError('')
      fetchJavIdolOptions({ limit: 30, search })
        .then((resp) => {
          if (cancelled) return
          const items = Array.isArray(resp?.items) ? resp.items : []
          setOptions(items.filter((option) => Number(option?.id) !== sourceId))
        })
        .catch((err) => {
          if (!cancelled) setError(err.message || zh('加载女优失败', 'Failed to load idols'))
        })
        .finally(() => {
          if (!cancelled) setLoading(false)
        })
    }, 180)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [open, search, sourceId])

  if (!open || !item) return null

  const selected = options.find((option) => Number(option?.id) === selectedId)
  const selectedName = selected ? rawIdolName(selected) : ''
  const canSubmit =
    Number.isFinite(sourceId) && sourceId > 0 && Number.isFinite(selectedId) && selectedId > 0

  const handleSubmit = async (event) => {
    event.preventDefault()
    if (!canSubmit || saving) return
    setSaving(true)
    setError('')
    try {
      const updated = await mergeJavIdols({
        canonicalId: selectedId,
        mergeIds: [sourceId],
        directoryIds,
      })
      onMerged?.(updated)
    } catch (err) {
      setError(err.message || zh('合并女优失败', 'Failed to merge idols'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-[1700] flex items-center justify-center bg-black/45 p-4">
      <form
        className="flex max-h-[85vh] w-full max-w-lg flex-col overflow-hidden rounded-lg bg-white shadow-2xl"
        onSubmit={handleSubmit}
      >
        <div className="flex items-center justify-between border-b px-4 py-3">
          <div className="min-w-0">
            <div className="text-base font-semibold text-gray-950">
              {zh('合并女优', 'Merge idol')}
            </div>
            <div className="truncate text-xs text-gray-500">
              {zh(`将 ${sourceName} 合并到目标女优`, `Merge ${sourceName} into target idol`)}
            </div>
          </div>
          <button
            type="button"
            className="flex h-8 w-8 items-center justify-center rounded-full text-gray-500 hover:bg-gray-100 hover:text-gray-900"
            aria-label={zh('关闭', 'Close')}
            onClick={onClose}
          >
            <CloseRoundedIcon sx={{ fontSize: 20 }} />
          </button>
        </div>

        <div className="flex flex-1 flex-col gap-3 overflow-hidden p-4">
          <label className="relative block">
            <SearchRoundedIcon
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
              sx={{ fontSize: 18 }}
            />
            <input
              value={search}
              onChange={(event) => {
                setSearch(event.target.value)
                setSelectedId(0)
              }}
              className="w-full rounded border border-gray-300 py-2 pl-9 pr-3 text-sm outline-none focus:border-gray-900"
              placeholder={zh('搜索要合并到的目标女优', 'Search target idol to merge into')}
            />
          </label>

          <div className="min-h-[12rem] overflow-y-auto rounded border border-gray-200">
            {loading ? (
              <div className="flex h-32 items-center justify-center text-sm text-gray-500">
                {zh('加载中…', 'Loading...')}
              </div>
            ) : options.length > 0 ? (
              <div className="divide-y divide-gray-100">
                {options.map((option) => {
                  const id = Number(option?.id)
                  const { primaryName: optionName, secondaryName: optionSecondaryName } =
                    getIdolDisplayNames(option, javMetadataLanguage, preferChineseName)
                  const optionMeta = buildMergeOptionMeta(option)
                  const checked = id === selectedId
                  return (
                    <button
                      key={option.id}
                      type="button"
                      className={`flex w-full flex-col gap-1 px-3 py-2 text-left text-sm hover:bg-gray-50 ${
                        checked ? 'bg-gray-100 text-gray-950' : 'text-gray-800'
                      }`}
                      onClick={() => setSelectedId(id)}
                    >
                      <span className="flex w-full min-w-0 items-center gap-2">
                        <span className="min-w-0 truncate font-medium">{optionName}</span>
                        {optionSecondaryName ? (
                          <span className="shrink-0 truncate text-xs text-gray-500">
                            {optionSecondaryName}
                          </span>
                        ) : null}
                      </span>
                      {optionMeta ? (
                        <span className="w-full truncate text-xs text-gray-500">{optionMeta}</span>
                      ) : null}
                    </button>
                  )
                })}
              </div>
            ) : (
              <div className="flex h-32 items-center justify-center text-sm text-gray-500">
                {zh('没有可合并的目标女优', 'No target idol found')}
              </div>
            )}
          </div>

          {selected ? (
            <div className="rounded border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
              {zh(
                `"${sourceName}" 将作为 "${selectedName}" 的别名存在，当前女优记录会被删除，相关数据迁移会自动完成。此操作无法撤回，请仔细核实后操作。`,
                `"${sourceName}" will exist as an alias of "${selectedName}". The current idol record will be deleted, and related data migration will be completed automatically. This action cannot be undone; verify carefully before continuing.`
              )}
            </div>
          ) : null}

          {error ? <div className="text-sm text-red-600">{error}</div> : null}
        </div>

        <div className="flex justify-end gap-2 border-t px-4 py-3">
          <button
            type="button"
            className="rounded border border-gray-300 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50"
            onClick={onClose}
            disabled={saving}
          >
            {zh('取消', 'Cancel')}
          </button>
          <button
            type="submit"
            className="rounded bg-gray-950 px-3 py-1.5 text-sm text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-300"
            disabled={!canSubmit || saving}
          >
            {saving ? zh('合并中…', 'Merging...') : zh('确认合并', 'Merge')}
          </button>
        </div>
      </form>
    </div>
  )
}

function displayIdolOptionName(item, javMetadataLanguage, preferChineseName = false) {
  return getIdolDisplayNames(item, javMetadataLanguage, preferChineseName).primaryName
}

function rawIdolName(item) {
  return String(item?.name || '').trim() || zh('未知女优', 'Unknown idol')
}

function buildMergeOptionMeta(item) {
  return joinUniqueDisplayParts(
    [
      typeof item?.height_cm === 'number' ? `${item.height_cm}cm` : '',
      formatBirthDateWithAge(item?.birth_date),
      formatBwh(item),
      formatCup(item?.cup),
    ],
    []
  )
}

function buildIdolEditForm(item) {
  return {
    name: String(item?.name || ''),
    roman_name: String(item?.roman_name || ''),
    japanese_name: String(item?.japanese_name || ''),
    chinese_name: String(item?.chinese_name || ''),
    height_cm: valueToInput(item?.height_cm),
    birth_date: formatBirthDate(item?.birth_date),
    bust: valueToInput(item?.bust),
    waist: valueToInput(item?.waist),
    hips: valueToInput(item?.hips),
    cup: valueToInput(item?.cup),
    aliases: mergeAliasLists([], Array.isArray(item?.aliases) ? item.aliases : []),
    alias_input: '',
  }
}

function buildIdolEditPayload(form) {
  const aliases = mergeAliasLists(form.aliases, textToList(form.alias_input))
  return {
    name: String(form.name || '').trim(),
    roman_name: String(form.roman_name || '').trim(),
    japanese_name: String(form.japanese_name || '').trim(),
    chinese_name: String(form.chinese_name || '').trim(),
    height_cm: parseNullableInt(form.height_cm),
    birth_date: String(form.birth_date || '').trim() || null,
    bust: parseNullableInt(form.bust),
    waist: parseNullableInt(form.waist),
    hips: parseNullableInt(form.hips),
    cup: parseNullableInt(form.cup),
    aliases,
  }
}

function valueToInput(value) {
  return typeof value === 'number' && Number.isFinite(value) ? String(value) : ''
}

function parseNullableInt(value) {
  const raw = String(value || '').trim()
  if (!raw) return null
  const parsed = Number.parseInt(raw, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

function textToList(value) {
  return String(value || '')
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function mergeAliasLists(current = [], incoming = []) {
  const seen = new Set()
  const aliases = []
  for (const value of [...current, ...incoming]) {
    const alias = String(value || '').trim()
    if (!alias) continue
    const key = alias.toLocaleLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    aliases.push(alias)
  }
  return aliases
}

function calculateCoverLeft({ cropLeft, frameWidth, renderedWidth }) {
  if (!Number.isFinite(frameWidth) || frameWidth <= 0) return 0
  if (!Number.isFinite(renderedWidth) || renderedWidth <= 0) return 0
  if (renderedWidth <= frameWidth) {
    return (frameWidth - renderedWidth) / 2
  }
  const maxOffset = renderedWidth - frameWidth
  return -Math.min(Math.max(cropLeft * renderedWidth, 0), maxOffset)
}

function formatBirthDate(value) {
  if (!value) return ''
  if (typeof value === 'string') {
    return value.slice(0, 10)
  }
  if (value instanceof Date && !Number.isNaN(value.getTime())) {
    return value.toISOString().slice(0, 10)
  }
  return ''
}

function formatBirthDateWithAge(value) {
  const birthDate = formatBirthDate(value)
  if (!birthDate) return ''

  const age = calculateAge(birthDate)
  if (!Number.isFinite(age) || age < 0) {
    return birthDate
  }
  return zh(`${birthDate}（${age}岁）`, `${birthDate} (${age} years old)`)
}

function calculateAge(birthDate) {
  const date = new Date(`${birthDate}T00:00:00`)
  if (Number.isNaN(date.getTime())) return null

  const now = new Date()
  let age = now.getFullYear() - date.getFullYear()
  const monthDiff = now.getMonth() - date.getMonth()
  const dayDiff = now.getDate() - date.getDate()
  if (monthDiff < 0 || (monthDiff === 0 && dayDiff < 0)) {
    age -= 1
  }
  return age
}

function formatBwh(item) {
  const bust = item?.bust
  const waist = item?.waist
  const hips = item?.hips
  if (typeof bust === 'number' && typeof waist === 'number' && typeof hips === 'number') {
    return `B${bust}-W${waist}-H${hips}`
  }
  return ''
}

function formatCup(value) {
  if (typeof value !== 'number' || value <= 0) return ''
  const letter = String.fromCharCode(64 + value)
  return zh(`${letter}罩杯`, `${letter} cup`)
}

function buildMetaRows({ birthDate, height, bwh, cup, aliases = [] }) {
  const rows = []
  const aliasText = joinUniqueDisplayParts(aliases, [], ', ')
  if (aliasText) {
    rows.push({
      key: 'aliases',
      wrap: true,
      items: [
        {
          key: `aliases-${aliasText}`,
          label: zh(`别名：${aliasText}`, `Alias: ${aliasText}`),
          wrap: true,
        },
      ],
    })
  }
  if (birthDate) {
    rows.push({ key: 'row-2', items: [{ key: `birth-${birthDate}`, label: birthDate }] })
  }

  const rowTwo = []
  if (height) {
    rowTwo.push({ key: `height-${height}`, label: height })
  }
  if (bwh) {
    rowTwo.push({ key: `bwh-${bwh}`, label: bwh })
  }
  if (cup) {
    rowTwo.push({ key: `cup-${cup}`, label: cup })
  }
  if (rowTwo.length > 0) {
    rows.push({ key: 'row-3', items: rowTwo })
  }
  return rows
}

function joinUniqueDisplayParts(values, exclude = [], separator = ' · ') {
  const excluded = new Set(exclude.map((value) => String(value || '').trim()).filter(Boolean))
  const seen = new Set()
  const parts = []
  for (const value of values) {
    const trimmed = String(value || '').trim()
    if (!trimmed || excluded.has(trimmed) || seen.has(trimmed)) {
      continue
    }
    seen.add(trimmed)
    parts.push(trimmed)
  }
  return parts.join(separator)
}
