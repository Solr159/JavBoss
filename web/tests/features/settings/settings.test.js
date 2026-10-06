import assert from 'node:assert/strict'
import test from 'node:test'
import { loadModules } from '../../helpers/modules.js'

test('settings drafts stay local and a reopened dialog reads persisted values', async (t) => {
  const [model, { useStore }] = await loadModules(t, ['features/settings/model.js', 'store.js'])
  const state = useStore.getState()
  const draft = model.createVideoSettingsDraft(state)
  draft.videoPageSizeInput = 100
  draft.videoHideJavInput = true
  assert.equal(useStore.getState().pageSize, state.pageSize)
  assert.equal(
    model.createVideoSettingsDraft(useStore.getState()).videoPageSizeInput,
    state.pageSize
  )
  const javDraft = model.createJavSettingsDraft({
    ...state,
    config: { jav_hide_tags: 'false', jav_hide_series: '1' },
  })
  assert.equal(javDraft.javHideTagsInput, false)
  assert.equal(javDraft.javHideSeriesInput, true)
  assert.equal(javDraft.javPortraitModeInput, false)
  assert.equal(
    model.createJavSettingsDraft({ ...state, config: { jav_portrait_mode: 'true' } })
      .javPortraitModeInput,
    true
  )
})

test('saving portrait mode persists it and reopening reads the saved choice', async (t) => {
  const [model, actions, { useStore }] = await loadModules(t, [
    'features/settings/model.js',
    'features/settings/actions.js',
    'store.js',
  ])
  t.mock.method(globalThis, 'fetch', async (_url, init) => Response.json(JSON.parse(init.body)))
  for (const enabled of [true, false]) {
    const draft = model.createJavSettingsDraft(useStore.getState())
    draft.javPortraitModeInput = enabled
    await actions.saveJavSettings(draft, () => {})
    assert.equal(useStore.getState().config.jav_portrait_mode, enabled)
    assert.equal(model.createJavSettingsDraft(useStore.getState()).javPortraitModeInput, enabled)
  }
})

test('saving video settings persists normalized values and clamps the current page', async (t) => {
  const [model, actions, { useStore }] = await loadModules(t, [
    'features/settings/model.js',
    'features/settings/actions.js',
    'store.js',
  ])
  useStore.setState({ page: 9, total: 100, pageSize: 25, randomMode: true, randomSeed: 7 })
  let payload
  t.mock.method(globalThis, 'fetch', async (_url, init) => {
    payload = JSON.parse(init.body)
    return Response.json(payload)
  })
  const draft = model.createVideoSettingsDraft(useStore.getState())
  draft.videoPageSizeInput = '40'
  draft.videoWaterfallDefaultInput = true
  const changes = []
  await actions.saveVideoSettings(draft, (...args) => changes.push(args))
  assert.equal(payload.video_page_size, 40)
  assert.equal(useStore.getState().page, 3)
  assert.equal(useStore.getState().randomMode, false)
  assert.equal(useStore.getState().randomSeed, null)
  assert.deepEqual(changes, [['video', true]])
})

test('a failed settings save leaves configuration and list state unchanged', async (t) => {
  const [model, actions, { useStore }] = await loadModules(t, [
    'features/settings/model.js',
    'features/settings/actions.js',
    'store.js',
  ])
  const before = useStore.getState()
  t.mock.method(globalThis, 'fetch', async () =>
    Response.json({ error_en: 'Save failed' }, { status: 500 })
  )
  await assert.rejects(
    actions.saveJavSettings(model.createJavSettingsDraft(before), () => {
      assert.fail('failed saves must not change the display mode')
    })
  )
  assert.equal(useStore.getState(), before)
})
