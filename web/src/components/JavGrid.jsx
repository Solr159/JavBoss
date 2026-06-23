import { useEffect, useMemo, useRef, useState } from 'react'
import { IconButton, Popper, Tooltip } from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'
import { MovieEdit } from '@mui/icons-material'
import ExpandLessIcon from '@mui/icons-material/ExpandLess'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import MovieCreationIcon from '@mui/icons-material/MovieCreation'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import PhotoLibraryOutlinedIcon from '@mui/icons-material/PhotoLibraryOutlined'
import SearchIcon from '@mui/icons-material/Search'
import StarBorderRoundedIcon from '@mui/icons-material/StarBorderRounded'
import StarRoundedIcon from '@mui/icons-material/StarRounded'
import VideocamOutlinedIcon from '@mui/icons-material/VideocamOutlined'
import VideoLibraryOutlinedIcon from '@mui/icons-material/VideoLibraryOutlined'

import {
  fetchJavIdolPreview,
  fetchJavIdolOptions,
  fetchJavJavDBURL,
  fetchJavSeriesPreview,
  fetchJavSeries,
  fetchJavStudioPreview,
  fetchJavStudios,
  updateJavItem,
} from '@/api'
import JavIdolCoverModal from '@/components/JavIdolCoverModal'
import { IdolCard, getIdolCardLayoutProps } from '@/components/JavIdolGrid'
import { SeriesCard } from '@/components/JavSeriesView'
import { StudioCard } from '@/components/JavStudioView'
import VideoGrid from '@/components/VideoGrid'
import { isUserJavTag } from '@/constants/jav'
import { getJavDisplayTitle } from '@/utils/jav'
import { directoryQueryIds, useStore, videoSelectionKey } from '@/store'
import { zh } from '@/utils/i18n'

function DurationIcon() {
  return (
    <svg viewBox="0 0 20 20" aria-hidden="true" className="h-4 w-4 shrink-0">
      <circle cx="10" cy="10" r="7" fill="#F59E0B" />
      <circle cx="10" cy="10" r="5.4" fill="#FEF3C7" />
      <path
        d="M10 6.7v3.5l2.5 1.6"
        fill="none"
        stroke="#7C3AED"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path d="M7.4 2.8h5.2" fill="none" stroke="#EF4444" strokeWidth="1.4" strokeLinecap="round" />
    </svg>
  )
}

function ReleaseIcon() {
  return (
    <svg viewBox="0 0 20 20" aria-hidden="true" className="h-4 w-4 shrink-0">
      <rect x="3.1" y="4.1" width="13.8" height="12.8" rx="2.4" fill="#A78BFA" />
      <rect x="3.9" y="7" width="12.2" height="8.9" rx="1.7" fill="#FFF7ED" />
      <rect
        x="3.1"
        y="4.1"
        width="13.8"
        height="12.8"
        rx="2.4"
        fill="none"
        stroke="#7C3AED"
        strokeWidth="0.8"
      />
      <path
        d="M6.4 3.2v2.8M13.6 3.2v2.8"
        fill="none"
        stroke="#EC4899"
        strokeWidth="1.6"
        strokeLinecap="round"
      />
      <path d="M5.8 8.8h8.4" fill="none" stroke="#F97316" strokeWidth="1.4" strokeLinecap="round" />
      <rect x="6.7" y="10.2" width="2.5" height="2.3" rx="0.5" fill="#22C55E" />
      <rect x="10.7" y="10.2" width="2.5" height="2.3" rx="0.5" fill="#3B82F6" />
      <rect x="6.7" y="13.4" width="2.5" height="2.3" rx="0.5" fill="#F43F5E" />
      <rect x="10.7" y="13.4" width="2.5" height="2.3" rx="0.5" fill="#14B8A6" />
    </svg>
  )
}

export default function JavGrid({
  items,
  columns = 0,
  titleMaxRows = 2,
  idolTagMaxRows = 2,
  tagMaxRows = 2,
  buildJavUrl,
  onPlay,
  onIdolClick,
  onOpenFavorites,
  onOpenJavFavorites,
  onStudioClick,
  onSeriesClick,
  onTagClick,
  onOpenFile,
  openFileLabel,
  onOpenScreenshots,
  onManageVideoPlay,
  onManageVideoOpenFile,
  onManageVideoRevealFile,
  onManageVideoOpenTagPicker,
  onManageVideoOpenScreenshots,
  onManageVideoOpenScrapeSettings,
  onManageVideoRename,
  onManageVideoDelete,
  onManageVideoTagClick,
}) {
  const directoryIds = useStore(directoryQueryIds)
  const javMetadataLanguage = useStore((state) =>
    state.config?.jav_metadata_language === 'en' ? 'en' : 'zh'
  )
  const idolPreviewCacheRef = useRef(new Map())
  const idolPreviewInflightRef = useRef(new Map())
  const studioPreviewCacheRef = useRef(new Map())
  const studioPreviewInflightRef = useRef(new Map())
  const seriesPreviewCacheRef = useRef(new Map())
  const seriesPreviewInflightRef = useRef(new Map())
  const [coverPreview, setCoverPreview] = useState(null)
  const [videoManagerItem, setVideoManagerItem] = useState(null)
  const hasItems = Array.isArray(items) && items.length > 0
  const columnCount = Number.isFinite(Number(columns)) ? Math.floor(Number(columns)) : 0
  const fixedColumnCount = columnCount > 0 ? Math.min(columnCount, 12) : 0
  const gridClassName = 'grid gap-4'
  const gridStyle = fixedColumnCount
    ? { gridTemplateColumns: `repeat(${fixedColumnCount}, minmax(0, 1fr))` }
    : { gridTemplateColumns: 'repeat(auto-fill, minmax(21rem, 1fr))' }

  const loadIdolPreview = async (idol) => {
    const idolId = Number(idol?.id)
    if (!Number.isFinite(idolId) || idolId <= 0) {
      return idol || null
    }

    const cacheKey = `${idolId}|${(directoryIds || []).join(',')}`
    const cached = idolPreviewCacheRef.current.get(cacheKey)
    if (cached) {
      return cached
    }

    const inflight = idolPreviewInflightRef.current.get(cacheKey)
    if (inflight) {
      return inflight
    }

    const request = fetchJavIdolPreview(idolId, { directoryIds })
      .then((preview) => {
        idolPreviewCacheRef.current.set(cacheKey, preview)
        return preview
      })
      .finally(() => {
        idolPreviewInflightRef.current.delete(cacheKey)
      })
    idolPreviewInflightRef.current.set(cacheKey, request)
    return request
  }

  const loadStudioPreview = async (studio) => {
    const studioId = Number(studio?.id)
    if (!Number.isFinite(studioId) || studioId <= 0) {
      return studio || null
    }

    const cacheKey = `${studioId}|${(directoryIds || []).join(',')}`
    const cached = studioPreviewCacheRef.current.get(cacheKey)
    if (cached) {
      return cached
    }

    const inflight = studioPreviewInflightRef.current.get(cacheKey)
    if (inflight) {
      return inflight
    }

    const request = fetchJavStudioPreview(studioId, { directoryIds })
      .then((preview) => {
        studioPreviewCacheRef.current.set(cacheKey, preview)
        return preview
      })
      .finally(() => {
        studioPreviewInflightRef.current.delete(cacheKey)
      })
    studioPreviewInflightRef.current.set(cacheKey, request)
    return request
  }

  const loadSeriesPreview = async (series) => {
    const seriesId = Number(series?.id)
    if (!Number.isFinite(seriesId) || seriesId <= 0) {
      return series || null
    }

    const cacheKey = `${seriesId}|${(directoryIds || []).join(',')}`
    const cached = seriesPreviewCacheRef.current.get(cacheKey)
    if (cached) {
      return cached
    }

    const inflight = seriesPreviewInflightRef.current.get(cacheKey)
    if (inflight) {
      return inflight
    }

    const request = fetchJavSeriesPreview(seriesId, { directoryIds })
      .then((preview) => {
        seriesPreviewCacheRef.current.set(cacheKey, preview)
        return preview
      })
      .finally(() => {
        seriesPreviewInflightRef.current.delete(cacheKey)
      })
    seriesPreviewInflightRef.current.set(cacheKey, request)
    return request
  }

  const handleIdolPreviewUpdated = (updated) => {
    const idolId = Number(updated?.id)
    if (!Number.isFinite(idolId) || idolId <= 0) return
    for (const [key, cached] of idolPreviewCacheRef.current.entries()) {
      if (String(key).startsWith(`${idolId}|`)) {
        idolPreviewCacheRef.current.set(key, { ...cached, ...updated })
      }
    }
  }

  if (!hasItems) {
    return (
      <div className="mt-4 flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
        {zh('暂无 JAV 数据', 'No JAV data')}
      </div>
    )
  }

  return (
    <>
      <div className={gridClassName} style={gridStyle}>
        {items.map((item) => (
          <JavCard
            key={item.id || item.code}
            item={item}
            onPlay={onPlay}
            buildJavUrl={buildJavUrl}
            onIdolClick={onIdolClick}
            onOpenFavorites={onOpenFavorites}
            onOpenJavFavorites={onOpenJavFavorites}
            onStudioClick={onStudioClick}
            onSeriesClick={onSeriesClick}
            onTagClick={onTagClick}
            onOpenFile={onOpenFile}
            openFileLabel={openFileLabel}
            onOpenScreenshots={onOpenScreenshots}
            onOpenVideoManager={setVideoManagerItem}
            loadIdolPreview={loadIdolPreview}
            loadStudioPreview={loadStudioPreview}
            loadSeriesPreview={loadSeriesPreview}
            onIdolPreviewUpdated={handleIdolPreviewUpdated}
            onOpenCoverPreview={setCoverPreview}
            directoryIds={directoryIds}
            javMetadataLanguage={javMetadataLanguage}
            titleMaxRows={titleMaxRows}
            idolTagMaxRows={idolTagMaxRows}
            tagMaxRows={tagMaxRows}
          />
        ))}
      </div>
      {coverPreview ? (
        <CoverPreviewModal preview={coverPreview} onClose={() => setCoverPreview(null)} />
      ) : null}
      <JavVideoManagerModal
        open={Boolean(videoManagerItem)}
        item={videoManagerItem}
        openFileLabel={openFileLabel}
        onClose={() => setVideoManagerItem(null)}
        onPlay={onManageVideoPlay}
        onOpenFile={onManageVideoOpenFile}
        onRevealFile={onManageVideoRevealFile}
        onOpenTagPicker={onManageVideoOpenTagPicker}
        onOpenScreenshots={onManageVideoOpenScreenshots}
        onOpenScrapeSettings={onManageVideoOpenScrapeSettings}
        onRenameVideo={onManageVideoRename}
        onDeleteVideo={onManageVideoDelete}
        onTagClick={onManageVideoTagClick}
      />
    </>
  )
}

