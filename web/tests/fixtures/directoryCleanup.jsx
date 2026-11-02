import { createRoot } from 'react-dom/client'
import DirectoryManager from '@/features/directories/components/DirectoryManager'
import '@/index.css'

window.cleanupRequests = []
window.confirmCleanup = false
window.confirm = () => window.confirmCleanup
window.fetch = async (input, init = {}) => {
  const path = new URL(input, location.origin).pathname
  const payload = init.body ? JSON.parse(init.body) : null
  window.cleanupRequests.push({ path, payload })
  if (window.failCleanup) return Response.json({ error_en: 'Directory is busy' }, { status: 409 })
  if (path.endsWith('/cleanup/preview')) {
    const response = Response.json({
      root_path: '/videos',
      files: [
        { path: 'ad.txt', size: 12, modified: 'snapshot-1', reason: 'misc' },
        ...(payload.include_system
          ? [{ path: 'nested/Thumbs.db', size: 0, modified: 'snapshot-2', reason: 'system' }]
          : []),
        ...(payload.include_torrents
          ? [{ path: 'ad.torrent', size: 24, modified: 'snapshot-3', reason: 'torrent' }]
          : []),
        ...(payload.include_downloads
          ? [{ path: 'unfinished.xltd', size: 500, modified: 'snapshot-4', reason: 'download' }]
          : []),
        ...(window.extraCleanupFiles || []),
      ],
      issues: [{ path: 'restricted', reason: 'unreadable' }],
    })
    if (window.holdPreview)
      return new Promise((resolve) => {
        window.finishPreview = () => resolve(response)
      })
    return response
  }
  if (path.endsWith('/cleanup'))
    return Response.json({
      deleted: payload.files.filter((file) => file.path === 'ad.txt').map((file) => file.path),
      deleted_bytes: payload.files.some((file) => file.path === 'ad.txt') ? 12 : 0,
      issues: payload.files
        .filter((file) => file.path !== 'ad.txt')
        .map((file) => ({ path: file.path, reason: 'changed' })),
    })
  throw new Error(`Unexpected request: ${path}`)
}

createRoot(document.getElementById('root')).render(
  <DirectoryManager open directories={[{ id: 1, path: '/videos', enabled: true }]} />
)
