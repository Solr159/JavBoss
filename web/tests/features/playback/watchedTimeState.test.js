import assert from 'node:assert/strict'
import test from 'node:test'
import {
  applyWatchedTimeTotals,
  collectWatchedTimeIDs,
  mergeWatchedTimeTotals,
} from '../../../src/features/playback/watchedTimeState.js'
import { loadModules } from '../../helpers/modules.js'

test('committed totals patch all copies and attributed JAVs without changing order or unrelated references', () => {
  const unchanged = { id: 2, watched_ms: 500 }
  const state = {
    videos: [
      { id: 1, location_id: 10, jav_id: 3, watched_ms: 0 },
      unchanged,
      { id: 1, location_id: 11, locations: [{ jav_id: 3, jav: { id: 3 } }] },
    ],
    javItems: [{ id: 3, videos: [{ id: 1, watched_ms: 0 }] }],
    page: 4,
    selectedVideoIds: new Set(['1:10']),
  }
  const totals = mergeWatchedTimeTotals(
    { videos: {}, javs: {} },
    { videos: [{ id: 1, watched_ms: 19000 }], javs: [{ id: 3, watched_ms: 30000 }] }
  )
  const next = applyWatchedTimeTotals(state, totals)
  assert.deepEqual(
    next.videos.map((v) => v.id),
    [1, 2, 1]
  )
  assert.equal(next.videos[1], unchanged)
  assert.equal(next.videos[0].watched_ms, 19000)
  assert.equal(next.videos[2].watched_ms, 19000)
  assert.equal(next.videos[2].locations[0].jav.watched_ms, 30000)
  assert.equal(next.javItems[0].videos[0].watched_ms, 19000)
  assert.equal(next.javItems[0].watched_ms, 30000)
  assert.equal(next.page, 4)
  assert.equal(next.selectedVideoIds, state.selectedVideoIds)
  assert.equal(state.videos[0].watched_ms, 0)
  assert.equal(applyWatchedTimeTotals(next, totals).videos, next.videos)
  assert.equal(
    mergeWatchedTimeTotals(totals, {
      videos: [{ id: 1, watched_ms: 16000 }, null, { id: 0, watched_ms: 9 }],
    }),
    totals
  )
  assert.deepEqual(collectWatchedTimeIDs(state), { videoIds: [1, 2], javIds: [3] })
})

test('a list response started before a committed update cannot overwrite newer totals', async (t) => {
  const [{ createAppState }] = await loadModules(t, ['store.js'])
  let state
  const get = () => state
  const set = (update) => {
    const patch = typeof update === 'function' ? update(state) : update
    state = { ...state, ...patch }
  }
  state = createAppState(set, get)
  const originalFetch = globalThis.fetch
  t.after(() => {
    globalThis.fetch = originalFetch
  })
  let finish
  globalThis.fetch = () =>
    new Promise((resolve) => {
      finish = resolve
    })
  const loading = state.loadVideos({ force: true })
  state.updateWatchedTimes({ videos: [{ id: 1, watched_ms: 19000 }], javs: [] })
  finish(
    Response.json({
      items: [
        { id: 1, watched_ms: 16000 },
        { id: 2, watched_ms: 30000 },
      ],
      total: 2,
    })
  )
  await loading
  assert.equal(state.videos[0].watched_ms, 19000)
  assert.equal(state.videos[1].watched_ms, 30000)
  assert.equal(state.total, 2)
})
