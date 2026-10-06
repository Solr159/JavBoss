export function needsTitleTranslation(item) {
  const title = String(item?.title || '').trim()
  return Number(item?.id) > 0 && Boolean(title) && !item.zh_title?.trim()
}
