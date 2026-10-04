import assert from 'node:assert/strict'
import test from 'node:test'
import { createListResource } from '../../src/query/createListResource.js'

function fixture() {
  let state = { search: 'first', scope: 1, items: [], total: 0, page: 1, limit: 2 }
  const requests = []
  const get = () => state
  const set = (patch) => {
    state = { ...state, ...patch }
  }
  const resource = createListResource({
    get,
    set,
    fields: {
      items: 'items',
      total: 'total',
      loading: 'loading',
      loadingMore: 'loadingMore',
      error: 'error',
    },
    key: (s) => JSON.stringify([s.search, s.scope, s.page, s.limit]),
    query: (s) => ({ search: s.search, limit: s.limit, offset: (s.page - 1) * s.limit }),
    fetcher: (params) =>
      new Promise((resolve, reject) => requests.push({ params, resolve, reject })),
  })
  return { get, set, requests, ...resource }
}

test('identical in-flight loads share work; successful loads are reused until invalidation', async () => {
  const f = fixture()
  const first = f.load()
  assert.equal(f.load(), first)
  assert.equal(f.requests.length, 1)
  f.requests[0].resolve({ items: [{ id: 1 }], total: 1 })
  await first
  await f.load()
  assert.equal(f.requests.length, 1)
  f.invalidate()
  const refresh = f.load()
  assert.equal(f.requests.length, 2)
  f.requests[1].resolve({ items: [], total: 0 })
  await refresh
})

test('a failed request can be retried without a force option', async () => {
  const f = fixture()
  const first = f.load()
  f.requests[0].reject(new Error('offline'))
  await first
  assert.equal(f.get().error, 'offline')
  assert.equal(f.get().loading, false)
  const retry = f.load()
  assert.equal(f.requests.length, 2)
  f.requests[1].resolve({ items: [{ id: 2 }], total: 1 })
  await retry
  assert.equal(f.get().error, null)
  assert.deepEqual(f.get().items, [{ id: 2 }])
})

test('changing filters cancels the old request and rejects a late response even if abort is ignored', async () => {
  const f = fixture()
  const first = f.load()
  f.set({ search: 'second' })
  const second = f.load()
  assert.equal(f.requests[0].params.signal.aborted, true)
  f.requests[1].resolve({ items: [{ id: 2 }], total: 1 })
  await second
  f.requests[0].resolve({ items: [{ id: 1 }], total: 1 })
  await first
  assert.deepEqual(f.get().items, [{ id: 2 }])
  assert.equal(f.get().loading, false)
})

test('a directory change invalidates an in-flight response even before the next load starts', async () => {
  const f = fixture()
  const first = f.load()
  f.set({ scope: 2 })
  f.requests[0].resolve({ items: [{ id: 1 }], total: 1 })
  await first
  assert.deepEqual(f.get().items, [])
})

test('append uses the base page offset, joins concurrent loads and keeps edits made while loading', async () => {
  const f = fixture()
  f.set({ page: 3 })
  const first = f.load()
  f.requests[0].resolve({ items: [{ id: 5 }, { id: 6 }], total: 10 })
  await first
  const more = f.loadMore()
  assert.equal(f.loadMore(), more)
  assert.equal(f.requests[1].params.offset, 6)
  f.set({ items: [{ id: 5, title: 'edited' }, { id: 6 }] })
  f.requests[1].resolve({ items: [{ id: 7 }, { id: 8 }], total: 10 })
  await more
  assert.deepEqual(f.get().items, [{ id: 5, title: 'edited' }, { id: 6 }, { id: 7 }, { id: 8 }])
})

test('refresh cancels append work and stale completion cannot append to the replacement list', async () => {
  const f = fixture()
  const first = f.load()
  f.requests[0].resolve({ items: [{ id: 1 }], total: 5 })
  await first
  const more = f.loadMore()
  const refresh = f.load({ force: true })
  assert.equal(f.requests[1].params.signal.aborted, true)
  f.requests[2].resolve({ items: [{ id: 3 }], total: 1 })
  await refresh
  f.requests[1].resolve({ items: [{ id: 2 }], total: 5 })
  await more
  assert.deepEqual(f.get().items, [{ id: 3 }])
  assert.equal(f.get().loadingMore, false)
})

test('an empty append stops repeated requests even when the server reports an outdated total', async () => {
  const f = fixture()
  const first = f.load()
  f.requests[0].resolve({ items: [{ id: 1 }], total: 5 })
  await first
  const more = f.loadMore()
  f.requests[1].resolve({ items: [], total: 5 })
  await more
  await f.loadMore()
  assert.equal(f.requests.length, 2)
})

test('invalidation cancels stale append responses while preserving the patched list and next offset', async () => {
  const f = fixture()
  f.set({ page: 3 })
  const first = f.load()
  f.requests[0].resolve({ items: [{ id: 5 }, { id: 6 }], total: 12 })
  await first
  const more = f.loadMore()
  f.requests[1].resolve({ items: [{ id: 7 }, { id: 8 }], total: 12 })
  await more
  const stale = f.loadMore()
  f.invalidate()
  f.set({ items: [{ id: 5 }, { id: 7 }, { id: 8 }], total: 11 })
  assert.equal(f.get().loading, false)
  assert.equal(f.get().loadingMore, false)
  assert.equal(f.requests.length, 3, 'invalidation does not refetch')
  assert.equal(f.requests[2].params.signal.aborted, true)
  f.requests[2].resolve({ items: [{ id: 9 }, { id: 10 }], total: 12 })
  await stale
  assert.deepEqual(f.get().items, [{ id: 5 }, { id: 7 }, { id: 8 }])
  const next = f.loadMore()
  assert.equal(f.requests[3].params.offset, 7)
  assert.equal(f.requests[3].params.limit, 2)
  f.requests[3].resolve({ items: [{ id: 9 }, { id: 10 }], total: 11 })
  await next
  assert.deepEqual(
    f.get().items.map((item) => item.id),
    [5, 7, 8, 9, 10]
  )
  const revisit = f.load()
  assert.equal(f.requests.length, 5, 'the next visit revalidates the stale list')
  f.requests[4].resolve({ items: [{ id: 5 }, { id: 7 }], total: 11 })
  await revisit
})

test('invalidation cancels an in-flight reload and does not append old data under new filters', async () => {
  const f = fixture()
  const first = f.load()
  f.requests[0].resolve({ items: [{ id: 1 }, { id: 2 }], total: 4 })
  await first
  const refresh = f.load({ force: true })
  f.invalidate()
  f.set({ items: [{ id: 2 }], total: 3 })
  f.requests[1].resolve({ items: [{ id: 1 }, { id: 2 }], total: 4 })
  await refresh
  assert.deepEqual(f.get().items, [{ id: 2 }])
  f.set({ search: 'changed' })
  await f.loadMore()
  assert.equal(f.requests.length, 2)
  const changed = f.load()
  assert.equal(f.get().loading, true)
  f.requests[2].resolve({ items: [], total: 0 })
  await changed
})
