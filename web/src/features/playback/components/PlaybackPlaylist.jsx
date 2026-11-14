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
        data-playlist-resize
        title={zh('左右拖动调整播放列表宽度', 'Drag left or right to resize the playlist')}
        className={`group flex w-2 shrink-0 cursor-col-resize touch-none select-none items-center justify-center hover:bg-white/5 ${resize.dragging ? 'bg-white/5' : ''}`}
        {...resize.separatorProps}
      >
        <span
          className={`h-8 w-0.5 rounded-full group-hover:bg-sky-400 ${resize.dragging ? 'bg-sky-400' : 'bg-slate-700'}`}
        />
      </div>
      <aside
        ref={resize.panelRef}
        style={{ width: resize.width }}
        id="browser-playlist"
        aria-label={zh('播放列表', 'Playlist')}
        className="player-playlist flex h-full min-h-0 min-w-0 shrink-0 flex-col overflow-hidden"
      >
        <h3 className="flex items-center justify-between gap-2 border-b border-white/5 px-3 py-4 text-xs font-medium text-slate-300">
          <span>{zh('播放列表', 'Playlist')}</span>
          <span className="rounded-md bg-white/5 px-2 py-0.5 tabular-nums text-slate-400">
            {items.length}
          </span>
        </h3>
        <ol className="min-h-0 flex-1 space-y-0.5 overflow-y-auto p-1">
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
                  className={`player-playlist-item flex w-full items-center gap-1.5 rounded-md px-1.5 py-1 text-left text-[11px] leading-4 ${active ? 'is-active font-medium' : ''}`}
                >
                  <span
                    className="player-playlist-number flex min-w-4 shrink-0 items-center justify-center whitespace-nowrap tabular-nums"
                    style={{ width: `${String(items.length).length + 1}ch` }}
                  >
                    {active ? (
                      <PlayArrowRoundedIcon sx={{ fontSize: 14 }} />
                    ) : (
                      String(index + 1).padStart(2, '0')
                    )}
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
