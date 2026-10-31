import { translationProfiles } from '@/features/settings/titleTranslation'
import { useStore } from '@/store'
import { normalizeWatchTimeIconMinutes } from '@/features/playback/watchTime'
import { normalizeVideoSort } from '@/constants/video'
import { updateConfig } from '@/features/settings/api'
import {
  normalizeJavSort,
  normalizeIdolSort,
  javSortRulesConfig,
  normalizeJavSortRules,
} from '@/constants/jav'

export async function saveVideoSettings(draft, onWaterfallChange) {
  const { videoPageSizeInput, videoSortInput, videoHideJavInput, videoWaterfallDefaultInput } =
    draft
  const { pageSize, page, total, videoHideJav } = useStore.getState()

  const size = Math.max(1, parseInt(videoPageSizeInput, 10) || pageSize)
  const normalizedSort = normalizeVideoSort(videoSortInput)
  const waterfallDefault = Boolean(videoWaterfallDefaultInput)

  const payload = {
    video_watch_time_icon_minutes: normalizeWatchTimeIconMinutes(
      draft.videoWatchTimeIconMinutesInput
    ),
    video_page_size: size,
    video_sort: normalizedSort,
    video_hide_jav: videoHideJavInput,
    video_waterfall_default: waterfallDefault,
    video_show_directory_bar: draft.videoShowDirectoryBarInput ?? true,
  }
  const cfg = await updateConfig(payload)
  const prevPage = page
  // ensure current page does not exceed last page after page size change
  const lastPage = Math.max(1, Math.ceil((total || 0) / size))
  const filterChanged = videoHideJavInput !== videoHideJav
  const nextPage = filterChanged ? 1 : prevPage > lastPage ? lastPage : prevPage

  onWaterfallChange('video', waterfallDefault)

  useStore.setState((state) => ({
    pageSize: size,
    sortOrder: normalizedSort,
    videoHideJav: videoHideJavInput,
    videoTempSort: '',
    page: nextPage,
    randomMode: false,
    randomSeed: null,
    config: {
      ...state.config,
      ...Object.fromEntries(Object.keys(payload).map((key) => [key, cfg[key]])),
    },
  }))
}

