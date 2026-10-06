const validID = (id) => Number.isSafeInteger(Number(id)) && Number(id) > 0

export function mergeWatchedTimeTotals(current, snapshot) {
  let next = current
  for (const kind of ['videos', 'javs']) {
    for (const entry of Array.isArray(snapshot?.[kind]) ? snapshot[kind] : []) {
      if (!entry) continue
      const value = Number(entry.watched_ms)
      if (!validID(entry.id) || !Number.isSafeInteger(value) || value < 0) continue
      // Counters only increase. Delayed snapshots/events must never undo a newer total.
      if (value <= (next[kind][entry.id] ?? -1)) continue
      if (next === current) next = { ...current }
      if (next[kind] === current[kind]) next[kind] = { ...current[kind] }
      next[kind][entry.id] = value
    }
  }
  return next
}

function mapUnchanged(items, update) {
  if (!Array.isArray(items)) return items
  const next = items.map(update)
  return next.some((item, index) => item !== items[index]) ? next : items
}

// Preserve list order, pagination, selection and references for unaffected rows.
export function applyWatchedTimeTotals(patch, totals) {
  if (!totals) return patch
  const counter = (item, kind) => {
    if (!item) return item
    const value = totals[kind][item.id]
    return value > (Number(item.watched_ms) || 0) ? { ...item, watched_ms: value } : item
  }
  const jav = (item) => {
    if (!item) return item
    const next = counter(item, 'javs')
    const videos = mapUnchanged(item.videos, video)
    return videos === item.videos ? next : { ...next, videos }
  }
  const location = (item) => {
    const nextJav = jav(item.jav)
    return nextJav === item.jav ? item : { ...item, jav: nextJav }
  }
  const video = (item) => {
    const next = counter(item, 'videos')
    const nextJav = jav(item.jav)
    const locations = mapUnchanged(item.locations, location)
    return nextJav === item.jav && locations === item.locations
      ? next
      : { ...next, jav: nextJav, locations }
  }
  const next = { ...patch }
  if (patch.videos) next.videos = mapUnchanged(patch.videos, video)
  if (patch.javItems) next.javItems = mapUnchanged(patch.javItems, jav)
  return next
}

export function collectWatchedTimeIDs(state) {
  const videos = new Set()
  const javs = new Set()
  const seen = new WeakSet()
  const visit = (item, kind) => {
    if (!item || typeof item !== 'object' || seen.has(item)) return
    seen.add(item)
    if (validID(item.id)) (kind === 'videos' ? videos : javs).add(Number(item.id))
    if (kind === 'videos') {
      if (validID(item.jav_id)) javs.add(Number(item.jav_id))
      visit(item.jav, 'javs')
      for (const location of item.locations || []) {
        if (validID(location.jav_id)) javs.add(Number(location.jav_id))
        visit(location.jav, 'javs')
      }
    } else {
      for (const video of item.videos || []) visit(video, 'videos')
    }
  }
  for (const video of state.videos || []) visit(video, 'videos')
  for (const jav of state.javItems || []) visit(jav, 'javs')
  return {
    videoIds: [...videos].sort((a, b) => a - b),
    javIds: [...javs].sort((a, b) => a - b),
  }
}
