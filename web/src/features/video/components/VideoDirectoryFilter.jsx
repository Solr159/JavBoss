import { useEffect, useMemo, useState } from 'react'
import { Button, Popover } from '@mui/material'
import FolderOutlinedIcon from '@mui/icons-material/FolderOutlined'
import ChevronRightIcon from '@mui/icons-material/ChevronRight'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import { useShallow } from 'zustand/react/shallow'
import { fetchVideoDirectories } from '@/features/video/api'
import { directoryScopeKey } from '@/query/listQueries'
import { useStore } from '@/store'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

const directoryLabel = (directory) =>
  directory.path === '.'
    ? directory.directory_path
    : `${directory.directory_path.replace(/[\\/]+$/, '')}/${directory.path}`

// Build missing ancestors too, so each library root remains a separate tree.
function buildDirectoryTree(directories) {
  const roots = new Map()
  for (const directory of directories) {
    let node = roots.get(directory.directory_id)
    if (!node) {
      node = { ...directory, path: '.', children: new Map() }
      roots.set(directory.directory_id, node)
    }
    if (directory.path === '.') continue
    const segments = directory.path.split('/')
    for (let index = 0; index < segments.length; index++) {
      const path = segments.slice(0, index + 1).join('/')
      if (!node.children.has(path)) {
        node.children.set(path, { ...directory, path, children: new Map() })
      }
      node = node.children.get(path)
    }
  }
  return Array.from(roots.values())
}

function DirectoryNode({ node, value, onSelect }) {
  const selected = value?.directory_id === node.directory_id && value.path === node.path
  const containsSelection =
    value?.directory_id === node.directory_id &&
    (node.path === '.' || value.path.startsWith(`${node.path}/`))
  const label = node.path === '.' ? node.directory_path : node.path.split('/').pop()
  const path = directoryLabel(node)
  const selection = (
    <button
      type="button"
      onClick={(event) => {
        // Selecting a directory is separate from the summary's native disclosure.
        event.preventDefault()
        onSelect(node)
      }}
      aria-label={zh(`选择目录 ${path}`, `Select directory ${path}`)}
      aria-pressed={selected}
      title={path}
      className={`flex min-w-0 flex-1 items-center gap-2 rounded px-2 py-1.5 text-left text-xs hover:bg-sky-50 ${selected ? 'bg-sky-50 font-semibold text-sky-700' : 'text-gray-700'}`}
    >
      <FolderOutlinedIcon className="shrink-0" fontSize="small" />
      <span className="truncate">{label}</span>
    </button>
  )

  return (
    <li>
      {node.children.size ? (
        <details className="[&[open]>summary>svg]:rotate-90" open={containsSelection}>
          <summary
            className="flex cursor-pointer list-none items-center rounded focus-visible:outline-sky-500 [&::-webkit-details-marker]:hidden"
            aria-label={zh(`展开或收起目录 ${path}`, `Expand or collapse directory ${path}`)}
          >
            <ChevronRightIcon className="shrink-0 text-gray-500" fontSize="small" />
            {selection}
          </summary>
          <ul className="ml-2.5 border-l border-gray-200 pl-2.5">
            {Array.from(node.children.values()).map((child) => (
              <DirectoryNode key={child.path} node={child} value={value} onSelect={onSelect} />
            ))}
          </ul>
        </details>
      ) : (
        <div className="flex pl-5">{selection}</div>
      )}
    </li>
  )
}

export default function VideoDirectoryFilter() {
  const { directoryId, subdirectory, directories, scopeKey, setDirectory } = useStore(
    useShallow((state) => ({
      directoryId: state.videoDirectoryId,
      subdirectory: state.videoSubdirectory,
      directories: state.directories,
      scopeKey: directoryScopeKey(state),
      setDirectory: state.setVideoDirectoryFilter,
    }))
  )
  const [anchorEl, setAnchorEl] = useState(null)
  const open = Boolean(anchorEl)
  const [options, setOptions] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [retry, setRetry] = useState(0)

  useEffect(() => {
    if (!open) return undefined
    const controller = new AbortController()
    setLoading(true)
    setError('')
    setOptions([])
    fetchVideoDirectories({ signal: controller.signal })
      .then((items) => {
        if (!controller.signal.aborted) setOptions(items)
      })
      .catch((err) => {
        if (!controller.signal.aborted) setError(getErrorMessage(err))
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    return () => controller.abort()
  }, [open, scopeKey, retry])

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
        className={`filter-action-button ${value ? 'filter-action-button--active' : ''}`}
        onClick={(event) => setAnchorEl(event.currentTarget)}
        aria-label={zh('选择子目录', 'Select subdirectory')}
        aria-haspopup="dialog"
        aria-expanded={open}
        title={value ? directoryLabel(value) : zh('全部目录', 'All directories')}
      >
        <FolderOutlinedIcon fontSize="small" />
        <span className="max-w-40 truncate">
          {value
            ? value.path === '.'
              ? value.directory_path
              : value.path
            : zh('子目录', 'Subdirectory')}
        </span>
        <ExpandMoreIcon fontSize="small" />
      </button>
      <Popover
        open={open}
        anchorEl={anchorEl}
        onClose={() => setAnchorEl(null)}
        disableScrollLock
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        transformOrigin={{ vertical: 'top', horizontal: 'left' }}
        slotProps={{ paper: { role: 'dialog', 'aria-label': zh('视频目录', 'Video directories') } }}
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
          <div className="max-h-[60vh] overflow-y-auto p-2">
            {loading ? (
              <div role="status" className="p-3 text-center text-sm text-gray-500">
                {zh('加载中…', 'Loading...')}
              </div>
            ) : error ? (
              <div role="alert" className="p-3 text-sm text-red-600">
                {error}
                <Button size="small" onClick={() => setRetry((current) => current + 1)}>
                  {zh('重试', 'Retry')}
                </Button>
              </div>
            ) : tree.length ? (
              <ul aria-label={zh('目录树', 'Directory tree')}>
                {tree.map((node) => (
                  <DirectoryNode
                    key={node.directory_id}
                    node={node}
                    value={value}
                    onSelect={selectDirectory}
                  />
                ))}
              </ul>
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
