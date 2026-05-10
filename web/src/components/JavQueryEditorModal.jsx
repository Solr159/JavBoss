import { useEffect, useMemo, useRef, useState } from 'react'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import SearchIcon from '@mui/icons-material/Search'

import { fetchJavIdols, fetchJavStudios } from '@/api'
import { zh } from '@/utils/i18n'

const JAV_FILTER_FETCH_LIMIT = 500

const cleanIds = (ids) =>
  Array.from(
    new Set((ids || []).map((id) => Number(id)).filter((id) => Number.isFinite(id) && id > 0))
  )

const fetchAllJavIdols = async ({ directoryIds = [] } = {}) => {
  const all = []
  let offset = 0
  let total = null

  while (total == null || offset < total) {
    const resp = await fetchJavIdols({
      limit: JAV_FILTER_FETCH_LIMIT,
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

const fetchAllJavStudios = async ({ directoryIds = [] } = {}) => {
  const all = []
  let offset = 0
  let total = null

  while (total == null || offset < total) {
    const resp = await fetchJavStudios({
      limit: JAV_FILTER_FETCH_LIMIT,
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

export default function JavQueryEditorModal({
  open,
  onClose,
  onApply,
  search = '',
  idolIds = [],
  idolOptions = [],
  tagIds = [],
  tagOptions = [],
  studioId = null,
  studioName = '',
  directoryIds = [],
}) {
  const studioInputRef = useRef(null)
  const [keyword, setKeyword] = useState('')
  const [selectedIdolIds, setSelectedIdolIds] = useState([])
  const [idolSearch, setIdolSearch] = useState('')
  const [idolPickerOpen, setIdolPickerOpen] = useState(false)
  const [allIdols, setAllIdols] = useState([])
  const [idolLoading, setIdolLoading] = useState(false)
  const [idolError, setIdolError] = useState('')
  const [selectedTagIds, setSelectedTagIds] = useState([])
  const [tagSearch, setTagSearch] = useState('')
  const [tagPickerOpen, setTagPickerOpen] = useState(false)
  const [selectedStudio, setSelectedStudio] = useState(null)
  const [studioSearch, setStudioSearch] = useState('')
  const [studioPickerOpen, setStudioPickerOpen] = useState(false)
  const [allStudios, setAllStudios] = useState([])
  const [studioLoading, setStudioLoading] = useState(false)
  const [studioError, setStudioError] = useState('')

  useEffect(() => {
    if (!open) return
    const trimmedStudioName = String(studioName || '').trim()
    const parsedStudioId = Number(studioId)
    setKeyword(String(search || '').trim())
    setSelectedIdolIds(cleanIds(idolIds))
    setIdolSearch('')
    setIdolPickerOpen(false)
    setIdolError('')
    setSelectedTagIds(cleanIds(tagIds))
    setTagSearch('')
    setTagPickerOpen(false)
    setSelectedStudio(
      Number.isFinite(parsedStudioId) && parsedStudioId > 0
        ? { id: parsedStudioId, name: trimmedStudioName || `#${parsedStudioId}` }
        : null
    )
    setStudioSearch('')
    setStudioPickerOpen(false)
    setStudioError('')
  }, [idolIds, open, search, studioId, studioName, tagIds])

  useEffect(() => {
    if (!open) return

    let cancelled = false
    setIdolLoading(true)
    setIdolError('')
    fetchAllJavIdols({ directoryIds })
      .then((items) => {
        if (!cancelled) setAllIdols(items)
      })
      .catch((err) => {
        if (!cancelled) {
          setAllIdols([])
          setIdolError(err.message || zh('加载女优失败', 'Failed to load idols'))
        }
      })
      .finally(() => {
        if (!cancelled) setIdolLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [directoryIds, open])

  useEffect(() => {
    if (open) return
    setAllIdols([])
    setAllStudios([])
  }, [open])

  useEffect(() => {
    if (!open) return

    let cancelled = false
    setStudioLoading(true)
    setStudioError('')
    fetchAllJavStudios({ directoryIds })
      .then((items) => {
        if (!cancelled) setAllStudios(items)
      })
      .catch((err) => {
        if (!cancelled) {
          setAllStudios([])
          setStudioError(err.message || zh('加载片商失败', 'Failed to load studios'))
        }
      })
      .finally(() => {
        if (!cancelled) setStudioLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [directoryIds, open])

  const tagMap = useMemo(
    () => new Map((tagOptions || []).map((tag) => [Number(tag.id), tag])),
    [tagOptions]
  )

  const idolMap = useMemo(() => {
    const map = new Map()
    const addIdol = (idol) => {
      const id = Number(idol?.id)
      if (!Number.isFinite(id) || id <= 0 || map.has(id)) return
      map.set(id, idol)
    }
    ;(idolOptions || []).forEach(addIdol)
    ;(allIdols || []).forEach(addIdol)
    return map
  }, [allIdols, idolOptions])

  const selectedIdols = useMemo(
    () => selectedIdolIds.map((id) => idolMap.get(id) || { id, name: `#${id}` }),
    [idolMap, selectedIdolIds]
  )

  const selectedTags = useMemo(
    () => selectedTagIds.map((id) => tagMap.get(id)).filter(Boolean),
    [selectedTagIds, tagMap]
  )

  const filteredTags = useMemo(() => {
    const query = tagSearch.trim().toLowerCase()
    const list = Array.isArray(tagOptions) ? tagOptions : []
    return [...list]
      .filter((tag) => {
        if (!query) return true
        return String(tag?.name || '')
          .toLowerCase()
          .includes(query)
      })
      .sort((a, b) => {
        const countA = Number.isFinite(a?.count) ? a.count : 0
        const countB = Number.isFinite(b?.count) ? b.count : 0
        if (countB !== countA) return countB - countA
        return String(a?.name || '').localeCompare(String(b?.name || ''))
      })
      .slice(0, 120)
  }, [tagOptions, tagSearch])

  const filteredIdols = useMemo(() => {
    const query = idolSearch.trim().toLowerCase()
    const merged = new Map(idolMap)
    ;(idolOptions || []).forEach((idol) => {
      const id = Number(idol?.id)
      if (Number.isFinite(id) && id > 0 && !merged.has(id)) merged.set(id, idol)
    })
    return Array.from(merged.values())
      .filter((idol) => {
        if (!query) return true
        return String(idol?.name || '')
          .toLowerCase()
          .includes(query)
      })
      .sort((a, b) => {
        const countA = Number.isFinite(a?.work_count) ? a.work_count : 0
        const countB = Number.isFinite(b?.work_count) ? b.work_count : 0
        if (countB !== countA) return countB - countA
        return String(a?.name || '').localeCompare(String(b?.name || ''))
      })
  }, [idolMap, idolOptions, idolSearch])

  const filteredStudios = useMemo(() => {
    const query = studioSearch.trim().toLowerCase()
    return [...allStudios]
      .filter((studio) => {
        if (!query) return true
        return String(studio?.name || '')
          .toLowerCase()
          .includes(query)
      })
      .sort((a, b) => {
        const countA = Number.isFinite(a?.work_count) ? a.work_count : 0
        const countB = Number.isFinite(b?.work_count) ? b.work_count : 0
        if (countB !== countA) return countB - countA
        return String(a?.name || '').localeCompare(String(b?.name || ''))
      })
  }, [allStudios, studioSearch])

  const toggleIdol = (id) => {
    const parsed = Number(id)
    if (!Number.isFinite(parsed) || parsed <= 0) return
    setSelectedIdolIds((prev) => {
      const next = new Set(prev)
      if (next.has(parsed)) next.delete(parsed)
      else next.add(parsed)
      return Array.from(next)
    })
  }

  const removeIdol = (id) => {
    const parsed = Number(id)
    setSelectedIdolIds((prev) => prev.filter((item) => item !== parsed))
  }

  const toggleTag = (id) => {
    const parsed = Number(id)
    if (!Number.isFinite(parsed) || parsed <= 0) return
    setSelectedTagIds((prev) => {
      const next = new Set(prev)
      if (next.has(parsed)) next.delete(parsed)
      else next.add(parsed)
      return Array.from(next)
    })
  }

  const removeTag = (id) => {
    const parsed = Number(id)
    setSelectedTagIds((prev) => prev.filter((item) => item !== parsed))
  }

  const clearAll = () => {
    setKeyword('')
    setSelectedIdolIds([])
    setIdolSearch('')
    setSelectedTagIds([])
    setTagSearch('')
    setSelectedStudio(null)
    setStudioSearch('')
    setStudioPickerOpen(false)
  }

  const applyQuery = () => {
    onApply?.({
      search: keyword.trim(),
      idolIds: selectedIdolIds,
      tagIds: selectedTagIds,
      studio: selectedStudio,
    })
  }

  const closePickerOnBlur = (setPickerOpen) => (event) => {
    const currentTarget = event.currentTarget
    window.setTimeout(() => {
      if (!currentTarget.contains(document.activeElement)) {
        setPickerOpen(false)
      }
    }, 0)
  }

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 px-4 backdrop-blur-sm">
      <div className="flex max-h-[88vh] w-full max-w-3xl flex-col overflow-hidden rounded-xl bg-white shadow-2xl ring-1 ring-slate-200">
        <div className="flex items-center justify-between border-b border-slate-200 bg-slate-50 px-5 py-4">
          <h2 className="text-base font-semibold text-slate-900">
            {zh('编辑 JAV 查询条件', 'Edit JAV Filters')}
          </h2>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-8 w-8 items-center justify-center rounded text-slate-500 hover:bg-slate-200"
            aria-label={zh('关闭查询条件编辑', 'Close query editor')}
          >
            <CloseOutlinedIcon fontSize="small" />
          </button>
        </div>

        <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-5 py-4">
          <section className="space-y-2">
            <label className="text-sm font-semibold text-slate-800" htmlFor="jav-query-keyword">
              {zh('关键词', 'Keyword')}
            </label>
            <div className="flex items-center gap-2 rounded border border-slate-200 bg-white px-3 py-2">
              <SearchIcon fontSize="small" className="text-slate-400" />
              <input
                id="jav-query-keyword"
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                className="min-w-0 flex-1 border-0 text-sm outline-none"
                placeholder={zh('番号或标题关键词', 'Code or title keyword')}
              />
              {keyword ? (
                <button
                  type="button"
                  onClick={() => setKeyword('')}
                  className="inline-flex h-7 w-7 items-center justify-center rounded text-slate-400 hover:bg-slate-100"
                  aria-label={zh('清空关键词', 'Clear keyword')}
                >
                  <CloseOutlinedIcon fontSize="inherit" />
                </button>
              ) : null}
            </div>
          </section>

          <section className="space-y-2">
            <div className="text-sm font-semibold text-slate-800">{zh('女优', 'Idols')}</div>
            {selectedIdols.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {selectedIdols.map((idol) => (
                  <span
                    key={idol.id}
                    className="inline-flex max-w-full items-center gap-1 rounded-full bg-blue-50 px-2.5 py-1 text-xs font-medium text-blue-700"
                  >
                    <span className="truncate">{idol.name}</span>
                    <button
                      type="button"
                      onClick={() => removeIdol(idol.id)}
                      className="inline-flex h-4 w-4 items-center justify-center rounded-full hover:bg-blue-100"
                      aria-label={zh(`删除女优 ${idol.name}`, `Remove idol ${idol.name}`)}
                    >
                      <CloseOutlinedIcon fontSize="inherit" />
                    </button>
                  </span>
                ))}
              </div>
            ) : null}
            <div onBlur={closePickerOnBlur(setIdolPickerOpen)}>
              <input
                value={idolSearch}
                onFocus={() => setIdolPickerOpen(true)}
                onChange={(event) => {
                  setIdolSearch(event.target.value)
                  setIdolPickerOpen(true)
                }}
                className="w-full rounded border border-slate-200 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                placeholder={zh('搜索女优', 'Search idols')}
              />
              {idolPickerOpen ? (
                <div className="mt-1 max-h-52 overflow-y-auto rounded border border-slate-200 bg-white p-1 shadow-lg">
                  {idolLoading ? (
                    <div className="px-2 py-3 text-sm text-slate-500">
                      {zh('加载中…', 'Loading...')}
                    </div>
                  ) : idolError ? (
                    <div className="px-2 py-3 text-sm text-rose-600">{idolError}</div>
                  ) : filteredIdols.length > 0 ? (
                    filteredIdols.map((idol) => {
                      const checked = selectedIdolIds.includes(Number(idol.id))
                      return (
                        <button
                          type="button"
                          role="checkbox"
                          aria-checked={checked}
                          key={idol.id}
                          onMouseDown={(event) => event.preventDefault()}
                          onClick={() => toggleIdol(idol.id)}
                          className="flex w-full cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-slate-50"
                        >
                          <input
                            type="checkbox"
                            checked={checked}
                            readOnly
                            tabIndex={-1}
                            className="pointer-events-none h-4 w-4 shrink-0 rounded border-slate-300 text-blue-600"
                            aria-hidden="true"
                          />
                          <span className="min-w-0 flex-1 truncate text-slate-800">
                            {idol.name}
                          </span>
                          {Number.isFinite(idol?.work_count) ? (
                            <span className="shrink-0 text-xs text-slate-400">
                              {zh(`${idol.work_count} 部`, `${idol.work_count} works`)}
                            </span>
                          ) : null}
                        </button>
                      )
                    })
                  ) : (
                    <div className="px-2 py-3 text-sm text-slate-500">
                      {zh('没有匹配女优', 'No matching idols')}
                    </div>
                  )}
                </div>
              ) : null}
            </div>
          </section>

          <section className="space-y-2">
            <div className="text-sm font-semibold text-slate-800">{zh('标签', 'Tags')}</div>
            {selectedTags.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {selectedTags.map((tag) => (
                  <span
                    key={tag.id}
                    className="inline-flex max-w-full items-center gap-1 rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700"
                  >
                    <span className="truncate">{tag.name}</span>
                    <button
                      type="button"
                      onClick={() => removeTag(tag.id)}
                      className="inline-flex h-4 w-4 items-center justify-center rounded-full hover:bg-emerald-100"
                      aria-label={zh(`删除标签 ${tag.name}`, `Remove tag ${tag.name}`)}
                    >
                      <CloseOutlinedIcon fontSize="inherit" />
                    </button>
                  </span>
                ))}
              </div>
            ) : null}
            <div onBlur={closePickerOnBlur(setTagPickerOpen)}>
              <input
                value={tagSearch}
                onFocus={() => setTagPickerOpen(true)}
                onChange={(event) => {
                  setTagSearch(event.target.value)
                  setTagPickerOpen(true)
                }}
                className="w-full rounded border border-slate-200 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                placeholder={zh('搜索标签', 'Search tags')}
              />
              {tagPickerOpen ? (
                <div className="mt-1 max-h-52 overflow-y-auto rounded border border-slate-200 bg-white p-1 shadow-lg">
                  {filteredTags.length > 0 ? (
                    filteredTags.map((tag) => {
                      const checked = selectedTagIds.includes(Number(tag.id))
                      return (
                        <button
                          type="button"
                          role="checkbox"
                          aria-checked={checked}
                          key={tag.id}
                          onMouseDown={(event) => event.preventDefault()}
                          onClick={() => toggleTag(tag.id)}
                          className="flex w-full cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-slate-50"
                        >
                          <input
                            type="checkbox"
                            checked={checked}
                            readOnly
                            tabIndex={-1}
                            className="pointer-events-none h-4 w-4 shrink-0 rounded border-slate-300 text-blue-600"
                            aria-hidden="true"
                          />
                          <span className="min-w-0 flex-1 truncate text-slate-800">{tag.name}</span>
                          {Number.isFinite(tag?.count) ? (
                            <span className="shrink-0 text-xs text-slate-400">
                              {zh(`${tag.count} 部`, `${tag.count} works`)}
                            </span>
                          ) : null}
                        </button>
                      )
                    })
                  ) : (
                    <div className="px-2 py-3 text-sm text-slate-500">
                      {zh('没有匹配标签', 'No matching tags')}
                    </div>
                  )}
                </div>
              ) : null}
            </div>
          </section>

          <section className="space-y-2">
            <div className="text-sm font-semibold text-slate-800">{zh('片商', 'Studio')}</div>
            {selectedStudio ? (
              <div className="flex items-center justify-between gap-2 rounded border border-violet-100 bg-violet-50 px-3 py-2 text-sm text-violet-800">
                <span className="min-w-0 truncate font-medium">{selectedStudio.name}</span>
                <button
                  type="button"
                  onClick={() => {
                    setSelectedStudio(null)
                    setStudioSearch('')
                  }}
                  className="inline-flex h-7 w-7 items-center justify-center rounded hover:bg-violet-100"
                  aria-label={zh('删除片商条件', 'Remove studio filter')}
                >
                  <DeleteOutlineIcon fontSize="small" />
                </button>
              </div>
            ) : null}
            <div onBlur={closePickerOnBlur(setStudioPickerOpen)}>
              <input
                ref={studioInputRef}
                value={studioSearch}
                onFocus={() => setStudioPickerOpen(true)}
                onChange={(event) => {
                  setStudioSearch(event.target.value)
                  setStudioPickerOpen(true)
                  setSelectedStudio(null)
                }}
                className="w-full rounded border border-slate-200 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                placeholder={zh('搜索并选择片商', 'Search and choose a studio')}
              />
              {studioPickerOpen ? (
                <div className="mt-1 max-h-52 overflow-y-auto rounded border border-slate-200 bg-white p-1 shadow-lg">
                  {studioLoading ? (
                    <div className="px-2 py-3 text-sm text-slate-500">
                      {zh('加载中…', 'Loading...')}
                    </div>
                  ) : studioError ? (
                    <div className="px-2 py-3 text-sm text-rose-600">{studioError}</div>
                  ) : filteredStudios.length > 0 ? (
                    filteredStudios.map((studio) => {
                      const checked = Number(selectedStudio?.id) === Number(studio.id)
                      return (
                        <button
                          key={studio.id}
                          type="button"
                          role="radio"
                          aria-checked={checked}
                          onMouseDown={(event) => event.preventDefault()}
                          onClick={() => {
                            setSelectedStudio({ id: studio.id, name: studio.name })
                            setStudioSearch('')
                            setStudioPickerOpen(false)
                            studioInputRef.current?.blur()
                          }}
                          className="flex w-full cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-slate-50"
                        >
                          <input
                            type="radio"
                            checked={checked}
                            readOnly
                            tabIndex={-1}
                            className="pointer-events-none h-4 w-4 shrink-0 border-slate-300 text-blue-600"
                            aria-hidden="true"
                          />
                          <span className="min-w-0 flex-1 truncate text-slate-800">
                            {studio.name}
                          </span>
                          <span className="shrink-0 text-xs text-slate-400">
                            {zh(`${studio.work_count || 0} 部`, `${studio.work_count || 0} works`)}
                          </span>
                        </button>
                      )
                    })
                  ) : (
                    <div className="px-2 py-3 text-sm text-slate-500">
                      {zh('没有匹配片商', 'No matching studios')}
                    </div>
                  )}
                </div>
              ) : null}
            </div>
          </section>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-2 border-t border-slate-200 bg-slate-50 px-5 py-4">
          <button
            type="button"
            onClick={clearAll}
            className="rounded border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 hover:bg-slate-100"
          >
            {zh('清空条件', 'Clear Filters')}
          </button>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 hover:bg-slate-100"
            >
              {zh('取消', 'Cancel')}
            </button>
            <button
              type="button"
              onClick={applyQuery}
              className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
            >
              {zh('应用查询', 'Apply Query')}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
