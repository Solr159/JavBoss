import assert from 'node:assert/strict'
import test from 'node:test'
import { createStore } from 'zustand/vanilla'
import { loadModules } from '../helpers/modules.js'

async function fixture(t, domain = 'video') {
  const [{ createAppState }] = await loadModules(t, ['store.js'])
  const store = createStore(createAppState)
  store.setState({
    viewMode: domain,
    pageSize: 2,
    javPageSize: 2,
    loadTags: async () => {},
    loadJavTags: async () => {},
    loadJavFavoriteGroups: async () => {},
  })
  let rows = Array.from({ length: 6 }, (_, i) => ({ id: i + 1, location_id: i + 1 }))
  const requests = []
  const response = (url) => {
    const matches = rows.filter((row) => !url.searchParams.get('search') || row.id > 2)
    const offset = Number(url.searchParams.get('offset'))
    const limit = Number(url.searchParams.get('limit'))
    return Response.json({ items: matches.slice(offset, offset + limit), total: matches.length })
  }
  t.mock.method(globalThis, 'fetch', async (input, init) => {
    const url = new URL(input, 'http://localhost')
    requests.push({ url, signal: init.signal })
    return response(url)
  })
  const remove = (ids) => {
    rows = rows.filter((row) => !ids.includes(row.id))
    return domain === 'video'
      ? store.getState().removeVideoLocations(ids.map((id) => `loc:${id}`))
      : store.getState().removeJavVideos(ids[0], ids)
  }
  return { store, requests, response, remove }
}

for (const domain of ['video', 'jav']) {
  const items = domain === 'video' ? 'videos' : 'javItems'
  const total = domain === 'video' ? 'total' : 'javTotal'
  const loading = domain === 'video' ? 'loading' : 'javLoading'
  const page = domain === 'video' ? 'page' : 'javPage'
  const load = domain === 'video' ? 'loadVideos' : 'loadJavs'
  const more = domain === 'video' ? 'loadMoreVideos' : 'loadMoreJavs'

  test(`${domain}: deletion refills numbered pages and moves an emptied last page back`, async (t) => {
    const f = await fixture(t, domain)
    const state = f.store.getState
    await state()[load]()
    const deleting = f.remove([1])
    assert.deepEqual(
      state()[items].map((row) => row.id),
      [2]
    )
    assert.equal(state()[loading], false, 'keep the list mounted during reconciliation')
    await deleting
    assert.deepEqual(
      state()[items].map((row) => row.id),
      [2, 3]
    )
    assert.equal(state()[total], 5)
    f.store.setState({ [page]: 2 })
    await state()[load]()
    assert.deepEqual(
      state()[items].map((row) => row.id),
      [4, 5]
    )
    f.store.setState({ [page]: 3 })
    await state()[load]()
    assert.deepEqual(
      state()[items].map((row) => row.id),
      [6]
    )
    await f.remove([6])
    assert.equal(state()[page], 2)
    assert.equal(state()[total], 4)
    assert.deepEqual(
      state()[items].map((row) => row.id),
      [4, 5]
    )
    if (domain === 'video') assert.equal(state().hasNext, false)
  })

  test(`${domain}: waterfall revalidates its loaded range and appends without gaps`, async (t) => {
    const f = await fixture(t, domain)
    const state = f.store.getState
    f.store.setState({ waterfallModes: { [domain]: true } })
    await state()[load]()
    await state()[more]()
    await f.remove([2])
    assert.equal(f.requests.at(-1).url.searchParams.get('limit'), '3')
    assert.deepEqual(
      state()[items].map((row) => row.id),
      [1, 3, 4]
    )
    assert.equal(state()[total], 5)
    await state()[more]()
    assert.equal(f.requests.at(-1).url.searchParams.get('offset'), '3')
    assert.deepEqual(
      state()[items].map((row) => row.id),
      [1, 3, 4, 5, 6]
    )
  })
}

