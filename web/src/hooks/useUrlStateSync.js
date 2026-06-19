import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useLocation, useNavigate, useNavigationType } from 'react-router-dom'
import { buildUrlFromState, parseUrlState } from '@/utils/urlState'

const HISTORY_INDEX_KEY = '__javbossHistoryIndex'
const HISTORY_SCROLL_KEY = '__javbossScroll'
const SCROLL_RESTORE_MAX_ATTEMPTS = 30

const getHistoryUserState = (state) =>
  state?.usr && typeof state.usr === 'object' ? state.usr : {}

const getHistoryAppStateValue = (state, key) => {
  const userState = getHistoryUserState(state)
  return state?.[key] ?? userState?.[key]
}

const withHistoryAppState = (state, entries) => ({
  ...(state || {}),
  ...entries,
  usr: {
    ...getHistoryUserState(state),
    ...entries,
  },
})

const readWindowScrollPosition = () => ({
  x: Math.max(0, Math.round(window.scrollX || window.pageXOffset || 0)),
  y: Math.max(0, Math.round(window.scrollY || window.pageYOffset || 0)),
})

const normalizeHistoryScrollPosition = (value) => {
  const x = Number(value?.x)
  const y = Number(value?.y)
  return {
    x: Number.isFinite(x) && x > 0 ? Math.round(x) : 0,
    y: Number.isFinite(y) && y > 0 ? Math.round(y) : 0,
  }
}

export default function useUrlStateSync({
  applyUrlState,
  configLoaded,
  currentUrlState,
  hydrated,
  initialViewMode,
  onParsedView,
}) {
  const location = useLocation()
  const navigate = useNavigate()
  const navigationType = useNavigationType()
  const currentRoute = useMemo(
    () => `${location.pathname}${location.search}`,
    [location.pathname, location.search]
  )
  const isPoppingRef = useRef(false)
  const lastUrlRef = useRef(currentRoute)
  const routeInitializedRef = useRef(false)
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

  useEffect(() => {
    if (!configLoaded) return
    ensureBrowserHistoryState()
    const fromRouterPop = routeInitializedRef.current && navigationType === 'POP'
    isPoppingRef.current = fromRouterPop
    lastUrlRef.current = currentRoute
    if (fromRouterPop) {
      const index = readBrowserHistoryIndex(location.state)
      const max = Math.max(browserHistoryMaxRef.current, index)
      pendingScrollRestoreRef.current = {
        ...normalizeHistoryScrollPosition(
          getHistoryAppStateValue(location.state, HISTORY_SCROLL_KEY)
        ),
        attempts: 0,
      }
      cancelScheduledScrollRestore()
      setBrowserNavigationFromIndex(index, max)
    }
    const parsed = parseUrlState(location.search, { defaultView: initialViewMode })
    onParsedView?.(parsed.view)
    applyUrlState(parsed, { fromPopstate: fromRouterPop, route: currentRoute })
    routeInitializedRef.current = true
  }, [
    applyUrlState,
    cancelScheduledScrollRestore,
    configLoaded,
    currentRoute,
    ensureBrowserHistoryState,
    initialViewMode,
    location.search,
    location.state,
    navigationType,
    onParsedView,
    readBrowserHistoryIndex,
    setBrowserNavigationFromIndex,
  ])

  useEffect(() => {
    if (!hydrated) return
    const nextUrl = buildUrlFromState(currentUrlState, location.pathname)
    const currentUrl = currentRoute
    if (nextUrl === currentUrl) {
      preNavigationScrollSaveUrlRef.current = null
      lastUrlRef.current = nextUrl
      isPoppingRef.current = false
      return
    }
    if (isPoppingRef.current) {
      preNavigationScrollSaveUrlRef.current = null
      lastUrlRef.current = nextUrl
      isPoppingRef.current = false
      return
    }
    pendingScrollRestoreRef.current = null
    cancelScheduledScrollRestore()
    if (preNavigationScrollSaveUrlRef.current === currentUrl) {
      preNavigationScrollSaveUrlRef.current = null
    } else {
      saveCurrentScrollPosition()
    }
    const nextIndex = browserHistoryIndexRef.current + 1
    const nextScroll = readWindowScrollPosition()
    navigate(nextUrl, {
      state: {
        [HISTORY_INDEX_KEY]: nextIndex,
        [HISTORY_SCROLL_KEY]: nextScroll,
      },
    })
    setBrowserNavigationFromIndex(nextIndex, nextIndex)
    lastUrlRef.current = nextUrl
  }, [
    cancelScheduledScrollRestore,
    currentRoute,
    currentUrlState,
    hydrated,
    location.pathname,
    navigate,
    saveCurrentScrollPosition,
    setBrowserNavigationFromIndex,
  ])

  return {
    browserNavigation,
    currentRoute,
    handleBrowserBack,
    handleBrowserForward,
    pathname: location.pathname,
    pendingScrollRestoreRef,
    saveScrollBeforeUrlStateChange,
    schedulePendingScrollRestore,
  }
}
