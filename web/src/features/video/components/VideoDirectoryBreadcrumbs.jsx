import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Menu, MenuItem } from '@mui/material'
import ChevronRightIcon from '@mui/icons-material/ChevronRight'
import FolderOutlinedIcon from '@mui/icons-material/FolderOutlined'
import { useShallow } from 'zustand/react/shallow'
import useVideoDirectories from '@/features/video/hooks/useVideoDirectories'
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

  const parent = menu?.crumb.directory
  const prefix = parent && parent.path !== '.' ? `${parent.path}/` : ''
  const children = options.filter((directory) => {
    if (!parent) return directory.path === '.'
    if (directory.directory_id !== parent.directory_id || directory.path === '.') return false
    if (!directory.path.startsWith(prefix)) return false
    const relative = directory.path.slice(prefix.length)
    return relative && !relative.includes('/')
  })
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
        className="fixed bottom-0 left-[var(--side-tabs-width)] right-0 z-40 border-t border-gray-200 bg-white pl-[var(--page-left-padding)] pr-[var(--page-right-padding)] shadow-[0_-2px_8px_rgba(15,23,42,0.04)]"
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
                  aria-haspopup="menu"
                  aria-expanded={menuOpen && menu?.index === index}
                  onClick={(event) => {
                    setMenu({ anchor: event.currentTarget, crumb, index })
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
      <Menu
        open={menuOpen}
        anchorEl={menu?.anchor}
        onClose={() => setMenuOpen(false)}
        disableScrollLock
        anchorOrigin={{ vertical: 'top', horizontal: 'left' }}
        transformOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        slotProps={{
          // Keep the current directory and anchor throughout the exit animation.
          transition: { onExited: () => setMenu(null) },
          paper: { sx: { minWidth: 220, maxWidth: 'calc(100vw - 32px)', maxHeight: '50vh' } },
          list: {
            dense: true,
            sx: { py: 0 },
            'aria-label': zh('选择视频目录', 'Choose video directory'),
          },
        }}
      >
        {loading ? (
          <MenuItem disabled>{zh('加载中…', 'Loading...')}</MenuItem>
        ) : error ? (
          [
            <MenuItem key="error" disabled sx={{ whiteSpace: 'normal' }}>
              {error}
            </MenuItem>,
            <MenuItem key="retry" onClick={retry}>
              {zh('重试', 'Retry')}
            </MenuItem>,
          ]
        ) : children.length ? (
          children.map((directory) => (
            <MenuItem
              key={`${directory.directory_id}:${directory.path}`}
              selected={
                directory.directory_id === directoryId &&
                (directory.path === '.' ||
                  subdirectory === directory.path ||
                  subdirectory.startsWith(`${directory.path}/`))
              }
              onClick={() => selectDirectory(directory)}
              title={directory.path === '.' ? directory.directory_path : directory.path}
            >
              <FolderOutlinedIcon className="mr-2 shrink-0 text-blue-600" fontSize="small" />
              <span className="truncate">
                {directory.path === '.'
                  ? directory.directory_path
                  : directory.path.split('/').pop()}
              </span>
            </MenuItem>
          ))
        ) : (
          <MenuItem disabled>{zh('没有子目录', 'No subdirectories')}</MenuItem>
        )}
        <MenuItem
          onClick={() => selectDirectory(parent || null)}
          sx={{
            position: 'sticky',
            bottom: 0,
            zIndex: 1,
            bgcolor: 'background.paper',
            borderTop: 1,
            borderColor: 'divider',
            '&:hover, &.Mui-focusVisible': { bgcolor: 'grey.100' },
          }}
        >
          {parent
            ? zh('查看此目录的视频', 'View videos in this directory')
            : zh('查看全部视频', 'View all videos')}
        </MenuItem>
      </Menu>
    </>
  )
}
