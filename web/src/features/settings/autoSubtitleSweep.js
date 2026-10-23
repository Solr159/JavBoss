import { zh } from '@/utils/i18n'

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
