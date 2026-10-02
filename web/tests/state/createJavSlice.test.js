import assert from 'node:assert/strict'
import test from 'node:test'
import { loadModules } from '../helpers/modules.js'

test('toggling compact mode persists the choice and remembers it on reload', async (t) => {
  const [{ useStore }] = await loadModules(t, ['store.js'])
  let patchPayload = null
  t.mock.method(globalThis, 'fetch', async (_url, init = {}) => {
    if (init?.body) {
      patchPayload = JSON.parse(init.body)
      return Response.json({ ...patchPayload })
    }
    return Response.json({ jav_compact_default: 'true' })
  })

  await useStore.getState().setJavCompactMode(true)
  assert.deepEqual(patchPayload, { jav_compact_default: true })
  assert.equal(useStore.getState().javCompactMode, true)

  // Simulate a fresh page load reading the stored preference.
  useStore.setState({ javCompactMode: false, config: {} })
  await useStore.getState().loadConfig()
  assert.equal(useStore.getState().javCompactMode, true)
})

test('a failed compact toggle rolls the choice back', async (t) => {
  const [{ useStore }] = await loadModules(t, ['store.js'])
  t.mock.method(globalThis, 'fetch', async () =>
    Response.json({ error_en: 'Save failed' }, { status: 500 })
  )

  useStore.setState({ javCompactMode: false, javError: null })
  await useStore.getState().setJavCompactMode(true)
  assert.equal(useStore.getState().javCompactMode, false)
  assert.ok(useStore.getState().javError)
})
