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

test('translation settings keep saved keys private and omit untouched credentials', async (t) => {
  const [model, actions, { useStore }] = await loadModules(t, [
    'features/settings/model.js',
    'features/settings/actions.js',
    'store.js',
  ])
  useStore.setState({
    config: {
      jav_title_translation_enabled: 'true',
      jav_title_translation_api_key_set: 'true',
      jav_title_translation_model: 'official-model',
      jav_title_translation_prompt: 'Custom prompt',
    },
  })
  let payload
  t.mock.method(globalThis, 'fetch', async (_url, init) => {
    payload = JSON.parse(init.body)
    return Response.json({
      ...payload,
      jav_title_translation_api_key: undefined,
      jav_title_translation_api_key_set:
        payload.jav_title_translation_api_key === '' ? 'false' : 'true',
    })
  })
  const draft = model.createTitleTranslationDraft(useStore.getState().config)
  assert.equal(draft.apiKey, '')
  assert.equal(draft.thinking, false)
  await actions.saveTitleTranslationSettings(draft)
  assert.equal(payload.jav_title_translation_thinking, false)
  assert.equal(Object.hasOwn(payload, 'jav_title_translation_api_key'), false)
  assert.equal(payload.jav_title_translation_prompt, 'Custom prompt')
  draft.apiKey = 'replacement-key'
  draft.thinking = true
  await actions.saveTitleTranslationSettings(draft)
  assert.equal(payload.jav_title_translation_thinking, true)
  assert.equal(model.createTitleTranslationDraft(useStore.getState().config).thinking, true)
  assert.equal(payload.jav_title_translation_api_key, 'replacement-key')
  assert.equal(useStore.getState().config.jav_title_translation_api_key, undefined)
  Object.assign(draft, {
    enabled: false,
    apiKey: '',
    clearApiKey: true,
    hasApiKey: false,
  })
  await actions.saveTitleTranslationSettings(draft)
  assert.equal(payload.jav_title_translation_api_key, '')
})

test('display settings only change the shared toggle and leave tool configuration untouched', async (t) => {
  const [model, actions, { useStore }] = await loadModules(t, [
    'features/settings/model.js',
    'features/settings/actions.js',
    'store.js',
  ])
  let payload
  t.mock.method(globalThis, 'fetch', async (_url, init) => {
    payload = JSON.parse(init.body)
    return Response.json(payload)
  })
  const draft = model.createJavSettingsDraft(useStore.getState())
  draft.titleTranslationEnabled = true
  await actions.saveJavSettings(draft, () => {})
  assert.equal(payload.jav_title_translation_enabled, true)
  assert.equal(Object.hasOwn(payload, 'jav_title_translation_api_key'), false)
  assert.equal(Object.hasOwn(payload, 'jav_title_translation_model'), false)
  assert.equal(Object.hasOwn(payload, 'jav_title_translation_prompt'), false)
  assert.equal(Object.hasOwn(payload, 'jav_title_translation_thinking'), false)
})
