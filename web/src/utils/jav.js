import { zh } from '@/utils/i18n'

export function getJavDisplayTitle(item) {
  const code = item?.code?.trim()
  const title = item?.title
  return title || code || zh('未知标题', 'Untitled')
}
