export const JAV_SCRAPE_OVERRIDE_SKIP = ':skip'

export const JAV_SCRAPE_OVERRIDE_MANUAL_PREFIX = ':manual:'

export function applyScrapeOverrideToVideo(video, override) {
  const nextOverride = String(override || '').trim()
  const next = { ...video, jav_scrape_override: nextOverride }
  if (!nextOverride) return next
  const effectiveOverride = nextOverride.toLowerCase().startsWith(JAV_SCRAPE_OVERRIDE_MANUAL_PREFIX)
    ? nextOverride.slice(JAV_SCRAPE_OVERRIDE_MANUAL_PREFIX.length).trim()
    : nextOverride
  const linkedCode = String(video?.jav?.code || video?.locations?.[0]?.jav?.code || '')
    .trim()
    .toUpperCase()
  if (nextOverride !== JAV_SCRAPE_OVERRIDE_SKIP && linkedCode === effectiveOverride.toUpperCase()) {
    return next
  }
  return {
    ...next,
    jav_id: null,
    jav: null,
    locations: Array.isArray(video?.locations)
      ? video.locations.map((location) => ({ ...location, jav_id: null, jav: null }))
      : video?.locations,
  }
}
