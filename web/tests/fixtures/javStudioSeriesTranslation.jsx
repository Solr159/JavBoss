import { StrictMode, useState } from 'react'
import { createRoot } from 'react-dom/client'
import JavStudioDetailModal from '@/features/jav/components/JavStudioDetailModal'
import { useStore } from '@/store'
import '@/index.css'

window.testStore = useStore
window.translationRequests = []
const params = new URLSearchParams(location.search)
const studio = {
  id: 7,
  name: 'Studio translation test',
  series: Array.from({ length: 24 }, (_, index) => ({
    id: index + 1,
    name: `Original series ${index + 1}`,
    zh_name: index === 23 ? '人工译名' : '',
    work_count: 24 - index,
  })),
}
useStore.setState({
  config: {
    jav_series_translation_enabled: 'true',
    jav_series_translation_auto_enabled: String(!params.has('manual')),
    jav_title_translation_provider: 'custom',
    jav_title_translation_profiles: JSON.stringify({ custom: { model: 'local' } }),
  },
  seriesItems: [],
  studioItems: [],
})
window.fetch = async (input, init) => {
  const path = new URL(input, location.origin).pathname
  if (path === '/jav/studios/7') return Response.json(studio)
  if (path === '/jav/series-translation/items') {
    const { ids } = JSON.parse(init.body)
    window.translationRequests.push(ids)
    if (window.translationFailure)
      return Response.json({ error_en: 'Translation failed' }, { status: 502 })
    const events = ids.map((id) => {
      const series = studio.series.find((entry) => entry.id === id)
      series.zh_name = `自动译名 ${id}`
      return JSON.stringify({ item: { ...series } })
    })
    return new Response(events.join('\n') + '\n{"done":true}\n', {
      headers: { 'Content-Type': 'application/x-ndjson' },
    })
  }
  throw new Error(`Unexpected request: ${path}`)
}
const cacheStudio = (item) => {
  window.cachedStudio = item
}
function Fixture() {
  const [open, setOpen] = useState(false)
  return (
    <>
      <button onClick={() => setOpen(true)}>Open studio</button>
      {open && (
        <JavStudioDetailModal
          studioId={7}
          initialItem={window.cachedStudio}
          initialState={{ prefixSort: 'name_asc', seriesSort: 'work_count_desc', scrollTop: 0 }}
          onLoaded={cacheStudio}
          onStateChange={() => {}}
          onClose={() => setOpen(false)}
          onSelectStudio={() => {}}
          buildJavUrl={({ seriesId, seriesName }) =>
            seriesId ? `/?series=${seriesId}&name=${encodeURIComponent(seriesName)}` : '/'
          }
        />
      )}
    </>
  )
}
createRoot(document.getElementById('root')).render(
  <StrictMode>
    <Fixture />
  </StrictMode>
)
