import { zh } from '@/utils/i18n'

export function formatPageTitle(...parts) {
  const content = parts.map((part) => String(part || '').trim()).filter(Boolean)
  return content.join(' - ')
}

export function buildLibraryPageTitle({
  isJavMode,
  javTab,
  filterItems = [],
  favoriteGroupName = '',
  javSearchTerm = '',
}) {
  const section = !isJavMode
    ? zh('视频', 'Videos')
    : {
        list: 'JAV',
        idol: zh('女优', 'Idols'),
        studio: zh('片商', 'Studios'),
        series: zh('系列', 'Series'),
      }[javTab] || 'JAV'
  const filters = filterItems.map((item) => item.label).filter(Boolean)
  // Idol search is displayed in the search box rather than a filter chip.
  if (isJavMode && javTab === 'idol' && javSearchTerm.trim()) {
    filters.unshift(zh(`搜索: ${javSearchTerm.trim()}`, `Search: ${javSearchTerm.trim()}`))
  }
  const context = [isJavMode ? favoriteGroupName : '', ...filters].filter(Boolean).join(' · ')
  return formatPageTitle(context, section)
}

export function buildJavDetailPageTitle(item, translateTitle = false) {
  const code = String(item?.code || '').trim()
  const title = String((translateTitle && item?.zh_title?.trim()) || item?.title || '').trim()
  return formatPageTitle(
    [...new Set([code, title].filter(Boolean))].join(' ') || zh('JAV 详情', 'JAV details')
  )
}
