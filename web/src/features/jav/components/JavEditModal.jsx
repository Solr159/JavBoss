import { useCloseOnOutsidePointer } from '@/shared/hooks/useCloseOnOutsidePointer'
import { useStore } from '@/store'
import { configFlag } from '@/utils/config'
import { useState, useRef, useMemo, useEffect } from 'react'
import {
  mergeOptionsById,
  includeSelectedOptions,
  filterOptionsByName,
  buildStudioSearchText,
  buildIdolSearchText,
  optionsByIds,
  javEditIdolNames,
  javEditScrapedTagNames,
  editableJavTitle,
  formatDateInputFromUnix,
  fetchAllJavEditOptions,
  JavEditDropdown,
  SelectedChip,
  javEditWorkCountLabel,
} from '@/features/jav/components/JavEditorFields'
import { isUserJavTag } from '@/constants/jav'
import { getJavTagDisplayName } from '@/utils/javTag'
import { findJavEditOptionByName } from '@/utils/javEdit'
import {
  fetchJavStudios,
  fetchJavSeries,
  fetchJavIdolOptions,
  createJavIdol,
  createJavScrapedTag,
  createJavTag,
  updateJavItem,
  deleteJavVideos,
} from '@/features/jav/api'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'
import AppModal from '@/shared/ui/AppModal'
import { getIdolDisplayName, getIdolDisplayNames } from '@/utils/javIdol'
import AddIcon from '@mui/icons-material/Add'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'

