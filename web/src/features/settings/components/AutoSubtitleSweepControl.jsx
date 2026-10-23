import { useEffect, useState } from 'react'
import { fetchAutoSubtitleSweep, runAutoSubtitleSweep } from '@/features/settings/api'
import { autoSubtitleRunSummary, autoSubtitleRunTime } from '@/features/settings/autoSubtitleSweep'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

const pollDelay = 2000

export default function AutoSubtitleSweepControl({ dirty = false }) {
  const [status, setStatus] = useState(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const running = Boolean(status?.running)
  const lastRun = status?.last_run

  useEffect(() => {
    const controller = new AbortController()
    let timer
    const poll = async () => {
      try {
        const next = await fetchAutoSubtitleSweep({ signal: controller.signal })
        if (controller.signal.aborted) return
        setStatus(next)
        setError('')
        if (next.running) timer = setTimeout(poll, pollDelay)
      } catch (err) {
        if (controller.signal.aborted) return
        setError(getErrorMessage(err))
        timer = setTimeout(poll, pollDelay * 2)
      }
    }
    void poll()
    return () => {
      controller.abort()
      clearTimeout(timer)
    }
  }, [running])

  const handleRun = async () => {
    if (busy || dirty || running) return
    setBusy(true)
    setError('')
    try {
      setStatus(await runAutoSubtitleSweep())
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-1.5 text-xs text-slate-500">
      <div className="flex items-center justify-between gap-3">
        <span>{zh('使用已保存的设置立即扫描一次。', 'Sweep once using the saved settings.')}</span>
        <button
          type="button"
          onClick={handleRun}
          disabled={busy || dirty || running || status?.available === false}
          className="shrink-0 rounded border border-slate-300 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 hover:border-blue-400 hover:bg-blue-50 disabled:opacity-50"
        >
          {running ? zh('扫描中…', 'Sweeping…') : zh('立即运行', 'Run now')}
        </button>
      </div>
      {dirty && (
        <p className="text-amber-700">
          {zh('请先保存设置，再运行扫描。', 'Save your changes before running a sweep.')}
        </p>
      )}
      {status?.available === false && (
        <p>{zh('自动字幕扫描尚未就绪。', 'The auto subtitle sweep is not ready yet.')}</p>
      )}
      {running && <p role="status">{zh('正在扫描视频库…', 'Scanning the library…')}</p>}
      {!running && lastRun && (
        <p role="status">
          {autoSubtitleRunSummary(lastRun)}
          {autoSubtitleRunTime(lastRun.finished_at) && (
            <span className="text-slate-400"> · {autoSubtitleRunTime(lastRun.finished_at)}</span>
          )}
        </p>
      )}
      {!running && lastRun?.error && (
        <p role="alert" className="break-words text-red-600">
          {lastRun.error}
        </p>
      )}
      {error && (
        <p role="alert" className="break-words text-red-600">
          {error}
        </p>
      )}
    </div>
  )
}
