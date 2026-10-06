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