export async function saveJavSettings(draft, onWaterfallChange, tab) {
  const {
    javPageSizeInput,
    javGridColumnsInput,
    javTitleMaxRowsInput,
    javIdolTagMaxRowsInput,
    javTagMaxRowsInput,
    javHideStudioInput,
    javHideSeriesInput,
    javHideIdolsInput,
    javHideTagsInput,
    javHideActionsInput,
    javFavoriteRatingShowFullInput,
    javWaterfallDefaultInput,
    idolPageSizeInput,
    idolWaterfallDefaultInput,
    studioPageSizeInput,
    studioWaterfallDefaultInput,
    seriesPageSizeInput,
    seriesWaterfallDefaultInput,
    javSortInput,
    javSortRulesInput,
    idolSortInput,
    javIdolPreferChineseNameInput,
    javTagShowSimplifiedInput,
  } = draft
  const {
    javPageSize,
    idolPageSize,
    studioPageSize,
    seriesPageSize,
    javPage,
    idolPage,
    studioPage,
    seriesPage,
    javTotal,
    idolTotal,
    studioTotal,
    seriesTotal,
  } = useStore.getState()

  const javSize = Math.max(1, parseInt(javPageSizeInput, 10) || javPageSize)
  const javGridColumnsRaw = parseInt(javGridColumnsInput, 10)
  const javColumns =
    Number.isFinite(javGridColumnsRaw) && javGridColumnsRaw > 0
      ? Math.min(javGridColumnsRaw, 12)
      : 0
  const javIdolTagRowsRaw = parseInt(javIdolTagMaxRowsInput, 10)
  const javIdolTagRows =
    Number.isFinite(javIdolTagRowsRaw) && javIdolTagRowsRaw > 0
      ? Math.min(javIdolTagRowsRaw, 12)
      : 0
  const javTitleRowsRaw = parseInt(javTitleMaxRowsInput, 10)
  const javTitleRows =
    Number.isFinite(javTitleRowsRaw) && javTitleRowsRaw >= 0 ? Math.min(javTitleRowsRaw, 12) : 2
  const javTagRowsRaw = parseInt(javTagMaxRowsInput, 10)
  const javTagRows =
    Number.isFinite(javTagRowsRaw) && javTagRowsRaw >= 0 ? Math.min(javTagRowsRaw, 12) : 2
  const idolSize = Math.max(1, parseInt(idolPageSizeInput, 10) || idolPageSize)
  const studioSize = Math.max(1, parseInt(studioPageSizeInput, 10) || studioPageSize)
  const seriesSize = Math.max(1, parseInt(seriesPageSizeInput, 10) || seriesPageSize)
  const normalizedSort = normalizeJavSort(javSortInput)
  const normalizedIdolSort = normalizeIdolSort(idolSortInput)
  const waterfallDefaults = {
    jav: Boolean(javWaterfallDefaultInput),
    idol: Boolean(idolWaterfallDefaultInput),
    studio: Boolean(studioWaterfallDefaultInput),
    series: Boolean(seriesWaterfallDefaultInput),
  }

  const settingsByTab = {
    jav: {
      config: {
        jav_watch_time_icon_minutes: normalizeWatchTimeIconMinutes(
          draft.javWatchTimeIconMinutesInput
        ),
        jav_page_size: javSize,
        jav_grid_columns: javColumns,
        jav_card_portrait: Boolean(draft.javCardPortraitInput),
        jav_title_max_rows: javTitleRows,
        jav_idol_tag_max_rows: javIdolTagRows,
        jav_tag_max_rows: javTagRows,
        jav_hide_studio: Boolean(javHideStudioInput),
        jav_hide_series: Boolean(javHideSeriesInput),
        jav_hide_idols: Boolean(javHideIdolsInput),
        jav_hide_tags: Boolean(javHideTagsInput),
        jav_hide_actions: Boolean(javHideActionsInput),
        jav_favorite_rating_show_full: Boolean(javFavoriteRatingShowFullInput),
        jav_waterfall_default: waterfallDefaults.jav,
        jav_sort: normalizedSort,
        jav_sort_rules: javSortRulesConfig(javSortRulesInput),
      },
      state: {
        javPageSize: javSize,
        javGridColumns: javColumns,
        javTitleMaxRows: javTitleRows,
        javIdolTagMaxRows: javIdolTagRows,
        javTagMaxRows: javTagRows,
        javSort: normalizedSort,
        javTempSort: '',
        javPage: Math.min(javPage, Math.max(1, Math.ceil((javTotal || 0) / javSize))),
        javRandomMode: false,
        javRandomSeed: null,
      },
    },
    idol: {
      config: {
        idol_page_size: idolSize,
        idol_waterfall_default: waterfallDefaults.idol,
        idol_sort: normalizedIdolSort,
        jav_idol_prefer_chinese_name: Boolean(javIdolPreferChineseNameInput),
      },
      state: {
        idolPageSize: idolSize,
        idolSort: normalizedIdolSort,
        idolTempSort: '',
        idolPage: Math.min(idolPage, Math.max(1, Math.ceil((idolTotal || 0) / idolSize))),
      },
    },
    studio: {
      config: {
        studio_page_size: studioSize,
        studio_waterfall_default: waterfallDefaults.studio,
      },
      state: {
        studioPageSize: studioSize,
        studioPage: Math.min(studioPage, Math.max(1, Math.ceil((studioTotal || 0) / studioSize))),
      },
    },
    series: {
      config: {
        series_page_size: seriesSize,
        series_waterfall_default: waterfallDefaults.series,
      },
      state: {
        seriesPageSize: seriesSize,
        seriesPage: Math.min(seriesPage, Math.max(1, Math.ceil((seriesTotal || 0) / seriesSize))),
      },
    },
    tag: {
      config: { jav_tag_show_simplified: Boolean(javTagShowSimplifiedInput) },
      state: {},
    },
  }
  const selected = tab ? [settingsByTab[tab]] : Object.values(settingsByTab)
  const payload = Object.assign({}, ...selected.map((settings) => settings.config))
  const cfg = await updateConfig(payload)
  const updates = Object.assign({}, ...selected.map((settings) => settings.state))
  if (!tab || tab === 'jav') {
    updates.javSortRules = normalizeJavSortRules(cfg?.jav_sort_rules)
  }
  Object.entries(waterfallDefaults).forEach(([key, enabled]) => {
    if (!tab || key === tab) onWaterfallChange(key, enabled)
  })
  useStore.setState((state) => ({
    ...updates,
    // A different tab may have finished saving while this request was in flight.
    config: tab
      ? {
          ...state.config,
          ...Object.fromEntries(Object.keys(payload).map((key) => [key, cfg[key]])),
        }
      : cfg,
  }))
}

export async function saveTitleTranslationSettings(draft) {
  const cfg = await updateConfig({
    jav_title_translation_provider: draft.provider,
    jav_title_translation_profiles: translationProfiles(draft),
    jav_title_translation_prompt: draft.prompt,
  })
  useStore.setState({ config: cfg })
}

export async function saveTitleTranslationEnabled(enabled, target = 'title', automatic = false) {
  const key = `jav_${target}_translation_${automatic ? 'auto_' : ''}enabled`
  const cfg = await updateConfig({ [key]: Boolean(enabled) })
  // Other switches may have saved while this response was in flight.
  useStore.setState((state) => ({ config: { ...state.config, [key]: cfg[key] } }))
}
