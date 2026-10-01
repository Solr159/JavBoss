import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { useState, useCallback, useEffect, useMemo } from 'react'
import { withJavTagDisplayName } from '@/utils/javTag'
import { configFlag } from '@/utils/config'
import { resolveJavIdols } from '@/features/jav/api'
import { generateRandomSeed } from '@/utils/urlState'
import {
  IDOL_PROFILE_FILTER_DEFINITIONS,
  normalizeIdolProfileFilters,
  createDefaultIdolProfileFilters,
} from '@/constants/jav'
import { zh } from '@/utils/i18n'
import { getIdolDisplayName } from '@/utils/javIdol'

export default function useLibraryFilters({
  saveScrollBeforeUrlStateChange,
  setSearchInput,
  setJavSearchInput,
  directoryStateKey,
  isJavMode,
  buildVideoUrl,
  searchInput,
  buildJavUrl,
  javSearchInput,
  setJavQueryEditorOpen,
}) {
  const {
    setSearchTerm,
    setSelectedTags,
    searchTerm,
    javSearchTerm,
    javTagOptions,
    config,
    idolItems,
    javItems,
    javTab,
    javIdolIds,
    loadJavRandom,
    selectedTags,
    randomMode,
    javTags,
    javStudioId,
    javStudioName,
    javSeriesId,
    javSeriesName,
    javPrefix,
    javSoloOnly,
    javFavoriteRatingEnabled,
    javFavoriteRatingMin,
    javFavoriteRatingMax,
    javRandomMode,
  } = useStore(
    useShallow((state) => ({
      setSearchTerm: state.setSearchTerm,
      setSelectedTags: state.setSelectedTags,
      searchTerm: state.searchTerm,
      javSearchTerm: state.javSearchTerm,
      javTagOptions: state.javTagOptions,
      config: state.config,
      idolItems: state.idolItems,
      javItems: state.javItems,
      javTab: state.javTab,
      javIdolIds: state.javIdolIds,
      loadJavRandom: state.loadJavRandom,
      selectedTags: state.selectedTags,
      randomMode: state.randomMode,
      javTags: state.javTags,
      javStudioId: state.javStudioId,
      javStudioName: state.javStudioName,
      javSeriesId: state.javSeriesId,
      javSeriesName: state.javSeriesName,
      javPrefix: state.javPrefix,
      javSoloOnly: state.javSoloOnly,
      javFavoriteRatingEnabled: state.javFavoriteRatingEnabled,
      javFavoriteRatingMin: state.javFavoriteRatingMin,
      javFavoriteRatingMax: state.javFavoriteRatingMax,
      javRandomMode: state.javRandomMode,
    }))
  )
  const [javResolvedIdols, setJavResolvedIdols] = useState({})

  const handleVideoTagClick = useCallback(
    (name) => {
      if (!name) return
      saveScrollBeforeUrlStateChange()
      setSearchTerm('', { resetPage: false, triggerLoad: false })
      setSelectedTags([name])
    },
    [saveScrollBeforeUrlStateChange, setSearchTerm, setSelectedTags]
  )

  const applyJavTagFilter = useCallback(
    (tagIds) => {
      const clean = Array.from(
        new Set(
          (tagIds || [])
            .map((id) => Number.parseInt(String(id), 10))
            .filter((value) => Number.isFinite(value) && value > 0)
        )
      )
      saveScrollBeforeUrlStateChange()
      useStore.setState({
        viewMode: 'jav',
        videoTempSort: '',
        javTab: 'list',
        javTempSort: '',
        idolTempSort: '',
        javRandomMode: false,
        javRandomSeed: null,
        javIdolIds: [],
        javStudioId: null,
        javStudioName: '',
        javSeriesId: null,
        javSeriesName: '',
        javPrefix: '',
        javSoloOnly: false,
        javFavoriteRatingEnabled: false,
        javFavoriteRatingMin: 0.5,
        javFavoriteRatingMax: 5,
        idolFavoriteGroupId: null,
        javTags: clean,
        javSearchTerm: '',
        javPage: 1,
        idolPage: 1,
        studioPage: 1,
        seriesPage: 1,
      })
    },
    [saveScrollBeforeUrlStateChange]
  )

  useEffect(() => {
    setSearchInput(searchTerm)
  }, [searchTerm, setSearchInput])

  useEffect(() => {
    setJavSearchInput(javSearchTerm)
  }, [javSearchTerm, setJavSearchInput])

  const displayJavTagOptions = useMemo(
    () =>
      (javTagOptions || []).map((tag) =>
        withJavTagDisplayName(tag, configFlag(config?.jav_tag_show_simplified))
      ),
    [config?.jav_tag_show_simplified, javTagOptions]
  )

  const javTagNameMap = useMemo(
    () => new Map(displayJavTagOptions.map((tag) => [tag.id, tag.name])),
    [displayJavTagOptions]
  )

  const javIdolOptionMap = useMemo(() => {
    const map = new Map()
    const addIdol = (idol) => {
      const id = Number(idol?.id)
      if (!Number.isFinite(id) || id <= 0 || map.has(id)) return
      map.set(id, idol)
    }
    Object.values(javResolvedIdols || {}).forEach(addIdol)
    ;(idolItems || []).forEach(addIdol)
    ;(javItems || []).forEach((item) => {
      ;(item?.idols || []).forEach(addIdol)
    })
    return map
  }, [idolItems, javItems, javResolvedIdols])

  const javIdolFilterOptions = useMemo(
    () => Array.from(javIdolOptionMap.values()),
    [javIdolOptionMap]
  )

  useEffect(() => {
    setJavResolvedIdols({})
  }, [directoryStateKey])

  useEffect(() => {
    if (!isJavMode || javTab !== 'list' || javIdolIds.length === 0) return undefined
    const missingIds = javIdolIds
      .map((id) => Number(id))
      .filter((id) => Number.isFinite(id) && id > 0 && !javIdolOptionMap.has(id))
    if (missingIds.length === 0) return undefined

    let cancelled = false
    resolveJavIdols(missingIds)
      .then((items) => {
        if (cancelled) return
        const loaded = {}
        for (const idol of items || []) {
          const id = Number(idol?.id)
          if (Number.isFinite(id) && id > 0) loaded[id] = idol
        }
        if (Object.keys(loaded).length > 0) {
          setJavResolvedIdols((current) => ({ ...current, ...loaded }))
        }
      })
      .catch((err) => {
        console.warn('resolve jav idol names failed', err)
      })

    return () => {
      cancelled = true
    }
  }, [isJavMode, javTab, javIdolIds, javIdolOptionMap, directoryStateKey])

  const searchHref = buildVideoUrl({
    search: searchInput,
    page: 1,
    tempSort: '',
  })

  const javSearchHref = buildJavUrl({
    search: javSearchInput,
    page: 1,
    tab: javTab,
    tempSort: '',
  })

  const handleJavRandomClick = useCallback(() => {
    const nextSeed = generateRandomSeed()
    useStore.setState({
      viewMode: 'jav',
      videoTempSort: '',
      javTab: 'list',
      idolTempSort: '',
      idolFavoriteGroupId: null,
      idolPage: 1,
      studioPage: 1,
      seriesPage: 1,
    })
    loadJavRandom(nextSeed)
  }, [loadJavRandom])

  const handleVideoRandomClick = useCallback(() => {
    const nextSeed = generateRandomSeed()
    useStore.setState({ viewMode: 'video' })
    useStore.getState().loadRandom(nextSeed)
  }, [])

  const updateVideoFilters = useCallback(
    (updates) => {
      saveScrollBeforeUrlStateChange()
      useStore.setState({
        videoTempSort: '',
        page: 1,
        ...updates,
      })
    },
    [saveScrollBeforeUrlStateChange]
  )

  const updateJavFilters = useCallback(
    (updates) => {
      saveScrollBeforeUrlStateChange()
      useStore.setState({
        javTempSort: '',
        idolTempSort: '',
        javPage: 1,
        ...updates,
      })
    },
    [saveScrollBeforeUrlStateChange]
  )

  const handleFavoriteRatingEnabledChange = useCallback(
    (enabled) => {
      updateJavFilters({ javFavoriteRatingEnabled: Boolean(enabled) })
    },
    [updateJavFilters]
  )

  const handleFavoriteRatingRangeChange = useCallback(
    (range) => {
      if (!Array.isArray(range) || range.length !== 2) return
      const min = Number(range[0])
      const max = Number(range[1])
      if (!Number.isFinite(min) || !Number.isFinite(max) || min > max) return
      updateJavFilters({
        javFavoriteRatingMin: min,
        javFavoriteRatingMax: max,
      })
    },
    [updateJavFilters]
  )

  const handleIdolProfileFilterChange = useCallback(
    (key, updates) => {
      if (!IDOL_PROFILE_FILTER_DEFINITIONS.some((definition) => definition.key === key)) return
      const current = normalizeIdolProfileFilters(useStore.getState().idolProfileFilters)
      const next = normalizeIdolProfileFilters({
        ...current,
        [key]: { ...current[key], ...(updates || {}) },
      })
      updateJavFilters({ idolProfileFilters: next, idolPage: 1 })
    },
    [updateJavFilters]
  )

  const activeFilterItems = useMemo(() => {
    if (!isJavMode) {
      const items = []
      const activeSearch = String(searchTerm || '').trim()
      if (activeSearch) {
        items.push({
          key: 'video-search',
          label: zh(`搜索: ${activeSearch}`, `Search: ${activeSearch}`),
          onRemove: () => {
            setSearchInput('')
            updateVideoFilters({ searchTerm: '' })
          },
        })
      }
      selectedTags.forEach((name) => {
        items.push({
          key: `video-tag-${name}`,
          label: zh(`标签: ${name}`, `Tag: ${name}`),
          onRemove: () =>
            updateVideoFilters({ selectedTags: selectedTags.filter((tag) => tag !== name) }),
        })
      })
      if (randomMode) {
        items.push({
          key: 'video-random',
          label: zh('随机', 'Random'),
          onRemove: () => updateVideoFilters({ randomMode: false, randomSeed: null }),
        })
      }
      return items
    }

    const items = []
    if (javTab === 'idol') return items
    const activeSearch = String(javSearchTerm || '').trim()
    if (activeSearch) {
      items.push({
        key: 'jav-search',
        label: zh(`搜索: ${activeSearch}`, `Search: ${activeSearch}`),
        onRemove: () => {
          setJavSearchInput('')
          updateJavFilters({
            javSearchTerm: '',
            ...(javTab === 'idol'
              ? { idolPage: 1 }
              : javTab === 'studio'
                ? { studioPage: 1 }
                : javTab === 'series'
                  ? { seriesPage: 1 }
                  : {}),
          })
        },
      })
    }
    if (javTab !== 'list') return items

    javIdolIds.forEach((id) => {
      const idol = javIdolOptionMap.get(Number(id))
      const name = idol
        ? getIdolDisplayName(idol, configFlag(config?.jav_idol_prefer_chinese_name))
        : `#${id}`
      items.push({
        key: `jav-idol-${id}`,
        label: zh(`女优: ${name}`, `Idol: ${name}`),
        onRemove: () =>
          updateJavFilters({
            javIdolIds: javIdolIds.filter((item) => Number(item) !== Number(id)),
          }),
      })
    })
    javTags.forEach((id) => {
      const name = javTagNameMap.get(id) || `#${id}`
      items.push({
        key: `jav-tag-${id}`,
        label: zh(`标签: ${name}`, `Tag: ${name}`),
        onRemove: () =>
          updateJavFilters({ javTags: javTags.filter((item) => Number(item) !== Number(id)) }),
      })
    })
    if (javStudioId !== null) {
      const name =
        javStudioName || (javStudioId === 0 ? zh('未知片商', 'Unknown studio') : `#${javStudioId}`)
      items.push({
        key: 'jav-studio',
        label: zh(`片商: ${name}`, `Studio: ${name}`),
        onRemove: () => updateJavFilters({ javStudioId: null, javStudioName: '' }),
      })
    }
    if (javSeriesId) {
      const name = javSeriesName || `#${javSeriesId}`
      items.push({
        key: 'jav-series',
        label: zh(`系列: ${name}`, `Series: ${name}`),
        onRemove: () => updateJavFilters({ javSeriesId: null, javSeriesName: '' }),
      })
    }
    if (javPrefix) {
      items.push({
        key: 'jav-prefix',
        label: zh(`番号: ${javPrefix}`, `Code: ${javPrefix}`),
        onRemove: () => updateJavFilters({ javPrefix: '' }),
      })
    }
    if (javSoloOnly) {
      items.push({
        key: 'jav-solo',
        label: zh('单体作品', 'Solo works'),
        onRemove: () => updateJavFilters({ javSoloOnly: false }),
      })
    }
    if (javFavoriteRatingEnabled) {
      const formatRating = (value) => {
        const rating = Number(value)
        return Number.isInteger(rating) ? String(rating) : rating.toFixed(1)
      }
      const range = `${formatRating(javFavoriteRatingMin)}–${formatRating(javFavoriteRatingMax)}`
      items.push({
        key: 'jav-favorite-rating',
        label: zh(`喜爱度: ${range}`, `Favorite rating: ${range}`),
        onRemove: () => updateJavFilters({ javFavoriteRatingEnabled: false }),
      })
    }
    if (javRandomMode) {
      items.push({
        key: 'jav-random',
        label: zh('随机', 'Random'),
        onRemove: () => updateJavFilters({ javRandomMode: false, javRandomSeed: null }),
      })
    }
    return items
  }, [
    config?.jav_idol_prefer_chinese_name,
    isJavMode,
    javFavoriteRatingEnabled,
    javFavoriteRatingMax,
    javFavoriteRatingMin,
    javIdolIds,
    javIdolOptionMap,
    javPrefix,
    javRandomMode,
    javSearchTerm,
    javSeriesId,
    javSeriesName,
    javSoloOnly,
    javStudioId,
    javStudioName,
    javTab,
    javTagNameMap,
    javTags,
    randomMode,
    searchTerm,
    selectedTags,
    setJavSearchInput,
    setSearchInput,
    updateJavFilters,
    updateVideoFilters,
  ])

  const handleClearActiveFilters = useCallback(() => {
    if (!isJavMode) {
      setSearchInput('')
      updateVideoFilters({
        selectedTags: [],
        searchTerm: '',
        randomMode: false,
        randomSeed: null,
      })
      return
    }
    setJavSearchInput('')
    const updates = { javSearchTerm: '' }
    if (javTab === 'list') {
      Object.assign(updates, {
        javIdolIds: [],
        javTags: [],
        javStudioId: null,
        javStudioName: '',
        javSeriesId: null,
        javSeriesName: '',
        javPrefix: '',
        javSoloOnly: false,
        javFavoriteRatingEnabled: false,
        javFavoriteRatingMin: 0.5,
        javFavoriteRatingMax: 5,
        javRandomMode: false,
        javRandomSeed: null,
      })
    } else if (javTab === 'idol') {
      Object.assign(updates, {
        idolPage: 1,
        idolProfileFilters: createDefaultIdolProfileFilters(),
      })
    } else if (javTab === 'studio') {
      Object.assign(updates, { studioPage: 1 })
    } else {
      Object.assign(updates, { seriesPage: 1 })
    }
    updateJavFilters(updates)
  }, [isJavMode, javTab, setJavSearchInput, setSearchInput, updateJavFilters, updateVideoFilters])

  const submitSearch = (e) => {
    e?.preventDefault()
    const nextSearch = (searchInput || '').trim()
    useStore.setState({
      viewMode: 'video',
      searchTerm: nextSearch,
      videoTempSort: '',
      page: 1,
    })
  }

  const submitJavSearch = (e) => {
    e?.preventDefault()
    useStore.setState({
      viewMode: 'jav',
      videoTempSort: '',
      javTempSort: '',
      idolTempSort: '',
      javSearchTerm: (javSearchInput || '').trim(),
      javPage: 1,
      idolPage: 1,
      studioPage: 1,
      seriesPage: 1,
    })
  }

  const handleApplyJavQuery = useCallback(
    (query) => {
      const nextSearch = String(query?.search || '').trim()
      const nextIdolIds = Array.from(
        new Set(
          (query?.idolIds || [])
            .map((id) => Number(id))
            .filter((id) => Number.isFinite(id) && id > 0)
        )
      )
      const nextTags = Array.from(
        new Set(
          (query?.tagIds || [])
            .map((id) => Number(id))
            .filter((id) => Number.isFinite(id) && id > 0)
        )
      )
      const nextStudioId = Number(query?.studio?.id)
      const hasStudio = Number.isFinite(nextStudioId) && nextStudioId >= 0
      const nextStudioName = hasStudio ? String(query?.studio?.name || '').trim() : ''
      const nextSeriesId = Number(query?.series?.id)
      const hasSeries = Number.isFinite(nextSeriesId) && nextSeriesId > 0
      const nextSeriesName = hasSeries ? String(query?.series?.name || '').trim() : ''
      const nextPrefix = String(query?.prefix || '')
        .trim()
        .toUpperCase()
      const nextFavoriteRatingEnabled = Boolean(query?.favoriteRatingEnabled)
      const normalizeFavoriteRating = (value, fallback) => {
        const parsed = Number(value)
        if (!Number.isFinite(parsed)) return fallback
        return Math.min(5, Math.max(0.5, Math.round(parsed * 2) / 2))
      }
      let nextFavoriteRatingMin = normalizeFavoriteRating(query?.favoriteRatingMin, 0.5)
      let nextFavoriteRatingMax = normalizeFavoriteRating(query?.favoriteRatingMax, 5)
      if (nextFavoriteRatingMin > nextFavoriteRatingMax) {
        nextFavoriteRatingMin = 0.5
        nextFavoriteRatingMax = 5
      }
      saveScrollBeforeUrlStateChange()
      useStore.setState({
        viewMode: 'jav',
        videoTempSort: '',
        javTab: 'list',
        javTempSort: '',
        idolTempSort: '',
        javSearchTerm: nextSearch,
        javIdolIds: nextIdolIds,
        javTags: nextTags,
        javStudioId: hasStudio ? nextStudioId : null,
        javStudioName: nextStudioName,
        javSeriesId: hasSeries ? nextSeriesId : null,
        javSeriesName: nextSeriesName,
        javPrefix: nextPrefix,
        javSoloOnly: Boolean(query?.soloOnly),
        javFavoriteRatingEnabled: nextFavoriteRatingEnabled,
        javFavoriteRatingMin: nextFavoriteRatingMin,
        javFavoriteRatingMax: nextFavoriteRatingMax,
        idolFavoriteGroupId: null,
        javPage: 1,
        idolPage: 1,
        studioPage: 1,
        seriesPage: 1,
      })
      setJavSearchInput(nextSearch)
      setJavQueryEditorOpen(false)
    },
    [saveScrollBeforeUrlStateChange, setJavQueryEditorOpen, setJavSearchInput]
  )
  return {
    handleVideoTagClick,
    applyJavTagFilter,
    displayJavTagOptions,
    javIdolFilterOptions,
    searchHref,
    javSearchHref,
    handleJavRandomClick,
    handleVideoRandomClick,
    handleFavoriteRatingEnabledChange,
    handleFavoriteRatingRangeChange,
    handleIdolProfileFilterChange,
    activeFilterItems,
    handleClearActiveFilters,
    submitSearch,
    submitJavSearch,
    handleApplyJavQuery,
  }
}
