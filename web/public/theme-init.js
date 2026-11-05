// Run before the body is painted; keep the storage key in sync with app/theme/preferences.js.
;(function () {
  let preference = 'light'
  try {
    const saved = localStorage.getItem('javboss.theme')
    if (['light', 'dark', 'system'].includes(saved)) preference = saved
  } catch {
    // Storage can be unavailable in private or restricted browser contexts.
  }
  const dark =
    preference === 'dark' ||
    (preference === 'system' && matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', dark)
  document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute('content', dark ? '#181818' : '#f9fafb')
})()
