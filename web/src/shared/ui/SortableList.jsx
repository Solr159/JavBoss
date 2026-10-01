import { useEffect, useRef, useState } from 'react'
import MenuRoundedIcon from '@mui/icons-material/MenuRounded'

import { zh } from '@/utils/i18n'

export default function SortableList({
  items,
  onReorder,
  onReorderCommit,
  getLabel,
  getMeta,
  isActive = () => false,
  renderLeading = null,
  renderLabel = null,
  renderActions = null,
  disabled = false,
}) {
  const containerRef = useRef(null)
  const rowRefs = useRef(new Map())
  const draftItemsRef = useRef(null)
  const [drag, setDrag] = useState(null)

  useEffect(() => {
    if (!drag) return undefined

    const handlePointerMove = (event) => {
      event.preventDefault()
      setDrag((current) =>
        current
          ? {
              ...current,
              pointerX: event.clientX,
              pointerY: event.clientY,
            }
          : current
      )

      const nextIndex = calculateDropIndex(items, rowRefs.current, drag.id, event.clientY)
      const currentIndex = items.findIndex((item) => String(item.id) === drag.id)
      if (nextIndex >= 0 && currentIndex >= 0 && nextIndex !== currentIndex) {
        const nextItems = moveItemToIndex(items, drag.id, nextIndex)
        draftItemsRef.current = nextItems
        onReorder?.(nextItems)
      }
    }

    const handlePointerUp = () => {
      const nextItems = draftItemsRef.current
      draftItemsRef.current = null
      setDrag(null)
      if (nextItems) onReorderCommit?.(nextItems)
    }

    window.addEventListener('pointermove', handlePointerMove, { passive: false })
    window.addEventListener('pointerup', handlePointerUp)
    window.addEventListener('pointercancel', handlePointerUp)
    return () => {
      window.removeEventListener('pointermove', handlePointerMove)
      window.removeEventListener('pointerup', handlePointerUp)
      window.removeEventListener('pointercancel', handlePointerUp)
    }
  }, [drag, items, onReorder, onReorderCommit])

  const startDrag = (event, item) => {
    if (disabled || event.button !== 0) return
    const id = String(item.id)
    const row = rowRefs.current.get(id)
    if (!row) return
    const rect = row.getBoundingClientRect()
    event.preventDefault()
    setDrag({
      id,
      pointerX: event.clientX,
      pointerY: event.clientY,
      offsetX: event.clientX - rect.left,
      offsetY: event.clientY - rect.top,
      width: rect.width,
      height: rect.height,
    })
  }

  const renderRow = (item, floating = false) => {
    const id = String(item.id)
    return (
      <SortableRow
        key={floating ? undefined : id}
        refCallback={
          floating
            ? undefined
            : (node) => {
                if (node) rowRefs.current.set(id, node)
                else rowRefs.current.delete(id)
              }
        }
        item={item}
        label={renderLabel ? renderLabel(item) : getLabel(item)}
        meta={getMeta?.(item)}
        leading={renderLeading?.(item)}
        actions={renderActions?.(item)}
        active={isActive(item)}
        dragging={!floating && drag?.id === id}
        floating={floating}
        disabled={disabled}
        onHandlePointerDown={(event) => startDrag(event, item)}
      />
    )
  }

  return (
    <div
      ref={containerRef}
      className={`rounded border border-gray-200 p-1 ${drag ? 'select-none' : ''}`}
    >
      {items.map((item) => renderRow(item))}
      {drag ? (
        <div
          className="pointer-events-none fixed z-[80]"
          style={{
            left: drag.pointerX - drag.offsetX,
            top: drag.pointerY - drag.offsetY,
            width: drag.width,
            height: drag.height,
          }}
        >
          {(() => {
            const item = items.find((candidate) => String(candidate.id) === drag.id)
            return item ? renderRow(item, true) : null
          })()}
        </div>
      ) : null}
    </div>
  )
}

function SortableRow({
  item,
  label,
  meta,
  leading = null,
  actions = null,
  active = false,
  dragging = false,
  floating = false,
  disabled = false,
  refCallback,
  onHandlePointerDown,
}) {
  return (
    <div
      ref={refCallback}
      className={`mb-1 flex items-center gap-2 rounded border px-2 py-1.5 last:mb-0 ${
        floating ? 'shadow-lg ring-1 ring-blue-200' : 'transition-[background-color,opacity]'
      } ${dragging ? 'opacity-0' : 'opacity-100'} ${
        active ? 'border-blue-200 bg-blue-50' : 'border-transparent bg-gray-50'
      }`}
      data-sortable-id={item.id}
    >
      {leading}
      {typeof label === 'string' ? (
        <span className="min-w-0 flex-1 truncate text-sm text-gray-900">{label}</span>
      ) : (
        <div className="min-w-0 flex-1">{label}</div>
      )}
      {meta ? <span className="shrink-0 text-xs text-gray-500">{meta}</span> : null}
      {actions}
      <button
        type="button"
        onPointerDown={onHandlePointerDown}
        disabled={disabled}
        className="inline-flex h-7 w-7 shrink-0 cursor-grab touch-none items-center justify-center rounded border border-gray-200 bg-white text-gray-500 active:cursor-grabbing disabled:cursor-not-allowed disabled:text-gray-300"
        aria-label={zh('拖动排序', 'Drag to reorder')}
        title={zh('拖动排序', 'Drag to reorder')}
      >
        <MenuRoundedIcon sx={{ fontSize: 17 }} />
      </button>
    </div>
  )
}

function calculateDropIndex(items, refs, draggingId, pointerY) {
  let nextIndex = 0
  for (const item of items) {
    const id = String(item.id)
    if (id === draggingId) continue
    const row = refs.get(id)
    if (!row) continue
    const rect = row.getBoundingClientRect()
    if (pointerY > rect.top + rect.height / 2) {
      nextIndex += 1
    }
  }
  return Math.min(nextIndex, items.length - 1)
}

function moveItemToIndex(items, sourceId, targetIndex) {
  const next = [...items]
  const from = next.findIndex((item) => String(item.id) === String(sourceId))
  if (from < 0) return next
  const [item] = next.splice(from, 1)
  const safeIndex = Math.max(0, Math.min(targetIndex, next.length))
  next.splice(safeIndex, 0, item)
  return next
}
