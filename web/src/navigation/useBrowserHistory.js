import { useRef, useState, useCallback, useEffect } from 'react'
import {
  getHistoryAppStateValue,
  HISTORY_INDEX_KEY,
  readWindowScrollPosition,
  normalizeHistoryScrollPosition,
  HISTORY_SCROLL_KEY,
  withHistoryAppState,
  SCROLL_RESTORE_MAX_ATTEMPTS,
} from '@/navigation/historyState'

export default function useBrowserHistory({ currentRoute }) {
  const browserInitialCanGoBackRef = useRef(window.history.length > 1)

  const browserHistoryIndexRef = useRef(0)

  const browserHistoryMaxRef = useRef(0)

  const pendingScrollRestoreRef = useRef(null)

  const scrollSaveFrameRef = useRef(null)

  const scrollRestoreFrameRef = useRef(null)

  const scrollRestoreTimerRef = useRef(null)

  const preNavigationScrollSaveUrlRef = useRef(null)

  const [browserNavigation, setBrowserNavigation] = useState({
    canGoBack: window.history.length > 1,
    canGoForward: false,
  })

  const setBrowserNavigationFromIndex = useCallback((index, max) => {
    browserHistoryIndexRef.current = index
    browserHistoryMaxRef.current = max
    setBrowserNavigation({
      canGoBack: index > 0 || (index === 0 && browserInitialCanGoBackRef.current),
      canGoForward: index < max,
    })
  }, [])

  const readBrowserHistoryIndex = useCallback((state = window.history.state) => {
    const rawIndex = Number(getHistoryAppStateValue(state, HISTORY_INDEX_KEY))
    return Number.isFinite(rawIndex) && rawIndex >= 0 ? Math.floor(rawIndex) : 0
  }, [])

  const saveCurrentScrollPosition = useCallback(() => {
    const currentState = window.history.state || {}
    const currentScroll = readWindowScrollPosition()
    const previousScroll = normalizeHistoryScrollPosition(
      getHistoryAppStateValue(currentState, HISTORY_SCROLL_KEY)
    )
    if (previousScroll.x === currentScroll.x && previousScroll.y === currentScroll.y) return
    window.history.replaceState(
      withHistoryAppState(currentState, { [HISTORY_SCROLL_KEY]: currentScroll }),
      '',
      currentRoute
    )
  }, [currentRoute])

  const saveScrollBeforeUrlStateChange = useCallback(() => {
    if (scrollSaveFrameRef.current) {
      window.cancelAnimationFrame(scrollSaveFrameRef.current)
      scrollSaveFrameRef.current = null
    }
    saveCurrentScrollPosition()
    preNavigationScrollSaveUrlRef.current = currentRoute
  }, [currentRoute, saveCurrentScrollPosition])

  const ensureBrowserHistoryState = useCallback(() => {
    const currentState = window.history.state || {}
    const stateIndex = getHistoryAppStateValue(currentState, HISTORY_INDEX_KEY)
    const stateScroll = getHistoryAppStateValue(currentState, HISTORY_SCROLL_KEY)
    const hasIndex = Number.isFinite(Number(stateIndex))
    const hasScroll = stateScroll && typeof stateScroll === 'object'
    const index = hasIndex ? readBrowserHistoryIndex(currentState) : browserHistoryIndexRef.current
    if (!hasIndex || !hasScroll) {
      window.history.replaceState(
        withHistoryAppState(currentState, {
          [HISTORY_INDEX_KEY]: index,
          [HISTORY_SCROLL_KEY]: hasScroll
            ? normalizeHistoryScrollPosition(stateScroll)
            : readWindowScrollPosition(),
        }),
        '',
        currentRoute
      )
    }
    setBrowserNavigationFromIndex(index, Math.max(browserHistoryMaxRef.current, index))
  }, [currentRoute, readBrowserHistoryIndex, setBrowserNavigationFromIndex])

  const handleBrowserBack = useCallback(() => {
    saveCurrentScrollPosition()
    window.history.back()
  }, [saveCurrentScrollPosition])

  const handleBrowserForward = useCallback(() => {
    saveCurrentScrollPosition()
    window.history.forward()
  }, [saveCurrentScrollPosition])

  useEffect(() => {
    if (!('scrollRestoration' in window.history)) return undefined
    const previousScrollRestoration = window.history.scrollRestoration
    window.history.scrollRestoration = 'manual'
    return () => {
      window.history.scrollRestoration = previousScrollRestoration
    }
  }, [])

  useEffect(() => {
    const flushScrollPosition = () => {
      if (scrollSaveFrameRef.current) {
        window.cancelAnimationFrame(scrollSaveFrameRef.current)
        scrollSaveFrameRef.current = null
      }
      saveCurrentScrollPosition()
    }
    const handleScroll = () => {
      if (scrollSaveFrameRef.current) return
      scrollSaveFrameRef.current = window.requestAnimationFrame(() => {
        scrollSaveFrameRef.current = null
        saveCurrentScrollPosition()
      })
    }
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'hidden') {
        flushScrollPosition()
      }
    }

    window.addEventListener('scroll', handleScroll, { passive: true })
    window.addEventListener('pagehide', flushScrollPosition)
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => {
      window.removeEventListener('scroll', handleScroll)
      window.removeEventListener('pagehide', flushScrollPosition)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      if (scrollSaveFrameRef.current) {
        window.cancelAnimationFrame(scrollSaveFrameRef.current)
        scrollSaveFrameRef.current = null
      }
    }
  }, [saveCurrentScrollPosition])

  const cancelScheduledScrollRestore = useCallback(() => {
    if (scrollRestoreFrameRef.current) {
      window.cancelAnimationFrame(scrollRestoreFrameRef.current)
      scrollRestoreFrameRef.current = null
    }
    if (scrollRestoreTimerRef.current) {
      window.clearTimeout(scrollRestoreTimerRef.current)
      scrollRestoreTimerRef.current = null
    }
  }, [])

  const schedulePendingScrollRestore = useCallback(() => {
    if (!pendingScrollRestoreRef.current) return
    cancelScheduledScrollRestore()

    const restore = () => {
      const pending = pendingScrollRestoreRef.current
      if (!pending) return

      const maxY = Math.max(
        0,
        Math.max(document.documentElement.scrollHeight, document.body.scrollHeight) -
          window.innerHeight
      )
      const targetX = Math.max(0, pending.x || 0)
      const targetY = Math.max(0, pending.y || 0)
      const nextY = Math.min(targetY, maxY)
      window.scrollTo({ left: targetX, top: nextY, behavior: 'auto' })

      const reached = Math.abs((window.scrollY || window.pageYOffset || 0) - targetY) <= 2
      const canReach = targetY <= maxY + 2
      if ((canReach && reached) || pending.attempts >= SCROLL_RESTORE_MAX_ATTEMPTS) {
        pendingScrollRestoreRef.current = null
        return
      }

      pending.attempts += 1
      scrollRestoreTimerRef.current = window.setTimeout(() => {
        scrollRestoreTimerRef.current = null
        scrollRestoreFrameRef.current = window.requestAnimationFrame(restore)
      }, 50)
    }

    scrollRestoreFrameRef.current = window.requestAnimationFrame(restore)
  }, [cancelScheduledScrollRestore])

  useEffect(() => cancelScheduledScrollRestore, [cancelScheduledScrollRestore])
  return {
    browserHistoryIndexRef,
    browserHistoryMaxRef,
    pendingScrollRestoreRef,
    preNavigationScrollSaveUrlRef,
    browserNavigation,
    setBrowserNavigationFromIndex,
    readBrowserHistoryIndex,
    saveCurrentScrollPosition,
    saveScrollBeforeUrlStateChange,
    ensureBrowserHistoryState,
    handleBrowserBack,
    handleBrowserForward,
    cancelScheduledScrollRestore,
    schedulePendingScrollRestore,
  }
}
