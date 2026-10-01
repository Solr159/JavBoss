import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { useCallback, useMemo, useEffect } from 'react'
import {
  parseWebHotkeys,
  webHotkeyKeyId,
  isWebHotkeyEditingTarget,
  webHotkeyFromKeyboardEvent,
} from '@/utils/webHotkeys'

export default function useWebHotkeys({ isJavMode, waterfallModes, setJavQueryEditorOpen }) {
  const {
    page,
    hasNext,
    setPage,
    javTotal,
    javPageSize,
    javPage,
    idolTotal,
    idolPageSize,
    idolPage,
    studioTotal,
    studioPageSize,
    studioPage,
    seriesTotal,
    seriesPageSize,
    seriesPage,
    config,
    randomMode,
    loading,
    javTab,
    idolLoading,
    setIdolPage,
    studioLoading,
    setStudioPage,
    seriesLoading,
    setSeriesPage,
    javRandomMode,
    javLoading,
    setJavPage,
    loadJavTags,
  } = useStore(
    useShallow((state) => ({
      page: state.page,
      hasNext: state.hasNext,
      setPage: state.setPage,
      javTotal: state.javTotal,
      javPageSize: state.javPageSize,
      javPage: state.javPage,
      idolTotal: state.idolTotal,
      idolPageSize: state.idolPageSize,
      idolPage: state.idolPage,
      studioTotal: state.studioTotal,
      studioPageSize: state.studioPageSize,
      studioPage: state.studioPage,
      seriesTotal: state.seriesTotal,
      seriesPageSize: state.seriesPageSize,
      seriesPage: state.seriesPage,
      config: state.config,
      randomMode: state.randomMode,
      loading: state.loading,
      javTab: state.javTab,
      idolLoading: state.idolLoading,
      setIdolPage: state.setIdolPage,
      studioLoading: state.studioLoading,
      setStudioPage: state.setStudioPage,
      seriesLoading: state.seriesLoading,
      setSeriesPage: state.setSeriesPage,
      javRandomMode: state.javRandomMode,
      javLoading: state.javLoading,
      setJavPage: state.setJavPage,
      loadJavTags: state.loadJavTags,
    }))
  )
  const canPrev = page > 1

  const canNext = hasNext

  const navigateVideoPage = useCallback(
    (targetPage) => {
      if (!targetPage || targetPage === page) return
      setPage(targetPage)
    },
    [page, setPage]
  )

  const javLastPage = Math.max(1, Math.ceil((javTotal || 0) / javPageSize))

  const javHasPrev = javPage > 1

  const javHasNext = javPage < javLastPage

  const idolLastPage = Math.max(1, Math.ceil((idolTotal || 0) / idolPageSize))

  const idolHasPrev = idolPage > 1

  const idolHasNext = idolPage < idolLastPage

  const studioLastPage = Math.max(1, Math.ceil((studioTotal || 0) / studioPageSize))

  const studioHasPrev = studioPage > 1

  const studioHasNext = studioPage < studioLastPage

  const seriesLastPage = Math.max(1, Math.ceil((seriesTotal || 0) / seriesPageSize))

  const seriesHasPrev = seriesPage > 1

  const seriesHasNext = seriesPage < seriesLastPage

  const webHotkeys = useMemo(() => parseWebHotkeys(config?.web_hotkeys), [config?.web_hotkeys])

  const navigateActivePageBy = useCallback(
    (direction) => {
      if (!isJavMode) {
        if (randomMode || waterfallModes.video || loading) return
        if (direction < 0 && canPrev) {
          navigateVideoPage(page - 1)
        } else if (direction > 0 && canNext) {
          navigateVideoPage(page + 1)
        }
        return
      }

      const waterfallKey = javTab === 'list' ? 'jav' : javTab
      const activeWaterfallMode = Boolean(waterfallModes[waterfallKey])
      if (activeWaterfallMode) return
      if (javTab === 'idol') {
        if (idolLoading) return
        if (direction < 0 && idolHasPrev) setIdolPage(idolPage - 1)
        else if (direction > 0 && idolHasNext) setIdolPage(idolPage + 1)
      } else if (javTab === 'studio') {
        if (studioLoading) return
        if (direction < 0 && studioHasPrev) setStudioPage(studioPage - 1)
        else if (direction > 0 && studioHasNext) setStudioPage(studioPage + 1)
      } else if (javTab === 'series') {
        if (seriesLoading) return
        if (direction < 0 && seriesHasPrev) setSeriesPage(seriesPage - 1)
        else if (direction > 0 && seriesHasNext) setSeriesPage(seriesPage + 1)
      } else {
        if (javRandomMode || javLoading) return
        if (direction < 0 && javHasPrev) setJavPage(javPage - 1)
        else if (direction > 0 && javHasNext) setJavPage(javPage + 1)
      }
    },
    [
      canNext,
      canPrev,
      idolHasNext,
      idolHasPrev,
      idolLoading,
      idolPage,
      isJavMode,
      javHasNext,
      javHasPrev,
      javLoading,
      javPage,
      javRandomMode,
      javTab,
      loading,
      navigateVideoPage,
      page,
      randomMode,
      seriesHasNext,
      seriesHasPrev,
      seriesLoading,
      seriesPage,
      setIdolPage,
      setJavPage,
      setSeriesPage,
      setStudioPage,
      studioHasNext,
      studioHasPrev,
      studioLoading,
      studioPage,
      waterfallModes,
    ]
  )

  useEffect(() => {
    const actionByKey = new Map(webHotkeys.map((item) => [webHotkeyKeyId(item.key), item.action]))
    const modifierKeys = new Set(['Alt', 'AltGraph', 'Control', 'Meta', 'OS', 'Shift'])
    let continuousAction = ''
    let continuousBaseKeyId = ''
    let continuousFrameId = null
    let previousFrameTime = 0

    const hasShortcutBlockingOverlay = (action = '') => {
      if (document.documentElement.classList.contains('app-modal-open')) {
        const dialogs = Array.from(document.querySelectorAll('[role="dialog"][aria-modal="true"]'))
        const topDialog = dialogs[dialogs.length - 1]
        const imageNavigationAllowed =
          (action === 'previous_page' || action === 'next_page') &&
          topDialog?.classList.contains('image-preview-modal')
        const onlyJavQueryEditorOpen =
          action === 'edit_jav_query' &&
          dialogs.length === 1 &&
          dialogs[0].classList.contains('jav-query-editor-modal')
        if (!onlyJavQueryEditorOpen && !imageNavigationAllowed) return true
      }
      return Array.from(document.querySelectorAll('.MuiPopover-root, .MuiMenu-root')).some(
        (overlay) =>
          action !== 'open_page_jump' || !overlay.querySelector('.pagination-jump-popover')
      )
    }

    const blurActiveControl = () => {
      if (document.activeElement instanceof HTMLElement) {
        document.activeElement.blur()
      }
    }

    const stopContinuousScroll = () => {
      if (continuousFrameId != null) window.cancelAnimationFrame(continuousFrameId)
      continuousAction = ''
      continuousBaseKeyId = ''
      continuousFrameId = null
      previousFrameTime = 0
    }

    const runContinuousScroll = (frameTime) => {
      if (!continuousAction || hasShortcutBlockingOverlay()) {
        stopContinuousScroll()
        return
      }
      if (previousFrameTime > 0) {
        const elapsed = Math.min(50, frameTime - previousFrameTime)
        const direction = continuousAction === 'continuous_scroll_up' ? -1 : 1
        window.scrollBy({ top: direction * elapsed * 0.4, left: 0, behavior: 'auto' })
      }
      previousFrameTime = frameTime
      continuousFrameId = window.requestAnimationFrame(runContinuousScroll)
    }

    const startContinuousScroll = (action, baseKey) => {
      if (continuousAction === action && continuousFrameId != null) return
      stopContinuousScroll()
      continuousAction = action
      continuousBaseKeyId = webHotkeyKeyId(baseKey)
      continuousFrameId = window.requestAnimationFrame(runContinuousScroll)
    }

    const handleKeyDown = (event) => {
      if (continuousAction && modifierKeys.has(event.key)) stopContinuousScroll()
      if (
        event.defaultPrevented ||
        event.isComposing ||
        event.altKey ||
        event.ctrlKey ||
        event.metaKey ||
        isWebHotkeyEditingTarget(event.target)
      ) {
        return
      }

      const pressedKey = webHotkeyFromKeyboardEvent(event)
      const action = actionByKey.get(webHotkeyKeyId(pressedKey))
      if (!action) return
      if (hasShortcutBlockingOverlay(action)) return
      if (action === 'edit_jav_query' && (!isJavMode || javTab !== 'list')) return
      const pageJumpTrigger =
        action === 'open_page_jump'
          ? document.querySelector('[data-page-jump-trigger="true"]:not(:disabled)')
          : null
      if (action === 'open_page_jump' && !pageJumpTrigger) return
      const imagePreviewOpen = Boolean(document.querySelector('.image-preview-modal'))
      const imageNavigationTrigger =
        imagePreviewOpen && (action === 'previous_page' || action === 'next_page')
          ? document.querySelector(
              `[data-image-navigation="${action === 'previous_page' ? 'previous' : 'next'}"]`
            )
          : null
      event.preventDefault()
      blurActiveControl()

      if (imagePreviewOpen && (action === 'previous_page' || action === 'next_page')) {
        imageNavigationTrigger?.click()
      } else if (action === 'edit_jav_query') {
        stopContinuousScroll()
        if (!event.repeat) {
          setJavQueryEditorOpen((current) => {
            if (!current) loadJavTags()
            return !current
          })
        }
      } else if (action === 'open_page_jump') {
        if (!event.repeat) pageJumpTrigger.click()
      } else if (action === 'continuous_scroll_up' || action === 'continuous_scroll_down') {
        startContinuousScroll(action, event.key)
      } else if (action === 'content_page_up' || action === 'content_page_down') {
        const viewportHeight = document.scrollingElement?.clientHeight || window.innerHeight || 1
        window.scrollBy({
          top: (action === 'content_page_up' ? -1 : 1) * Math.max(1, viewportHeight * 0.9),
          left: 0,
          behavior: 'smooth',
        })
      } else if (action === 'previous_page') {
        navigateActivePageBy(-1)
      } else if (action === 'next_page') {
        navigateActivePageBy(1)
      } else if (action === 'browser_back') {
        window.history.back()
      } else if (action === 'browser_forward') {
        window.history.forward()
      }
    }

    const handleKeyUp = (event) => {
      if (!continuousAction) return
      if (modifierKeys.has(event.key) || webHotkeyKeyId(event.key) === continuousBaseKeyId) {
        stopContinuousScroll()
      }
    }

    const handleVisibilityChange = () => {
      if (document.hidden) stopContinuousScroll()
    }

    window.addEventListener('keydown', handleKeyDown)
    window.addEventListener('keyup', handleKeyUp)
    window.addEventListener('blur', stopContinuousScroll)
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => {
      stopContinuousScroll()
      window.removeEventListener('keydown', handleKeyDown)
      window.removeEventListener('keyup', handleKeyUp)
      window.removeEventListener('blur', stopContinuousScroll)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
    }
  }, [isJavMode, javTab, loadJavTags, navigateActivePageBy, setJavQueryEditorOpen, webHotkeys])
  return { webHotkeys }
}
