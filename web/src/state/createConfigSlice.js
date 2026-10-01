import { fetchConfig } from '@/features/settings/api'
import { normalizeVideoSort } from '@/constants/video'
import {
  JAV_GRID_COLUMNS_AUTO,
  JAV_TITLE_MAX_ROWS_DEFAULT,
  JAV_IDOL_TAG_MAX_ROWS_DEFAULT,
  JAV_TAG_MAX_ROWS_DEFAULT,
} from '@/state/model'
import { normalizeJavSort, normalizeJavSortRules, normalizeIdolSort } from '@/constants/jav'

export function createConfigSlice({ get, set }) {
  return {
    config: {},
    loadConfig: async () => {
      try {
        const cfg = await fetchConfig()
        const state = get()
        const clamp = (raw) => {
          const n = parseInt(raw, 10)
          if (!Number.isFinite(n) || n <= 0) return null
          return Math.min(n, 500)
        }
        const updates = { config: cfg }
        const videoSize = clamp(cfg?.video_page_size)
        const videoSort = normalizeVideoSort((cfg?.video_sort || '').toLowerCase(), '')
        const videoHideJav = String(cfg?.video_hide_jav || '').toLowerCase() === 'true'
        const javSize = clamp(cfg?.jav_page_size)
        const javGridColumnsRaw = parseInt(cfg?.jav_grid_columns, 10)
        const javGridColumns =
          Number.isFinite(javGridColumnsRaw) && javGridColumnsRaw > 0
            ? Math.min(javGridColumnsRaw, 12)
            : JAV_GRID_COLUMNS_AUTO
        const javTitleMaxRowsRaw = parseInt(cfg?.jav_title_max_rows, 10)
        const javTitleMaxRows =
          Number.isFinite(javTitleMaxRowsRaw) && javTitleMaxRowsRaw >= 0
            ? Math.min(javTitleMaxRowsRaw, 12)
            : JAV_TITLE_MAX_ROWS_DEFAULT
        const javIdolTagMaxRowsRaw = parseInt(cfg?.jav_idol_tag_max_rows, 10)
        const javIdolTagMaxRows =
          Number.isFinite(javIdolTagMaxRowsRaw) && javIdolTagMaxRowsRaw >= 0
            ? Math.min(javIdolTagMaxRowsRaw, 12)
            : JAV_IDOL_TAG_MAX_ROWS_DEFAULT
        const javTagMaxRowsRaw = parseInt(cfg?.jav_tag_max_rows, 10)
        const javTagMaxRows =
          Number.isFinite(javTagMaxRowsRaw) && javTagMaxRowsRaw >= 0
            ? Math.min(javTagMaxRowsRaw, 12)
            : JAV_TAG_MAX_ROWS_DEFAULT
        const idolSize = clamp(cfg?.idol_page_size)
        const studioSize = clamp(cfg?.studio_page_size)
        const seriesSize = clamp(cfg?.series_page_size)
        const javSort = normalizeJavSort((cfg?.jav_sort || '').toLowerCase(), '')
        const javSortRules = normalizeJavSortRules(cfg?.jav_sort_rules)
        const idolSort = normalizeIdolSort((cfg?.idol_sort || '').toLowerCase(), '')
        if (videoSize && videoSize !== state.pageSize) {
          updates.pageSize = videoSize
        }
        if (videoSort) {
          updates.sortOrder = videoSort
        }
        if (videoHideJav !== state.videoHideJav) {
          updates.videoHideJav = videoHideJav
        }
        if (javSort) {
          updates.javSort = javSort
        }
        updates.javSortRules = javSortRules
        if (idolSort) {
          updates.idolSort = idolSort
        }
        if (javSize && javSize !== state.javPageSize) {
          updates.javPageSize = javSize
        }
        if (javGridColumns !== state.javGridColumns) {
          updates.javGridColumns = javGridColumns
        }
        if (javTitleMaxRows !== state.javTitleMaxRows) {
          updates.javTitleMaxRows = javTitleMaxRows
        }
        if (javIdolTagMaxRows !== state.javIdolTagMaxRows) {
          updates.javIdolTagMaxRows = javIdolTagMaxRows
        }
        if (javTagMaxRows !== state.javTagMaxRows) {
          updates.javTagMaxRows = javTagMaxRows
        }
        if (idolSize && idolSize !== state.idolPageSize) {
          updates.idolPageSize = idolSize
        }
        if (studioSize && studioSize !== state.studioPageSize) {
          updates.studioPageSize = studioSize
        }
        if (seriesSize && seriesSize !== state.seriesPageSize) {
          updates.seriesPageSize = seriesSize
        }
        set(updates)
        return cfg
      } catch (e) {
        console.error('load config failed', e)
        return null
      }
    },
  }
}
