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
