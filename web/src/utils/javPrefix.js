export const JAV_PREFIX_INITIAL_OPTIONS = Array.from('0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ')
export const JAV_PREFIX_PREFERENCES_STORAGE_KEY = 'javboss.javPrefixModal.preferences'

const DEFAULT_JAV_PREFIX_PREFERENCES = {
  censorMode: 'all',
  sortMode: 'count',
}

export function readJavPrefixPreferences(storage) {
  try {
    const stored = JSON.parse(storage?.getItem(JAV_PREFIX_PREFERENCES_STORAGE_KEY) || '{}')
    return {
      censorMode: ['all', 'censored', 'uncensored'].includes(stored?.censorMode)
        ? stored.censorMode
        : DEFAULT_JAV_PREFIX_PREFERENCES.censorMode,
      sortMode: ['count', 'az'].includes(stored?.sortMode)
        ? stored.sortMode
        : DEFAULT_JAV_PREFIX_PREFERENCES.sortMode,
    }
  } catch {
    return { ...DEFAULT_JAV_PREFIX_PREFERENCES }
  }
}

export function writeJavPrefixPreferences(storage, preferences) {
  try {
    storage?.setItem(JAV_PREFIX_PREFERENCES_STORAGE_KEY, JSON.stringify(preferences))
  } catch {
    return
  }
}

export function getAvailableJavPrefixInitials(items) {
  const availableInitials = new Set(
    (items || []).map((item) =>
      String(item?.prefix || '')
        .trim()
        .charAt(0)
        .toUpperCase()
    )
  )

  return JAV_PREFIX_INITIAL_OPTIONS.filter((initial) => availableInitials.has(initial))
}

export function matchesJavPrefixInitial(prefix, initial) {
  const selectedInitial = String(initial || '')
    .trim()
    .toUpperCase()
  if (!selectedInitial) return true

  return (
    String(prefix || '')
      .trim()
      .charAt(0)
      .toUpperCase() === selectedInitial
  )
}
