import { useState } from 'react'
import JavSettings from '@/features/settings/components/JavSettings'
import { createRoot } from 'react-dom/client'
import TitleTranslationToolSettings from '@/features/settings/components/TitleTranslationToolSettings'
import { useStore } from '@/store'
import '@/index.css'

window.testStore = useStore
window.translationRequests = []
window.testFailure = false
const capabilities = {
  reasoning: 'toggle',
  temperature: true,
  temperature_max: 2,
  efforts: ['low', 'medium', 'high'],
}
let config = {
  jav_title_prefer_chinese: 'true',
  jav_title_translation_enabled: 'false',
  jav_title_translation_provider: 'deepseek',
  jav_title_translation_catalog: JSON.stringify([
    { id: 'deepseek', name: 'DeepSeek', base_url: 'https://api.deepseek.com', protocol: 'chat' },
    { id: 'custom', name: 'Custom', base_url: '', protocol: 'chat' },
  ]),
  jav_title_translation_profiles: JSON.stringify({
    deepseek: { model: 'saved-model', api_key_set: true },
  }),
  jav_title_translation_capabilities: JSON.stringify(capabilities),
  jav_title_translation_model: 'saved-model',
  jav_title_translation_api_key_set: 'true',
  jav_title_translation_prompt: 'Saved prompt',
  jav_title_translation_default_prompt: 'Default prompt',
}
let batch = { running: false, total: 0 }
window.loadTranslationConfig = () => useStore.setState({ config })
useStore.setState({
  config: new URLSearchParams(location.search).has('deferConfig') ? {} : config,
  loadJavs: async () => {},
})
window.fetch = async (input, init = {}) => {
  const url = new URL(input, location.origin)
  const payload = init.body ? JSON.parse(init.body) : null
  window.translationRequests.push({ path: url.pathname, method: init.method || 'GET', payload })
  if (url.pathname === '/config') {
    if (window.configFailure) return Response.json({ error_en: 'Save failed' }, { status: 500 })
    config = { ...config, ...payload }
    if (payload.jav_title_translation_profiles) {
      config.jav_title_translation_profiles = JSON.stringify(
        Object.fromEntries(
          Object.entries(payload.jav_title_translation_profiles).map(([id, p]) => [
            id,
            { ...p, api_key: undefined, api_key_set: true },
          ])
        )
      )
    }
    return Response.json(config)
  }
  if (url.pathname.endsWith('/models'))
    return Response.json({
      models: [
        { id: 'saved-model', name: 'Saved model' },
        { id: 'another-model', name: 'Another model' },
      ],
      capabilities,
    })
  if (url.pathname.endsWith('/test')) {
    if (window.testFailure)
      return Response.json(
        { error_en: 'Credit balance exhausted (HTTP 429 / credit_balance_exhausted)' },
        { status: 429 }
      )
    return Response.json({ zh_title: '雨天在车站重逢的两人' })
  }
  if (url.pathname.endsWith('/batch/preview'))
    return Response.json(
      url.searchParams.get('scope') === 'all'
        ? { scope: 'all', total: 10, existing: 7 }
        : { scope: 'missing', total: 3, existing: 0 }
    )
  if (url.pathname.endsWith('/batch')) {
    if (init.method === 'POST')
      batch = {
        running: false,
        total: payload.scope === 'all' ? 10 : 3,
        processed: 3,
        translated: 3,
        skipped: 0,
      }
    return Response.json(batch)
  }
  throw new Error(`Unexpected request: ${url.pathname}`)
}
function Fixture() {
  const [displayOpen, setDisplayOpen] = useState(false)
  return (
    <main className="mx-auto max-w-2xl p-4">
      <button onClick={() => setDisplayOpen(true)}>Display settings</button>
      <TitleTranslationToolSettings />
      {displayOpen && (
        <JavSettings
          onClose={() => setDisplayOpen(false)}
          onError={(message) => {
            window.displaySettingsError = message
          }}
          onWaterfallChange={() => {}}
        />
      )}
    </main>
  )
}
createRoot(document.getElementById('root')).render(<Fixture />)
