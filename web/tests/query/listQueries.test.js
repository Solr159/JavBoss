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
