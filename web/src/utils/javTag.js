import { isUserJavTag } from '@/constants/jav'

export function getJavTagDisplayName(tag, showSimplified = false) {
  const name = String(tag?.original_name || tag?.name || '').trim()
  if (!showSimplified || isUserJavTag(tag)) return name
  return String(tag?.simplified_name || name).trim() || name
}

export function withJavTagDisplayName(tag, showSimplified = false) {
  if (!tag || typeof tag !== 'object') return tag
  const originalName = String(tag?.original_name || tag?.name || '').trim()
  const displayName = getJavTagDisplayName(tag, showSimplified)
  if (displayName === tag.name && (!tag.original_name || tag.original_name === originalName)) {
    return tag
  }
  return {
    ...tag,
    name: displayName,
    original_name: originalName,
  }
}
