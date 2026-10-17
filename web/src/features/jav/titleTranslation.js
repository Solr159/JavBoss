import { configFlag } from '@/utils/config'

export function preferChineseTitle(config) {
  return configFlag(config?.jav_title_translation_enabled)
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
