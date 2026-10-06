import assert from 'node:assert/strict'
import test from 'node:test'
import {
  collectWatchedTimeIDs,
  mergeWatchedTimeTotals,
} from '../../../src/features/playback/watchedTimeState.js'
import { loadModules } from '../../helpers/modules.js'

test('totals merge monotonically and snapshot queries include nested entities and duplicate copies', () => {
  const state = {
    videos: [
      { id: 1, location_id: 10, jav_id: 3, watched_ms: 0 },
      { id: 2, watched_ms: 500 },
      { id: 1, location_id: 11, locations: [{ jav_id: 3, jav: { id: 3 } }] },
    ],
    javItems: [{ id: 3, videos: [{ id: 1, watched_ms: 0 }] }],
  }
  const original = { videos: {}, javs: {} }
  const totals = mergeWatchedTimeTotals(original, {
    videos: [{ id: 1, watched_ms: 19000 }],
    javs: [{ id: 3, watched_ms: 30000 }],
  })
  assert.deepEqual(totals, { videos: { 1: 19000 }, javs: { 3: 30000 } })
  assert.deepEqual(original, { videos: {}, javs: {} })
  assert.equal(
    mergeWatchedTimeTotals(totals, {
      videos: [{ id: 1, watched_ms: 16000 }, null, { id: 0, watched_ms: 9 }],
    }),
    totals
  )
  assert.deepEqual(collectWatchedTimeIDs(state), { videoIds: [1, 2], javIds: [3] })
})

test('live totals leave list snapshots and unrelated state untouched; late list responses cannot overwrite the cache', async (t) => {
  const [{ useStore: store }] = await loadModules(t, ['store.js'])
  store.setState({
    videos: [{ id: 1, watched_ms: 0, jav: { id: 3, watched_ms: 0 } }],
    javItems: [{ id: 3, watched_ms: 0, videos: [{ id: 1, watched_ms: 0 }] }],
    page: 4,
    selectedVideoIds: new Set(['loc:10']),
  })
  const originalFetch = globalThis.fetch
  t.after(() => {
    globalThis.fetch = originalFetch
  })
  let finish
  globalThis.fetch = () =>
    new Promise((resolve) => {
      finish = resolve
    })
  const loading = store.getState().loadVideos({ force: true })
  const before = store.getState()
  const snapshot = {
    videos: [{ id: 1, watched_ms: 19000 }],
    javs: [{ id: 3, watched_ms: 30000 }],
  }
  store.getState().updateWatchedTimes(snapshot)
  const after = store.getState()
  for (const key of Object.keys(before).filter((key) => key !== 'watchedTimes')) {
    assert.equal(after[key], before[key], `${key} changed during a watched-time update`)
  }
  assert.equal(after.videos[0].watched_ms, 0)
  assert.equal(after.javItems[0].watched_ms, 0)
  assert.deepEqual(after.watchedTimes, { videos: { 1: 19000 }, javs: { 3: 30000 } })
  store.getState().updateWatchedTimes(snapshot)
  assert.equal(store.getState(), after, 'duplicate totals should not notify subscribers')
  finish(
    Response.json({
      items: [
        { id: 1, watched_ms: 16000 },
        { id: 2, watched_ms: 30000 },
      ],
      total: 100,
    })
  )
  await loading
  assert.equal(store.getState().videos[0].watched_ms, 16000)
  assert.equal(store.getState().videos[1].watched_ms, 30000)
  assert.equal(store.getState().watchedTimes, after.watchedTimes)
  assert.equal(store.getState().page, 4)
  assert.equal(store.getState().selectedVideoIds, before.selectedVideoIds)
})
