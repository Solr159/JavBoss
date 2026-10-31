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

const tabKey = (state) => (state.view === 'video' ? 'video' : state.jav.tab)

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
  // Reuse URL state as each tab's snapshot so links and history restore the same filters.
  const tabPagesRef = useRef(new Map())
  const rememberTabPage = useCallback(
    (route, scroll) => {
      if (route !== withJavDetail(withStudioDetail(route, null), null)) return
      const state = parseUrlState(new URL(route, window.location.origin).search, {
        defaultView: initialViewMode,
      })
      tabPagesRef.current.set(tabKey(state), { state, scroll })
    },
    [initialViewMode]
  )

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
  } = useBrowserHistory({ currentRoute, onSaveScroll: rememberTabPage })
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

  const switchLibraryTab = useCallback(
    (tab) => {
      if (tabKey(currentUrlState) === tab && currentRoute === pageRoute) return false
      saveScrollBeforeUrlStateChange()
      const saved = tabPagesRef.current.get(tab)
      const state =
        saved?.state || parseUrlState(tab === 'video' ? '?view=video' : `?view=jav&tab=${tab}`)
      const nextUrl = buildUrlFromState(state, location.pathname)
      const scroll = saved?.scroll || { x: 0, y: 0 }
      const nextIndex = browserHistoryIndexRef.current + 1
      cancelScheduledScrollRestore()
      pendingScrollRestoreRef.current = { ...scroll, attempts: 0 }
      // Commit the route and its filters together, before mounting the destination grid.
      // The URL effect must not hydrate these same filters a second time.
      lastAppliedPageRouteRef.current = nextUrl
      leaveDetailRef.current = false
      navigate(nextUrl, {
        state: { [HISTORY_INDEX_KEY]: nextIndex, [HISTORY_SCROLL_KEY]: scroll },
      })
      applyUrlState(state)
      setBrowserNavigationFromIndex(nextIndex, nextIndex)
      if (scroll.y === 0) schedulePendingScrollRestore()
      return true
    },
    [
      applyUrlState,
      browserHistoryIndexRef,
      cancelScheduledScrollRestore,
      currentRoute,
      currentUrlState,
      location.pathname,
      navigate,
      pageRoute,
      pendingScrollRestoreRef,
      saveScrollBeforeUrlStateChange,
      schedulePendingScrollRestore,
      setBrowserNavigationFromIndex,
    ]
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
    rememberTabPage(
      currentRoute,
      normalizeHistoryScrollPosition(
        getHistoryAppStateValue(window.history.state, HISTORY_SCROLL_KEY)
      )
    )
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
    rememberTabPage,
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
    const previous = parseUrlState(location.search, { defaultView: initialViewMode })
    const directoryChanged =
      currentUrlState.view === 'video' &&
      (previous.view !== 'video' ||
        previous.video.directoryId !== currentUrlState.video.directoryId ||
        previous.video.subdirectory !== currentUrlState.video.subdirectory)
    const restoreScroll = directoryChanged ? { x: 0, y: 0 } : null
    const nextScroll = restoreScroll || readWindowScrollPosition()
    if (restoreScroll) pendingScrollRestoreRef.current = { ...restoreScroll, attempts: 0 }
    navigate(nextUrl, {
      state: {
        [HISTORY_INDEX_KEY]: nextIndex,
        [HISTORY_SCROLL_KEY]: nextScroll,
      },
    })
    setBrowserNavigationFromIndex(nextIndex, nextIndex)
    if (restoreScroll?.y === 0) schedulePendingScrollRestore()
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
    schedulePendingScrollRestore,
    setBrowserNavigationFromIndex,
    pendingScrollRestoreRef,
    preNavigationScrollSaveUrlRef,
    browserHistoryIndexRef,
  ])

  return {
    switchLibraryTab,
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
