import FolderOutlinedIcon from '@mui/icons-material/FolderOutlined'
import ChevronRightIcon from '@mui/icons-material/ChevronRight'
import { zh } from '@/utils/i18n'

export const directoryLabel = (directory) =>
  directory.path === '.'
    ? directory.directory_path
    : `${directory.directory_path.replace(/[\\/]+$/, '')}/${directory.path}`

// Build missing ancestors too, so each library root remains a separate tree.
export function buildDirectoryTree(directories) {
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

function DirectoryNode({ node, value, onSelect, onToggle }) {
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
        <details
          className="[&[open]>summary>svg]:rotate-90"
          open={containsSelection}
          onToggle={onToggle}
        >
          <summary
            className="flex cursor-pointer list-none items-center rounded focus-visible:outline-sky-500 [&::-webkit-details-marker]:hidden"
            aria-label={zh(`展开或收起目录 ${path}`, `Expand or collapse directory ${path}`)}
          >
            <ChevronRightIcon className="shrink-0 text-gray-500" fontSize="small" />
            {selection}
          </summary>
          <ul className="ml-2.5 border-l border-gray-200 pl-2.5">
            {Array.from(node.children.values()).map((child) => (
              <DirectoryNode
                key={child.path}
                node={child}
                value={value}
                onSelect={onSelect}
                onToggle={onToggle}
              />
            ))}
          </ul>
        </details>
      ) : (
        <div className="flex pl-5">{selection}</div>
      )}
    </li>
  )
}

export default function VideoDirectoryTree({ nodes, value, onSelect, onToggle }) {
  return (
    <ul aria-label={zh('目录树', 'Directory tree')}>
      {nodes.map((node) => (
        <DirectoryNode
          key={`${node.directory_id}:${node.path}`}
          node={node}
          value={value}
          onSelect={onSelect}
          onToggle={onToggle}
        />
      ))}
    </ul>
  )
}
