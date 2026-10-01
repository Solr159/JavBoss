export function getStudioCodePrefixes(item) {
  return Array.isArray(item?.code_prefixes)
    ? item.code_prefixes
        .map((prefixItem) => {
          if (typeof prefixItem === 'string') {
            const prefix = prefixItem.trim()
            return prefix ? { prefix, work_count: null } : null
          }
          const prefix = String(prefixItem?.prefix || '').trim()
          if (!prefix) return null
          const prefixWorkCount = Number(prefixItem?.work_count)
          return {
            prefix,
            work_count:
              Number.isFinite(prefixWorkCount) && prefixWorkCount > 0 ? prefixWorkCount : null,
          }
        })
        .filter(Boolean)
    : []
}

export function getStudioSeries(item) {
  return Array.isArray(item?.series)
    ? item.series.filter((series) => Number(series?.id) > 0 && String(series?.name || '').trim())
    : []
}
