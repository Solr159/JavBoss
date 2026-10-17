import { configFlag } from '@/utils/config'

export function preferChineseTitle(config) {
  return configFlag(config?.jav_title_prefer_chinese, true)
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
