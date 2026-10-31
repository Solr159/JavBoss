import { useCallback, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { Button, Popover } from '@mui/material'
import FolderOutlinedIcon from '@mui/icons-material/FolderOutlined'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import { useShallow } from 'zustand/react/shallow'
import useVideoDirectories from '@/features/video/hooks/useVideoDirectories'
import VideoDirectoryTree, {
  buildDirectoryTree,
  directoryLabel,
} from '@/features/video/components/VideoDirectoryTree'
import { useStore } from '@/store'
import { zh } from '@/utils/i18n'
import { displayHostPath, hostPathsEnabled } from '@/utils/hostPath'

export default function VideoDirectoryFilter() {
  const { directoryId, subdirectory, directories, setDirectory, useHostPaths } = useStore(
    useShallow((state) => ({
      directoryId: state.videoDirectoryId,
      subdirectory: state.videoSubdirectory,
      directories: state.directories,
      setDirectory: state.setVideoDirectoryFilter,
      useHostPaths: hostPathsEnabled(state.config),
    }))
  )
  const [anchorEl, setAnchorEl] = useState(null)
  const open = Boolean(anchorEl)
  const { options, loading, error, retry } = useVideoDirectories(open)
  const treeContainerRef = useRef(null)

  const revealSelection = useCallback(() => {
    treeContainerRef.current
      ?.querySelector('button[aria-pressed="true"]')
      ?.scrollIntoView({ block: 'center', inline: 'nearest' })
  }, [])

  useLayoutEffect(() => {
    if (!open || loading) return
    // Handle data arriving after the popover has finished opening.
    revealSelection()
  }, [open, loading, options, directoryId, subdirectory, revealSelection])

  const value = useMemo(
    () =>
      directoryId
        ? {
            directory_id: directoryId,
            directory_path:
              directories.find((directory) => directory.id === directoryId)?.path ||
              `#${directoryId}`,
            path: subdirectory || '.',
          }
        : null,
    [directories, directoryId, subdirectory]
  )

  const tree = useMemo(() => buildDirectoryTree(options), [options])
  const selectDirectory = (directory) => {
    setDirectory(directory)
    setAnchorEl(null)
  }

  return (
    <>
      <button
        type="button"
        className={`filter-action-button favorite-menu-trigger ${value ? 'filter-action-button--active' : ''}`}
        onClick={(event) => setAnchorEl(event.currentTarget)}
        aria-label={zh('选择子目录', 'Select subdirectory')}
        aria-haspopup="dialog"
        aria-expanded={open}
        title={value ? directoryLabel(value, useHostPaths) : zh('全部目录', 'All directories')}
      >
        <FolderOutlinedIcon fontSize="small" />
        {value ? (
          <span className="max-w-28 truncate">
            {value.path === '.' ? displayHostPath(value.directory_path, useHostPaths) : value.path}
          </span>
        ) : null}
        <ExpandMoreIcon fontSize="small" className="favorite-menu-trigger__chevron" />
      </button>
      <Popover
        open={open}
        anchorEl={anchorEl}
        onClose={() => setAnchorEl(null)}
        disableScrollLock
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        transformOrigin={{ vertical: 'top', horizontal: 'left' }}
        slotProps={{
          paper: { role: 'dialog', 'aria-label': zh('视频目录', 'Video directories') },
          transition: { onEntered: revealSelection },
        }}
      >
        <div className="w-96 max-w-[calc(100vw-2rem)]">
          <div className="flex items-center justify-between gap-3 border-b border-gray-100 px-3 py-2">
            <span className="text-xs text-gray-500">
              {zh('包含所有下级目录', 'Includes all descendant directories')}
            </span>
            <Button
              size="small"
              onClick={() => selectDirectory(null)}
              aria-label={zh('清除目录筛选', 'Clear directory filter')}
            >
              {zh('全部目录', 'All directories')}
            </Button>
          </div>
          <div ref={treeContainerRef} className="max-h-[60vh] overflow-y-auto p-2">
            {loading ? (
              <div role="status" className="p-3 text-center text-sm text-gray-500">
                {zh('加载中…', 'Loading...')}
              </div>
            ) : error ? (
              <div role="alert" className="p-3 text-sm text-red-600">
                {error}
                <Button size="small" onClick={retry}>
                  {zh('重试', 'Retry')}
                </Button>
              </div>
            ) : tree.length ? (
              <VideoDirectoryTree
                nodes={tree}
                value={value}
                useHostPaths={useHostPaths}
                onSelect={selectDirectory}
              />
            ) : (
              <div className="p-3 text-center text-sm text-gray-500">
                {zh('暂无视频目录', 'No video directories')}
              </div>
            )}
          </div>
        </div>
      </Popover>
    </>
  )
}
