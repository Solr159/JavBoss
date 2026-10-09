import { zh } from '@/utils/i18n'

export function getJavDisplayTitle(item, translateTitle = false) {
  const code = item?.code?.trim()
  const title = (translateTitle && item?.zh_title?.trim()) || item?.title
  return title || code || zh('未知标题', 'Untitled')
}

export function getJavSeriesDisplayName(item, translateTitle = false) {
  const name = (translateTitle && item?.zh_name?.trim()) || item?.name
  return name || zh('未知系列', 'Unknown series')
}
