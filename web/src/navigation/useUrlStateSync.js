import { useLocation, useNavigate, useNavigationType } from 'react-router-dom'
import { useMemo, useRef, useState, useCallback, useEffect } from 'react'
import {
  withJavDetail,
  normalizeJavDetailState,
  JAV_DETAIL_HISTORY_KEY,
} from '@/utils/javDetailHistory'
import {
  withStudioDetail,
  normalizeStudioDetailState,
  STUDIO_DETAIL_HISTORY_KEY,
} from '@/utils/studioDetailHistory'
import useBrowserHistory from '@/navigation/useBrowserHistory'
import useDetailNavigation from '@/navigation/useDetailNavigation'
import {
  normalizeHistoryScrollPosition,
  getHistoryAppStateValue,
  HISTORY_SCROLL_KEY,
  readWindowScrollPosition,
  HISTORY_INDEX_KEY,
} from '@/navigation/historyState'
import { parseUrlState, buildUrlFromState } from '@/utils/urlState'

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
  const pageRoute = useMemo(
    () => withJavDetail(withStudioDetail(currentRoute, null), null),
    [currentRoute]
  )

  const lastAppliedPageRouteRef = useRef(null)
  const leaveDetailRef = useRef(false)
  const [detailNavigationRequest, setDetailNavigationRequest] = useState(0)
  const isPoppingRef = useRef(false)
  const lastUrlRef = useRef(currentRoute)
  const routeInitializedRef = useRef(false)

  const {
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
  } = useBrowserHistory({ currentRoute })
  const {
    javDetailId,
    javDetailCacheRef,
    studioDetailId,
    studioDetailCacheRef,
    cacheStudioDetail,
    openStudioDetail,
    closeStudioDetail,
    saveStudioDetailState,
    cacheJavDetail,
    openJavDetail,
    closeJavDetail,
    saveJavDetailState,
  } = useDetailNavigation({
    location,
    currentRoute,
    saveCurrentScrollPosition,
    browserHistoryIndexRef,
    navigate,
    setBrowserNavigationFromIndex,
    handleBrowserBack,
    pageRoute,
  })

  const navigateFromStudioDetail = useCallback(
    (action) => {
      saveCurrentScrollPosition()
      leaveDetailRef.current = true
      action()
      // Also push a result entry when its filters match the underlying page.
      setDetailNavigationRequest((request) => request + 1)
    },
    [saveCurrentScrollPosition]
  )

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
    // Opening or closing a detail must not reload the underlying list.
    if (lastAppliedPageRouteRef.current !== pageRoute) {
      const parsed = parseUrlState(location.search, { defaultView: initialViewMode })
      onParsedView?.(parsed.view)
      applyUrlState(parsed, { fromPopstate: fromRouterPop, route: currentRoute })
      lastAppliedPageRouteRef.current = pageRoute
    }
    routeInitializedRef.current = true
  }, [
    applyUrlState,
    browserHistoryMaxRef,
    cancelScheduledScrollRestore,
    configLoaded,
    currentRoute,
    ensureBrowserHistoryState,
    initialViewMode,
    location.search,
    location.state,
    navigationType,
    onParsedView,
    pageRoute,
    pendingScrollRestoreRef,
    readBrowserHistoryIndex,
    setBrowserNavigationFromIndex,
  ])

  useEffect(() => {
    if (!hydrated) return
    const nextUrl = buildUrlFromState(currentUrlState, location.pathname)
    const currentUrl = currentRoute
    const canonicalPageRoute = buildUrlFromState(
      parseUrlState(location.search, { defaultView: initialViewMode }),
      location.pathname
    )
    if (
      !leaveDetailRef.current &&
      (nextUrl === pageRoute || ((studioDetailId || javDetailId) && nextUrl === canonicalPageRoute))
    ) {
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
    leaveDetailRef.current = false
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
    detailNavigationRequest,
    hydrated,
    initialViewMode,
    location.pathname,
    location.search,
    pageRoute,
    studioDetailId,
    javDetailId,
    navigate,
    saveCurrentScrollPosition,
    setBrowserNavigationFromIndex,
    pendingScrollRestoreRef,
    preNavigationScrollSaveUrlRef,
    browserHistoryIndexRef,
  ])

  return {
    javDetailId,
    javDetailItem: javDetailCacheRef.current.get(javDetailId) || null,
    javDetailState: normalizeJavDetailState(
      getHistoryAppStateValue(window.history.state, JAV_DETAIL_HISTORY_KEY)
    ),
    cacheJavDetail,
    openJavDetail,
    closeJavDetail,
    saveJavDetailState,
    navigateFromJavDetail: navigateFromStudioDetail,
    studioDetailId,
    studioDetailKey: location.key,
    studioDetailItem: studioDetailCacheRef.current.get(studioDetailId) || null,
    cacheStudioDetail,
    studioDetailState: normalizeStudioDetailState(
      getHistoryAppStateValue(window.history.state, STUDIO_DETAIL_HISTORY_KEY)
    ),
    openStudioDetail,
    closeStudioDetail,
    saveStudioDetailState,
    navigateFromStudioDetail,
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
