import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from '@/App'
import { AuthProvider } from '@/features/auth/components/AuthProvider'
import { useStore } from '@/store'
import '@/index.css'

window.testStore = useStore
window.appErrors = []
window.addEventListener('error', (event) => window.appErrors.push(event.message))
window.requests = []
const config = { video_page_size: '25', jav_page_size: '24', initial_view_mode: 'video' }
const jav = { id: 1, code: 'ABC-001', title: 'Test JAV', videos: [], tags: [], idols: [] }
window.fetch = async (input, init = {}) => {
  const url = new URL(input, location.origin)
  window.requests.push({ url: url.pathname + url.search, method: init.method || 'GET' })
  if (url.pathname === '/auth/status') return Response.json({ authenticated: true })
  if (url.pathname === '/config') {
    if (init.body) Object.assign(config, JSON.parse(init.body))
    return Response.json(config)
  }
  if (url.pathname === '/directories')
    return Response.json([{ id: 1, path: '/videos', enabled: true }])
  if (url.pathname === '/videos') return Response.json({ items: [], total: 0 })
  if (url.pathname === '/downloads')
    return Response.json({ items: [], total: 0, counts: { active: 0, completed: 0, failed: 0 } })
  if (url.pathname === '/jav') return Response.json({ items: [jav], total: 1 })
  if (url.pathname === '/jav/items/1') return Response.json(jav)
  if (['/jav/idols', '/jav/studios', '/jav/series'].includes(url.pathname))
    return Response.json({ items: [], total: 0 })
  if (url.pathname.endsWith('/sample-images')) return Response.json({ sample_images: [] })
  if (url.pathname.endsWith('/favorite-groups')) return Response.json({ selected_group_ids: [] })
  return Response.json([])
}

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <App />
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>
)
