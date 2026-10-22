import { configFlag } from '@/utils/config'
import { zh } from '@/utils/i18n'

export function getJavDisplayTitle(item, config = {}) {
  const code = item?.code?.trim()
  const title = item?.title?.trim()
  const zhTitle = item?.zh_title?.trim()
  const enTitle = item?.en_title?.trim()
  // Language: 'zh' = show Chinese translation, 'en' = show English, else show original
  const enabled = configFlag(config?.jav_title_translation_enabled)
  const lang = String(config?.jav_title_language || 'zh')
    .trim()
    .toLowerCase()
  if (!enabled) return title || code || zh('未知标题', 'Untitled')
  if (lang === 'en' && enTitle) return enTitle
  if (lang === 'zh' && zhTitle) return zhTitle
  return title || code || zh('未知标题', 'Untitled')
}
