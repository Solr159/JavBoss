import { zh } from '@/utils/i18n'

export const BULK_PLAY_CONFIRM_THRESHOLD = 500

export const confirmLargePlaylist = (count) => {
  if (count <= BULK_PLAY_CONFIRM_THRESHOLD) return true
  return window.confirm(
    zh(
      `即将播放 ${count} 个视频。视频数量较多，可能造成播放器加载卡顿，是否继续？`,
      `You are about to play ${count} videos. A large playlist may cause the player to load slowly. Continue?`
    )
  )
}

export const normalizeDefaultPlayer = (value) => {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
  if (normalized === 'browser' || normalized === 'system') return normalized
  return 'mpv'
}
