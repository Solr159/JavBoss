// Portals and deferred image/screenshot sections can initially be too short.
export function restoreDetailScroll(node, target, onFinish) {
  let finished = false
  let resizeObserver
  let mutationObserver
  const events = ['wheel', 'touchstart', 'pointerdown', 'keydown']
  const cleanup = () => {
    resizeObserver?.disconnect()
    mutationObserver?.disconnect()
    for (const event of events) node.removeEventListener(event, finish)
  }
  const finish = () => {
    if (finished) return
    finished = true
    cleanup()
    onFinish()
  }
  const attempt = () => {
    if (finished) return
    node.scrollTop = target
    if (Math.abs(node.scrollTop - target) <= 1) finish()
  }
  attempt()
  if (!finished) {
    resizeObserver = new ResizeObserver(attempt)
    const observeChildren = () => {
      resizeObserver.observe(node)
      for (const child of node.children) resizeObserver.observe(child)
      attempt()
    }
    mutationObserver = new MutationObserver(observeChildren)
    mutationObserver.observe(node, { childList: true, subtree: true })
    observeChildren()
    if (!finished) {
      for (const event of events) node.addEventListener(event, finish, { passive: true })
    }
  }
  return () => {
    finished = true
    cleanup()
  }
}
