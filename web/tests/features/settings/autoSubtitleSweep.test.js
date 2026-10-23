import assert from 'node:assert/strict'
import test from 'node:test'
import { loadModules } from '../../helpers/modules.js'

test('auto subtitle sweep helpers describe the last run', async (t) => {
  const [sweep] = await loadModules(t, ['features/settings/autoSubtitleSweep.js'])

  const summary = sweep.autoSubtitleRunSummary({
    scanned: 12,
    downloaded: 3,
    skipped: 8,
    failed: 1,
  })
  for (const value of ['12', '3', '8', '1']) {
    assert.ok(summary.includes(value), `summary should mention ${value}: ${summary}`)
  }
  assert.equal(sweep.autoSubtitleRunSummary(null), '')
  assert.ok(
    sweep.autoSubtitleRunSummary({}).length > 0,
    'an empty summary still renders counts instead of failing'
  )

  assert.equal(sweep.autoSubtitleRunTime('not-a-date'), '')
  assert.match(sweep.autoSubtitleRunTime('2026-10-23T10:00:00Z'), /\d{2}:\d{2}/)
})

test('auto subtitle sweep errors are localized by code', async (t) => {
  const [sweep] = await loadModules(t, ['features/settings/autoSubtitleSweep.js'])

  const gateway = sweep.autoSubtitleRunError({
    error: 'auto subtitle gateway is unavailable: gateway is unreachable: dial tcp 127.0.0.1:8765',
    error_code: 'gateway_unavailable',
  })
  assert.equal(gateway, 'The AutoSub gateway is unavailable. Make sure it is running.')
  assert.doesNotMatch(gateway, /dial tcp/)

  assert.equal(
    sweep.autoSubtitleRunError({ error: 'server detail', error_code: 'future_code' }),
    'server detail'
  )
  assert.equal(sweep.autoSubtitleRunError({ error: 'server detail' }), 'server detail')
  assert.equal(sweep.autoSubtitleRunError({}), '')
  assert.equal(sweep.autoSubtitleRunError(null), '')
})

test('auto subtitle sweep errors follow the Chinese locale', async (t) => {
  const original = Object.getOwnPropertyDescriptor(globalThis, 'navigator')
  Object.defineProperty(globalThis, 'navigator', {
    value: { languages: ['zh-CN'] },
    configurable: true,
    writable: true,
  })
  t.after(() => {
    if (original) Object.defineProperty(globalThis, 'navigator', original)
    else delete globalThis.navigator
  })

  const [sweep] = await loadModules(t, ['features/settings/autoSubtitleSweep.js'])
  assert.equal(
    sweep.autoSubtitleRunError({ error: 'server detail', error_code: 'gateway_unavailable' }),
    '自动字幕网关不可用，请确认网关已启动。'
  )
})

test('auto subtitle sweep API targets the status and run endpoints', async (t) => {
  const [api] = await loadModules(t, ['features/settings/api.js'])
  const calls = []
  t.mock.method(globalThis, 'fetch', async (url, init = {}) => {
    calls.push([String(url), init.method || 'GET'])
    return Response.json({ available: true, running: true, started: true, last_run: null })
  })

  const status = await api.fetchAutoSubtitleSweep()
  assert.equal(status.running, true)
  const started = await api.runAutoSubtitleSweep()
  assert.equal(started.started, true)

  assert.deepEqual(calls, [
    ['/autosub/auto/status', 'GET'],
    ['/autosub/auto/run', 'POST'],
  ])
})

test('auto subtitle sweep API surfaces server errors', async (t) => {
  const [api] = await loadModules(t, ['features/settings/api.js'])
  t.mock.method(globalThis, 'fetch', async () =>
    Response.json(
      { error: 'not ready', error_en: 'The auto subtitle sweep is not ready yet.' },
      { status: 503 }
    )
  )

  await assert.rejects(api.runAutoSubtitleSweep(), /not ready yet/)
})
