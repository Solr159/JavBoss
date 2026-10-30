import { configFlag } from '@/utils/config'

export function isTitleTranslationEnabled(config) {
  return configFlag(config?.jav_title_translation_enabled)
}

export function titleLanguage(config) {
  return String(config?.jav_title_language || 'zh')
    .trim()
    .toLowerCase()
}

export function isSeriesTranslationEnabled(config) {
  return configFlag(config?.jav_series_translation_enabled)
}

// Backward-compatible aliases for existing code that still reads the old boolean flags.
export function preferChineseTitle(config) {
  return isTitleTranslationEnabled(config) && titleLanguage(config) === 'zh'
}

export function preferChineseSeries(config) {
  return isSeriesTranslationEnabled(config)
}

export function needsTitleTranslation(item) {
  const title = String(item?.title || '').trim()
  return Number(item?.id) > 0 && Boolean(title) && !item.zh_title?.trim()
}

export function parseTranslationConfig(value, fallback = {}) {
  try {
    return JSON.parse(value) ?? fallback
  } catch {
    return fallback
  }
}

export function getSeriesDisplayName(item, preferChinese = true) {
  return (preferChinese && String(item?.zh_name || '').trim()) || String(item?.name || '').trim()
}
