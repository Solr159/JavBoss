import { useEffect, useRef } from 'react'
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded'
import { getVideoDisplayName } from '@/utils/display'
import { zh } from '@/utils/i18n'
import usePlaylistResize from '@/features/playback/hooks/usePlaylistResize'

export default function PlaybackPlaylist({ items, currentIndex, onSelect }) {
  const activeRef = useRef(null)
  const resize = usePlaylistResize()
  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: 'nearest' })
  }, [currentIndex, items])

  return (
    <>
      <div
        role="separator"
        aria-orientation="vertical"
        aria-label={zh('调整播放列表宽度', 'Resize playlist')}
        aria-controls="browser-playlist"
        // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- Resizable separators support keyboard adjustment.
        tabIndex={0}
        data-playlist-resize
        title={zh('左右拖动调整播放列表宽度', 'Drag left or right to resize the playlist')}
        className={`group flex w-2 shrink-0 cursor-col-resize touch-none select-none items-center justify-center outline-none hover:bg-blue-50 focus-visible:bg-blue-50 ${resize.dragging ? 'bg-blue-50' : ''}`}
        {...resize.separatorProps}
      >
        <span
          className={`h-8 w-0.5 rounded-full group-hover:bg-blue-400 group-focus-visible:bg-blue-400 ${resize.dragging ? 'bg-blue-400' : 'bg-zinc-300'}`}
        />
      </div>
      <aside
        ref={resize.panelRef}
        style={{ width: resize.width }}
        id="browser-playlist"
        aria-label={zh('播放列表', 'Playlist')}
        className="flex h-full min-h-0 min-w-0 shrink-0 flex-col overflow-hidden rounded bg-zinc-100"
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
                  <span
                    className="min-w-5 shrink-0 whitespace-nowrap text-center tabular-nums"
                    style={{ width: `${String(items.length).length}ch` }}
                  >
                    {active ? <PlayArrowRoundedIcon sx={{ fontSize: 18 }} /> : index + 1}
                  </span>
                  <span className="min-w-0 truncate">{title}</span>
                </button>
              </li>
            )
          })}
        </ol>
      </aside>
    </>
  )
}
