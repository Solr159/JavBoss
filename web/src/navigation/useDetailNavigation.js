import {
  getJavDetailId,
  withJavDetail,
  JAV_DETAIL_HISTORY_KEY,
  normalizeJavDetailState,
} from '@/utils/javDetailHistory'
import { useRef, useCallback } from 'react'
import {
  withStudioDetail,
  getStudioDetailId,
  STUDIO_DETAIL_HISTORY_KEY,
  normalizeStudioDetailState,
  canReturnToStudioBackground,
} from '@/utils/studioDetailHistory'
import {
  getHistoryUserState,
  HISTORY_INDEX_KEY,
  HISTORY_SCROLL_KEY,
  readWindowScrollPosition,
  getHistoryAppStateValue,
  withHistoryAppState,
} from '@/navigation/historyState'

export default function useDetailNavigation({
  location,
  currentRoute,
  saveCurrentScrollPosition,
  browserHistoryIndexRef,
  navigate,
  setBrowserNavigationFromIndex,
  handleBrowserBack,
  pageRoute,
}) {
  const javDetailId = getJavDetailId(location.search)

  const javDetailCacheRef = useRef(new Map())

  const studioBackgroundRoute = withStudioDetail(currentRoute, null)

  const studioDetailId = getStudioDetailId(location.search)

  const studioDetailCacheRef = useRef(new Map())

  const cacheStudioDetail = useCallback((item) => {
    const id = Number(item?.id)
    if (Number.isSafeInteger(id) && id > 0) studioDetailCacheRef.current.set(id, item)
  }, [])

  const openStudioDetail = useCallback(
    (item) => {
      const id = Number(item?.id)
      if (!Number.isSafeInteger(id) || id <= 0 || id === studioDetailId) return
      cacheStudioDetail(item)
      saveCurrentScrollPosition()
      const nextIndex = browserHistoryIndexRef.current + 1
      navigate(withStudioDetail(studioBackgroundRoute, id), {
        state: {
          ...getHistoryUserState(window.history.state),
          [HISTORY_INDEX_KEY]: nextIndex,
          [HISTORY_SCROLL_KEY]: readWindowScrollPosition(),
          [STUDIO_DETAIL_HISTORY_KEY]: {
            id,
            backgroundRoute: studioBackgroundRoute,
            ...normalizeStudioDetailState(),
          },
        },
      })
      setBrowserNavigationFromIndex(nextIndex, nextIndex)
    },
    [
      studioDetailId,
      cacheStudioDetail,
      saveCurrentScrollPosition,
      browserHistoryIndexRef,
      navigate,
      studioBackgroundRoute,
      setBrowserNavigationFromIndex,
    ]
  )

  const closeStudioDetail = useCallback(() => {
    const historyState = window.history.state
    const detailState = getHistoryAppStateValue(historyState, STUDIO_DETAIL_HISTORY_KEY)
    if (canReturnToStudioBackground(detailState, studioDetailId, studioBackgroundRoute)) {
      handleBrowserBack()
      return
    }
    // A direct link has no background entry to return to.
    navigate(studioBackgroundRoute, {
      replace: true,
      state: { ...getHistoryUserState(historyState), [STUDIO_DETAIL_HISTORY_KEY]: null },
    })
  }, [handleBrowserBack, navigate, studioBackgroundRoute, studioDetailId])

  const saveStudioDetailState = useCallback(
    (changes) => {
      if (!studioDetailId) return
      const currentState = window.history.state || {}
      const detailState = getHistoryAppStateValue(currentState, STUDIO_DETAIL_HISTORY_KEY)
      window.history.replaceState(
        withHistoryAppState(currentState, {
          [STUDIO_DETAIL_HISTORY_KEY]: { ...detailState, id: studioDetailId, ...changes },
        }),
        '',
        currentRoute
      )
    },
    [currentRoute, studioDetailId]
  )

  const cacheJavDetail = useCallback((item) => {
    const id = Number(item?.id)
    if (Number.isSafeInteger(id) && id > 0) javDetailCacheRef.current.set(id, item)
  }, [])

  const openJavDetail = useCallback(
    (item) => {
      const id = Number(item?.id)
      if (!Number.isSafeInteger(id) || id <= 0 || id === javDetailId) return
      cacheJavDetail(item)
      saveCurrentScrollPosition()
      const nextIndex = browserHistoryIndexRef.current + 1
      navigate(withJavDetail(pageRoute, id), {
        state: {
          [HISTORY_INDEX_KEY]: nextIndex,
          [HISTORY_SCROLL_KEY]: readWindowScrollPosition(),
          [JAV_DETAIL_HISTORY_KEY]: {
            id,
            backgroundRoute: pageRoute,
            ...normalizeJavDetailState(),
          },
        },
      })
      setBrowserNavigationFromIndex(nextIndex, nextIndex)
    },
    [
      browserHistoryIndexRef,
      cacheJavDetail,
      javDetailId,
      navigate,
      pageRoute,
      saveCurrentScrollPosition,
      setBrowserNavigationFromIndex,
    ]
  )

  const closeJavDetail = useCallback(() => {
    const historyState = window.history.state
    const detailState = getHistoryAppStateValue(historyState, JAV_DETAIL_HISTORY_KEY)
    if (detailState?.id === javDetailId && detailState?.backgroundRoute === pageRoute) {
      handleBrowserBack()
      return
    }
    navigate(pageRoute, {
      replace: true,
      state: { ...getHistoryUserState(historyState), [JAV_DETAIL_HISTORY_KEY]: null },
    })
  }, [handleBrowserBack, javDetailId, navigate, pageRoute])

  const saveJavDetailState = useCallback(
    (changes) => {
      if (!javDetailId) return
      const currentState = window.history.state || {}
      const detailState = getHistoryAppStateValue(currentState, JAV_DETAIL_HISTORY_KEY)
      window.history.replaceState(
        withHistoryAppState(currentState, {
          [JAV_DETAIL_HISTORY_KEY]: { ...detailState, id: javDetailId, ...changes },
        }),
        '',
        currentRoute
      )
    },
    [currentRoute, javDetailId]
  )
  return {
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
  }
}
