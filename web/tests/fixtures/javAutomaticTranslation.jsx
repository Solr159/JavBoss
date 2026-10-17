import { StrictMode, useState } from 'react'
import { createRoot } from 'react-dom/client'
import useJavTitleTranslation from '@/features/jav/hooks/useJavTitleTranslation'
import { JavEditModal } from '@/features/jav/components/JavEditModal'
import { useStore } from '@/store'

window.testStore = useStore
window.translationRequests = []
window.writeTranslationEvent = (index, event) => {
  const bytes = new TextEncoder().encode(JSON.stringify(event) + '\n')
  const split = Math.max(1, bytes.findIndex((byte) => byte > 127) + 1)
  const { controller } = window.translationRequests[index]
  // Deliberately split UTF-8 characters and JSON lines across network chunks.
  controller.enqueue(bytes.slice(0, split))
  controller.enqueue(bytes.slice(split))
}
window.emitTranslation = (index, ids) => {
  const request = window.translationRequests[index]
  for (const id of ids) {
    if (request.completed.has(id)) continue
    request.completed.add(id)
    window.writeTranslationEvent(index, {
      item: { id, title: `Source ${id}`, zh_title: `译文 ${id}` },
    })
  }
}
window.completeTranslation = (index, fail = false) => {
  const request = window.translationRequests[index]
  window.emitTranslation(index, fail ? request.ids.slice(0, 1) : request.ids)
  if (fail)
    window.writeTranslationEvent(index, {
      error_en: 'HTTP 429 / credit_balance_exhausted: Add credits.',
    })
  window.writeTranslationEvent(index, { done: true })
  request.controller.close()
}
window.fetch = (input, init) => {
  const path = new URL(input, location.origin).pathname
  if (['/jav/studios', '/jav/series', '/jav/idols/options'].includes(path))
    return Promise.resolve(Response.json({ items: [], total: 0 }))
  if (path === '/jav/tags') return Promise.resolve(Response.json([]))
  if (init?.method === 'PUT' && path.startsWith('/jav/items/')) {
    const id = Number(path.split('/').at(-1))
    window.savedEditorPayload = JSON.parse(init.body)
    return Promise.resolve(
      Response.json({
        ...useStore.getState().javItems.find((item) => item.id === id),
        ...window.savedEditorPayload,
      })
    )
  }
  if (input !== '/jav/title-translation/items') throw new Error(`Unexpected request: ${input}`)
  const request = { ...JSON.parse(init.body), completed: new Set() }
  const stream = new ReadableStream({
    start(controller) {
      request.controller = controller
    },
  })
  window.translationRequests.push(request)
  return Promise.resolve(
    new Response(stream, { headers: { 'Content-Type': 'application/x-ndjson' } })
  )
}
useStore.setState({
  config: {
    jav_title_translation_enabled: 'true',
    jav_title_translation_provider: 'deepseek',
    jav_title_translation_profiles: JSON.stringify(
      new URLSearchParams(location.search).has('unconfigured')
        ? {}
        : { deepseek: { api_key_set: true, model: 'model' } }
    ),
  },
  javItems: [
    ...Array.from({ length: 25 }, (_, i) => ({ id: i + 1, title: `Source ${i + 1}` })),
    { id: 26, title: 'Corrected source', zh_title: '人工校对' },
    { id: 27, title: ' ' },
  ],
})

function Translation() {
  const items = useStore((state) => state.javItems)
  const { error, retry } = useJavTitleTranslation(items)
  return (
    <>
      {error && <p role="alert">{error}</p>}
      <button onClick={retry}>Retry</button>
      <output>{items.filter((item) => item.zh_title).length}</output>
    </>
  )
}
function Fixture() {
  const [revision, setRevision] = useState(0)
  const [editingId, setEditingId] = useState(null)
  const item = useStore((state) => state.javItems.find((item) => item.id === editingId))
  window.editTranslationItem = setEditingId
  return (
    <>
      <button onClick={() => setRevision((value) => value + 1)}>Remount</button>
      <Translation key={revision} />
      <JavEditModal
        open={editingId !== null}
        item={item}
        onClose={() => setEditingId(null)}
        onSaved={(updated) => {
          useStore.getState().patchJavItem(updated)
          setEditingId(null)
        }}
      />
    </>
  )
}
createRoot(document.getElementById('root')).render(
  <StrictMode>
    <Fixture />
  </StrictMode>
)
