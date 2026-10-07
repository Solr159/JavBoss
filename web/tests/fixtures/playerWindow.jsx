import { StrictMode, useState } from 'react'
import { createRoot } from 'react-dom/client'
import PlayerModal from '@/features/playback/components/PlayerModal'
import '@/index.css'

const videos = [
  { id: 1, filename: 'First video' },
  { id: 2, filename: 'Second video' },
]
window.streamRequests = []
window.appErrors = []
window.addEventListener('error', (event) => window.appErrors.push(event.message))
window.fetch = async (url) => {
  window.streamRequests.push(url)
  return Response.json({ sources: [] })
}

function Fixture() {
  const params = new URLSearchParams(window.location.search)
  const [open, setOpen] = useState(false)
  const [index, setIndex] = useState(0)
  return (
    <>
      <button onClick={() => setOpen(true)}>Open player</button>
      <PlayerModal
        video={open ? videos[index] : null}
        playlist={videos}
        currentIndex={index}
        onSelectVideo={setIndex}
        onClose={() => setOpen(false)}
        showHotkeyHint={false}
        resumePlayback={params.get('resume') !== 'false'}
        startTime={params.has('start') ? Number(params.get('start')) : null}
      />
    </>
  )
}

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <Fixture />
  </StrictMode>
)
