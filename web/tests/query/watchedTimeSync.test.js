import assert from 'node:assert/strict'
import test from 'node:test'
import { loadModules } from '../helpers/modules.js'
import { mergeWatchedTimeTotals } from '../../src/features/playback/watchedTimeState.js'

function fixture(t, start, fetchSnapshot, videoCount = 1) {
  const listeners = new Set()
  const state = {
    videos: Array.from({ length: videoCount }, (_, i) => ({ id: i + 1 })),
    javItems: [{ id: 7 }],
    watchedTimes: { videos: {}, javs: {} },
  }
  state.updateWatchedTimes = (snapshot) => {
    state.watchedTimes = mergeWatchedTimeTotals(state.watchedTimes, snapshot)
  }
  const store = {
    getState: () => state,
    subscribe: (listener) => {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
  }
  const page = new EventTarget()
  page.document = new EventTarget()
  page.document.visibilityState = 'visible'
  const source = new EventTarget()
  source.close = () => {
    source.closed = true
  }
  const stop = start({ store, fetchSnapshot, createSource: () => source, page })
  t.after(stop)
  return {
    state,
    source,
    page,
    stop,
    listeners,
    emit: (snapshot) =>
      source.dispatchEvent(new MessageEvent('watched-time', { data: JSON.stringify(snapshot) })),
  }
}
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

test('events win over older in-flight snapshots; focus/reconnect resync and cleanup cancels work', async (t) => {
  const [{ startWatchedTimeSync }] = await loadModules(t, ['query/watchedTimeSync.js'])
  const requests = []
  const f = fixture(
    t,
    startWatchedTimeSync,
    (ids, signal) => new Promise((resolve) => requests.push({ ids, signal, resolve }))
  )
  await sleep(100)
  assert.equal(requests.length, 1)
  assert.deepEqual(requests[0].ids, { videoIds: [1], javIds: [7] })
  f.emit({ videos: [{ id: 1, watched_ms: 19000 }], javs: [{ id: 7, watched_ms: 19000 }] })
  requests[0].resolve({ videos: [{ id: 1, watched_ms: 16000 }], javs: [] })
  await sleep(10)
  assert.equal(f.state.watchedTimes.videos[1], 19000)
  f.page.dispatchEvent(new Event('focus'))
  f.source.dispatchEvent(new Event('open'))
  await sleep(100)
  assert.equal(requests.length, 2)
  requests[1].resolve({ videos: [{ id: 1, watched_ms: 30000 }], javs: [] })
  await sleep(10)
  assert.equal(f.state.watchedTimes.videos[1], 30000)
  f.source.dispatchEvent(new Event('error'))
  await sleep(100)
  assert.equal(requests.length, 3)
  f.stop()
  assert.ok(f.source.closed)
  assert.ok(requests[2].signal.aborted)
  assert.equal(f.listeners.size, 0)
  requests[2].resolve({ videos: [{ id: 1, watched_ms: 40000 }], javs: [] })
  f.emit({ videos: [{ id: 1, watched_ms: 50000 }], javs: [] })
  f.page.dispatchEvent(new Event('focus'))
  await sleep(100)
  assert.equal(requests.length, 3)
  assert.equal(f.state.watchedTimes.videos[1], 30000)
})

test('snapshot queries are batched and failed snapshots retry without another event', async (t) => {
  const [{ startWatchedTimeSync }] = await loadModules(t, ['query/watchedTimeSync.js'])
  const requests = []
  let failed = false
  fixture(
    t,
    startWatchedTimeSync,
    async (ids) => {
      requests.push(ids)
      if (!failed) {
        failed = true
        throw new Error('offline')
      }
      return { videos: [], javs: [] }
    },
    401
  )
  await sleep(2300)
  assert.deepEqual(
    requests.map((r) => r.videoIds.length),
    [200, 200, 200, 1]
  )
  assert.deepEqual(requests[3].videoIds, [401])
})