function CoverPreviewModal({ preview, onClose }) {
  const [scale, setScale] = useState(1)

  useEffect(() => {
    if (!preview?.src) return undefined

    const previousOverflow = document.body.style.overflow
    const previousHtmlOverflow = document.documentElement.style.overflow
    const handleWheel = (event) => {
      event.preventDefault()
      event.stopPropagation()
      const direction = event.deltaY < 0 ? 1 : -1
      setScale((current) => Math.min(5, Math.max(0.5, current + direction * 0.2)))
    }

    document.body.style.overflow = 'hidden'
    document.documentElement.style.overflow = 'hidden'
    window.addEventListener('wheel', handleWheel, { passive: false, capture: true })

    return () => {
      document.body.style.overflow = previousOverflow
      document.documentElement.style.overflow = previousHtmlOverflow
      window.removeEventListener('wheel', handleWheel, true)
    }
  }, [preview?.src])

  if (!preview?.src) return null

  return (
    <div
      className="fixed inset-0 z-[1500] flex items-center justify-center bg-black/80 p-4"
      role="dialog"
      aria-modal="true"
      aria-label={zh('封面预览', 'Cover preview')}
    >
      <button
        type="button"
        className="absolute inset-0 cursor-default"
        aria-label={zh('关闭封面预览', 'Close cover preview')}
        onClick={onClose}
      />
      <button
        type="button"
        onClick={onClose}
        className="absolute right-4 top-4 z-10 rounded bg-black/50 px-3 py-1 text-xl leading-none text-white hover:bg-black/70"
        aria-label={zh('关闭封面预览', 'Close cover preview')}
      >
        ×
      </button>
      <img
        src={preview.src}
        alt={preview.alt || zh('JAV 封面', 'JAV cover')}
        className="relative z-10 max-h-[92vh] max-w-[94vw] transform-gpu cursor-zoom-in object-contain shadow-2xl"
        style={{ transform: `scale(${scale})` }}
      />
    </div>
  )
}

function formatDateInputFromUnix(value) {
  const unix = Number(value)
  if (!Number.isFinite(unix) || unix <= 0) return ''
  return new Date(unix * 1000).toISOString().slice(0, 10)
}

const JAV_EDIT_FETCH_LIMIT = 500

async function fetchAllJavEditOptions(fetcher, { directoryIds = [] } = {}) {
  const all = []
  let offset = 0
  let total = null
  while (total == null || offset < total) {
    const resp = await fetcher({
      limit: JAV_EDIT_FETCH_LIMIT,
      offset,
      search: '',
      directoryIds,
    })
    const items = Array.isArray(resp?.items) ? resp.items : []
    all.push(...items)
    total = Number.isFinite(Number(resp?.total)) ? Number(resp.total) : all.length
    if (items.length === 0) break
    offset += items.length
  }
  return all
}

function mergeOptionsById(options, selectedOptions) {
  const map = new Map()
  for (const option of [...(selectedOptions || []), ...(options || [])]) {
    const id = Number(option?.id)
    if (Number.isFinite(id) && id > 0) {
      map.set(id, option)
    }
  }
  return Array.from(map.values())
}

function filterOptionsByName(options, search) {
  const q = String(search || '')
    .trim()
    .toLowerCase()
  if (!q) return options
  return (options || []).filter((option) =>
    String(option?.name || '')
      .toLowerCase()
      .includes(q)
  )
}

function includeSelectedOptions(options, allOptions, selectedIds) {
  const selectedSet = new Set((selectedIds || []).map((id) => String(id)))
  if (selectedSet.size === 0) return options
  const map = new Map((options || []).map((option) => [String(option?.id), option]))
  for (const option of allOptions || []) {
    const id = String(option?.id)
    if (selectedSet.has(id) && !map.has(id)) {
      map.set(id, option)
    }
  }
  return Array.from(map.values())
}

function optionById(options, id) {
  const key = String(id || '')
  if (!key) return null
  return (options || []).find((option) => String(option?.id) === key) || null
}

function optionsByIds(options, ids) {
  const lookup = new Map((options || []).map((option) => [String(option?.id), option]))
  return (ids || []).map((id) => lookup.get(String(id))).filter(Boolean)
}

