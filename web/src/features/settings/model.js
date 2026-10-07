import { configFlag } from '@/utils/config'
import { normalizeWatchTimeIconMinutes } from '@/features/playback/watchTime'

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
  }
}

export function createJavSettingsDraft(state) {
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
  return {
    javWatchTimeIconMinutesInput: normalizeWatchTimeIconMinutes(
      config?.jav_watch_time_icon_minutes
    ),
    javPageSizeInput: javPageSize,
    javGridColumnsInput: javGridColumns,
    javTitleMaxRowsInput: javTitleMaxRows,
    javIdolTagMaxRowsInput: javIdolTagMaxRows,
    javTagMaxRowsInput: javTagMaxRows,
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
}
