export const PLAYER_HOTKEY_ACTIONS = {
  SEEK: 'seek',
  VOLUME: 'volume',
  SCREENSHOT: 'screenshot',
}

export const DEFAULT_PLAYER_HOTKEYS = [
  { key: 'a', action: PLAYER_HOTKEY_ACTIONS.SEEK, amount: -1 },
  { key: 'z', action: PLAYER_HOTKEY_ACTIONS.SEEK, amount: 1 },
  { key: 's', action: PLAYER_HOTKEY_ACTIONS.SEEK, amount: -5 },
  { key: 'x', action: PLAYER_HOTKEY_ACTIONS.SEEK, amount: 5 },
  { key: 'd', action: PLAYER_HOTKEY_ACTIONS.SEEK, amount: -30 },
  { key: 'c', action: PLAYER_HOTKEY_ACTIONS.SEEK, amount: 30 },
  { key: 'f', action: PLAYER_HOTKEY_ACTIONS.SEEK, amount: -300 },
  { key: 'v', action: PLAYER_HOTKEY_ACTIONS.SEEK, amount: 300 },
  { key: 'q', action: PLAYER_HOTKEY_ACTIONS.VOLUME, amount: -5 },
  { key: 'w', action: PLAYER_HOTKEY_ACTIONS.VOLUME, amount: 5 },
  { key: 'e', action: PLAYER_HOTKEY_ACTIONS.SCREENSHOT, amount: 0 },
]

function cloneDefaultPlayerHotkeys() {
  return DEFAULT_PLAYER_HOTKEYS.map((item) => ({ ...item }))
}

const INVALID_SINGLE_KEYS = new Set([''])
const MODIFIER_KEYS = new Set([
  'Alt',
  'AltGraph',
  'CapsLock',
  'Control',
  'Fn',
  'Meta',
  'NumLock',
  'ScrollLock',
  'Shift',
  'Symbol',
  'Tab',
])

export function normalizePlayerHotkeyKey(raw) {
  if (raw == null) return ''
  const text = String(raw)
  if (text === ' ') return ' '
  const trimmed = text.trim()
  if (!trimmed) return ''
  if (trimmed === 'Spacebar') return ' '
  if (trimmed === 'Esc') return 'Escape'
  if (trimmed.length === 1) return trimmed.toLowerCase()
  return trimmed
}

export function formatPlayerHotkeyKey(key) {
  const normalized = normalizePlayerHotkeyKey(key)
  if (!normalized) return ''
  if (normalized === ' ') return 'Space'
  return normalized.length === 1 ? normalized.toUpperCase() : normalized
}

export function isReservedPlayerHotkeyKey(key) {
  const normalized = normalizePlayerHotkeyKey(key)
  return normalized === ' ' || normalized === 'Escape'
}

export function isSupportedPlayerHotkeyKey(key) {
  const normalized = normalizePlayerHotkeyKey(key)
  if (INVALID_SINGLE_KEYS.has(normalized) || MODIFIER_KEYS.has(normalized)) {
    return false
  }
  return normalized !== 'Unidentified' && normalized !== 'Process'
}

export function normalizePlayerHotkey(entry) {
  if (!entry || typeof entry !== 'object') return null
  const key = normalizePlayerHotkeyKey(entry.key)
  const action =
    entry.action === PLAYER_HOTKEY_ACTIONS.VOLUME
      ? PLAYER_HOTKEY_ACTIONS.VOLUME
      : entry.action === PLAYER_HOTKEY_ACTIONS.SEEK
        ? PLAYER_HOTKEY_ACTIONS.SEEK
        : entry.action === PLAYER_HOTKEY_ACTIONS.SCREENSHOT
          ? PLAYER_HOTKEY_ACTIONS.SCREENSHOT
          : ''
  const amount = Number(entry.amount)
  if (!key || !action || !Number.isFinite(amount)) {
    return null
  }
  if (action !== PLAYER_HOTKEY_ACTIONS.SCREENSHOT && amount === 0) {
    return null
  }
  if (!isSupportedPlayerHotkeyKey(key) || isReservedPlayerHotkeyKey(key)) {
    return null
  }
  if (action === PLAYER_HOTKEY_ACTIONS.VOLUME && Math.abs(amount) > 100) {
    return null
  }
  return { key, action, amount: action === PLAYER_HOTKEY_ACTIONS.SCREENSHOT ? 0 : amount }
}

export function normalizePlayerHotkeysList(entries) {
  if (!Array.isArray(entries)) return []
  if (entries.length === 0) return cloneDefaultPlayerHotkeys()
  const normalized = []
  const seen = new Set()
  for (const entry of entries) {
    const item = normalizePlayerHotkey(entry)
    if (!item || seen.has(item.key)) continue
    seen.add(item.key)
    normalized.push(item)
  }
  if (
    !normalized.some((item) => item.action === PLAYER_HOTKEY_ACTIONS.SCREENSHOT) &&
    !seen.has('e')
  ) {
    normalized.push({ key: 'e', action: PLAYER_HOTKEY_ACTIONS.SCREENSHOT, amount: 0 })
  }
  return normalized
}

export function parsePlayerHotkeys(rawValue) {
  if (Array.isArray(rawValue)) {
    return normalizePlayerHotkeysList(rawValue)
  }
  if (rawValue == null || String(rawValue).trim() === '') {
    return cloneDefaultPlayerHotkeys()
  }
  try {
    const parsed = JSON.parse(String(rawValue))
    if (!Array.isArray(parsed)) {
      return cloneDefaultPlayerHotkeys()
    }
    return normalizePlayerHotkeysList(parsed)
  } catch {
    return cloneDefaultPlayerHotkeys()
  }
}