test('cross-filter selections do not undercount results; unloaded matching deletions still update the total', async (t) => {
  const f = await fixture(t)
  const state = f.store.getState
  f.store.setState({ waterfallModes: { video: true } })
  await state().loadVideos()
  state().videos.forEach(state().toggleSelectVideo)
  state().setSearchTerm('filtered')
  await state().loadVideos()
  assert.equal(state().selectedVideoIds.size, 2)
  const deleting = f.remove([1, 2])
  assert.equal(state().total, 4, 'unrelated deletion must not reduce the filtered total')
  await deleting
  assert.equal(state().total, 4)
  assert.equal(state().selectedVideoIds.size, 0)
  await f.remove([6])
  assert.equal(state().total, 3, 'server counts matching rows outside the loaded range')
  await state().loadMoreVideos()
  assert.deepEqual(
    state().videos.map((row) => row.id),
    [3, 4, 5]
  )
})

for (const change of ['filter', 'page', 'section']) {
  test(`deletion restarts an interrupted ${change} query and rejects its stale response`, async (t) => {
    const f = await fixture(t)
    const state = f.store.getState
    await state().loadVideos()
    const pending = []
    t.mock.method(
      globalThis,
      'fetch',
      (input, init) =>
        new Promise((resolve) => {
          pending.push({ url: new URL(input, 'http://localhost'), signal: init.signal, resolve })
        })
    )
    if (change === 'section') state().setViewMode('jav')
    else if (change === 'filter') state().setSearchTerm('filtered')
    else state().setPage(2)
    const load = change === 'section' ? 'loadJavs' : 'loadVideos'
    const items = change === 'section' ? 'javItems' : 'videos'
    const oldLoad = state()[load]()
    const deleting = f.remove([1])
    assert.equal(pending.length, 2)
    assert.equal(pending[0].signal.aborted, true)
    assert.equal(pending[1].url.href, pending[0].url.href)
    pending[1].resolve(f.response(pending[1].url))
    await deleting
    const expected = state()[items]
    assert.ok(expected.length > 0)
    pending[0].resolve(Response.json({ items: [{ id: 999 }], total: 999 }))
    await oldLoad
    assert.deepEqual(state()[items], expected)
    assert.equal(state()[change === 'section' ? 'javLoading' : 'loading'], false)
  })
}

test('overlapping reconciliations reject stale results and block append until the current range is ready', async (t) => {
  const f = await fixture(t)
  const state = f.store.getState
  f.store.setState({ waterfallModes: { video: true } })
  await state().loadVideos()
  await state().loadMoreVideos()
  const pending = []
  t.mock.method(
    globalThis,
    'fetch',
    (input, init) =>
      new Promise((resolve) => {
        pending.push({ url: new URL(input, 'http://localhost'), signal: init.signal, resolve })
      })
  )
  const append = state().loadMoreVideos()
  const first = f.remove([1])
  const second = f.remove([2])
  assert.equal(state().loading, false)
  const blockedAppend = state().loadMoreVideos()
  assert.equal(pending.length, 3)
  assert.equal(pending[0].signal.aborted, true)
  assert.equal(pending[1].signal.aborted, true)
  pending[2].resolve(f.response(pending[2].url))
  await Promise.all([second, blockedAppend])
  for (const request of pending.slice(0, 2)) {
    request.resolve(Response.json({ items: [{ id: 999 }], total: 999 }))
  }
  await Promise.all([append, first])
  assert.deepEqual(
    state().videos.map((row) => row.id),
    [3, 4]
  )
  assert.equal(state().total, 4)
  assert.equal(state().videoLoadingMore, false)
})

test('a failed background reconciliation retains remaining rows and supports retry', async (t) => {
  const f = await fixture(t)
  const state = f.store.getState
  await state().loadVideos()
  t.mock.method(globalThis, 'fetch', async () => {
    throw new Error('offline')
  })
  await f.remove([1])
  assert.deepEqual(
    state().videos.map((row) => row.id),
    [2]
  )
  assert.equal(state().loading, false)
  assert.equal(state().videoLoadingMore, false)
  assert.equal(state().error, 'offline')
  t.mock.method(globalThis, 'fetch', async (input) =>
    f.response(new URL(input, 'http://localhost'))
  )
  await state().loadVideos()
  assert.deepEqual(
    state().videos.map((row) => row.id),
    [2, 3]
  )
  assert.equal(state().error, null)
})
