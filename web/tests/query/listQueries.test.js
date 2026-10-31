import assert from 'node:assert/strict'
import test from 'node:test'
import { createStore } from 'zustand/vanilla'
import { loadModules } from '../helpers/modules.js'

test('query keys include every effective filter and directory visibility', async (t) => {
  const [queries, { createAppState }] = await loadModules(t, ['query/listQueries.js', 'store.js'])
  const store = createStore(createAppState)
  const state = store.getState()
  assert.notEqual(
    queries.videoQueryKey(state),
    queries.videoQueryKey({ ...state, videoHideJav: true })
  )
  assert.notEqual(
    queries.videoQueryKey(state),
    queries.videoQueryKey({ ...state, directories: [{ id: 1, enabled: true }] })
  )
  assert.notEqual(
    queries.javQueryKey(state),
    queries.javQueryKey({ ...state, javFavoriteRatingEnabled: true, javFavoriteRatingMin: 3 })
  )
  assert.equal(queries.javQuery({ ...state, javStudioId: 0 }).studioId, 0)
  assert.equal(queries.idolQuery({ ...state, idolFavoriteGroupId: 1 }).sort, '')
  assert.equal(
    queries.idolQuery({ ...state, idolFavoriteGroupId: 1, idolTempSort: 'name' }).sort,
    'name'
  )
})

test('list resources are independent per store and forward cancellation to the API', async (t) => {
  const [{ createAppState }] = await loadModules(t, ['store.js'])
  const first = createStore(createAppState)
  const second = createStore(createAppState)
  const requests = []
  t.mock.method(globalThis, 'fetch', async (url, init) => {
    requests.push({ url, init })
    return Response.json({ items: [{ id: requests.length }], total: 100 })
  })
  await first.getState().loadVideos()
  await second.getState().loadVideos()
  assert.equal(requests.length, 2)
  assert.ok(requests[0].init.signal instanceof AbortSignal)
  assert.notDeepEqual(first.getState().videos, second.getState().videos)
  first.setState({ javRandomMode: true, javRandomSeed: 42, javPage: 8 })
  await first.getState().loadJavs()
  assert.equal(first.getState().javTotal, 1)
  const url = new URL(requests[2].url, 'http://localhost')
  assert.equal(url.searchParams.get('seed'), '42')
  assert.equal(url.searchParams.get('offset'), '0')
  assert.equal(url.searchParams.get('sort'), 'random')
  await first.getState().loadMoreJavs()
  assert.equal(requests.length, 3)
})

test('item edits merge into list data without losing unrelated fields or other entries', async (t) => {
  const [{ createAppState }] = await loadModules(t, ['store.js'])
  const store = createStore(createAppState)
  const other = { id: 2, title: 'other' }
  store.setState({
    javItems: [
      { id: 1, title: 'original', videos: [{ id: 7 }], idols: [{ id: 3, name: 'before' }] },
      other,
    ],
  })
  store.getState().patchJavItem({ id: 1, favorite_rating: 4 })
  store.getState().patchJavIdol({ id: 3, name: 'after' })
  assert.equal(store.getState().javItems[0].favorite_rating, 4)
  assert.equal(store.getState().javItems[0].title, 'original')
  assert.deepEqual(store.getState().javItems[0].videos, [{ id: 7 }])
  assert.equal(store.getState().javItems[0].idols[0].name, 'after')
  assert.equal(store.getState().javItems[1], other)
})

test('directory changes invalidate videos and carry through paging, waterfall and random requests', async (t) => {
  const [queries, { createAppState }] = await loadModules(t, ['query/listQueries.js', 'store.js'])
  const store = createStore(createAppState)
  const requests = []
  t.mock.method(globalThis, 'fetch', async (url) => {
    requests.push(new URL(url, 'http://localhost').searchParams)
    return Response.json({
      items: Array.from({ length: 25 }, (_, index) => ({
        id: Number(requests.at(-1).get('offset')) + index + 1,
        location_id: Number(requests.at(-1).get('offset')) + index + 1,
      })),
      total: 100,
    })
  })
  const before = queries.videoQueryKey(store.getState())
  store.setState({
    page: 4,
    searchTerm: 'movie',
    selectedTags: ['favorite'],
    videoTempSort: 'filename',
    randomMode: true,
    randomSeed: 42,
  })
  store.getState().setVideoDirectoryFilter({ directory_id: 2, path: '旅行/100%_fun' })
  assert.equal(store.getState().page, 1)
  assert.equal(store.getState().searchTerm, '')
  assert.deepEqual(store.getState().selectedTags, [])
  assert.equal(store.getState().videoTempSort, '')
  assert.equal(store.getState().randomMode, false)
  assert.equal(store.getState().randomSeed, null)
  const filtered = queries.videoQueryKey(store.getState())
  assert.notEqual(before, filtered)
  assert.notEqual(
    filtered,
    queries.videoQueryKey({ ...store.getState(), videoSubdirectory: 'elsewhere' })
  )
  assert.notEqual(filtered, queries.videoQueryKey({ ...store.getState(), videoDirectoryId: 3 }))
  await store.getState().loadVideos()
  assert.equal(requests[0].has('search'), false)
  assert.equal(requests[0].has('tags'), false)
  assert.equal(requests[0].has('seed'), false)
  assert.equal(requests[0].get('sort'), 'recent')
  store.getState().setPage(2)
  await store.getState().loadVideos()
  store.setState({ waterfallModes: { video: true } })
  await store.getState().loadMoreVideos()
  store.getState().loadRandom(42)
  await store.getState().loadVideos()
  assert.equal(requests.length, 4)
  assert.equal(requests[1].get('offset'), '25')
  assert.equal(requests[2].get('offset'), '50')
  assert.equal(requests[3].get('seed'), '42')
  for (const query of requests) {
    assert.equal(query.get('directory_id'), '2')
    assert.equal(query.get('subdirectory'), '旅行/100%_fun')
  }
  store.setState({ searchTerm: 'movie', selectedTags: ['favorite'], videoTempSort: 'filename' })
  store.getState().setVideoDirectoryFilter(null)
  await store.getState().loadVideos()
  assert.equal(requests.at(-1).has('directory_id'), false)
  assert.equal(requests.at(-1).has('subdirectory'), false)
  assert.equal(requests.at(-1).has('search'), false)
  assert.equal(requests.at(-1).has('tags'), false)
  assert.equal(requests.at(-1).has('seed'), false)
  assert.equal(requests.at(-1).get('sort'), 'recent')
})
