import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { Popover } from '@mui/material'
import ChevronRightIcon from '@mui/icons-material/ChevronRight'
import FolderOutlinedIcon from '@mui/icons-material/FolderOutlined'
import { useShallow } from 'zustand/react/shallow'
import useVideoDirectories from '@/features/video/hooks/useVideoDirectories'
import VideoDirectoryTree, {
  buildDirectoryTree,
} from '@/features/video/components/VideoDirectoryTree'
import { useStore } from '@/store'
import { zh } from '@/utils/i18n'

export default function VideoDirectoryBreadcrumbs() {
  const { directoryId, subdirectory, directories, setDirectory } = useStore(
    useShallow((state) => ({
      directoryId: state.videoDirectoryId,
      subdirectory: state.videoSubdirectory,
      directories: state.directories,
      setDirectory: state.setVideoDirectoryFilter,
    }))
  )
  const [menu, setMenu] = useState(null)
  const [menuOpen, setMenuOpen] = useState(false)
  const currentRef = useRef(null)
  const popoverRef = useRef(null)
  const treeContainerRef = useRef(null)
  const { options, loading, error, retry } = useVideoDirectories(menuOpen)
  const rootPath =
    directories.find((directory) => directory.id === directoryId)?.path || `#${directoryId}`
  const crumbs = [{ label: zh('全部目录', 'All directories'), directory: null }]
  if (directoryId) {
    crumbs.push({
      label: rootPath,
      directory: { directory_id: directoryId, directory_path: rootPath, path: '.' },
    })
    const segments = subdirectory && subdirectory !== '.' ? subdirectory.split('/') : []
    segments.forEach((label, index) => {
      crumbs.push({
        label,
        directory: {
          directory_id: directoryId,
          directory_path: rootPath,
          path: segments.slice(0, index + 1).join('/'),
        },
      })
    })
  }

  useEffect(() => setMenuOpen(false), [directoryId, subdirectory])
  useLayoutEffect(() => {
    // Long paths stay horizontally scrollable, with the current directory in view.
    currentRef.current?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
  }, [directoryId, subdirectory, rootPath])

  const revealSelection = useCallback(() => {
    treeContainerRef.current
      ?.querySelector('button[aria-pressed="true"]')
      ?.scrollIntoView({ block: 'center', inline: 'nearest' })
  }, [])
  useLayoutEffect(() => {
    if (menuOpen && !loading) revealSelection()
  }, [menuOpen, loading, options, revealSelection])

  const directory = menu?.crumb.directory
  const tree = useMemo(() => buildDirectoryTree(options), [options])
  let parentNode = tree.find((node) => node.directory_id === directory?.directory_id)
  const segments = directory && directory.path !== '.' ? directory.path.split('/').slice(0, -1) : []
  segments.forEach((_, index) => {
    parentNode = parentNode?.children.get(segments.slice(0, index + 1).join('/'))
  })
  const nodes = directory && directory.path !== '.' ? (parentNode ? [parentNode] : []) : tree
  const selectDirectory = (directory) => {
    setMenuOpen(false)
    setDirectory(directory)
  }
  const title = (crumb) =>
    !crumb.directory
      ? crumb.label
      : crumb.directory.path === '.'
        ? rootPath
        : `${rootPath.replace(/[\\/]+$/, '')}/${crumb.directory.path}`

  return (
    <>
      <div className="h-[calc(2rem+env(safe-area-inset-bottom))]" aria-hidden="true" />
      <nav
        aria-label={zh('当前视频目录', 'Current video directory')}
        className="fixed bottom-0 left-[var(--side-tabs-width)] right-0 z-40 border-t border-gray-200 bg-white pl-3 pr-[var(--page-right-padding)] shadow-[0_-2px_8px_rgba(15,23,42,0.04)]"
        style={{ paddingBottom: 'env(safe-area-inset-bottom)' }}
      >
        <ol className="flex h-8 items-center gap-0.5 overflow-x-auto whitespace-nowrap text-[11px]">
          {crumbs.map((crumb, index) => {
            const current = index === crumbs.length - 1
            return (
              <li key={index} className="flex shrink-0 items-center gap-0.5">
                {index > 0 ? (
                  <ChevronRightIcon className="text-gray-400" sx={{ fontSize: 16 }} />
                ) : null}
                <button
                  ref={current ? currentRef : undefined}
                  type="button"
                  aria-current={current ? 'location' : undefined}
                  aria-haspopup="dialog"
                  aria-expanded={menuOpen && menu?.index === index}
                  onClick={(event) => {
                    setMenu({
                      anchor: event.currentTarget,
                      crumb,
                      index,
                      value:
                        crumb.directory ||
                        (directoryId ? { directory_id: directoryId, path: '.' } : null),
                    })
                    setMenuOpen(true)
                  }}
                  title={title(crumb)}
                  className={`inline-flex h-6 items-center gap-1 rounded px-1.5 hover:bg-blue-50 hover:text-blue-700 ${current ? 'bg-blue-50 font-medium text-blue-700' : 'text-gray-600'}`}
                >
                  {index === 0 ? <FolderOutlinedIcon sx={{ fontSize: 16 }} /> : null}
                  <span className="max-w-48 truncate">{crumb.label}</span>
                </button>
              </li>
            )
          })}
        </ol>
      </nav>
      <Popover
        action={popoverRef}
        elevation={3}
        open={menuOpen}
        anchorEl={menu?.anchor}
        onClose={() => setMenuOpen(false)}
        disableScrollLock
        anchorOrigin={{ vertical: 'top', horizontal: 'left' }}
        transformOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        slotProps={{
          // Keep the current directory and anchor throughout the exit animation.
          transition: { onEntered: revealSelection, onExited: () => setMenu(null) },
          paper: {
            role: 'dialog',
            'aria-label': zh('选择视频目录', 'Choose video directory'),
            sx: {
              width: 320,
              maxWidth: 'calc(100vw - 32px)',
              maxHeight: '50vh',
              display: 'flex',
              flexDirection: 'column',
              overflow: 'hidden',
            },
          },
        }}
      >
        <div ref={treeContainerRef} className="min-h-0 overflow-y-auto p-1">
          {loading ? (
            <div role="status" className="px-3 py-2 text-xs text-gray-500">
              {zh('加载中…', 'Loading...')}
            </div>
          ) : error ? (
            <>
              <div role="alert" className="px-3 py-2 text-xs text-red-600">
                {error}
              </div>
              <button type="button" onClick={retry} className="px-3 py-2 text-xs text-blue-600">
                {zh('重试', 'Retry')}
              </button>
            </>
          ) : nodes.length ? (
            <VideoDirectoryTree
              nodes={nodes}
              value={menu?.value}
              expandSelection={Boolean(directory)}
              onSelect={selectDirectory}
              // Native disclosure changes the height without a React render.
              onToggle={() => popoverRef.current?.updatePosition()}
            />
          ) : (
            <div role="status" className="px-3 py-2 text-xs text-gray-500">
              {zh('暂无视频目录', 'No video directories')}
            </div>
          )}
        </div>
        {!directory && (
          <button
            type="button"
            onClick={() => selectDirectory(null)}
            className="shrink-0 border-t border-gray-200 bg-white px-4 py-2 text-left text-xs hover:bg-gray-100 focus-visible:bg-gray-100"
          >
            {zh('查看全部视频', 'View all videos')}
          </button>
        )}
      </Popover>
    </>
  )
}
