import assert from 'node:assert/strict'
import test from 'node:test'
import { runProviderChecks } from '../src/utils/providerAvailability.js'

test('checks at most three providers concurrently and publishes partial results', async () => {
  const providers = Array.from({ length: 5 }, (_, id) => ({ id }))
  const pending = new Map()
  const results = []
  let active = 0
  let peak = 0
  const check = (id) => {
    active++
    peak = Math.max(peak, active)
    return new Promise((resolve) => {
      pending.set(id, () => {
        active--
        resolve({ status: 'ok' })
      })
    })
  }
  const done = runProviderChecks(
    providers,
    check,
    (id, result) => results.push([id, result.status]),
    new AbortController().signal
  )
  assert.deepEqual([...pending.keys()], [0, 1, 2])
  pending.get(1)()
  await Promise.resolve()
  assert.ok(results.some(([id, status]) => id === 1 && status === 'ok'))
  assert.ok(pending.has(3))
  pending.get(0)()
  await Promise.resolve()
  for (const id of [2, 3, 4]) pending.get(id)()
  await done
  assert.equal(peak, 3)
  assert.equal(results.filter(([, status]) => status === 'ok').length, 5)
})

test('a failed API request does not stop other provider checks', async () => {
  const results = new Map()
  await runProviderChecks(
    [{ id: 1 }, { id: 2 }],
    async (id) => {
      if (id === 1) throw new Error('server unavailable')
      return { status: 'http_error', http_status: 403 }
    },
    (id, result) => results.set(id, result),
    new AbortController().signal
  )
  assert.equal(results.get(1).status, 'request_error')
  assert.equal(results.get(2).http_status, 403)
})

test('cancellation stops queued work and ignores late results', async () => {
  const controller = new AbortController()
  const pending = []
  const results = []
  const done = runProviderChecks(
    Array.from({ length: 6 }, (_, id) => ({ id })),
    (_, { signal }) => {
      assert.equal(signal, controller.signal)
      return new Promise((resolve) => pending.push(resolve))
    },
    (id, result) => results.push([id, result.status]),
    controller.signal
  )
  controller.abort()
  for (const resolve of pending) resolve({ status: 'ok' })
  await done
  assert.equal(pending.length, 3)
  assert.deepEqual(results, [
    [0, 'checking'],
    [1, 'checking'],
    [2, 'checking'],
  ])
})
