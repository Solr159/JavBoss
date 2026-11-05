export const THEME_STORAGE_KEY = 'javboss.theme'
export const SYSTEM_THEME_QUERY = '(prefers-color-scheme: dark)'

export function normalizeTheme(value) {
  return ['light', 'dark', 'system'].includes(value) ? value : 'light'
}

export function readTheme() {
  try {
    return normalizeTheme(localStorage.getItem(THEME_STORAGE_KEY))
  } catch {
    return 'light'
  }
}

export function resolveTheme(preference, systemDark) {
  return preference === 'dark' || (preference === 'system' && systemDark) ? 'dark' : 'light'
}

export function applyTheme(mode) {
  document.documentElement.classList.toggle('dark', mode === 'dark')
  document.documentElement.style.colorScheme = mode
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute('content', mode === 'dark' ? '#181818' : '#f9fafb')
}
