import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import JavSeriesView from '@/features/jav/components/JavSeriesView'
import { useStore } from '@/store'
import '@/index.css'

window.testStore = useStore
window.translationRequests = []
window.savedTranslations = []
const automatic = new URLSearchParams(location.search).has('automatic')
useStore.setState({
  config: {
    jav_title_translation_enabled: 'false',
    jav_series_translation_enabled: String(automatic),
    jav_title_prefer_chinese: 'true',
    jav_title_translation_provider: 'custom',
    jav_title_translation_profiles: JSON.stringify({ custom: { model: 'local' } }),
  },
  seriesItems: [
    { id: 1, name: '雨の日', zh_name: '', work_count: 2 },
    { id: 2, name: 'Original two', zh_name: '人工译名', work_count: 1 },
  ],
})
window.fetch = async (input, init) => {
  const path = new URL(input, location.origin).pathname
  if (path === '/jav/series-translation/items') {
    const { ids } = JSON.parse(init.body)
    window.translationRequests.push(ids)
    return new Response(
      ids
        .map((id) => JSON.stringify({ item: { id, name: '雨の日', zh_name: '自动系列译名' } }))
        .join('\n') + '\n{"done":true}\n',
      { headers: { 'Content-Type': 'application/x-ndjson' } }
    )
  }
  if (path === '/jav/series/1/name-translation') {
    window.previewPayload = JSON.parse(init.body)
    if (window.translationFailure)
      return Response.json({ error_en: 'Translation failed' }, { status: 502 })
    return Response.json({ id: 1, name: '雨の日', zh_name: 'AI 系列译名' })
  }
  if (path === '/jav/series/1' && init.method === 'PATCH') {
    const payload = JSON.parse(init.body)
    window.savedTranslations.push(payload)
    return Response.json({ id: 1, ...payload })
  }
  throw new Error(`Unexpected request: ${path}`)
}

function Fixture() {
  const items = useStore((state) => state.seriesItems)
  return <JavSeriesView items={items} page={1} lastPage={1} totalItems={2} />
}
createRoot(document.getElementById('root')).render(
  <StrictMode>
    <Fixture />
  </StrictMode>
)
