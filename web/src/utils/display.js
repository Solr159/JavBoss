import { zh } from '@/utils/i18n'

export const getVideoDisplayName = (video) => {
  if (!video) return ''
  if (video.filename) {
    return video.filename
  }
  const segments = (video.path || '').split('/').filter(Boolean)
  if (segments.length > 0) {
    return segments[segments.length - 1]
  }
  if (video.path) {
    return video.path
  }
  return video.id != null ? zh(`视频 #${video.id}`, `Video #${video.id}`) : ''
}

export const buildVideoFullPath = (video) => {
  if (!video) return ''
  const rawPath = String(video.path || '').trim()
  const dirPath = String(video.directory?.path || video.directory_path || '').trim()
  if (!dirPath) return rawPath
  if (!rawPath) return dirPath
  const isAbs = rawPath.startsWith('/') || /^[A-Za-z]:[\\/]/.test(rawPath)
  if (isAbs) return rawPath
  const separator = dirPath.includes('\\') ? '\\' : '/'
  const cleanedDir = dirPath.replace(/[\\/]+$/, '')
  const cleanedRel = rawPath.replace(/^[\\/]+/, '')
  return `${cleanedDir}${separator}${cleanedRel}`
}

export const parseVideoFingerprint = (fp) => {
  if (!fp || typeof fp !== 'string') return {}
  const parts = fp.split('|')
  if (parts.length < 6) return {}
  const res = parts[0]
  const sizePart = parts[parts.length - 1]
  const [w, h] = res.split('x').map((v) => parseInt(v, 10))
  const size = parseInt(sizePart, 10)
  return {
    width: Number.isFinite(w) ? w : null,
    height: Number.isFinite(h) ? h : null,
    size: Number.isFinite(size) ? size : null,
  }
}

export const formatBytes = (bytes) => {
  const size = Number(bytes)
  if (!Number.isFinite(size) || size <= 0) return ''
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let val = size
  let idx = 0
  while (val >= 1024 && idx < units.length - 1) {
    val /= 1024
    idx++
  }
  const rounded = val >= 10 ? Math.round(val) : Math.round(val * 10) / 10
  return `${rounded} ${units[idx]}`
}