export function JavEditModal({
  open,
  item,
  preferChineseName = false,
  onClose,
  onSaved,
  onDeleted,
}) {
  const tagOptions = useStore((state) => state.javTagOptions || [])
  const loadJavTags = useStore((state) => state.loadJavTags)
  const showSimplifiedTags = useStore((state) => configFlag(state.config?.jav_tag_show_simplified))
  const [title, setTitle] = useState('')
  const [coverUrl, setCoverUrl] = useState('')
  const [selectedTagIds, setSelectedTagIds] = useState([])
  const [selectedIdolIds, setSelectedIdolIds] = useState([])
  const [selectedScrapedTagIds, setSelectedScrapedTagIds] = useState([])
  const [createdUserTags, setCreatedUserTags] = useState([])
  const [createdScrapedTags, setCreatedScrapedTags] = useState([])
  const [createdIdols, setCreatedIdols] = useState([])
  const [selectedStudioId, setSelectedStudioId] = useState('')
  const [selectedSeriesId, setSelectedSeriesId] = useState('')
  const [idolOptions, setIdolOptions] = useState([])
  const [studioOptions, setStudioOptions] = useState([])
  const [seriesOptions, setSeriesOptions] = useState([])
  const [idolSearch, setIdolSearch] = useState('')
  const [tagSearch, setTagSearch] = useState('')
  const [scrapedTagSearch, setScrapedTagSearch] = useState('')
  const [studioSearch, setStudioSearch] = useState('')
  const [seriesSearch, setSeriesSearch] = useState('')
  const [idolPickerOpen, setIdolPickerOpen] = useState(false)
  const [tagPickerOpen, setTagPickerOpen] = useState(false)
  const [scrapedTagPickerOpen, setScrapedTagPickerOpen] = useState(false)
  const [studioDropdownOpen, setStudioDropdownOpen] = useState(false)
  const [seriesDropdownOpen, setSeriesDropdownOpen] = useState(false)
  const [optionsLoading, setOptionsLoading] = useState(false)
  const [tagOptionsLoading, setTagOptionsLoading] = useState(false)
  const [optionsError, setOptionsError] = useState('')
  const [releaseDate, setReleaseDate] = useState('')
  const [durationMin, setDurationMin] = useState('')
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [creatingUserTag, setCreatingUserTag] = useState(false)
  const [creatingScrapedTag, setCreatingScrapedTag] = useState(false)
  const [creatingIdol, setCreatingIdol] = useState(false)
  const [error, setError] = useState('')
  const idolPickerRef = useRef(null)
  const scrapedTagPickerRef = useRef(null)
  const tagPickerRef = useRef(null)
  useCloseOnOutsidePointer(idolPickerOpen, idolPickerRef, setIdolPickerOpen)
  useCloseOnOutsidePointer(scrapedTagPickerOpen, scrapedTagPickerRef, setScrapedTagPickerOpen)
  useCloseOnOutsidePointer(tagPickerOpen, tagPickerRef, setTagPickerOpen)
  const code = String(item?.code || '').trim()
  const userTagOptions = useMemo(() => tagOptions.filter((tag) => isUserJavTag(tag)), [tagOptions])
  const scrapedTagOptions = useMemo(
    () => tagOptions.filter((tag) => !isUserJavTag(tag)),
    [tagOptions]
  )
  const currentScrapedTags = useMemo(
    () => (Array.isArray(item?.tags) ? item.tags.filter((tag) => !isUserJavTag(tag)) : []),
    [item?.tags]
  )
  const currentUserTags = useMemo(
    () => (Array.isArray(item?.tags) ? item.tags.filter((tag) => isUserJavTag(tag)) : []),
    [item?.tags]
  )
  const currentSeries = item?.series
  const mergedUserTagOptions = useMemo(
    () => mergeOptionsById(userTagOptions, [...currentUserTags, ...createdUserTags]),
    [createdUserTags, currentUserTags, userTagOptions]
  )
  const mergedScrapedTagOptions = useMemo(
    () => mergeOptionsById(scrapedTagOptions, [...currentScrapedTags, ...createdScrapedTags]),
    [createdScrapedTags, currentScrapedTags, scrapedTagOptions]
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
    () =>
      mergeOptionsById(idolOptions, [
        ...(Array.isArray(item?.idols) ? item.idols : []),
        ...createdIdols,
      ]),
    [createdIdols, idolOptions, item?.idols]
  )
  const visibleStudioOptions = useMemo(
    () =>
      includeSelectedOptions(
        filterOptionsByName(mergedStudioOptions, studioSearch, buildStudioSearchText),
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
        filterOptionsByName(mergedIdolOptions, idolSearch, (idol) =>
          buildIdolSearchText(idol, preferChineseName)
        ),
        mergedIdolOptions,
        selectedIdolIds
      ),
    [idolSearch, mergedIdolOptions, preferChineseName, selectedIdolIds]
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
  const visibleScrapedTagOptions = useMemo(
    () =>
      filterOptionsByName(mergedScrapedTagOptions, scrapedTagSearch, (tag) =>
        [tag?.original_name, tag?.name, tag?.simplified_name, getJavTagDisplayName(tag, true)]
          .filter(Boolean)
          .join(' ')
      ),
    [mergedScrapedTagOptions, scrapedTagSearch]
  )
  const selectedIdolOptions = useMemo(
    () => optionsByIds(mergedIdolOptions, selectedIdolIds),
    [mergedIdolOptions, selectedIdolIds]
  )
  const selectedTagOptions = useMemo(
    () => optionsByIds(mergedUserTagOptions, selectedTagIds),
    [mergedUserTagOptions, selectedTagIds]
  )
  const selectedScrapedTagOptions = useMemo(
    () => optionsByIds(mergedScrapedTagOptions, selectedScrapedTagIds),
    [mergedScrapedTagOptions, selectedScrapedTagIds]
  )
  const matchingIdolOption = useMemo(
    () =>
      findJavEditOptionByName(mergedIdolOptions, idolSearch, (idol) =>
        javEditIdolNames(idol, preferChineseName)
      ),
    [idolSearch, mergedIdolOptions, preferChineseName]
  )
  const matchingScrapedTagOption = useMemo(
    () =>
      findJavEditOptionByName(mergedScrapedTagOptions, scrapedTagSearch, (tag) =>
        javEditScrapedTagNames(tag, showSimplifiedTags)
      ),
    [mergedScrapedTagOptions, scrapedTagSearch, showSimplifiedTags]
  )
  const matchingUserTagOption = useMemo(
    () => findJavEditOptionByName(mergedUserTagOptions, tagSearch),
    [mergedUserTagOptions, tagSearch]
  )
  const availableIdolOptions = useMemo(
    () => visibleIdolOptions.filter((idol) => !selectedIdolIds.includes(String(idol.id))),
    [selectedIdolIds, visibleIdolOptions]
  )
  const availableTagOptions = useMemo(
    () => visibleTagOptions.filter((tag) => !selectedTagIds.includes(String(tag.id))),
    [selectedTagIds, visibleTagOptions]
  )
  const availableScrapedTagOptions = useMemo(
    () => visibleScrapedTagOptions.filter((tag) => !selectedScrapedTagIds.includes(String(tag.id))),
    [selectedScrapedTagIds, visibleScrapedTagOptions]
  )

  useEffect(() => {
    if (!open) return undefined
    let cancelled = false
    setTitle(editableJavTitle(item))
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
    setSelectedScrapedTagIds(currentScrapedTags.map((tag) => String(tag.id)))
    setCreatedUserTags([])
    setCreatedScrapedTags([])
    setCreatedIdols([])
    setSelectedStudioId(item?.studio?.id ? String(item.studio.id) : '')
    setSelectedSeriesId(currentSeries?.id ? String(currentSeries.id) : '')
    setIdolSearch('')
    setTagSearch('')
    setScrapedTagSearch('')
    setStudioSearch('')
    setSeriesSearch('')
    setIdolPickerOpen(false)
    setTagPickerOpen(false)
    setScrapedTagPickerOpen(false)
    setStudioDropdownOpen(false)
    setSeriesDropdownOpen(false)
    setOptionsError('')
    setReleaseDate(formatDateInputFromUnix(item?.release_unix))
    setDurationMin(item?.duration_min ? String(item.duration_min) : '')
    setError('')
    setSaving(false)
    setCreatingUserTag(false)
    setCreatingScrapedTag(false)
    setCreatingIdol(false)
    setTagOptionsLoading(true)
    Promise.resolve(loadJavTags?.({ skipUnchanged: true })).finally(() => {
      if (!cancelled) setTagOptionsLoading(false)
    })
    return () => {
      cancelled = true
    }
  }, [currentScrapedTags, currentSeries?.id, item, loadJavTags, open])

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
        setOptionsError(getErrorMessage(err))
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

  const addIdol = async () => {
    const name = idolSearch.trim()
    if (!name || creatingIdol || optionsLoading) return
    if (matchingIdolOption?.id) {
      toggleIdol(matchingIdolOption.id, true)
      setIdolSearch('')
      return
    }
    setCreatingIdol(true)
    setError('')
    try {
      const created = await createJavIdol(name)
      if (!created?.id) throw new Error(zh('创建女优失败', 'Failed to create idol'))
      setCreatedIdols((current) => mergeOptionsById(current, [created]))
      toggleIdol(created.id, true)
      setIdolSearch('')
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setCreatingIdol(false)
    }
  }

  const addScrapedTag = async () => {
    const name = scrapedTagSearch.trim()
    if (!name || creatingScrapedTag || tagOptionsLoading) return
    if (matchingScrapedTagOption?.id) {
      setSelectedScrapedTagIds((current) =>
        Array.from(new Set([...current, String(matchingScrapedTagOption.id)]))
      )
      setScrapedTagSearch('')
      return
    }
    setCreatingScrapedTag(true)
    setError('')
    try {
      const created = await createJavScrapedTag(name)
      if (!created?.id) {
        throw new Error(zh('创建刮削标签失败', 'Failed to create scraped tag'))
      }
      setCreatedScrapedTags((current) => mergeOptionsById(current, [created]))
      setSelectedScrapedTagIds((current) => Array.from(new Set([...current, String(created.id)])))
      setScrapedTagSearch('')
      void loadJavTags?.({ force: true })
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setCreatingScrapedTag(false)
    }
  }

  const addCustomTag = async (value = tagSearch) => {
    const name = value.trim()
    if (!name || creatingUserTag) return
    if (matchingUserTagOption?.id) {
      toggleTag(matchingUserTagOption.id, true)
      setTagSearch('')
      return
    }
    setCreatingUserTag(true)
    setError('')
    try {
      const created = await createJavTag(name)
      if (!created?.id) throw new Error(zh('创建自定义标签失败', 'Failed to create custom tag'))
      setCreatedUserTags((current) => mergeOptionsById(current, [created]))
      toggleTag(created.id, true)
      setTagSearch('')
      void loadJavTags?.({ force: true })
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setCreatingUserTag(false)
    }
  }

  const handleSave = async () => {
    if (saving || deleting) return
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
        scraped_tag_ids: selectedScrapedTagIds.map((id) => Number(id)).filter(Boolean),
        studio_id: selectedStudioId ? Number(selectedStudioId) : 0,
        series_id: selectedSeriesId ? Number(selectedSeriesId) : 0,
        release_date: releaseDate,
        duration_min: duration,
      }
      const updated = await updateJavItem(item.id, payload)
      void loadJavTags?.({ force: true })
      const normalizedUpdated = {
        ...updated,
        ...(payload.idol_ids.length === 0 ? { idols: [] } : {}),
        ...(payload.tag_ids.length === 0 &&
        payload.scraped_tag_ids.length === 0 &&
        !Array.isArray(updated?.tags)
          ? { tags: [] }
          : {}),
        ...(payload.studio_id ? {} : { studio_id: null, studio: null }),
        ...(payload.series_id ? {} : { series_id: null, series: null }),
      }
      onSaved?.(normalizedUpdated, Boolean(trimmedCoverUrl))
    } catch (err) {
      const message = getErrorMessage(err)
      setError(
        trimmedCoverUrl ? zh(`${message}。请重试。`, `${message}. Please try again.`) : message
      )
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!item?.id || saving || deleting) return
    if (
      !window.confirm(
        zh(
          `确定删除“${code}”的吗？所有相关文件（包括视频、同名字幕、NFO、视频截图）和记录都会被删除。`,
          `Delete “${code}”? All related files (including videos, matching subtitles, NFO files and video screenshots) and records will be deleted.`
        )
      )
    )
      return
    setDeleting(true)
    setError('')
    try {
      const result = await deleteJavVideos(item.id)
      onDeleted?.(item.id, result.video_ids || [])
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setDeleting(false)
    }
  }

  const creatingOption = creatingIdol || creatingScrapedTag || creatingUserTag || deleting

  return (
    <AppModal
      ariaLabel={zh('编辑 JAV 信息', 'Edit JAV info')}
      className="p-4"
      closeDisabled={saving || creatingOption}
      contentClassName="flex max-h-[90vh] w-full max-w-2xl flex-col rounded-lg bg-white shadow-2xl"
      onClose={onClose}
      zIndex={1600}
    >
      <div className="mb-4 flex items-center gap-2 px-5 pt-5">
        <div className="shrink-0">
          <div className="text-base font-semibold text-gray-900">{zh('编辑 JAV', 'Edit JAV')}</div>
        </div>
        {code ? (
          <div className="max-w-[50%] truncate text-xs font-medium text-gray-700">{code}</div>
        ) : null}
        <button
          type="button"
          className="ml-auto rounded px-2 py-1 text-xl leading-none text-gray-500 hover:bg-gray-100 hover:text-gray-900"
          onClick={onClose}
          disabled={saving || creatingOption}
          aria-label={zh('关闭', 'Close')}
        >
          ×
        </button>
      </div>
      <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-5 pb-5">
        <div>
          <label
            className="block text-[13px] font-semibold text-black"
            htmlFor={`jav-title-${item?.id || 'new'}`}
          >
            {zh('标题', 'Title')}
          </label>
          <textarea
            id={`jav-title-${item?.id || 'new'}`}
            rows={2}
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
            className="block text-[13px] font-semibold text-black"
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
          <label className="block text-[13px] font-semibold text-black">
            {zh('发行日期', 'Release date')}
            <input
              type="date"
              value={releaseDate}
              onChange={(event) => setReleaseDate(event.target.value)}
              className="mt-2 w-full rounded-md border border-gray-300 px-3 py-2 text-sm font-medium text-gray-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
              disabled={saving}
            />
          </label>
          <label className="block text-[13px] font-semibold text-black">
            {zh('时长（分钟）', 'Duration (min)')}
            <input
              type="number"
              min="0"
              step="1"
              value={durationMin}
              onChange={(event) => setDurationMin(event.target.value)}
              className="mt-2 w-full rounded-md border border-gray-300 px-3 py-2 text-sm font-medium text-gray-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
              disabled={saving}
            />
          </label>
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          <JavEditDropdown
            label={zh('片商', 'Studio')}
            selectedId={selectedStudioId}
            options={visibleStudioOptions}
            search={studioSearch}
            onSearchChange={setStudioSearch}
            onSelect={setSelectedStudioId}
            open={studioDropdownOpen}
            onOpenChange={setStudioDropdownOpen}
            emptyLabel={zh('无片商', 'No studio')}
            searchPlaceholder={zh('搜索已有片商', 'Search existing studios')}
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
        <div ref={idolPickerRef}>
          <div className="text-[13px] font-semibold text-black">{zh('女优', 'Idols')}</div>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            {selectedIdolOptions.map((idol) => (
              <SelectedChip
                key={idol.id}
                label={getIdolDisplayName(idol, preferChineseName)}
                disabled={saving}
                compact
                onRemove={() => toggleIdol(idol.id, false)}
              />
            ))}
            <button
              type="button"
              className="inline-flex h-5 w-5 items-center justify-center rounded-full border border-gray-300 text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
              onClick={() => {
                setIdolPickerOpen((current) => !current)
                setScrapedTagPickerOpen(false)
                setTagPickerOpen(false)
                setScrapedTagSearch('')
                setTagSearch('')
              }}
              disabled={saving || creatingIdol}
              title={zh('新增女优', 'Add idol')}
              aria-label={zh('新增女优', 'Add idol')}
            >
              <AddIcon sx={{ fontSize: 13 }} />
            </button>
          </div>
          {idolPickerOpen ? (
            <div className="mt-2 rounded-md border border-gray-200 p-2">
              <div className="mb-2 flex items-center gap-2">
                <input
                  type="text"
                  value={idolSearch}
                  onChange={(event) => setIdolSearch(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key !== 'Enter' || event.nativeEvent.isComposing) return
                    event.preventDefault()
                    void addIdol()
                  }}
                  placeholder={zh('搜索或输入女优名称', 'Search or enter an idol name')}
                  className="min-w-0 flex-1 rounded border border-gray-300 px-2 py-1.5 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                  disabled={saving || creatingIdol || optionsLoading}
                />
                <button
                  type="button"
                  className="inline-flex shrink-0 items-center gap-1 rounded-md border border-gray-300 px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50"
                  onClick={() => {
                    setIdolSearch('')
                    setIdolPickerOpen(false)
                  }}
                >
                  <CloseOutlinedIcon sx={{ fontSize: 14 }} />
                  {zh('完成', 'Done')}
                </button>
              </div>
              <div className="max-h-44 overflow-y-auto">
                {!optionsLoading && idolSearch.trim() && !matchingIdolOption ? (
                  <button
                    type="button"
                    className="mb-1 flex w-full items-center gap-1 rounded bg-blue-50 px-2 py-1.5 text-left text-sm text-blue-700 hover:bg-blue-100"
                    onClick={() => void addIdol()}
                    disabled={saving || creatingIdol}
                  >
                    <AddIcon sx={{ fontSize: 15 }} />
                    {creatingIdol
                      ? zh('创建中...', 'Creating...')
                      : zh(`新建“${idolSearch.trim()}”`, `Create “${idolSearch.trim()}”`)}
                  </button>
                ) : null}
                {optionsLoading ? (
                  <div className="px-2 py-1 text-sm text-gray-500">
                    {zh('加载中...', 'Loading...')}
                  </div>
                ) : availableIdolOptions.length === 0 && !idolSearch.trim() ? (
                  <div className="px-2 py-1 text-sm text-gray-500">
                    {zh('暂无可添加女优', 'No idols to add')}
                  </div>
                ) : (
                  availableIdolOptions.map((idol) => {
                    const { primaryName, secondaryName } = getIdolDisplayNames(idol, false)
                    return (
                      <button
                        key={idol.id}
                        type="button"
                        className="flex w-full items-center gap-3 rounded px-2 py-1.5 text-left text-sm text-gray-800 hover:bg-gray-50"
                        onClick={() => toggleIdol(idol.id, true)}
                        disabled={saving}
                      >
                        <span className="flex min-w-0 flex-1 items-baseline gap-2">
                          <span className="truncate">{primaryName}</span>
                          {secondaryName ? (
                            <span className="truncate text-xs text-gray-500">{secondaryName}</span>
                          ) : null}
                        </span>
                        <span className="shrink-0 text-xs tabular-nums text-gray-400">
                          {javEditWorkCountLabel(idol?.work_count)}
                        </span>
                      </button>
                    )
                  })
                )}
              </div>
            </div>
          ) : null}
        </div>
        {optionsError ? <div className="text-sm text-red-600">{optionsError}</div> : null}
        <div ref={scrapedTagPickerRef}>
          <div className="text-[13px] font-semibold text-black">
            {zh('刮削标签', 'Scraped tags')}
          </div>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            {selectedScrapedTagOptions.map((tag) => (
              <SelectedChip
                key={tag.id}
                label={getJavTagDisplayName(tag, showSimplifiedTags)}
                disabled={saving}
                compact
                onRemove={() =>
                  setSelectedScrapedTagIds((current) =>
                    current.filter((id) => id !== String(tag.id))
                  )
                }
              />
            ))}
            <button
              type="button"
              className="inline-flex h-5 w-5 items-center justify-center rounded-full border border-gray-300 text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
              onClick={() => {
                setScrapedTagPickerOpen((current) => !current)
                setIdolPickerOpen(false)
                setTagPickerOpen(false)
                setIdolSearch('')
                setTagSearch('')
              }}
              disabled={saving || creatingScrapedTag}
              title={zh('新增刮削标签', 'Add scraped tag')}
              aria-label={zh('新增刮削标签', 'Add scraped tag')}
            >
              <AddIcon sx={{ fontSize: 13 }} />
            </button>
          </div>
          {scrapedTagPickerOpen ? (
            <div className="mt-2 rounded-md border border-gray-200 p-2">
              <div className="mb-2 flex items-center gap-2">
                <input
                  type="text"
                  value={scrapedTagSearch}
                  onChange={(event) => setScrapedTagSearch(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key !== 'Enter' || event.nativeEvent.isComposing) return
                    event.preventDefault()
                    void addScrapedTag()
                  }}
                  placeholder={zh('搜索或输入刮削标签', 'Search or enter a scraped tag')}
                  className="min-w-0 flex-1 rounded border border-gray-300 px-2 py-1.5 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                  disabled={saving || creatingScrapedTag || tagOptionsLoading}
                />
                <button
                  type="button"
                  className="inline-flex shrink-0 items-center gap-1 rounded-md border border-gray-300 px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50"
                  onClick={() => {
                    setScrapedTagSearch('')
                    setScrapedTagPickerOpen(false)
                  }}
                >
                  <CloseOutlinedIcon sx={{ fontSize: 14 }} />
                  {zh('完成', 'Done')}
                </button>
              </div>
              <div className="max-h-40 overflow-y-auto">
                {!tagOptionsLoading && scrapedTagSearch.trim() && !matchingScrapedTagOption ? (
                  <button
                    type="button"
                    className="mb-1 flex w-full items-center gap-1 rounded bg-blue-50 px-2 py-1.5 text-left text-sm text-blue-700 hover:bg-blue-100"
                    onClick={() => void addScrapedTag()}
                    disabled={saving || creatingScrapedTag}
                  >
                    <AddIcon sx={{ fontSize: 15 }} />
                    {creatingScrapedTag
                      ? zh('创建中...', 'Creating...')
                      : zh(
                          `新建“${scrapedTagSearch.trim()}”`,
                          `Create “${scrapedTagSearch.trim()}”`
                        )}
                  </button>
                ) : null}
                {tagOptionsLoading ? (
                  <div className="px-2 py-1 text-sm text-gray-500">
                    {zh('加载中...', 'Loading...')}
                  </div>
                ) : availableScrapedTagOptions.length === 0 && !scrapedTagSearch.trim() ? (
                  <div className="px-2 py-1 text-sm text-gray-500">
                    {zh('暂无可添加刮削标签', 'No scraped tags to add')}
                  </div>
                ) : (
                  availableScrapedTagOptions.map((tag) => (
                    <button
                      key={`${tag.id}-${tag.name}`}
                      type="button"
                      className="flex w-full items-center gap-3 rounded px-2 py-1.5 text-left text-sm text-gray-800 hover:bg-gray-50"
                      onClick={() =>
                        setSelectedScrapedTagIds((current) =>
                          Array.from(new Set([...current, String(tag.id)]))
                        )
                      }
                      disabled={saving}
                    >
                      <span className="min-w-0 flex-1 truncate">
                        {getJavTagDisplayName(tag, showSimplifiedTags)}
                      </span>
                      <span className="shrink-0 text-xs tabular-nums text-gray-400">
                        {javEditWorkCountLabel(tag?.count)}
                      </span>
                    </button>
                  ))
                )}
              </div>
            </div>
          ) : null}
        </div>
        <div ref={tagPickerRef}>
          <div className="text-[13px] font-semibold text-black">
            {zh('自定义标签', 'Custom tags')}
          </div>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            {selectedTagOptions.map((tag) => (
              <SelectedChip
                key={`${tag.id}-${tag.provider || 0}`}
                label={tag.name}
                disabled={saving}
                compact
                onRemove={() => toggleTag(tag.id, false)}
              />
            ))}
            <button
              type="button"
              className="inline-flex h-5 w-5 items-center justify-center rounded-full border border-gray-300 text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
              onClick={() => {
                setTagPickerOpen((current) => !current)
                setIdolPickerOpen(false)
                setScrapedTagPickerOpen(false)
                setIdolSearch('')
                setScrapedTagSearch('')
              }}
              disabled={saving || creatingUserTag}
              title={zh('新增自定义标签', 'Add custom tag')}
              aria-label={zh('新增自定义标签', 'Add custom tag')}
            >
              <AddIcon sx={{ fontSize: 13 }} />
            </button>
          </div>
          {tagPickerOpen ? (
            <div className="mt-2 rounded-md border border-gray-200 p-2">
              <div className="mb-2 flex items-center gap-2">
                <input
                  type="text"
                  value={tagSearch}
                  onChange={(event) => setTagSearch(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key !== 'Enter' || event.nativeEvent.isComposing) return
                    event.preventDefault()
                    void addCustomTag()
                  }}
                  placeholder={zh('搜索或输入自定义标签', 'Search or enter a custom tag')}
                  className="min-w-0 flex-1 rounded border border-gray-300 px-2 py-1.5 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                  disabled={saving || creatingUserTag}
                />
                <button
                  type="button"
                  className="inline-flex shrink-0 items-center gap-1 rounded-md border border-gray-300 px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50"
                  onClick={() => {
                    setTagSearch('')
                    setTagPickerOpen(false)
                  }}
                >
                  <CloseOutlinedIcon sx={{ fontSize: 14 }} />
                  {zh('完成', 'Done')}
                </button>
              </div>
              <div className="max-h-40 overflow-y-auto">
                {tagSearch.trim() && !matchingUserTagOption ? (
                  <button
                    type="button"
                    className="mb-1 flex w-full items-center gap-1 rounded bg-blue-50 px-2 py-1.5 text-left text-sm text-blue-700 hover:bg-blue-100 disabled:cursor-wait disabled:opacity-60"
                    onClick={() => void addCustomTag()}
                    disabled={saving || creatingUserTag}
                  >
                    <AddIcon sx={{ fontSize: 15 }} />
                    {creatingUserTag
                      ? zh('创建中...', 'Creating...')
                      : zh(`新建“${tagSearch.trim()}”`, `Create “${tagSearch.trim()}”`)}
                  </button>
                ) : null}
                {availableTagOptions.length === 0 && !tagSearch.trim() ? (
                  <div className="px-2 py-1 text-sm text-gray-500">
                    {zh('暂无可添加标签', 'No tags to add')}
                  </div>
                ) : (
                  availableTagOptions.map((tag) => (
                    <button
                      key={`${tag.id}-${tag.provider || 0}`}
                      type="button"
                      className="flex w-full items-center gap-3 rounded px-2 py-1.5 text-left text-sm text-gray-800 hover:bg-gray-50"
                      onClick={() => toggleTag(tag.id, true)}
                      disabled={saving}
                    >
                      <span className="min-w-0 flex-1 truncate">{tag.name}</span>
                      <span className="shrink-0 text-xs tabular-nums text-gray-400">
                        {javEditWorkCountLabel(tag?.count)}
                      </span>
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
          className="mr-auto rounded-md border border-red-300 px-4 py-2 text-sm text-red-600 hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50"
          onClick={handleDelete}
          disabled={!item?.id || saving || creatingOption}
        >
          {deleting ? zh('删除中...', 'Deleting...') : zh('删除', 'Delete')}
        </button>
        <button
          type="button"
          className="rounded-md border border-gray-300 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50"
          onClick={onClose}
          disabled={saving || creatingOption}
        >
          {zh('取消', 'Cancel')}
        </button>
        <button
          type="button"
          className={`rounded-md px-4 py-2 text-sm font-medium text-white ${
            saving ? 'cursor-wait bg-blue-400' : 'bg-blue-600 hover:bg-blue-700'
          }`}
          onClick={handleSave}
          disabled={saving || creatingOption}
        >
          {saving ? zh('保存中...', 'Saving...') : zh('保存', 'Save')}
        </button>
      </div>
    </AppModal>
  )
}
