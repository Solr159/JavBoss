import { useCallback, useEffect, useMemo, useState } from 'react'
import { cancelDownloadJob, deleteDownloadJob, fetchDownloadJobs, retryDownloadJob } from '@/api'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

const statusLabels = {
  queued: ['等待处理', 'Queued'],
  offline_downloading: ['云端离线中', 'Offline downloading'],
  resolving_files: ['正在解析文件', 'Resolving files'],
  waiting_local_download: ['等待本地下载', 'Waiting for local download'],
  local_downloading: ['正在下载到本地', 'Downloading locally'],
  completed: ['已完成', 'Completed'],
  failed: ['失败', 'Failed'],
  canceled: ['已取消', 'Canceled'],
}

const activeStatuses = new Set([
  'queued',
  'offline_downloading',
  'resolving_files',
  'waiting_local_download',
  'local_downloading',
])

const providerLabels = { clouddrive2: 'CloudDrive2', openlist: 'OpenList' }

function formatBytes(value) {
  let bytes = Number(value)
  if (!Number.isFinite(bytes) || bytes <= 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let index = 0
  while (bytes >= 1024 && index < units.length - 1) {
    bytes /= 1024
    index += 1
  }
  return `${bytes.toFixed(index >= 3 ? 2 : 1)} ${units[index]}`
}

function formatTime(value) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function statusLabel(status) {
  const labels = statusLabels[status] || [status, status]
  return zh(labels[0], labels[1])
}

export default function JavDiscoveryDownloadsView() {
  const [jobs, setJobs] = useState([])
  const [loading, setLoading] = useState(true)
  const [busyJobIds, setBusyJobIds] = useState(() => new Set())
  const [error, setError] = useState('')

  const loadJobs = useCallback(async () => {
    const payload = await fetchDownloadJobs()
    setJobs(Array.isArray(payload?.items) ? payload.items : [])
  }, [])

  useEffect(() => {
    let cancelled = false
    loadJobs()
      .catch((loadError) => {
        if (!cancelled) setError(getErrorMessage(loadError))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [loadJobs])

  useEffect(() => {
    const timer = window.setInterval(() => loadJobs().catch(() => {}), 3000)
    return () => window.clearInterval(timer)
  }, [loadJobs])

  const counts = useMemo(() => {
    let active = 0
    let failed = 0
    let completed = 0
    jobs.forEach((job) => {
      if (activeStatuses.has(job.status)) active += 1
      else if (job.status === 'failed') failed += 1
      else if (job.status === 'completed') completed += 1
    })
    return { active, failed, completed }
  }, [jobs])

  const runJobAction = async (job, action) => {
    setBusyJobIds((current) => new Set(current).add(job.id))
    setError('')
    try {
      await action(job.id)
      await loadJobs()
    } catch (actionError) {
      setError(getErrorMessage(actionError))
    } finally {
      setBusyJobIds((current) => {
        const next = new Set(current)
        next.delete(job.id)
        return next
      })
    }
  }

  return (
    <div className="space-y-5">
      {error ? (
        <div role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
          {error}
        </div>
      ) : null}

      <section>
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h2 className="text-2xl font-bold text-gray-900">{zh('下载队列', 'Download queue')}</h2>
            <p className="mt-1 text-sm text-gray-500">
              {zh(
                `进行中 ${counts.active} · 已完成 ${counts.completed} · 失败 ${counts.failed}`,
                `${counts.active} active · ${counts.completed} completed · ${counts.failed} failed`
              )}
            </p>
          </div>
          <button
            type="button"
            onClick={() => loadJobs().catch((loadError) => setError(getErrorMessage(loadError)))}
            className="rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-600 hover:bg-gray-50"
          >
            {zh('刷新', 'Refresh')}
          </button>
        </div>

        {loading ? (
          <div className="mt-4 rounded-xl border border-dashed border-gray-200 bg-white p-10 text-center text-sm text-gray-400">
            {zh('加载下载队列…', 'Loading download queue...')}
          </div>
        ) : jobs.length === 0 ? (
          <div className="mt-4 rounded-xl border border-dashed border-gray-200 bg-white p-10 text-center text-sm text-gray-400">
            {zh(
              '队列为空，可在作品详情的磁力列表中创建任务',
              'The queue is empty. Add a job from an item’s magnet list.'
            )}
          </div>
        ) : (
          <div className="mt-4 space-y-3">
            {jobs.map((job) => {
              const progress =
                job.status === 'completed'
                  ? 100
                  : job.bytes_total > 0
                    ? Math.max(0, Math.min(100, (job.bytes_downloaded * 100) / job.bytes_total))
                    : 0
              const showLocalProgress = ['local_downloading', 'completed'].includes(job.status)
              const busy = busyJobIds.has(job.id)
              const terminal = ['completed', 'failed', 'canceled'].includes(job.status)
              return (
                <article
                  key={job.id}
                  className="rounded-xl border border-gray-200 bg-white p-4 shadow-sm"
                >
                  <div className="flex flex-wrap items-start gap-3">
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-mono text-base font-bold text-gray-900">
                          {job.code}
                        </span>
                        <span
                          className={`rounded-full px-2 py-0.5 text-xs font-semibold ${
                            job.status === 'completed'
                              ? 'bg-emerald-100 text-emerald-700'
                              : job.status === 'failed'
                                ? 'bg-red-100 text-red-700'
                                : job.status === 'canceled'
                                  ? 'bg-gray-100 text-gray-500'
                                  : 'bg-blue-100 text-blue-700'
                          }`}
                        >
                          {statusLabel(job.status)}
                        </span>
                        <span className="rounded-full bg-violet-50 px-2 py-0.5 text-xs font-medium text-violet-700">
                          {providerLabels[job.provider] || job.provider}
                        </span>
                      </div>
                      <div className="mt-1 truncate text-sm text-gray-600" title={job.magnet_name}>
                        {job.magnet_name || job.info_hash}
                      </div>
                      <div
                        className="mt-1 truncate text-xs text-gray-400"
                        title={job.directory_path}
                      >
                        {zh('本地：', 'Local: ')}
                        {job.directory_path} · {formatTime(job.created_at)}
                      </div>
                    </div>
                    <div className="flex shrink-0 gap-2">
                      {['failed', 'canceled'].includes(job.status) ? (
                        <button
                          type="button"
                          disabled={busy}
                          onClick={() => runJobAction(job, retryDownloadJob)}
                          className="rounded-md border border-blue-200 bg-blue-50 px-3 py-1.5 text-xs font-semibold text-blue-700 disabled:opacity-50"
                        >
                          {zh('重试', 'Retry')}
                        </button>
                      ) : null}
                      {activeStatuses.has(job.status) ? (
                        <button
                          type="button"
                          disabled={busy}
                          onClick={() => runJobAction(job, cancelDownloadJob)}
                          className="rounded-md border border-amber-200 bg-amber-50 px-3 py-1.5 text-xs font-semibold text-amber-700 disabled:opacity-50"
                        >
                          {zh('取消', 'Cancel')}
                        </button>
                      ) : null}
                      {terminal ? (
                        <button
                          type="button"
                          disabled={busy}
                          onClick={() => runJobAction(job, deleteDownloadJob)}
                          className="rounded-md border border-gray-200 bg-white px-3 py-1.5 text-xs font-semibold text-gray-600 disabled:opacity-50"
                        >
                          {zh('移除记录', 'Remove')}
                        </button>
                      ) : null}
                    </div>
                  </div>
                  {showLocalProgress ? (
                    <>
                      <div className="mt-3 h-2 overflow-hidden rounded-full bg-gray-100">
                        <div
                          className="h-full rounded-full bg-blue-600 transition-all"
                          style={{ width: `${progress}%` }}
                        />
                      </div>
                      <div className="mt-1 flex justify-between text-xs text-gray-400">
                        <span>{progress.toFixed(1)}%</span>
                        <span>
                          {formatBytes(job.bytes_downloaded)} / {formatBytes(job.bytes_total)}
                        </span>
                      </div>
                    </>
                  ) : null}
                  {job.error_message ? (
                    <div className="mt-2 break-words rounded-md bg-red-50 px-2.5 py-2 text-xs text-red-700">
                      {job.error_message}
                    </div>
                  ) : null}
                  {Array.isArray(job.local_files) && job.local_files.length > 0 ? (
                    <div className="mt-2 text-xs text-emerald-700">
                      {zh(
                        `已保存 ${job.local_files.length} 个视频文件`,
                        `${job.local_files.length} video files saved`
                      )}
                    </div>
                  ) : null}
                </article>
              )
            })}
          </div>
        )}
      </section>
    </div>
  )
}
