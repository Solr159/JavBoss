import { createRoot } from 'react-dom/client'
import TitleTranslationToolSettings from '@/features/settings/components/TitleTranslationToolSettings'
import { useStore } from '@/store'
import '@/index.css'

window.testStore = useStore
window.translationRequests = []
window.testFailure = false
let config = {
  jav_title_prefer_chinese: 'true',
  jav_title_translation_enabled: 'false',
  jav_title_translation_provider: 'deepseek',
  jav_title_translation_model: 'saved-model',
  jav_title_translation_api_key_set: 'true',
  jav_title_translation_prompt: 'Saved prompt',
  jav_title_translation_default_prompt: 'Default prompt',
}
let batch = { running: false, total: 0 }
useStore.setState({ config, loadJavs: async () => {} })
window.fetch = async (input, init = {}) => {
  const url = new URL(input, location.origin)
  const payload = init.body ? JSON.parse(init.body) : null
  window.translationRequests.push({ path: url.pathname, method: init.method || 'GET', payload })
  if (url.pathname === '/config') {
    config = {
      ...config,
      ...payload,
      jav_title_translation_profiles: JSON.stringify(
        Object.fromEntries(
          Object.entries(payload.jav_title_translation_profiles).map(([id, p]) => [
            id,
            { ...p, api_key: undefined, api_key_set: true },
          ])
        )
      ),
    }
    return Response.json(config)
  }
  if (url.pathname.endsWith('/models'))
    return Response.json({
      models: [
        { id: 'saved-model', name: 'Saved model' },
        { id: 'another-model', name: 'Another model' },
      ],
      capabilities: {
        reasoning: 'toggle',
        temperature: true,
        temperature_max: 2,
        efforts: ['low', 'medium', 'high'],
      },
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
createRoot(document.getElementById('root')).render(
  <main className="mx-auto max-w-2xl p-4">
    <TitleTranslationToolSettings />
  </main>
)
