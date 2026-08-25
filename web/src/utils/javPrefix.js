export const JAV_PREFIX_INITIAL_OPTIONS = Array.from('0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ')

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
