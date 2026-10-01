export const HISTORY_INDEX_KEY = '__javbossHistoryIndex'

export const HISTORY_SCROLL_KEY = '__javbossScroll'

export const SCROLL_RESTORE_MAX_ATTEMPTS = 30

export const getHistoryUserState = (state) =>
  state?.usr && typeof state.usr === 'object' ? state.usr : {}

export const getHistoryAppStateValue = (state, key) => {
  const userState = getHistoryUserState(state)
  return state?.[key] ?? userState?.[key]
}

export const withHistoryAppState = (state, entries) => ({
  ...(state || {}),
  ...entries,
  usr: {
    ...getHistoryUserState(state),
    ...entries,
  },
})

export const readWindowScrollPosition = () => ({
  x: Math.max(0, Math.round(window.scrollX || window.pageXOffset || 0)),
  y: Math.max(0, Math.round(window.scrollY || window.pageYOffset || 0)),
})

export const normalizeHistoryScrollPosition = (value) => {
  const x = Number(value?.x)
  const y = Number(value?.y)
  return {
    x: Number.isFinite(x) && x > 0 ? Math.round(x) : 0,
    y: Number.isFinite(y) && y > 0 ? Math.round(y) : 0,
  }
}
