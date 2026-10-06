import { useEffect, useRef } from 'react'
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded'
import { getVideoDisplayName } from '@/utils/display'
import { zh } from '@/utils/i18n'

export default function PlaybackPlaylist({ items, currentIndex, onSelect }) {
  const activeRef = useRef(null)
  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: 'nearest' })
  }, [currentIndex, items])

  return (
    <aside
      id="browser-playlist"
      aria-label={zh('播放列表', 'Playlist')}
      className="flex h-full min-h-0 w-[38%] max-w-64 shrink-0 flex-col overflow-hidden rounded bg-zinc-100"
    >
      <h3 className="border-b border-zinc-200 px-3 py-2 text-sm font-semibold">
        {zh('播放列表', 'Playlist')} ({items.length})
      </h3>
      <ol className="min-h-0 flex-1 overflow-y-auto p-1">
        {items.map((item, index) => {
          const active = index === currentIndex
          const title = getVideoDisplayName(item)
          return (
            <li key={`${item.id}-${item.location_id || 0}-${index}`}>
              <button
                ref={active ? activeRef : null}
                type="button"
                aria-current={active ? 'true' : undefined}
                title={title}
                onClick={() => onSelect?.(index)}
                className={`flex w-full items-center gap-2 rounded px-2 py-2 text-left text-xs ${active ? 'bg-blue-100 font-semibold text-blue-700' : 'text-zinc-700 hover:bg-zinc-200'}`}
              >
                <span className="w-5 shrink-0 text-center">
                  {active ? <PlayArrowRoundedIcon sx={{ fontSize: 18 }} /> : index + 1}
                </span>
                <span className="min-w-0 truncate">{title}</span>
              </button>
            </li>
          )
        })}
      </ol>
    </aside>
  )
}
