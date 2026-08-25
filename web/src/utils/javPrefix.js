export const JAV_PREFIX_INITIAL_OPTIONS = Array.from('0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ')

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
