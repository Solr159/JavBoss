import assert from 'node:assert/strict'
import test from 'node:test'
import { createStore } from 'zustand/vanilla'
import { loadModules } from '../helpers/modules.js'

test('link overrides and URL state serialization round trip through one query model', async (t) => {
  const [links, urls, { createAppState }] = await loadModules(t, [
    'navigation/queryLinks.js',
    'utils/urlState.js',
    'store.js',
  ])
  const store = createStore(createAppState)
  const parse = (link) => urls.parseUrlState(new URL(link, 'http://localhost').search)
  store.setState({
    tags: [{ id: 9, name: 'tag' }],
    selectedTags: ['tag'],
    page: 3,
    searchTerm: ' query ',
  })
  const state = store.getState()
  const video = links.buildVideoQueryLink(state, {}, '/library')
  assert.equal(
    video,
    urls.buildUrlFromState(
      urls.normalizeUrlStateFromStore(state, new Map([['tag', 9]])),
      '/library'
    )
  )
  assert.equal(parse(video).video.page, 3)
  assert.deepEqual(parse(video).video.tagIds, [9])
  assert.equal(parse(video).video.search, 'query')
  const random = links.buildVideoQueryLink(
    state,
    { random: true, seed: 123, tempSort: 'recent' },
    '/library'
  )
  assert.equal(parse(random).video.seed, 123)
  assert.equal(new URL(random, 'http://localhost').searchParams.has('page'), false)
  assert.equal(new URL(random, 'http://localhost').searchParams.has('temp_sort'), false)
})

test('JAV links clear explicit relations and keep unknown studio zero distinct from no filter', async (t) => {
  const [links, urls, { createAppState }] = await loadModules(t, [
    'navigation/queryLinks.js',
    'utils/urlState.js',
    'store.js',
  ])
  const store = createStore(createAppState)
  store.setState({
    javStudioId: 5,
    javStudioName: 'Old studio',
    javSeriesId: 3,
    javSeriesName: 'Old series',
    javFavoriteGroupId: 7,
  })
  const query = (options) =>
    urls.parseUrlState(
      new URL(links.buildJavQueryLink(store.getState(), options), 'http://localhost').search
    ).jav
  assert.equal(query({ studioId: 0 }).studioId, 0)
  assert.equal(query({ studioId: 0 }).studioName, '')
  const cleared = query({ studioId: null, seriesId: null, favoriteGroupId: null })
  assert.equal(cleared.studioId, null)
  assert.equal(cleared.seriesId, null)
  assert.equal(cleared.favoriteGroupId, null)
  assert.equal(cleared.studioName, '')
  assert.equal(cleared.seriesName, '')
})

test('links to another catalog use its own page, favorites and sorting', async (t) => {
  const [links, urls, { createAppState }] = await loadModules(t, [
    'navigation/queryLinks.js',
    'utils/urlState.js',
    'store.js',
  ])
  const store = createStore(createAppState)
  store.setState({
    javRandomMode: true,
    javRandomSeed: 12,
    javPage: 6,
    idolPage: 3,
    idolFavoriteGroupId: 4,
    javFavoriteGroupId: 7,
  })
  const link = links.buildJavQueryLink(store.getState(), { tab: 'idol', tempSort: '' })
  const query = urls.parseUrlState(new URL(link, 'http://localhost').search).jav
  assert.equal(query.page, 3)
  assert.equal(query.favoriteGroupId, 4)
  assert.equal(query.random, false)
  assert.equal(query.tempSort, '')
})

test('video directory filters survive pagination and random links and can be cleared', async (t) => {
  const [links, urls, { createAppState }] = await loadModules(t, [
    'navigation/queryLinks.js',
    'utils/urlState.js',
    'store.js',
  ])
  const store = createStore(createAppState)
  store.setState({
    videoDirectoryId: 7,
    videoSubdirectory: '旅行/100%_fun & friends',
    searchTerm: 'movie',
  })
  const query = (options) =>
    urls.parseUrlState(
      new URL(links.buildVideoQueryLink(store.getState(), options), 'http://localhost').search
    ).video
  for (const options of [{ page: 3 }, { random: true, seed: 42 }, { search: 'another' }]) {
    assert.equal(query(options).directoryId, 7)
    assert.equal(query(options).subdirectory, '旅行/100%_fun & friends')
  }
  const cleared = query({ directoryId: null, subdirectory: '' })
  assert.equal(cleared.directoryId, null)
  assert.equal(cleared.subdirectory, '')
})
