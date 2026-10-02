import { useEffect, useRef } from 'react'

const titles = new Map()
let originalTitle = null

function updateTitle() {
  const active = [...titles.values()].sort((a, b) => b.priority - a.priority)[0]
  document.title = active?.title ?? originalTitle
  if (!active) originalTitle = null
}

// A detail or overlay keeps ownership while its background page updates.
export default function useDocumentTitle(title, { priority = 0, enabled = true } = {}) {
  const owner = useRef({})
  useEffect(() => {
    if (!enabled) return undefined
    if (titles.size === 0) originalTitle = document.title
    const key = owner.current
    titles.set(key, { title, priority })
    updateTitle()
    return () => {
      titles.delete(key)
      updateTitle()
    }
  }, [enabled, priority, title])
}