function JavEditDropdown({
  label,
  selectedId,
  options,
  search,
  onSearchChange,
  onSelect,
  open,
  onOpenChange,
  emptyLabel,
  searchPlaceholder,
  disabled,
}) {
  const selected = optionById(options, selectedId)

  return (
    <div className="relative">
      <div className="block text-sm font-medium text-gray-700">{label}</div>
      <button
        type="button"
        className="mt-2 flex w-full items-center justify-between rounded-md border border-gray-300 bg-white px-3 py-2 text-left text-sm text-gray-900 outline-none hover:border-gray-400 focus:border-blue-500 focus:ring-2 focus:ring-blue-100 disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-500"
        onClick={() => onOpenChange?.(!open)}
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="min-w-0 truncate">{selected?.name || emptyLabel}</span>
        <span
          aria-hidden="true"
          className={`ml-2 h-1.5 w-1.5 shrink-0 rotate-45 border-b border-r border-gray-400 transition-transform ${
            open ? 'rotate-[225deg]' : ''
          }`}
        />
      </button>
      {open ? (
        <div className="absolute left-0 right-0 z-20 mt-1 rounded-md border border-gray-200 bg-white p-2 shadow-xl">
          <input
            type="search"
            value={search}
            onChange={(event) => onSearchChange?.(event.target.value)}
            placeholder={searchPlaceholder}
            className="mb-2 w-full rounded border border-gray-300 px-2 py-1.5 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
          />
          <div className="max-h-52 overflow-y-auto" role="listbox">
            <button
              type="button"
              className={`block w-full rounded px-2 py-1.5 text-left text-sm hover:bg-gray-50 ${
                selectedId ? 'text-gray-700' : 'bg-blue-50 text-blue-700'
              }`}
              onClick={() => {
                onSelect?.('')
                onOpenChange?.(false)
              }}
            >
              {emptyLabel}
            </button>
            {options.map((option) => {
              const active = String(option.id) === String(selectedId || '')
              return (
                <button
                  key={option.id}
                  type="button"
                  className={`block w-full rounded px-2 py-1.5 text-left text-sm hover:bg-gray-50 ${
                    active ? 'bg-blue-50 text-blue-700' : 'text-gray-800'
                  }`}
                  onClick={() => {
                    onSelect?.(String(option.id))
                    onOpenChange?.(false)
                  }}
                  role="option"
                  aria-selected={active}
                >
                  {option.name}
                </button>
              )
            })}
            {options.length === 0 ? (
              <div className="px-2 py-1.5 text-sm text-gray-500">
                {zh('没有匹配结果', 'No matches')}
              </div>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  )
}

function SelectedChip({ label, onRemove, disabled }) {
  return (
    <span className="inline-flex min-w-0 items-center gap-1 rounded-full bg-gray-100 px-2 py-1 text-sm text-gray-800">
      <span className="truncate">{label}</span>
      <button
        type="button"
        className="inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-gray-500 hover:bg-gray-200 hover:text-gray-900 disabled:cursor-not-allowed disabled:opacity-50"
        onClick={onRemove}
        disabled={disabled}
        aria-label={zh(`移除 ${label}`, `Remove ${label}`)}
      >
        <CloseOutlinedIcon sx={{ fontSize: 13 }} />
      </button>
    </span>
  )
}

function editableJavTitle(item, javMetadataLanguage) {
  return String(javMetadataLanguage === 'en' ? item?.title_en || '' : item?.title || '')
}

function JavEditModal({ open, item, directoryIds, javMetadataLanguage, onClose, onSaved }) {
  const tagOptions = useStore((state) => state.javTagOptions || [])
  const loadJavTags = useStore((state) => state.loadJavTags)
  const [title, setTitle] = useState('')
  const [coverUrl, setCoverUrl] = useState('')
  const [selectedTagIds, setSelectedTagIds] = useState([])
  const [selectedIdolIds, setSelectedIdolIds] = useState([])
  const [selectedStudioId, setSelectedStudioId] = useState('')
  const [selectedSeriesId, setSelectedSeriesId] = useState('')
  const [idolOptions, setIdolOptions] = useState([])
  const [studioOptions, setStudioOptions] = useState([])
  const [seriesOptions, setSeriesOptions] = useState([])
  const [idolSearch, setIdolSearch] = useState('')
  const [tagSearch, setTagSearch] = useState('')
  const [studioSearch, setStudioSearch] = useState('')
  const [seriesSearch, setSeriesSearch] = useState('')
  const [idolPickerOpen, setIdolPickerOpen] = useState(false)
  const [tagPickerOpen, setTagPickerOpen] = useState(false)
  const [studioDropdownOpen, setStudioDropdownOpen] = useState(false)
  const [seriesDropdownOpen, setSeriesDropdownOpen] = useState(false)
  const [optionsLoading, setOptionsLoading] = useState(false)
  const [optionsError, setOptionsError] = useState('')
  const [releaseDate, setReleaseDate] = useState('')
  const [durationMin, setDurationMin] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const code = String(item?.code || '').trim()
  const itemTitle = item ? getJavDisplayTitle(item, javMetadataLanguage) : ''
  const userTagOptions = useMemo(() => tagOptions.filter((tag) => isUserJavTag(tag)), [tagOptions])
  const currentUserTags = useMemo(
    () => (Array.isArray(item?.tags) ? item.tags.filter((tag) => isUserJavTag(tag)) : []),
    [item?.tags]
  )
  const currentSeries = javMetadataLanguage === 'en' ? item?.series_en : item?.series
  const mergedUserTagOptions = useMemo(
    () => mergeOptionsById(userTagOptions, currentUserTags),
    [currentUserTags, userTagOptions]
  )
  const mergedStudioOptions = useMemo(
    () => mergeOptionsById(studioOptions, item?.studio ? [item.studio] : []),
    [item?.studio, studioOptions]
  )
  const mergedSeriesOptions = useMemo(
    () => mergeOptionsById(seriesOptions, currentSeries ? [currentSeries] : []),
    [currentSeries, seriesOptions]
  )
  const mergedIdolOptions = useMemo(
    () => mergeOptionsById(idolOptions, Array.isArray(item?.idols) ? item.idols : []),
    [idolOptions, item?.idols]
  )
  const visibleStudioOptions = useMemo(
    () =>
      includeSelectedOptions(
        filterOptionsByName(mergedStudioOptions, studioSearch),
        mergedStudioOptions,
        [selectedStudioId]
      ),
    [mergedStudioOptions, selectedStudioId, studioSearch]
  )
  const visibleSeriesOptions = useMemo(
    () =>
      includeSelectedOptions(
        filterOptionsByName(mergedSeriesOptions, seriesSearch),
        mergedSeriesOptions,
        [selectedSeriesId]
      ),
    [mergedSeriesOptions, selectedSeriesId, seriesSearch]
  )
  const visibleIdolOptions = useMemo(
    () =>
      includeSelectedOptions(
        filterOptionsByName(mergedIdolOptions, idolSearch),
        mergedIdolOptions,
        selectedIdolIds
      ),
    [idolSearch, mergedIdolOptions, selectedIdolIds]
  )
  const visibleTagOptions = useMemo(
    () =>
      includeSelectedOptions(
        filterOptionsByName(mergedUserTagOptions, tagSearch),
        mergedUserTagOptions,
        selectedTagIds
      ),
    [mergedUserTagOptions, selectedTagIds, tagSearch]
  )
  const selectedIdolOptions = useMemo(
    () => optionsByIds(mergedIdolOptions, selectedIdolIds),
    [mergedIdolOptions, selectedIdolIds]
  )
  const selectedTagOptions = useMemo(
    () => optionsByIds(mergedUserTagOptions, selectedTagIds),
    [mergedUserTagOptions, selectedTagIds]
  )
  const availableIdolOptions = useMemo(
    () => visibleIdolOptions.filter((idol) => !selectedIdolIds.includes(String(idol.id))),
    [selectedIdolIds, visibleIdolOptions]
  )
  const availableTagOptions = useMemo(
    () => visibleTagOptions.filter((tag) => !selectedTagIds.includes(String(tag.id))),
    [selectedTagIds, visibleTagOptions]
  )

  useEffect(() => {
    if (!open) return
    setTitle(editableJavTitle(item, javMetadataLanguage))
    setCoverUrl('')
    setSelectedTagIds(
      Array.isArray(item?.tags)
        ? item.tags.filter((tag) => isUserJavTag(tag)).map((tag) => String(tag.id))
        : []
    )
    setSelectedIdolIds(
      Array.isArray(item?.idols)
        ? item.idols
            .map((idol) => Number(idol?.id))
            .filter((id) => Number.isFinite(id) && id > 0)
            .map((id) => String(id))
        : []
    )
    setSelectedStudioId(item?.studio?.id ? String(item.studio.id) : '')
    setSelectedSeriesId(currentSeries?.id ? String(currentSeries.id) : '')
    setIdolSearch('')
    setTagSearch('')
    setStudioSearch('')
    setSeriesSearch('')
    setIdolPickerOpen(false)
    setTagPickerOpen(false)
    setStudioDropdownOpen(false)
    setSeriesDropdownOpen(false)
    setOptionsError('')
    setReleaseDate(formatDateInputFromUnix(item?.release_unix))
    setDurationMin(item?.duration_min ? String(item.duration_min) : '')
    setError('')
    setSaving(false)
    void loadJavTags?.({ skipUnchanged: true })
  }, [currentSeries?.id, item, javMetadataLanguage, loadJavTags, open])

  useEffect(() => {
    if (!open) return undefined
    let cancelled = false
    setOptionsLoading(true)
    setOptionsError('')
    Promise.all([
      fetchAllJavEditOptions(fetchJavStudios),
      fetchAllJavEditOptions(fetchJavSeries),
      fetchAllJavEditOptions(fetchJavIdolOptions),
    ])
      .then(([studios, series, idols]) => {
        if (cancelled) return
        setStudioOptions(studios)
        setSeriesOptions(series)
        setIdolOptions(idols)
      })
      .catch((err) => {
        if (cancelled) return
        setOptionsError(err?.message || zh('加载已有数据失败', 'Failed to load existing data'))
        setStudioOptions([])
        setSeriesOptions([])
        setIdolOptions([])
      })
      .finally(() => {
        if (!cancelled) setOptionsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [open])

  if (!open) return null

  const toggleTag = (tagId, checked) => {
    const id = String(tagId)
    setSelectedTagIds((current) => {
      const next = new Set(current)
      if (checked) {
        next.add(id)
      } else {
        next.delete(id)
      }
      return Array.from(next)
    })
  }

  const toggleIdol = (idolId, checked) => {
    const id = String(idolId)
    setSelectedIdolIds((current) => {
      const next = new Set(current)
      if (checked) {
        next.add(id)
      } else {
        next.delete(id)
      }
      return Array.from(next)
    })
  }

  const handleSave = async () => {
    if (!item?.id) {
      setError(zh('缺少 JAV ID', 'Missing JAV ID'))
      return
    }
    const duration = durationMin === '' ? 0 : Math.floor(Number(durationMin))
    if (!Number.isFinite(duration) || duration < 0) {
      setError(zh('时长必须是非负数字', 'Duration must be a non-negative number'))
      return
    }
    setSaving(true)
    setError('')
    const trimmedCoverUrl = coverUrl.trim()
    try {
      const payload = {
        title: title.trim(),
        ...(trimmedCoverUrl ? { cover_url: trimmedCoverUrl } : {}),
        tag_ids: selectedTagIds.map((id) => Number(id)).filter(Boolean),
        idol_ids: selectedIdolIds.map((id) => Number(id)).filter(Boolean),
        studio_id: selectedStudioId ? Number(selectedStudioId) : 0,
        series_id: selectedSeriesId ? Number(selectedSeriesId) : 0,
        release_date: releaseDate,
        duration_min: duration,
      }
      const updated = await updateJavItem(item.id, payload, { directoryIds })
      const normalizedUpdated = {
        ...updated,
        ...(payload.idol_ids.length === 0 ? { idols: [] } : {}),
        ...(payload.tag_ids.length === 0 && !Array.isArray(updated?.tags) ? { tags: [] } : {}),
        ...(payload.studio_id ? {} : { studio_id: null, studio: null }),
        ...(payload.series_id
          ? {}
          : javMetadataLanguage === 'en'
            ? { series_en_id: null, series_en: null }
            : { series_id: null, series: null }),
      }
      onSaved?.(normalizedUpdated, Boolean(trimmedCoverUrl))
    } catch (err) {
      const message = err?.message || zh('保存 JAV 信息失败', 'Failed to save JAV info')
      setError(
        trimmedCoverUrl ? zh(`${message}。请重试。`, `${message}. Please try again.`) : message
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-[1600] flex items-center justify-center bg-black/60 p-4"
      role="dialog"
      aria-modal="true"
      aria-label={zh('编辑 JAV 信息', 'Edit JAV info')}
    >
      <button
        type="button"
        className="absolute inset-0 cursor-default"
        aria-label={zh('关闭', 'Close')}
        onClick={onClose}
      />
      <div className="relative z-10 flex max-h-[90vh] w-full max-w-2xl flex-col rounded-lg bg-white shadow-2xl">
        <div className="mb-4 flex items-center justify-between gap-3">
          <div className="min-w-0 p-5 pb-0">
            <div className="text-base font-semibold text-gray-900">
              {zh('编辑 JAV', 'Edit JAV')}
            </div>
            <div className="mt-1 truncate text-xs text-gray-500">
              {code}
              {itemTitle ? ` · ${itemTitle}` : ''}
            </div>
          </div>
          <button
            type="button"
            className="mr-5 mt-5 rounded px-2 py-1 text-xl leading-none text-gray-500 hover:bg-gray-100 hover:text-gray-900"
            onClick={onClose}
            aria-label={zh('关闭', 'Close')}
          >
            ×
          </button>
        </div>
        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-5 pb-5">
          <div>
            <label
              className="block text-sm font-medium text-gray-700"
              htmlFor={`jav-title-${item?.id || 'new'}`}
            >
              {zh('标题', 'Title')}
            </label>
            <textarea
              id={`jav-title-${item?.id || 'new'}`}
              rows={3}
              value={title}
              onChange={(event) => {
                setTitle(event.target.value)
                if (error) setError('')
              }}
              className="mt-2 w-full resize-y rounded-md border border-gray-300 px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
              disabled={saving}
            />
          </div>
          <div>
            <label
              className="block text-sm font-medium text-gray-700"
              htmlFor={`jav-cover-url-${item?.id || 'new'}`}
            >
              {zh('封面链接', 'Cover URL')}
            </label>
            <input
              id={`jav-cover-url-${item?.id || 'new'}`}
              type="url"
              value={coverUrl}
              onChange={(event) => {
                setCoverUrl(event.target.value)
                if (error) setError('')
              }}
              placeholder="https://..."
              className="mt-2 w-full rounded-md border border-gray-300 px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
              disabled={saving}
            />
            <div className="mt-1 text-xs text-gray-500">
              {zh(
                '当封面缺失或显示错误时，可手动输入封面图片链接；保存后会自动下载到本地并完成更新。',
                'If the cover is missing or incorrect, enter an image URL; saving downloads it locally and updates the cover.'
              )}
            </div>
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="block text-sm font-medium text-gray-700">
              {zh('发行日期', 'Release date')}
              <input
                type="date"
                value={releaseDate}
                onChange={(event) => setReleaseDate(event.target.value)}
                className="mt-2 w-full rounded-md border border-gray-300 px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                disabled={saving}
              />
            </label>
            <label className="block text-sm font-medium text-gray-700">
              {zh('时长（分钟）', 'Duration (min)')}
              <input
                type="number"
                min="0"
                step="1"
                value={durationMin}
                onChange={(event) => setDurationMin(event.target.value)}
                className="mt-2 w-full rounded-md border border-gray-300 px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                disabled={saving}
              />
            </label>
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <JavEditDropdown
              label={zh('厂商', 'Studio')}
              selectedId={selectedStudioId}
              options={visibleStudioOptions}
              search={studioSearch}
              onSearchChange={setStudioSearch}
              onSelect={setSelectedStudioId}
              open={studioDropdownOpen}
              onOpenChange={setStudioDropdownOpen}
              emptyLabel={zh('无厂商', 'No studio')}
              searchPlaceholder={zh('搜索已有厂商', 'Search existing studios')}
              disabled={saving || optionsLoading}
            />
            <JavEditDropdown
              label={zh('系列', 'Series')}
              selectedId={selectedSeriesId}
              options={visibleSeriesOptions}
              search={seriesSearch}
              onSearchChange={setSeriesSearch}
              onSelect={setSelectedSeriesId}
              open={seriesDropdownOpen}
              onOpenChange={setSeriesDropdownOpen}
              emptyLabel={zh('无系列', 'No series')}
              searchPlaceholder={zh('搜索已有系列', 'Search existing series')}
              disabled={saving || optionsLoading}
            />
          </div>
          <div>
            <div className="flex items-center justify-between gap-3">
              <div className="text-sm font-medium text-gray-700">{zh('女优', 'Idols')}</div>
              <button
                type="button"
                className="inline-flex items-center gap-1 rounded-md border border-gray-300 px-2 py-1 text-xs text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
                onClick={() => setIdolPickerOpen((current) => !current)}
                disabled={saving || optionsLoading}
              >
                <AddIcon sx={{ fontSize: 15 }} />
                {zh('新增', 'Add')}
              </button>
            </div>
            {selectedIdolOptions.length > 0 ? (
              <div className="mt-2 flex flex-wrap gap-2">
                {selectedIdolOptions.map((idol) => (
                  <SelectedChip
                    key={idol.id}
                    label={idol.name}
                    disabled={saving}
                    onRemove={() => toggleIdol(idol.id, false)}
                  />
                ))}
              </div>
            ) : null}
            {idolPickerOpen ? (
              <div className="mt-2 rounded-md border border-gray-200 p-2">
                <div className="mb-2 flex items-center gap-2">
                  <input
                    type="search"
                    value={idolSearch}
                    onChange={(event) => setIdolSearch(event.target.value)}
                    placeholder={zh('搜索已有女优', 'Search existing idols')}
                    className="min-w-0 flex-1 rounded border border-gray-300 px-2 py-1.5 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                    disabled={saving || optionsLoading}
                  />
                  <button
                    type="button"
                    className="inline-flex shrink-0 items-center gap-1 rounded-md border border-gray-300 px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50"
                    onClick={() => setIdolPickerOpen(false)}
                  >
                    <CloseOutlinedIcon sx={{ fontSize: 14 }} />
                    {zh('完成', 'Done')}
                  </button>
                </div>
                <div className="max-h-44 overflow-y-auto">
                  {optionsLoading ? (
                    <div className="px-2 py-1 text-sm text-gray-500">
                      {zh('加载中...', 'Loading...')}
                    </div>
                  ) : availableIdolOptions.length === 0 ? (
                    <div className="px-2 py-1 text-sm text-gray-500">
                      {zh('暂无可添加女优', 'No idols to add')}
                    </div>
                  ) : (
                    availableIdolOptions.map((idol) => (
                      <button
                        key={idol.id}
                        type="button"
                        className="block w-full rounded px-2 py-1.5 text-left text-sm text-gray-800 hover:bg-gray-50"
                        onClick={() => toggleIdol(idol.id, true)}
                        disabled={saving}
                      >
                        {idol.name}
                      </button>
                    ))
                  )}
                </div>
              </div>
            ) : null}
          </div>
          {optionsError ? <div className="text-sm text-red-600">{optionsError}</div> : null}
          <div>
            <div className="flex items-center justify-between gap-3">
              <div className="text-sm font-medium text-gray-700">{zh('标签', 'Tags')}</div>
              <button
                type="button"
                className="inline-flex items-center gap-1 rounded-md border border-gray-300 px-2 py-1 text-xs text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
                onClick={() => setTagPickerOpen((current) => !current)}
                disabled={saving}
              >
                <AddIcon sx={{ fontSize: 15 }} />
                {zh('新增', 'Add')}
              </button>
            </div>
            {selectedTagOptions.length > 0 ? (
              <div className="mt-2 flex flex-wrap gap-2">
                {selectedTagOptions.map((tag) => (
                  <SelectedChip
                    key={`${tag.id}-${tag.provider || 0}`}
                    label={tag.name}
                    disabled={saving}
                    onRemove={() => toggleTag(tag.id, false)}
                  />
                ))}
              </div>
            ) : null}
            {tagPickerOpen ? (
              <div className="mt-2 rounded-md border border-gray-200 p-2">
                <div className="mb-2 flex items-center gap-2">
                  <input
                    type="search"
                    value={tagSearch}
                    onChange={(event) => setTagSearch(event.target.value)}
                    placeholder={zh('搜索已有标签', 'Search existing tags')}
                    className="min-w-0 flex-1 rounded border border-gray-300 px-2 py-1.5 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                    disabled={saving}
                  />
                  <button
                    type="button"
                    className="inline-flex shrink-0 items-center gap-1 rounded-md border border-gray-300 px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50"
                    onClick={() => setTagPickerOpen(false)}
                  >
                    <CloseOutlinedIcon sx={{ fontSize: 14 }} />
                    {zh('完成', 'Done')}
                  </button>
                </div>
                <div className="max-h-40 overflow-y-auto">
                  {availableTagOptions.length === 0 ? (
                    <div className="px-2 py-1 text-sm text-gray-500">
                      {zh('暂无可添加标签', 'No tags to add')}
                    </div>
                  ) : (
                    availableTagOptions.map((tag) => (
                      <button
                        key={`${tag.id}-${tag.provider || 0}`}
                        type="button"
                        className="block w-full rounded px-2 py-1.5 text-left text-sm text-gray-800 hover:bg-gray-50"
                        onClick={() => toggleTag(tag.id, true)}
                        disabled={saving}
                      >
                        {tag.name}
                      </button>
                    ))
                  )}
                </div>
              </div>
            ) : null}
          </div>
          {error ? <div className="text-sm text-red-600">{error}</div> : null}
        </div>
        <div className="flex justify-end gap-2 border-t border-gray-200 p-5">
          <button
            type="button"
            className="rounded-md border border-gray-300 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50"
            onClick={onClose}
            disabled={saving}
          >
            {zh('取消', 'Cancel')}
          </button>
          <button
            type="button"
            className={`rounded-md px-4 py-2 text-sm font-medium text-white ${
              saving ? 'cursor-wait bg-blue-400' : 'bg-blue-600 hover:bg-blue-700'
            }`}
            onClick={handleSave}
            disabled={saving}
          >
            {saving ? zh('保存中...', 'Saving...') : zh('保存', 'Save')}
          </button>
        </div>
      </div>
    </div>
  )
}

function JavCoverImage({ src, alt }) {
  return (
    <img src={src} alt={alt} className="h-full w-full object-contain object-top" loading="lazy" />
  )
}

function normalizeIdolTagMaxRows(value) {
  const rows = Math.floor(Number(value))
  return Number.isFinite(rows) && rows > 0 ? Math.min(rows, 12) : 0
}

function normalizeJavTagMaxRows(value) {
  const rows = Math.floor(Number(value))
  return Number.isFinite(rows) && rows > 0 ? Math.min(rows, 12) : 0
}

function normalizeJavTitleMaxRows(value) {
  const rows = Math.floor(Number(value))
  return Number.isFinite(rows) && rows >= 0 ? Math.min(rows, 12) : 2
}

function TagCollapseToggleButton({
  expanded,
  count,
  title,
  expandedClassName,
  collapsedClassName,
  onToggle,
}) {
  const [tooltipOpen, setTooltipOpen] = useState(false)
  const [activeTooltipTitle, setActiveTooltipTitle] = useState(title)
  const className = expanded ? expandedClassName : collapsedClassName

  const button = (
    <button
      type="button"
      onClick={() => {
        setTooltipOpen(false)
        onToggle?.()
      }}
      aria-label={title}
      className={className}
    >
      {expanded ? (
        <ExpandLessIcon sx={{ fontSize: 15 }} />
      ) : (
        <>
          <span>{count}</span>
          <ExpandMoreIcon sx={{ fontSize: 15 }} />
        </>
      )}
    </button>
  )

  return (
    <Tooltip
      title={activeTooltipTitle}
      open={tooltipOpen}
      onOpen={() => {
        setActiveTooltipTitle(title)
        setTooltipOpen(true)
      }}
      onClose={() => setTooltipOpen(false)}
      TransitionProps={{ timeout: 0 }}
    >
      {button}
    </Tooltip>
  )
}

function IdolTagList({
  idols,
  maxRows,
  buildIdolFilterHref,
  onIdolClick,
  onFilterLinkClick,
  onIdolHoverStart,
  onIdolHoverEnd,
}) {
  const measureRef = useRef(null)
  const [expanded, setExpanded] = useState(false)
  const [overflowing, setOverflowing] = useState(false)
  const [visibleCount, setVisibleCount] = useState(idols.length)
  const rowLimit = normalizeIdolTagMaxRows(maxRows)
  const identity = useMemo(
    () => (idols || []).map((idol) => idol?.id || idol?.name || '').join('|'),
    [idols]
  )

  useEffect(() => {
    setExpanded(false)
    setVisibleCount(idols.length)
  }, [identity, idols.length, rowLimit])

  useEffect(() => {
    if (rowLimit <= 0) {
      setOverflowing(false)
      setVisibleCount(idols.length)
      return undefined
    }

    const measureList = measureRef.current
    if (!measureList) return undefined

    const measure = () => {
      const containerWidth = measureList.clientWidth
      const tagNodes = Array.from(measureList.querySelectorAll('[data-idol-tag-measure]'))
      const toggleNode = measureList.querySelector('[data-idol-toggle-measure]')

      if (containerWidth <= 0 || tagNodes.length === 0 || !toggleNode) {
        setOverflowing(false)
        setVisibleCount(idols.length)
        return
      }

      const tagWidths = tagNodes.map((node) => node.offsetWidth)
      const toggleWidth = toggleNode.offsetWidth
      const gap = Number.parseFloat(window.getComputedStyle(measureList).columnGap) || 0
      const fullRows = countFlexRows(tagWidths, 0, containerWidth, gap)
      const isOverflowing = fullRows > rowLimit
      setOverflowing(isOverflowing)
      if (!isOverflowing) {
        setVisibleCount(idols.length)
        return
      }

      let low = 0
      let high = tagWidths.length
      let best = 0
      while (low <= high) {
        const mid = Math.floor((low + high) / 2)
        const rows = countFlexRows(tagWidths.slice(0, mid), toggleWidth, containerWidth, gap)
        if (rows <= rowLimit) {
          best = mid
          low = mid + 1
        } else {
          high = mid - 1
        }
      }
      setVisibleCount(best)
    }

    measure()
    const resizeObserver = typeof ResizeObserver === 'function' ? new ResizeObserver(measure) : null
    resizeObserver?.observe(measureList)
    window.addEventListener('resize', measure)
    return () => {
      resizeObserver?.disconnect()
      window.removeEventListener('resize', measure)
    }
  }, [identity, idols.length, rowLimit])

  const showToggle = rowLimit > 0 && overflowing
  const renderedIdols = showToggle && !expanded ? idols.slice(0, visibleCount) : idols
  const toggleTitle = expanded
    ? zh('点击收回', 'Click to collapse')
    : zh(`共 ${idols.length} 位女优，点击展开`, `${idols.length} actresses total, click to expand`)

  return (
    <div className="relative">
      <div className="flex min-w-0 flex-1 flex-wrap gap-1">
        {renderedIdols.map((idol) => (
          <a
            key={idol.id || idol.name}
            href={buildIdolFilterHref(idol)}
            className="rounded-full bg-purple-100 px-2 py-1 text-xs font-medium text-purple-700 transition hover:bg-purple-200"
            onMouseEnter={(event) => onIdolHoverStart(idol, event)}
            onMouseLeave={onIdolHoverEnd}
            onFocus={(event) => onIdolHoverStart(idol, event)}
            onBlur={onIdolHoverEnd}
            onClick={(event) => onFilterLinkClick(event, () => onIdolClick?.(idol))}
          >
            {idol.name}
          </a>
        ))}
        {showToggle ? (
          <TagCollapseToggleButton
            expanded={expanded}
            count={idols.length}
            title={toggleTitle}
            expandedClassName="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded border border-gray-300 bg-gray-50 text-gray-600 shadow-sm transition hover:border-gray-400 hover:bg-gray-100"
            collapsedClassName="inline-flex h-6 shrink-0 items-center gap-1 rounded-md border border-purple-300 bg-white px-1.5 text-[11px] font-semibold text-purple-700 shadow-sm transition hover:border-purple-500 hover:bg-purple-50"
            onToggle={() => setExpanded((current) => !current)}
          />
        ) : null}
      </div>
      {rowLimit > 0 ? (
        <div
          ref={measureRef}
          aria-hidden="true"
          className="pointer-events-none absolute inset-x-0 top-0 flex flex-wrap gap-1 opacity-0"
        >
          {idols.map((idol) => (
            <span
              key={idol.id || idol.name}
              data-idol-tag-measure
              className="rounded-full bg-purple-100 px-2 py-1 text-xs font-medium"
            >
              {idol.name}
            </span>
          ))}
          <span
            data-idol-toggle-measure
            className="inline-flex h-6 shrink-0 items-center gap-1 rounded-md border px-1.5 text-[11px] font-semibold"
          >
            <span>{idols.length}</span>
            <ExpandMoreIcon sx={{ fontSize: 15 }} />
          </span>
        </div>
      ) : null}
    </div>
  )
}

function countFlexRows(itemWidths, trailingWidth, containerWidth, gap) {
  const widths = trailingWidth > 0 ? [...itemWidths, trailingWidth] : itemWidths
  if (widths.length === 0) return 0

  let rows = 1
  let rowWidth = 0
  for (const width of widths) {
    const nextWidth = rowWidth === 0 ? width : rowWidth + gap + width
    if (rowWidth > 0 && nextWidth > containerWidth) {
      rows += 1
      rowWidth = width
    } else {
      rowWidth = nextWidth
    }
  }
  return rows
}

function JavTagList({ tags, maxRows, buildTagFilterHref, onTagClick, onFilterLinkClick }) {
  const measureRef = useRef(null)
  const [expanded, setExpanded] = useState(false)
  const [overflowing, setOverflowing] = useState(false)
  const [visibleCount, setVisibleCount] = useState(tags.length)
  const rowLimit = normalizeJavTagMaxRows(maxRows)
  const identity = useMemo(
    () => (tags || []).map((tag) => tag?.id || tag?.name || '').join('|'),
    [tags]
  )

  useEffect(() => {
    setExpanded(false)
    setVisibleCount(tags.length)
  }, [identity, tags.length, rowLimit])

  useEffect(() => {
    if (rowLimit <= 0) {
      setOverflowing(false)
      setVisibleCount(tags.length)
      return undefined
    }

    const measureList = measureRef.current
    if (!measureList) return undefined

    const measure = () => {
      const containerWidth = measureList.clientWidth
      const tagNodes = Array.from(measureList.querySelectorAll('[data-jav-tag-measure]'))
      const toggleNode = measureList.querySelector('[data-jav-tag-toggle-measure]')

      if (containerWidth <= 0 || tagNodes.length === 0 || !toggleNode) {
        setOverflowing(false)
        setVisibleCount(tags.length)
        return
      }

      const tagWidths = tagNodes.map((node) => node.offsetWidth)
      const toggleWidth = toggleNode.offsetWidth
      const gap = Number.parseFloat(window.getComputedStyle(measureList).columnGap) || 0
      const fullRows = countFlexRows(tagWidths, 0, containerWidth, gap)
      const isOverflowing = fullRows > rowLimit
      setOverflowing(isOverflowing)
      if (!isOverflowing) {
        setVisibleCount(tags.length)
        return
      }

      let low = 0
      let high = tagWidths.length
      let best = 0
      while (low <= high) {
        const mid = Math.floor((low + high) / 2)
        const rows = countFlexRows(tagWidths.slice(0, mid), toggleWidth, containerWidth, gap)
        if (rows <= rowLimit) {
          best = mid
          low = mid + 1
        } else {
          high = mid - 1
        }
      }
      setVisibleCount(best)
    }

    measure()
    const resizeObserver = typeof ResizeObserver === 'function' ? new ResizeObserver(measure) : null
    resizeObserver?.observe(measureList)
    window.addEventListener('resize', measure)
    return () => {
      resizeObserver?.disconnect()
      window.removeEventListener('resize', measure)
    }
  }, [identity, rowLimit, tags.length])

  const showToggle = rowLimit > 0 && overflowing
  const renderedTags = showToggle && !expanded ? tags.slice(0, visibleCount) : tags
  const toggleTitle = expanded
    ? zh('点击收回', 'Click to collapse')
    : zh(`共 ${tags.length} 个标签，点击展开`, `${tags.length} tags total, click to expand`)

  return (
    <div className="relative">
      <div className="flex min-w-0 flex-1 flex-wrap gap-1">
        {renderedTags.map((tag) => {
          const isUser = isUserJavTag(tag)
          const tagClass = isUser
            ? 'bg-emerald-500 hover:bg-emerald-600'
            : 'bg-orange-500 hover:bg-orange-600'
          return (
            <a
              key={`${tag.id || tag.name}-${tag.provider || 0}`}
              href={buildTagFilterHref(tag)}
              className={`rounded-full px-2 py-1 text-xs font-medium text-white transition ${tagClass}`}
              onClick={(event) => onFilterLinkClick(event, () => onTagClick?.(tag))}
            >
              {tag.name}
            </a>
          )
        })}
        {showToggle ? (
          <TagCollapseToggleButton
            expanded={expanded}
            count={tags.length}
            title={toggleTitle}
            expandedClassName="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded border border-gray-300 bg-gray-50 text-gray-600 shadow-sm transition hover:border-gray-400 hover:bg-gray-100"
            collapsedClassName="inline-flex h-6 shrink-0 items-center gap-1 rounded-md border border-orange-300 bg-white px-1.5 text-[11px] font-semibold text-orange-700 shadow-sm transition hover:border-orange-500 hover:bg-orange-50"
            onToggle={() => setExpanded((current) => !current)}
          />
        ) : null}
      </div>
      {rowLimit > 0 ? (
        <div
          ref={measureRef}
          aria-hidden="true"
          className="pointer-events-none absolute inset-x-0 top-0 flex flex-wrap gap-1 opacity-0"
        >
          {tags.map((tag) => (
            <span
              key={`${tag.id || tag.name}-${tag.provider || 0}`}
              data-jav-tag-measure
              className="rounded-full px-2 py-1 text-xs font-medium"
            >
              {tag.name}
            </span>
          ))}
          <span
            data-jav-tag-toggle-measure
            className="inline-flex h-6 shrink-0 items-center gap-1 rounded-md border px-1.5 text-[11px] font-semibold"
          >
            <span>{tags.length}</span>
            <ExpandMoreIcon sx={{ fontSize: 15 }} />
          </span>
        </div>
      ) : null}
    </div>
  )
}

function JavCard({
  item,
  onPlay,
  buildJavUrl,
  onIdolClick,
  onOpenFavorites,
  onOpenJavFavorites,
  onStudioClick,
  onSeriesClick,
  onTagClick,
  onOpenFile,
  openFileLabel,
  onOpenScreenshots,
  onOpenVideoManager,
  loadIdolPreview,
  loadStudioPreview,
  loadSeriesPreview,
  onIdolPreviewUpdated,
  onOpenCoverPreview,
  directoryIds,
  javMetadataLanguage,
  titleMaxRows,
  idolTagMaxRows,
  tagMaxRows,
}) {
  const primaryVideo = useMemo(() => (item?.videos || [])[0], [item])
  const { coverAspectPercent } = useMemo(() => getIdolCardLayoutProps(), [])
  const code = item?.code?.trim()
  const [coverVersion, setCoverVersion] = useState(0)
  const [editorOpen, setEditorOpen] = useState(false)
  const [javdbURL, setJavdbURL] = useState('')
  const [javdbOpening, setJavdbOpening] = useState(false)
  const coverBase = code ? `/jav/${encodeURIComponent(code)}/cover` : null
  const cover = coverBase ? `${coverBase}${coverVersion ? `?v=${coverVersion}` : ''}` : null

  const release =
    item?.release_unix && Number.isFinite(item.release_unix)
      ? new Date(item.release_unix * 1000)
      : null
  const releaseText = release ? release.toISOString().slice(0, 10) : zh('未知', 'Unknown')
  const durationText = item?.duration_min
    ? zh(`${item.duration_min} 分钟`, `${item.duration_min} min`)
    : ''
  const studioText = String(item?.studio?.name || '').trim()
  const canFilterStudio = studioText && typeof onStudioClick === 'function'
  const preferredSeries = javMetadataLanguage === 'en' ? item?.series_en : item?.series
  const seriesText = String(preferredSeries?.name || '').trim()
  const canFilterSeries = seriesText && typeof onSeriesClick === 'function'
  const codeText = code
  const mainTitle = getJavDisplayTitle(item, javMetadataLanguage)
  const titleText = [codeText, mainTitle].filter(Boolean).join(' ')
  const normalizedTitleMaxRows = normalizeJavTitleMaxRows(titleMaxRows)
  const titleClampStyle =
    normalizedTitleMaxRows > 0
      ? {
          display: '-webkit-box',
          WebkitBoxOrient: 'vertical',
          WebkitLineClamp: normalizedTitleMaxRows,
          overflow: 'hidden',
        }
      : undefined
  const videos = item?.videos || []
  const openableVideos = videos.filter((video) =>
    Boolean(video?.path && (video?.directory?.path || video?.directory_path))
  )
  const canOpen = openableVideos.length > 0
  const encodedCode = code ? encodeURIComponent(code) : ''
  const javdbSearchURL = encodedCode ? `https://javdb.com/search?q=${encodedCode}&f=all` : ''
  const favoriteCount = Number(item?.favorite_count) || 0

  useEffect(() => {
    setJavdbURL('')
    setJavdbOpening(false)
  }, [code])

  const openExternalURL = (popup, targetURL) => {
    if (!targetURL) {
      popup?.close()
      return
    }
    if (popup) {
      popup.location.replace(targetURL)
    } else {
      window.open(targetURL, '_blank', 'noopener,noreferrer')
    }
  }

  const handleOpenJavDB = async (event) => {
    event.preventDefault()
    event.stopPropagation()
    if (!code || !javdbSearchURL || javdbOpening) return

    const popup = window.open('about:blank', '_blank')
    if (popup) {
      popup.opener = null
    }

    try {
      setJavdbOpening(true)
      let targetURL = javdbURL
      if (!targetURL) {
        targetURL = await fetchJavJavDBURL({ code })
        setJavdbURL(targetURL)
      }
      openExternalURL(popup, targetURL || javdbSearchURL)
    } catch (error) {
      console.warn('open javdb movie failed', error)
      openExternalURL(popup, javdbSearchURL)
    } finally {
      setJavdbOpening(false)
    }
  }

  const externalLinks = encodedCode
    ? [
        {
          key: 'javlibrary',
          name: 'JavLibrary',
          href: `https://www.javlibrary.com/cn/vl_searchbyid.php?keyword=${encodedCode}`,
          icon: '/ico/javlibrary.ico',
        },
        {
          key: 'javbus',
          name: 'JavBus',
          href: `https://www.javbus.com/${encodedCode}`,
          icon: '/ico/javbus.ico',
        },
        {
          key: 'javdb',
          name: 'JavDB',
          href: javdbURL || javdbSearchURL,
          icon: '/ico/javdb.png',
          onClick: handleOpenJavDB,
          loading: javdbOpening,
        },
        {
          key: 'missav',
          name: 'MissAV',
          href: `https://missav.ws/cn/${encodedCode}`,
          icon: '/ico/missav.ico',
        },
        {
          key: 'jabel',
          name: 'Jabel',
          href: `https://jable.tv/videos/${encodedCode}/`,
          icon: '/ico/jabel.ico',
        },
      ]
    : []

  const handleOpenFile = (event) => {
    event.stopPropagation()
    if (!canOpen) return
    onOpenFile?.(openableVideos[0] || primaryVideo, item)
  }

  const handleOpenScreenshots = (event) => {
    event.stopPropagation()
    if (!canOpen) return
    onOpenScreenshots?.(openableVideos[0] || primaryVideo, item)
  }

  const handleOpenVideoManager = (event) => {
    event.stopPropagation()
    onOpenVideoManager?.(item)
  }

  const handleOpenCoverPreview = (event) => {
    event.stopPropagation()
    if (!cover) return
    onOpenCoverPreview?.({ src: cover, alt: titleText })
  }

  const handleOpenEditor = (event) => {
    event.stopPropagation()
    setEditorOpen(true)
  }

  const handleOpenJavFavorites = (event) => {
    event.preventDefault()
    event.stopPropagation()
    onOpenJavFavorites?.(item)
  }

  const handleEditorSaved = (updated, coverUpdated) => {
    if (updated?.id) {
      useStore.setState((state) => {
        if (!Array.isArray(state.javItems)) return {}
        return {
          javItems: state.javItems.map((current) =>
            Number(current?.id) === Number(updated.id) ? { ...current, ...updated } : current
          ),
        }
      })
    }
    if (coverUpdated) {
      setCoverVersion(Date.now())
    }
    setEditorOpen(false)
  }

  const canPlay = Boolean(primaryVideo && primaryVideo.id)
  const handlePlay = (event) => {
    event?.stopPropagation()
    if (!canPlay) return
    onPlay?.(primaryVideo, item)
  }
  const tags = useMemo(() => {
    const rawTags = Array.isArray(item?.tags) ? item.tags : []
    const userTags = rawTags.filter((tag) => isUserJavTag(tag))
    const scrapedTags = rawTags.filter((tag) => !isUserJavTag(tag))
    return [...userTags, ...scrapedTags]
  }, [item?.tags])
  const [previewIdol, setPreviewIdol] = useState(null)
  const [idolHoverAnchorEl, setIdolHoverAnchorEl] = useState(null)
  const [previewStudio, setPreviewStudio] = useState(null)
  const [studioHoverAnchorEl, setStudioHoverAnchorEl] = useState(null)
  const [previewSeries, setPreviewSeries] = useState(null)
  const [seriesHoverAnchorEl, setSeriesHoverAnchorEl] = useState(null)
  const [idolCoverEditorItem, setIdolCoverEditorItem] = useState(null)
  const closeTimerRef = useRef(null)
  const activeIdolHoverIdRef = useRef(null)
  const activeStudioHoverIdRef = useRef(null)
  const activeSeriesHoverIdRef = useRef(null)

  const isModifiedClick = (event) =>
    event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0

  const handleFilterLinkClick = (event, action) => {
    event.stopPropagation()
    if (isModifiedClick(event)) return
    event.preventDefault()
    action?.()
  }

  const buildIdolFilterHref = (idol) => {
    const id = Number(idol?.id)
    if (!Number.isFinite(id) || id <= 0) return '#'
    return (
      buildJavUrl?.({
        tab: 'list',
        page: 1,
        search: '',
        idolIds: [id],
        tagIds: [],
        studioId: null,
        studioName: '',
        seriesId: null,
        seriesName: '',
        random: false,
        tempSort: '',
      }) || '#'
    )
  }

  const buildStudioFilterHref = (studio) => {
    const id = Number(studio?.id)
    if (!Number.isFinite(id) || id <= 0) return '#'
    return (
      buildJavUrl?.({
        tab: 'list',
        page: 1,
        search: '',
        idolIds: [],
        tagIds: [],
        studioId: id,
        studioName: studio?.name || '',
        seriesId: null,
        seriesName: '',
        random: false,
        tempSort: '',
      }) || '#'
    )
  }

  const buildSeriesFilterHref = (series) => {
    const id = Number(series?.id)
    if (!Number.isFinite(id) || id <= 0) return '#'
    return (
      buildJavUrl?.({
        tab: 'list',
        page: 1,
        search: '',
        idolIds: [],
        tagIds: [],
        studioId: null,
        studioName: '',
        seriesId: id,
        seriesName: series?.name || '',
        random: false,
        tempSort: '',
      }) || '#'
    )
  }

  const buildTagFilterHref = (tag) => {
    const id = Number(tag?.id)
    if (!Number.isFinite(id) || id <= 0) return '#'
    return (
      buildJavUrl?.({
        tab: 'list',
        page: 1,
        search: '',
        idolIds: [],
        tagIds: [id],
        studioId: null,
        studioName: '',
        seriesId: null,
        seriesName: '',
        random: false,
        tempSort: '',
      }) || '#'
    )
  }

  useEffect(() => {
    return () => {
      if (closeTimerRef.current) {
        window.clearTimeout(closeTimerRef.current)
      }
    }
  }, [])

  const clearHoverCloseTimer = () => {
    if (closeTimerRef.current) {
      window.clearTimeout(closeTimerRef.current)
      closeTimerRef.current = null
    }
  }

  const clearHoverPreview = () => {
    activeIdolHoverIdRef.current = null
    activeStudioHoverIdRef.current = null
    activeSeriesHoverIdRef.current = null
    setPreviewIdol(null)
    setIdolHoverAnchorEl(null)
    setPreviewStudio(null)
    setStudioHoverAnchorEl(null)
    setPreviewSeries(null)
    setSeriesHoverAnchorEl(null)
  }

  const scheduleHoverClose = () => {
    clearHoverCloseTimer()
    closeTimerRef.current = window.setTimeout(() => {
      clearHoverPreview()
      closeTimerRef.current = null
    }, 120)
  }

  const handleIdolHoverStart = (idol, event) => {
    clearHoverCloseTimer()
    const idolId = Number(idol?.id)
    activeIdolHoverIdRef.current = Number.isFinite(idolId) ? idolId : null
    activeStudioHoverIdRef.current = null
    activeSeriesHoverIdRef.current = null
    setPreviewIdol(idol || null)
    setIdolHoverAnchorEl(event.currentTarget)
    setPreviewStudio(null)
    setStudioHoverAnchorEl(null)
    setPreviewSeries(null)
    setSeriesHoverAnchorEl(null)

    void loadIdolPreview?.(idol)
      .then((loadedIdol) => {
        if (!loadedIdol) return
        if (activeIdolHoverIdRef.current !== Number(loadedIdol.id)) return
        setPreviewIdol((current) =>
          current && current.id === loadedIdol.id ? { ...current, ...loadedIdol } : current
        )
      })
      .catch((error) => {
        console.warn('load idol preview failed', error)
      })
  }

  const handleOpenIdolCoverEditor = (idol) => {
    clearHoverCloseTimer()
    setIdolCoverEditorItem(idol)
  }

  const handleIdolCoverSaved = (updated) => {
    const updatedId = Number(updated?.id)
    if (!Number.isFinite(updatedId) || updatedId <= 0) return
    onIdolPreviewUpdated?.(updated)
    setPreviewIdol((current) =>
      current && Number(current.id) === updatedId ? { ...current, ...updated } : current
    )
  }

  const handleStudioHoverStart = (studio, event) => {
    clearHoverCloseTimer()
    const studioId = Number(studio?.id)
    activeStudioHoverIdRef.current = Number.isFinite(studioId) ? studioId : null
    activeIdolHoverIdRef.current = null
    activeSeriesHoverIdRef.current = null
    setPreviewStudio(studio || null)
    setStudioHoverAnchorEl(event.currentTarget)
    setPreviewIdol(null)
    setIdolHoverAnchorEl(null)
    setPreviewSeries(null)
    setSeriesHoverAnchorEl(null)

    void loadStudioPreview?.(studio)
      .then((loadedStudio) => {
        if (!loadedStudio) return
        if (activeStudioHoverIdRef.current !== Number(loadedStudio.id)) return
        setPreviewStudio((current) =>
          current && current.id === loadedStudio.id ? { ...current, ...loadedStudio } : current
        )
      })
      .catch((error) => {
        console.warn('load studio preview failed', error)
      })
  }

  const handleSeriesHoverStart = (series, event) => {
    clearHoverCloseTimer()
    const seriesId = Number(series?.id)
    activeSeriesHoverIdRef.current = Number.isFinite(seriesId) ? seriesId : null
    activeIdolHoverIdRef.current = null
    activeStudioHoverIdRef.current = null
    setPreviewSeries(series || null)
    setSeriesHoverAnchorEl(event.currentTarget)
    setPreviewIdol(null)
    setIdolHoverAnchorEl(null)
    setPreviewStudio(null)
    setStudioHoverAnchorEl(null)

    void loadSeriesPreview?.(series)
      .then((loadedSeries) => {
        if (!loadedSeries) return
        if (activeSeriesHoverIdRef.current !== Number(loadedSeries.id)) return
        setPreviewSeries((current) =>
          current && current.id === loadedSeries.id ? { ...current, ...loadedSeries } : current
        )
      })
      .catch((error) => {
        console.warn('load series preview failed', error)
      })
  }

  const showIdolWorkCount =
    typeof previewIdol?.work_count === 'number' && previewIdol.work_count > 0

  return (
    <>
      <div className="flex flex-col overflow-hidden rounded-lg border bg-white shadow-sm transition hover:shadow-lg">
        <div className="group relative aspect-[800/538] overflow-hidden bg-white">
          {cover ? (
            <JavCoverImage src={cover} alt={item?.code || zh('JAV 封面', 'JAV cover')} />
          ) : (
            <div className="flex h-full w-full items-center justify-center bg-gradient-to-br from-gray-100 to-gray-200 text-lg font-semibold text-gray-600">
              {item?.code || zh('未知番号', 'Unknown code')}
            </div>
          )}
          <div className="pointer-events-none absolute inset-0 flex items-center justify-center bg-black/0 text-white opacity-0 transition-opacity group-hover:opacity-100">
            <button
              onClick={handlePlay}
              disabled={!canPlay}
              className={`pointer-events-auto rounded-full p-3 ${
                canPlay ? 'bg-black/60 hover:bg-black/80' : 'cursor-not-allowed bg-black/30'
              }`}
              aria-label={zh('播放', 'Play')}
              title={zh('播放', 'Play')}
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="currentColor"
                className="h-10 w-10"
              >
                <path d="M8 5v14l11-7z" />
              </svg>
            </button>
          </div>
          {externalLinks.length > 0 ? (
            <div className="absolute left-2 top-2 z-10 flex max-w-[calc(100%-1rem)] items-center gap-1.5 opacity-0 transition-opacity group-hover:opacity-100">
              {externalLinks.map((site) => (
                <Tooltip
                  key={site.key}
                  title={zh(`在 ${site.name} 中打开`, `Open in ${site.name}`)}
                  placement="top"
                  arrow
                >
                  <a
                    href={site.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-black/70 shadow-lg shadow-black/60 transition hover:bg-black/85"
                    aria-label={zh(`在 ${site.name} 中打开`, `Open in ${site.name}`)}
                    onClick={site.onClick || ((event) => event.stopPropagation())}
                  >
                    <img
                      src={site.icon}
                      alt={site.name}
                      className={`h-4 w-4 ${site.loading ? 'animate-pulse' : ''}`}
                      loading="lazy"
                    />
                  </a>
                </Tooltip>
              ))}
            </div>
          ) : null}
          <button
            type="button"
            className={`absolute right-2 top-2 z-10 flex h-8 w-8 items-center justify-center rounded-full shadow-lg shadow-black/40 transition ${
              favoriteCount > 0
                ? 'bg-amber-400 text-amber-950 hover:bg-amber-300'
                : 'bg-black/65 text-white opacity-0 hover:bg-black/80 group-focus-within:opacity-100 group-hover:opacity-100'
            }`}
            title={zh('加入作品收藏夹', 'Add to JAV favorite groups')}
            aria-label={zh('加入作品收藏夹', 'Add to JAV favorite groups')}
            onClick={handleOpenJavFavorites}
          >
            {favoriteCount > 0 ? (
              <StarRoundedIcon sx={{ fontSize: 18 }} />
            ) : (
              <StarBorderRoundedIcon sx={{ fontSize: 18 }} />
            )}
          </button>
          {cover || canOpen ? (
            <div className="absolute bottom-2 left-2 z-10 flex items-center gap-2 opacity-0 transition-opacity group-hover:opacity-100">
              {cover ? (
                <button
                  type="button"
                  onClick={handleOpenCoverPreview}
                  title={zh('查看封面', 'View cover')}
                  aria-label={zh('查看封面', 'View cover')}
                  className="flex h-8 w-8 items-center justify-center rounded-full bg-black/70 text-white shadow-lg shadow-black/60 hover:bg-black/85"
                >
                  <SearchIcon className="h-5 w-5 text-white" fontSize="inherit" />
                </button>
              ) : null}
              <button
                type="button"
                onClick={handleOpenScreenshots}
                disabled={!canOpen}
                title={zh('查看截图', 'View screenshots')}
                aria-label={zh('查看截图', 'View screenshots')}
                className={`flex h-8 w-8 items-center justify-center rounded-full text-white shadow-lg shadow-black/60 ${
                  canOpen ? 'bg-black/70 hover:bg-black/85' : 'cursor-not-allowed bg-black/30'
                }`}
              >
                <PhotoLibraryOutlinedIcon className="h-5 w-5 text-white" fontSize="inherit" />
              </button>
            </div>
          ) : null}
        </div>
        <div className="flex flex-1 flex-col gap-2 p-3">
          <div className="text-sm leading-tight" title={titleText} style={titleClampStyle}>
            {codeText ? <span className="font-semibold text-gray-800">{codeText}</span> : null}
            {codeText ? ' ' : null}
            <span className="font-medium text-gray-800">{mainTitle}</span>
          </div>
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-600">
            <span className="inline-flex items-center gap-1">
              <Tooltip title={zh('时长', 'Duration')} arrow>
                <span className="inline-flex">
                  <DurationIcon />
                </span>
              </Tooltip>
              <span>{durationText || zh('时长未知', 'Unknown duration')}</span>
            </span>
            <span className="inline-flex items-center gap-1">
              <Tooltip title={zh('发行日期', 'Release date')} arrow>
                <span className="inline-flex">
                  <ReleaseIcon />
                </span>
              </Tooltip>
              <span>{releaseText}</span>
            </span>
            {studioText ? (
              <span className="inline-flex min-w-0 items-center gap-1">
                <Tooltip title={zh('片商', 'Studio')} arrow>
                  <span className="inline-flex">
                    <VideocamOutlinedIcon sx={{ fontSize: 16 }} className="shrink-0 text-sky-600" />
                  </span>
                </Tooltip>
                <button
                  type="button"
                  className={`min-w-0 truncate text-left ${
                    canFilterStudio ? 'cursor-pointer hover:text-blue-700 hover:underline' : ''
                  }`}
                  onClick={() => {
                    if (canFilterStudio) onStudioClick(item.studio)
                  }}
                  onMouseEnter={(event) => handleStudioHoverStart(item.studio, event)}
                  onMouseLeave={scheduleHoverClose}
                  onFocus={(event) => handleStudioHoverStart(item.studio, event)}
                  onBlur={scheduleHoverClose}
                  disabled={!canFilterStudio}
                >
                  {studioText}
                </button>
              </span>
            ) : null}
          </div>
          {seriesText ? (
            <div className="flex min-w-0 items-start gap-1 text-xs text-gray-600">
              <Tooltip title={zh('系列', 'Series')} arrow>
                <span className="inline-flex">
                  <MovieCreationIcon sx={{ fontSize: 16 }} className="shrink-0 text-emerald-600" />
                </span>
              </Tooltip>
              <button
                type="button"
                className={`min-w-0 whitespace-normal break-words text-left leading-snug ${
                  canFilterSeries ? 'cursor-pointer hover:text-blue-700 hover:underline' : ''
                }`}
                onClick={() => {
                  if (canFilterSeries) onSeriesClick(preferredSeries)
                }}
                onMouseEnter={(event) => handleSeriesHoverStart(preferredSeries, event)}
                onMouseLeave={scheduleHoverClose}
                onFocus={(event) => handleSeriesHoverStart(preferredSeries, event)}
                onBlur={scheduleHoverClose}
                disabled={!canFilterSeries}
              >
                {seriesText}
              </button>
            </div>
          ) : null}
          <Popper
            open={Boolean(previewStudio && studioHoverAnchorEl)}
            anchorEl={studioHoverAnchorEl}
            placement="right-start"
            className="z-[1400]"
            modifiers={[
              {
                name: 'offset',
                options: {
                  offset: [10, 0],
                },
              },
            ]}
          >
            <div
              className="w-[220px]"
              onMouseEnter={clearHoverCloseTimer}
              onMouseLeave={scheduleHoverClose}
            >
              {previewStudio ? (
                <StudioCard
                  item={previewStudio}
                  href={buildStudioFilterHref(previewStudio)}
                  onSelectStudio={(studio) => onStudioClick?.(studio)}
                />
              ) : null}
            </div>
          </Popper>
          <Popper
            open={Boolean(previewSeries && seriesHoverAnchorEl)}
            anchorEl={seriesHoverAnchorEl}
            placement="right-start"
            className="z-[1400]"
            modifiers={[
              {
                name: 'offset',
                options: {
                  offset: [10, 0],
                },
              },
            ]}
          >
            <div
              className="w-[260px]"
              onMouseEnter={clearHoverCloseTimer}
              onMouseLeave={scheduleHoverClose}
            >
              {previewSeries ? (
                <SeriesCard
                  item={previewSeries}
                  href={buildSeriesFilterHref(previewSeries)}
                  onSelectSeries={(series) => onSeriesClick?.(series)}
                  onSelectStudio={(studio) => onStudioClick?.(studio)}
                />
              ) : null}
            </div>
          </Popper>
          {Array.isArray(item?.idols) && item.idols.length > 0 && (
            <>
              <IdolTagList
                idols={item.idols}
                maxRows={idolTagMaxRows}
                buildIdolFilterHref={buildIdolFilterHref}
                onIdolClick={onIdolClick}
                onFilterLinkClick={handleFilterLinkClick}
                onIdolHoverStart={handleIdolHoverStart}
                onIdolHoverEnd={scheduleHoverClose}
              />
              <Popper
                open={Boolean(previewIdol && idolHoverAnchorEl)}
                anchorEl={idolHoverAnchorEl}
                placement="right-start"
                className="z-[1400]"
                modifiers={[
                  {
                    name: 'offset',
                    options: {
                      offset: [10, 0],
                    },
                  },
                ]}
              >
                <div
                  className="w-[220px]"
                  onMouseEnter={clearHoverCloseTimer}
                  onMouseLeave={scheduleHoverClose}
                >
                  {previewIdol ? (
                    <IdolCard
                      item={previewIdol}
                      onSelectIdol={(idol) => onIdolClick?.(idol)}
                      onOpenFavorites={onOpenFavorites}
                      onOpenCoverEditor={handleOpenIdolCoverEditor}
                      href={buildIdolFilterHref(previewIdol)}
                      coverAspectPercent={coverAspectPercent}
                      showWorkCount={showIdolWorkCount}
                      javMetadataLanguage={javMetadataLanguage}
                    />
                  ) : null}
                </div>
              </Popper>
              <JavIdolCoverModal
                key={idolCoverEditorItem?.id || 'closed'}
                open={Boolean(idolCoverEditorItem)}
                item={idolCoverEditorItem}
                directoryIds={directoryIds}
                javMetadataLanguage={javMetadataLanguage}
                onClose={() => setIdolCoverEditorItem(null)}
                onSaved={handleIdolCoverSaved}
              />
            </>
          )}
          {tags.length > 0 && (
            <JavTagList
              tags={tags}
              maxRows={tagMaxRows}
              buildTagFilterHref={buildTagFilterHref}
              onTagClick={onTagClick}
              onFilterLinkClick={handleFilterLinkClick}
            />
          )}
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex flex-wrap items-center gap-2">
              <Tooltip title={openFileLabel || zh('用默认程序打开', 'Open with default app')}>
                <IconButton
                  size="small"
                  onClick={handleOpenFile}
                  disabled={!canOpen}
                  aria-label={openFileLabel || zh('打开文件', 'Open file')}
                  className="h-6 w-6"
                >
                  <PlayArrowIcon fontSize="inherit" />
                </IconButton>
              </Tooltip>
              <Tooltip title={zh('编辑 JAV', 'Edit JAV')}>
                <IconButton
                  size="small"
                  onClick={handleOpenEditor}
                  aria-label={zh('编辑 JAV', 'Edit JAV')}
                  className="h-6 w-6"
                >
                  <MovieEdit fontSize="inherit" />
                </IconButton>
              </Tooltip>
              <Tooltip title={zh('视频管理', 'Manage videos')}>
                <IconButton
                  size="small"
                  onClick={handleOpenVideoManager}
                  disabled={!Array.isArray(item?.videos) || item.videos.length === 0}
                  aria-label={zh('视频管理', 'Manage videos')}
                  className="h-6 w-6"
                >
                  <VideoLibraryOutlinedIcon fontSize="inherit" />
                </IconButton>
              </Tooltip>
            </div>
            {Array.isArray(item?.videos) && item.videos.length > 1 && (
              <span className="text-xs text-gray-500">
                {zh(`${item.videos.length} 个视频`, `${item.videos.length} video files`)}
              </span>
            )}
          </div>
        </div>
      </div>
      <JavEditModal
        open={editorOpen}
        item={item}
        directoryIds={directoryIds}
        javMetadataLanguage={javMetadataLanguage}
        onClose={() => setEditorOpen(false)}
        onSaved={handleEditorSaved}
      />
    </>
  )
}

function JavVideoManagerModal({
  open,
  item,
  openFileLabel,
  onClose,
  onPlay,
  onOpenFile,
  onRevealFile,
  onOpenTagPicker,
  onOpenScreenshots,
  onOpenScrapeSettings,
  onRenameVideo,
  onDeleteVideo,
  onTagClick,
}) {
  const [selectedIds, setSelectedIds] = useState(() => new Set())

  useEffect(() => {
    if (open) setSelectedIds(new Set())
  }, [item?.id, open])

  if (!open) return null

  const videos = Array.isArray(item?.videos) ? item.videos : []
  const title = getJavDisplayTitle(item)
  const toggleSelectVideo = (video) => {
    const key = videoSelectionKey(video)
    if (!key) return
    setSelectedIds((current) => {
      const next = new Set(current)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 px-4">
      <div className="flex max-h-[90vh] w-full max-w-6xl flex-col rounded-lg bg-white p-4 shadow-xl">
        <div className="mb-3 flex items-center justify-between gap-3">
          <div className="min-w-0">
            <h2 className="truncate text-base font-semibold">{zh('视频管理', 'Manage videos')}</h2>
            <div className="mt-1 truncate text-xs text-gray-500">
              {item?.code || zh('未知番号', 'Unknown code')}
              {title && title !== item?.code ? ` · ${title}` : ''}
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded px-2 py-1 text-gray-500 hover:bg-gray-100"
            aria-label={zh('关闭视频管理', 'Close video manager')}
          >
            ✕
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto pr-1">
          {videos.length > 0 ? (
            <VideoGrid
              videos={videos}
              selectedIds={selectedIds}
              onToggleSelect={toggleSelectVideo}
              showSelection={false}
              onPlay={onPlay}
              onOpenFile={onOpenFile}
              onRevealFile={onRevealFile}
              openFileLabel={openFileLabel}
              onOpenTagPicker={onOpenTagPicker}
              showTagEditor={false}
              onOpenScreenshots={onOpenScreenshots}
              onOpenScrapeSettings={onOpenScrapeSettings}
              onRenameVideo={onRenameVideo}
              onDeleteVideo={onDeleteVideo}
              onTagClick={onTagClick}
            />
          ) : (
            <div className="flex min-h-[160px] items-center justify-center rounded border border-dashed border-gray-200 text-sm text-gray-500">
              {zh('暂无关联视频', 'No linked videos')}
            </div>
          )}
        </div>
        <div className="mt-3 flex justify-end">
          <button
            type="button"
            onClick={onClose}
            className="rounded border px-3 py-1.5 text-sm hover:bg-gray-50"
          >
            {zh('关闭', 'Close')}
          </button>
        </div>
      </div>
    </div>
  )
}
