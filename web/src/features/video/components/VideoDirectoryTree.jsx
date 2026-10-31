import { useState } from 'react'
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

function DirectoryNode({ node, value, showSelection, expandSelection, onSelect, onToggle }) {
  const selected =
    showSelection && value?.directory_id === node.directory_id && value.path === node.path
  const containsSelection =
    expandSelection &&
    value?.directory_id === node.directory_id &&
    (node.path === '.' || value.path.startsWith(`${node.path}/`))
  const label = node.path === '.' ? node.directory_path : node.path.split('/').pop()
  const path = directoryLabel(node)
  const rowClassName = `group/directory flex min-w-0 items-center rounded text-xs hover:bg-sky-50 ${selected ? 'bg-sky-50 font-semibold text-sky-700' : 'text-gray-700'}`
  const name = (
    <span className="flex min-w-0 flex-1 items-center gap-2 px-2 py-1.5" title={path}>
      <FolderOutlinedIcon className="shrink-0" fontSize="small" />
      <span className="truncate">{label}</span>
    </span>
  )
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
      className="pointer-events-none mr-1 shrink-0 rounded px-2 py-1 text-[11px] font-medium text-blue-600 opacity-0 hover:bg-blue-100 focus-visible:pointer-events-auto focus-visible:opacity-100 focus-visible:outline-blue-500 group-hover/directory:pointer-events-auto group-hover/directory:opacity-100 [@media(hover:none)]:pointer-events-auto [@media(hover:none)]:opacity-100"
    >
      {zh('选择', 'Select')}
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
            className={`${rowClassName} cursor-pointer list-none focus-visible:outline-sky-500 [&::-webkit-details-marker]:hidden`}
            aria-label={zh(`展开或收起目录 ${path}`, `Expand or collapse directory ${path}`)}
          >
            <ChevronRightIcon className="shrink-0 text-gray-500" fontSize="small" />
            {name}
            {selection}
          </summary>
          <ul className="ml-2.5 border-l border-gray-200 pl-2.5">
            {Array.from(node.children.values()).map((child) => (
              <DirectoryNode
                key={child.path}
                node={child}
                value={value}
                showSelection={showSelection}
                expandSelection={expandSelection}
                onSelect={onSelect}
                onToggle={onToggle}
              />
            ))}
          </ul>
        </details>
      ) : (
        <div className={`${rowClassName} pl-5`}>
          {name}
          {selection}
        </div>
      )}
    </li>
  )
}

export default function VideoDirectoryTree({
  nodes,
  value,
  expandSelection = true,
  onSelect,
  onToggle,
}) {
  const [showSelection, setShowSelection] = useState(true)
  return (
    <ul aria-label={zh('目录树', 'Directory tree')} onClickCapture={() => setShowSelection(false)}>
      {nodes.map((node) => (
        <DirectoryNode
          key={`${node.directory_id}:${node.path}`}
          node={node}
          value={value}
          showSelection={showSelection}
          expandSelection={expandSelection}
          onSelect={onSelect}
          onToggle={onToggle}
        />
      ))}
    </ul>
  )
}
