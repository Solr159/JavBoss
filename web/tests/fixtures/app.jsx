import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from '@/App'
import AppThemeProvider from '@/app/theme/AppThemeProvider'
import { AuthProvider } from '@/features/auth/components/AuthProvider'
import { useStore } from '@/store'
import '@/index.css'

window.testStore = useStore
window.appErrors = []
window.addEventListener('error', (event) => window.appErrors.push(event.message))
window.requests = []
window.watchTimeSources = []
window.EventSource = class extends EventTarget {
  constructor(url) {
    super()
    this.url = url
    this.closed = false
    window.watchTimeSources.push(this)
    queueMicrotask(() => {
      if (!this.closed) this.dispatchEvent(new Event('open'))
    })
  }
  close() {
    this.closed = true
  }
}
const config = { video_page_size: '25', jav_page_size: '24', initial_view_mode: 'video' }
const jav = { id: 1, code: 'ABC-001', title: 'Test JAV', videos: [], tags: [], idols: [] }
const providers = [
  {
    id: 1,
    name: 'javbus',
    domain: 'www.javbus.com',
    sample: 'SSIS-001',
    base_url: '',
    default_base_url: 'https://www.javbus.com',
    configurable: true,
    last_result: { status: 'ok', elapsed_ms: 21 },
  },
  {
    id: 12,
    name: 'javdb-api',
    domain: 'jdforrepam.com',
    sample: 'SSIS-001',
    base_url: '',
    default_base_url: '',
    configurable: false,
  },
]
const updateProviderBaseURL = (provider, body) => {
  const raw = String(JSON.parse(body || '{}').base_url || '').trim()
  let normalized = ''
  if (raw) {
    let parsed = null
    try {
      parsed = new URL(raw)
    } catch {
      parsed = null
    }
    const invalidOrigin =
      !parsed ||
      !/^https:\/\/[^/:?#@]+\/?$/i.test(raw) ||
      !/^[a-z0-9-]+(?:\.[a-z0-9-]+)*\.?$/i.test(parsed.hostname) ||
      /^\d+\.\d+\.\d+\.\d+$/.test(parsed.hostname)
    if (invalidOrigin)
      return Response.json(
        { error_zh: '数据源地址无效', error_en: 'Invalid provider URL' },
        { status: 400 }
      )
    const origin = `https://${parsed.hostname.replace(/\.$/, '')}`
    normalized = origin === provider.default_base_url ? '' : origin
  }
  if (provider.base_url !== normalized) delete provider.last_result
  provider.base_url = normalized
  provider.domain = new URL(normalized || provider.default_base_url).hostname
  return Response.json({
    provider: provider.id,
    base_url: provider.base_url,
    default_base_url: provider.default_base_url,
    domain: provider.domain,
  })
}
window.fetch = async (input, init = {}) => {
  const url = new URL(input, location.origin)
  window.requests.push({ url: url.pathname + url.search, method: init.method || 'GET' })
  if (url.pathname === '/auth/status')
    return Response.json({ authenticated: !new URLSearchParams(location.search).has('login') })
  if (url.pathname === '/config') {
    if (init.body) Object.assign(config, JSON.parse(init.body))
    return Response.json(config)
  }
  if (url.pathname === '/directories')
    return Response.json([{ id: 1, path: '/videos', enabled: true }])
  if (url.pathname === '/videos') return Response.json({ items: [], total: 0 })
  if (url.pathname === '/videos/watched-time') return Response.json({ videos: [], javs: [] })
  if (url.pathname === '/downloads')
    return Response.json({ items: [], total: 0, counts: { active: 0, completed: 0, failed: 0 } })
  if (url.pathname === '/jav/providers') return Response.json(providers)
  if (url.pathname.startsWith('/jav/providers/') && url.pathname.endsWith('/base_url')) {
    const provider = providers.find((item) => url.pathname === `/jav/providers/${item.id}/base_url`)
    if (!provider) return Response.json({ error_en: 'Invalid provider' }, { status: 400 })
    return updateProviderBaseURL(provider, init.body)
  }
  if (url.pathname === '/jav') return Response.json({ items: [jav], total: 1 })
  if (url.pathname === '/jav/items/1') {
    if (init.method === 'PUT') Object.assign(jav, JSON.parse(init.body))
    return Response.json(jav)
  }
  if (['/jav/idols', '/jav/studios', '/jav/series'].includes(url.pathname))
    return Response.json({ items: [], total: 0 })
  if (url.pathname.endsWith('/sample-images')) return Response.json({ sample_images: [] })
  if (url.pathname.endsWith('/favorite-groups')) return Response.json({ selected_group_ids: [] })
  return Response.json([])
}

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <AppThemeProvider>
      <BrowserRouter>
        <AuthProvider>
          <App />
        </AuthProvider>
      </BrowserRouter>
    </AppThemeProvider>
  </StrictMode>
)
