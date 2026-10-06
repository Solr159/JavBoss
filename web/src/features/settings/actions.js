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

  const cfg = await updateConfig({
    video_watch_time_icon_minutes: normalizeWatchTimeIconMinutes(
      draft.videoWatchTimeIconMinutesInput
    ),
    video_page_size: size,
    video_sort: normalizedSort,
    video_hide_jav: videoHideJavInput,
    video_waterfall_default: waterfallDefault,
  })
  const prevPage = page
  // ensure current page does not exceed last page after page size change
  const lastPage = Math.max(1, Math.ceil((total || 0) / size))
  const filterChanged = videoHideJavInput !== videoHideJav
  const nextPage = filterChanged ? 1 : prevPage > lastPage ? lastPage : prevPage

  onWaterfallChange('video', waterfallDefault)

  useStore.setState({
    pageSize: size,
    sortOrder: normalizedSort,
    videoHideJav: videoHideJavInput,
    videoTempSort: '',
    page: nextPage,
    randomMode: false,
    randomSeed: null,
    config: cfg,
  })
}

export async function saveJavSettings(draft, onWaterfallChange) {
  const {
    javPageSizeInput,
    javGridColumnsInput,
    javTitleMaxRowsInput,
    javIdolTagMaxRowsInput,
    javTagMaxRowsInput,
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

  const cfg = await updateConfig({
    jav_watch_time_icon_minutes: normalizeWatchTimeIconMinutes(draft.javWatchTimeIconMinutesInput),
    jav_page_size: javSize,
    jav_title_translation_enabled: Boolean(draft.titleTranslationEnabled),
    jav_grid_columns: javColumns,
    jav_title_max_rows: javTitleRows,
    jav_idol_tag_max_rows: javIdolTagRows,
    jav_tag_max_rows: javTagRows,
    jav_hide_series: Boolean(javHideSeriesInput),
    jav_hide_idols: Boolean(javHideIdolsInput),
    jav_hide_tags: Boolean(javHideTagsInput),
    jav_hide_actions: Boolean(javHideActionsInput),
    jav_favorite_rating_show_full: Boolean(javFavoriteRatingShowFullInput),
    jav_waterfall_default: waterfallDefaults.jav,
    idol_page_size: idolSize,
    idol_waterfall_default: waterfallDefaults.idol,
    studio_page_size: studioSize,
    studio_waterfall_default: waterfallDefaults.studio,
    series_page_size: seriesSize,
    series_waterfall_default: waterfallDefaults.series,
    jav_sort: normalizedSort,
    jav_sort_rules: javSortRulesConfig(javSortRulesInput),
    idol_sort: normalizedIdolSort,
    jav_idol_prefer_chinese_name: Boolean(javIdolPreferChineseNameInput),
    jav_tag_show_simplified: Boolean(javTagShowSimplifiedInput),
  })
  const prevJavPage = javPage
  const prevIdolPage = idolPage
  const prevStudioPage = studioPage
  const prevSeriesPage = seriesPage
  const javLast = Math.max(1, Math.ceil((javTotal || 0) / javSize))
  const idolLast = Math.max(1, Math.ceil((idolTotal || 0) / idolSize))
  const studioLast = Math.max(1, Math.ceil((studioTotal || 0) / studioSize))
  const seriesLast = Math.max(1, Math.ceil((seriesTotal || 0) / seriesSize))
  Object.entries(waterfallDefaults).forEach(([key, enabled]) => {
    onWaterfallChange(key, enabled)
  })
  useStore.setState({
    javPageSize: javSize,
    javGridColumns: javColumns,
    javTitleMaxRows: javTitleRows,
    javIdolTagMaxRows: javIdolTagRows,
    javTagMaxRows: javTagRows,
    idolPageSize: idolSize,
    studioPageSize: studioSize,
    seriesPageSize: seriesSize,
    javSort: normalizedSort,
    javSortRules: normalizeJavSortRules(cfg?.jav_sort_rules),
    javTempSort: '',
    idolSort: normalizedIdolSort,
    idolTempSort: '',
    javPage: Math.min(prevJavPage, javLast),
    idolPage: Math.min(prevIdolPage, idolLast),
    studioPage: Math.min(prevStudioPage, studioLast),
    seriesPage: Math.min(prevSeriesPage, seriesLast),
    javRandomMode: false,
    javRandomSeed: null,
    config: cfg,
  })
}

export async function saveTitleTranslationSettings(draft) {
  const apiKey = draft.apiKey.trim()
  const cfg = await updateConfig({
    jav_title_translation_enabled: Boolean(draft.enabled),
    jav_title_translation_model: draft.model,
    jav_title_translation_thinking: Boolean(draft.thinking),
    jav_title_translation_prompt: draft.prompt,
    jav_title_translation_api_key: apiKey || (draft.clearApiKey ? '' : undefined),
  })
  useStore.setState({ config: cfg })
}

export async function setTitleTranslationEnabled(enabled) {
  const cfg = await updateConfig({ jav_title_translation_enabled: enabled })
  useStore.setState({ config: cfg })
}
