import { useState } from 'react'

import { cancelDirectoryTranscode } from '@/api'
import { getErrorMessage } from '@/utils/errors'
import { transcodeIsActive, transcodeOverallPercent } from '@/utils/directoryTranscode'
import { zh } from '@/utils/i18n'

export default function DirectoryTranscodeProgress({ directoryId, progress }) {
  const [stopping, setStopping] = useState(false)
  const [error, setError] = useState('')
  if (!progress) return null
  const active = transcodeIsActive(progress)
  const percent = transcodeOverallPercent(progress)
  const labels = {
    discovering: zh('正在查找视频', 'Finding videos'),
    probing: zh('正在检查兼容性', 'Checking compatibility'),
    transcoding: zh('正在转码', 'Transcoding'),
    verifying: zh('正在校验输出', 'Validating output'),
    finalizing: zh('正在更新记录并删除源文件', 'Updating records and deleting source'),
    completed: progress.failed
      ? zh('转码结束，部分文件失败', 'Finished with some failures')
      : zh('转码完成', 'Transcoding complete'),
    failed: zh('转码任务失败', 'Transcode job failed'),
    cancelled: zh('转码已停止', 'Transcoding stopped'),
  }
  const stop = async () => {
    setStopping(true)
    setError('')
    try {
      await cancelDirectoryTranscode(directoryId)
    } catch (err) {
      setError(getErrorMessage(err))
      setStopping(false)
    }
  }
  return (
    <div className="min-w-0 space-y-2 rounded-lg border border-blue-100 bg-blue-50/50 p-3 text-xs text-zinc-700">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="font-medium">{labels[progress.phase] || progress.phase}</span>
        <span className="tabular-nums">
          {progress.processed} / {progress.total} · {Math.floor((progress.elapsed_ms || 0) / 1000)}{' '}
          {zh('秒', 'sec')}
        </span>
        {active && (
          <button
            type="button"
            onClick={stop}
            disabled={stopping}
            className="rounded border bg-white px-2 py-1 hover:bg-zinc-50 disabled:opacity-60"
          >
            {stopping ? zh('正在停止…', 'Stopping...') : zh('停止转码', 'Stop transcoding')}
          </button>
        )}
      </div>
      <progress
        aria-label={zh('目录转码总进度', 'Overall directory transcode progress')}
        max="100"
        value={progress.phase === 'discovering' ? undefined : percent}
        className="block h-2 w-full accent-blue-600"
      />
      {active && progress.current_file && (
        <div className="space-y-1">
          <div className="break-all">{progress.current_file}</div>
          <div className="flex items-center justify-between gap-2 tabular-nums">
            <span>
              {progress.duration_seconds > 0
                ? `${Math.floor(progress.current_percent || 0)}%`
                : zh('时长未知', 'Unknown duration')}
              {progress.speed && progress.speed !== 'N/A' ? ` · ${progress.speed}` : ''}
            </span>
          </div>
          <progress
            aria-label={zh('当前视频转码进度', 'Current video transcode progress')}
            max="100"
            value={progress.duration_seconds > 0 ? progress.current_percent || 0 : undefined}
            className="block h-1.5 w-full accent-blue-500"
          />
        </div>
      )}
      <div className="flex flex-wrap gap-x-3 gap-y-1">
        <span>
          {zh('成功', 'Converted')} {progress.converted}
        </span>
        <span>
          {zh('已兼容跳过', 'Already compatible')} {progress.skipped}
        </span>
        <span>
          {zh('失败', 'Failed')} {progress.failed}
        </span>
      </div>
      {(error || progress.error) && (
        <div role="alert" className="break-all text-red-700">
          {error || progress.error}
        </div>
      )}
      {progress.issues?.length > 0 && (
        <details>
          <summary className="cursor-pointer text-red-700">
            {zh('查看失败原因', 'View failures')}
          </summary>
          <ul className="mt-2 max-h-48 space-y-2 overflow-y-auto">
            {progress.issues.map((issue, index) => (
              <li key={`${issue.path}-${index}`} className="break-all">
                <div className="font-medium">{issue.path}</div>
                <div className="text-red-700">{issue.error}</div>
              </li>
            ))}
          </ul>
        </details>
      )}
      {!active && (
        <div className="text-zinc-500">
          {zh(
            '完整结果见目录内的 JavBoss-转码报告.txt',
            'Full results: JavBoss-转码报告.txt in this directory'
          )}
        </div>
      )}
    </div>
  )
}
