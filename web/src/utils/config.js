export const configFlag = (value, fallback = false) => {
  if (value == null || value === '') return fallback
  return !['0', 'false', 'no', 'off'].includes(String(value).trim().toLowerCase())
}

export const normalizeInitialViewMode = (value) =>
  String(value || '')
    .trim()
    .toLowerCase() === 'jav'
    ? 'jav'
    : 'video'
