export function javEditNameKey(value) {
  return String(value || '')
    .trim()
    .normalize('NFKC')
    .toLowerCase()
}

export function findJavEditOptionByName(options, value, getNames = (option) => [option?.name]) {
  const key = javEditNameKey(value)
  if (!key) return null

  return (
    (options || []).find((option) =>
      (getNames(option) || []).some((name) => javEditNameKey(name) === key)
    ) || null
  )
}
