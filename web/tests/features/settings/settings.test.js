import assert from 'node:assert/strict'
import test from 'node:test'
import { loadModules } from '../../helpers/modules.js'

test('settings drafts stay local and a reopened dialog reads persisted values', async (t) => {
  const [model, { useStore }] = await loadModules(t, ['features/settings/model.js', 'store.js'])
  const state = useStore.getState()
  const draft = model.createVideoSettingsDraft(state)
  assert.equal(draft.videoWatchTimeIconMinutesInput, 30)
  draft.videoPageSizeInput = 100
  draft.videoHideJavInput = true
  assert.equal(useStore.getState().pageSize, state.pageSize)
  assert.equal(
    model.createVideoSettingsDraft(useStore.getState()).videoPageSizeInput,
    state.pageSize
  )
  const javDraft = model.createJavSettingsDraft({
    ...state,
    config: { jav_hide_tags: 'false', jav_hide_series: '1', jav_watch_time_icon_minutes: '15' },
  })
  assert.equal(javDraft.javWatchTimeIconMinutesInput, 15)
  assert.equal(model.createJavSettingsDraft(state).javWatchTimeIconMinutesInput, 30)
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
  draft.videoWatchTimeIconMinutesInput = '10'
  draft.videoWaterfallDefaultInput = true
  draft.autoSubtitleEnabledInput = true
  draft.autoSubtitleLanguageInput = ' English '
  const changes = []
  await actions.saveVideoSettings(draft, (...args) => changes.push(args))
  assert.equal(payload.video_page_size, 40)
  assert.equal(payload.video_watch_time_icon_minutes, 10)
  assert.equal(payload.autosub_auto_enabled, true)
  assert.equal(payload.autosub_auto_language, 'English')
  assert.equal(
    model.createVideoSettingsDraft(useStore.getState()).videoWatchTimeIconMinutesInput,
    10
  )
  assert.equal(useStore.getState().page, 3)
  assert.equal(useStore.getState().randomMode, false)
  assert.equal(useStore.getState().randomSeed, null)
  assert.deepEqual(changes, [['video', true]])
  const savedDraft = model.createVideoSettingsDraft(useStore.getState())
  assert.equal(savedDraft.autoSubtitleEnabledInput, true)
  assert.equal(savedDraft.autoSubtitleLanguageInput, 'English')
})

