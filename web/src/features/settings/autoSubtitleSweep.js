import { zh } from '@/utils/i18n'

// sweepErrorMessages maps the server's stable error codes to readable text.
const sweepErrorMessages = {
  gateway_unavailable: [
    '自动字幕网关不可用，请确认网关已启动。',
    'The AutoSub gateway is unavailable. Make sure it is running.',
  ],
  settings_unavailable: ['无法读取自动字幕设置。', 'Could not read the auto subtitle settings.'],
  locations_unavailable: [
    '无法读取视频库，稍后会自动重试。',
    'Could not read the video library; the sweep will retry later.',
  ],
}

// autoSubtitleRunError localizes a failed sweep by its error code, falling back
// to the server message for codes this build does not know.
export function autoSubtitleRunError(run) {
  if (!run?.error) return ''
  const message = sweepErrorMessages[run.error_code]
  return message ? zh(message[0], message[1]) : run.error
}

// autoSubtitleRunSummary describes the outcome of the last subtitle sweep.
export function autoSubtitleRunSummary(run) {
  if (!run) return ''
  const scanned = Number(run.scanned || 0)
  const downloaded = Number(run.downloaded || 0)
  const skipped = Number(run.skipped || 0)
  const failed = Number(run.failed || 0)
  return zh(
    `上次扫描：检查 ${scanned}，下载 ${downloaded}，跳过 ${skipped}，失败 ${failed}`,
    `Last sweep: scanned ${scanned}, downloaded ${downloaded}, skipped ${skipped}, failed ${failed}`
  )
}

// autoSubtitleRunTime formats a run timestamp for the settings summary.
export function autoSubtitleRunTime(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}
