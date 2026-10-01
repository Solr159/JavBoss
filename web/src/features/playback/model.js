import { zh } from '@/utils/i18n'

export const MPV_BULK_PLAY_CONFIRM_THRESHOLD = 500

export const confirmLargeMPVPlaylist = (count) => {
  if (count <= MPV_BULK_PLAY_CONFIRM_THRESHOLD) return true
  return window.confirm(
    zh(
      `即将使用 MPV 播放 ${count} 个视频。视频数量较多，可能造成 MPV 加载卡顿，是否继续？`,
      `You are about to play ${count} videos with MPV. A large playlist may cause MPV to load slowly. Continue?`
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
