import { useState } from 'react'
import { createRoot } from 'react-dom/client'
import AppThemeProvider from '@/app/theme/AppThemeProvider'
import { JavEditModal } from '@/features/jav/components/JavEditModal'
import { JavIdolEditModal } from '@/features/jav/components/JavIdolGrid'
import JavQueryEditorModal from '@/features/jav/components/JavQueryEditorModal'
import VideoScrapeSettingsModal from '@/features/video/components/VideoScrapeSettingsModal'
import '@/index.css'

const item = { id: 1, code: 'ABC-001', title: 'Test title', tags: [], idols: [] }
const idol = { id: 1, name: 'Test actress', aliases: ['Alias'] }
const video = { id: 1, filename: 'ABC-001.mp4', jav_scrape_override: 'ABC-001' }
const emptyOptions = []
window.fetch = async (input) => {
  const path = new URL(input, location.origin).pathname
  return Response.json(path === '/jav/tags' ? [] : { items: [], total: 0 })
}

function Fixture() {
  const [saving, setSaving] = useState(false)
  window.setFixtureSaving = setSaving
  const props = { open: true, onClose: () => {} }
  switch (new URLSearchParams(location.search).get('dialog')) {
    case 'idol':
      return <JavIdolEditModal {...props} item={idol} />
    case 'query':
      return (
        <JavQueryEditorModal
          {...props}
          idolIds={emptyOptions}
          idolOptions={emptyOptions}
          tagIds={emptyOptions}
          tagOptions={emptyOptions}
        />
      )
    case 'scrape':
      return <VideoScrapeSettingsModal {...props} video={video} saving={saving} />
    default:
      return <JavEditModal {...props} item={item} />
  }
}

createRoot(document.getElementById('root')).render(
  <AppThemeProvider>
    <Fixture />
  </AppThemeProvider>
)
