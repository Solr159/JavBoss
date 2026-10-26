import { configFlag } from '@/utils/config'
import { normalizeWatchTimeIconMinutes } from '@/features/playback/watchTime'

export const DEFAULT_SUBTITLE_LANGUAGE = 'Chinese Simplified'

export function normalizeSubtitleLanguage(value) {
  const text = typeof value === 'string' ? value.trim() : ''
  return text || DEFAULT_SUBTITLE_LANGUAGE
}

export function createDisplaySettingsDraft(state, tab) {
  if (tab === 'video') return createVideoSettingsDraft(state)
  if (tab) return createJavSettingsDraft(state, tab)
  return { ...createVideoSettingsDraft(state), ...createJavSettingsDraft(state) }
}

export function createVideoSettingsDraft(state) {
  const { pageSize, sortOrder, videoHideJav, config } = state
  return {
    videoWatchTimeIconMinutesInput: normalizeWatchTimeIconMinutes(
      config?.video_watch_time_icon_minutes
    ),
    videoPageSizeInput: pageSize,
    videoSortInput: sortOrder,
    videoHideJavInput: videoHideJav,
    videoWaterfallDefaultInput: configFlag(config?.video_waterfall_default),
    autoSubtitleEnabledInput: configFlag(config?.autosub_auto_enabled),
    autoSubtitleLanguageInput: normalizeSubtitleLanguage(config?.autosub_auto_language),
  }
}

const javSettingsFieldsByTab = {
  jav: [
    'javWatchTimeIconMinutesInput',
    'javPageSizeInput',
    'javGridColumnsInput',
    'javCardPortraitInput',
    'javTitleMaxRowsInput',
    'javIdolTagMaxRowsInput',
    'javTagMaxRowsInput',
    'javHideStudioInput',
    'javHideSeriesInput',
    'javHideIdolsInput',
    'javHideTagsInput',
    'javHideActionsInput',
    'javFavoriteRatingShowFullInput',
    'javWaterfallDefaultInput',
    'javSortInput',
    'javSortRulesInput',
  ],
  idol: [
    'idolPageSizeInput',
    'idolWaterfallDefaultInput',
    'idolSortInput',
    'javIdolPreferChineseNameInput',
  ],
  studio: ['studioPageSizeInput', 'studioWaterfallDefaultInput'],
  series: ['seriesPageSizeInput', 'seriesWaterfallDefaultInput'],
  tag: ['javTagShowSimplifiedInput'],
}

export function createJavSettingsDraft(state, tab) {
  const {
    javPageSize,
    javGridColumns,
    javTitleMaxRows,
    javIdolTagMaxRows,
    javTagMaxRows,
    config,
    idolPageSize,
    studioPageSize,
    seriesPageSize,
    javSort,
    javSortRules,
    idolSort,
  } = state
  const draft = {
    javWatchTimeIconMinutesInput: normalizeWatchTimeIconMinutes(
      config?.jav_watch_time_icon_minutes
    ),
    javPageSizeInput: javPageSize,
    javGridColumnsInput: javGridColumns,
    javCardPortraitInput: configFlag(config?.jav_card_portrait, true),
    javTitleMaxRowsInput: javTitleMaxRows,
    javIdolTagMaxRowsInput: javIdolTagMaxRows,
    javTagMaxRowsInput: javTagMaxRows,
    javHideStudioInput: configFlag(config?.jav_hide_studio),
    javHideSeriesInput: configFlag(config?.jav_hide_series),
    javHideIdolsInput: configFlag(config?.jav_hide_idols),
    javHideTagsInput: configFlag(config?.jav_hide_tags),
    javHideActionsInput: configFlag(config?.jav_hide_actions),
    javFavoriteRatingShowFullInput: configFlag(config?.jav_favorite_rating_show_full, false),
    javWaterfallDefaultInput: configFlag(config?.jav_waterfall_default),
    idolPageSizeInput: idolPageSize,
    idolWaterfallDefaultInput: configFlag(config?.idol_waterfall_default),
    studioPageSizeInput: studioPageSize,
    studioWaterfallDefaultInput: configFlag(config?.studio_waterfall_default),
    seriesPageSizeInput: seriesPageSize,
    seriesWaterfallDefaultInput: configFlag(config?.series_waterfall_default),
    javSortInput: javSort,
    javSortRulesInput: javSortRules,
    idolSortInput: idolSort,
    javIdolPreferChineseNameInput: configFlag(config?.jav_idol_prefer_chinese_name),
    javTagShowSimplifiedInput: configFlag(config?.jav_tag_show_simplified),
  }
  return tab
    ? Object.fromEntries(javSettingsFieldsByTab[tab].map((key) => [key, draft[key]]))
    : draft
}