test('auto subtitle settings default to disabled with the gateway language', async (t) => {
  const [model] = await loadModules(t, ['features/settings/model.js'])
  const draft = model.createVideoSettingsDraft({ config: {} })
  assert.equal(draft.autoSubtitleEnabledInput, false)
  assert.equal(draft.autoSubtitleLanguageInput, 'Chinese Simplified')
  assert.equal(model.normalizeSubtitleLanguage('  '), 'Chinese Simplified')
  assert.equal(model.normalizeSubtitleLanguage(' Japanese '), 'Japanese')
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

test('JAV watch icon settings persist independently and invalid values use 30 minutes', async (t) => {
  const [model, actions, { useStore }] = await loadModules(t, [
    'features/settings/model.js',
    'features/settings/actions.js',
    'store.js',
  ])
  let config = { video_watch_time_icon_minutes: '10' }
  useStore.setState({ config })
  t.mock.method(globalThis, 'fetch', async (_url, init) => {
    config = { ...config, ...JSON.parse(init.body) }
    return Response.json(config)
  })
  for (const [input, expected] of [
    ['45', 45],
    ['', 30],
    ['0', 30],
    ['-1', 30],
    ['abc', 30],
    ['1.5', 30],
  ]) {
    const draft = model.createJavSettingsDraft(useStore.getState())
    draft.javWatchTimeIconMinutesInput = input
    await actions.saveJavSettings(draft, () => {})
    assert.equal(
      model.createJavSettingsDraft(useStore.getState()).javWatchTimeIconMinutesInput,
      expected
    )
    assert.equal(
      model.createVideoSettingsDraft(useStore.getState()).videoWatchTimeIconMinutesInput,
      10
    )
  }
})

test('translation settings keep saved keys private and omit untouched credentials', async (t) => {
  const [model, actions, { useStore }] = await loadModules(t, [
    'features/settings/titleTranslation.js',
    'features/settings/actions.js',
    'store.js',
  ])
  useStore.setState({
    config: {
      jav_title_translation_enabled: 'true',
      jav_title_translation_profiles: JSON.stringify({
        deepseek: { api_key_set: true, model: 'official-model' },
      }),
      jav_title_translation_prompt: 'Custom prompt',
    },
  })
  let payload
  t.mock.method(globalThis, 'fetch', async (_url, init) => {
    payload = JSON.parse(init.body)
    const profile = payload.jav_title_translation_profiles.deepseek
    return Response.json({
      ...payload,
      jav_title_translation_enabled: useStore.getState().config.jav_title_translation_enabled,
      jav_title_translation_profiles: JSON.stringify({
        deepseek: { ...profile, api_key: undefined, api_key_set: profile.api_key !== '' },
      }),
    })
  })
  const draft = model.createTitleTranslationDraft(useStore.getState().config)
  assert.equal(draft.apiKey, '')
  assert.equal(draft.thinking, false)
  await actions.saveTitleTranslationSettings(draft)
  assert.equal(payload.jav_title_translation_profiles.deepseek.thinking, false)
  assert.equal(Object.hasOwn(payload.jav_title_translation_profiles.deepseek, 'api_key'), false)
  assert.equal(payload.jav_title_translation_prompt, 'Custom prompt')
  assert.equal(Object.hasOwn(payload, 'jav_title_translation_enabled'), false)
  draft.apiKey = 'replacement-key'
  draft.thinking = true
  await actions.saveTitleTranslationSettings(draft)
  assert.equal(payload.jav_title_translation_profiles.deepseek.thinking, true)
  assert.equal(model.createTitleTranslationDraft(useStore.getState().config).thinking, true)
  assert.equal(payload.jav_title_translation_profiles.deepseek.api_key, 'replacement-key')
  assert.equal(useStore.getState().config.jav_title_translation_api_key, undefined)
  Object.assign(draft, {
    apiKey: '',
    clearApiKey: true,
    hasApiKey: false,
  })
  await actions.saveTitleTranslationSettings(draft)
  assert.equal(payload.jav_title_translation_profiles.deepseek.api_key, '')
})

test('display settings omit translation configuration', async (t) => {
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
  assert.equal(Object.hasOwn(draft, 'javTitlePreferChineseInput'), false)
  await actions.saveJavSettings(draft, () => {})
  assert.equal(Object.hasOwn(payload, 'jav_title_prefer_chinese'), false)
  assert.equal(Object.hasOwn(payload, 'jav_title_translation_enabled'), false)
  assert.equal(Object.hasOwn(payload, 'jav_title_translation_prompt'), false)
  assert.equal(Object.hasOwn(payload, 'jav_title_translation_profiles'), false)
})

test('provider switching preserves independent unsaved profiles and request templates', async (t) => {
  const [settings] = await loadModules(t, ['features/settings/titleTranslation.js'])
  let draft = settings.createTitleTranslationDraft({
    jav_title_translation_profiles: JSON.stringify({
      deepseek: { api_key_set: true, model: 'deepseek-flash' },
      kimi: { api_key_set: true, model: 'kimi-k2.5' },
    }),
  })
  draft.apiKey = 'new-deepseek-key'
  draft = settings.changeTranslationDraft(draft, { provider: 'kimi' })
  assert.equal(draft.apiKey, '')
  assert.equal(draft.model, 'kimi-k2.5')
  draft.model = 'kimi-k2.6'
  draft = settings.changeTranslationDraft(draft, { provider: 'custom' })
  draft.baseUrl = 'http://localhost:1234/v1'
  draft.model = 'local'
  draft.request = JSON.stringify({
    url: '{{base_url}}/translate',
    body: { text: '{{title}}' },
    response_path: '/translation',
  })
  const profiles = settings.translationProfiles(draft)
  assert.equal(profiles.deepseek.api_key, 'new-deepseek-key')
  assert.equal(profiles.kimi.api_key, undefined)
  assert.equal(profiles.kimi.model, 'kimi-k2.6')
  assert.equal(profiles.custom.request.body.text, '{{title}}')
  draft = settings.changeTranslationDraft(draft, { provider: 'deepseek' })
  assert.equal(draft.apiKey, 'new-deepseek-key')
  assert.equal(draft.model, 'deepseek-flash')
})

test('provider picker uses only the backend catalog and its endpoint defaults', async (t) => {
  const [settings] = await loadModules(t, ['features/settings/titleTranslation.js'])
  for (const catalog of [undefined, '', '[]', '{}']) {
    const draft = settings.createTitleTranslationDraft({ jav_title_translation_catalog: catalog })
    assert.deepEqual(draft.catalog, [])
  }
  const catalog = [
    { id: 'deepseek', name: 'DeepSeek', base_url: 'https://deepseek.test', protocol: 'chat' },
    { id: 'openai', name: 'OpenAI', base_url: 'https://openai.test/v1', protocol: 'responses' },
    { id: 'custom', name: 'Custom', base_url: '', protocol: 'chat' },
  ]
  const draft = settings.createTitleTranslationDraft({
    jav_title_translation_catalog: JSON.stringify(catalog),
  })
  assert.deepEqual(draft.catalog, catalog)
  assert.equal(draft.baseUrl, catalog[0].base_url)
  assert.equal(draft.customAddress, false)
  const openai = settings.changeTranslationDraft(draft, { provider: 'openai' })
  assert.equal(openai.baseUrl, catalog[1].base_url)
  assert.equal(openai.protocol, 'responses')
  const custom = settings.changeTranslationDraft(openai, { provider: 'custom' })
  assert.equal(custom.baseUrl, '')
})

test('Chinese display follows independent master switches regardless of automatic translation', async (t) => {
  const [{ preferChineseTitle, preferChineseSeries }] = await loadModules(t, [
    'features/jav/titleTranslation.js',
  ])
  assert.equal(preferChineseTitle({}), false)
  assert.equal(preferChineseSeries({}), false)
  for (const title of [true, false]) {
    for (const series of [true, false]) {
      for (const automatic of [true, false]) {
        const config = {
          jav_title_translation_enabled: String(title),
          jav_series_translation_enabled: String(series),
          jav_title_translation_auto_enabled: String(automatic),
          jav_series_translation_auto_enabled: String(automatic),
        }
        assert.equal(preferChineseTitle(config), title)
        assert.equal(preferChineseSeries(config), series)
      }
    }
  }
})
