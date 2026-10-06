import { zh } from '@/utils/i18n'

export function getJavDisplayTitle(item, translateTitle = false) {
  const code = item?.code?.trim()
  const title = (translateTitle && item?.zh_title?.trim()) || item?.title
  return title || code || zh('未知标题', 'Untitled')
}
