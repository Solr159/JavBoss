import { useEffect, useState } from 'react'
import { useStore } from '@/store'
import {
  fetchTitleTranslationBatch,
  previewTitleTranslationBatch,
  startTitleTranslationBatch,
  stopTitleTranslationBatch,
} from '@/features/jav/api'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

export default function TitleTranslationBatchSettings({ dirty, target = 'title' }) {
  const series = target === 'series'
  const [expanded, setExpanded] = useState(false)
  const [batch, setBatch] = useState(null)
  const [scope, setScope] = useState('missing')
  const [preview, setPreview] = useState(null)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [previewError, setPreviewError] = useState('')
  const [previewRevision, setPreviewRevision] = useState(0)
  const [confirmed, setConfirmed] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const running = Boolean(batch?.running)
  useEffect(() => {
    if (running) setExpanded(true)
  }, [running])
  useEffect(() => {
    const controller = new AbortController()
    let timer
    const poll = async () => {
      try {
        const status = await fetchTitleTranslationBatch({ signal: controller.signal, target })
        if (controller.signal.aborted) return
        setBatch(status)
        if (status.running) timer = setTimeout(poll, 1000)
        else if (running) {
          const state = useStore.getState()
          const reloads = series
            ? [state.loadJavSeries, state.loadJavs, state.loadJavStudios]
            : [state.loadJavs]
          await Promise.all(
            reloads.map((reload) => reload({ force: true, reconcile: true, background: true }))
          )
        }
      } catch (err) {
        if (!controller.signal.aborted) {
          setError(getErrorMessage(err))
          timer = setTimeout(poll, 2000)
        }
      }
    }
    void poll()
    return () => {
      controller.abort()
      clearTimeout(timer)
    }
  }, [running, target, series])
  useEffect(() => {
    if (!expanded || running) return undefined
    const controller = new AbortController()
    setPreviewLoading(true)
    setPreview(null)
    setConfirmed(false)
    setPreviewError('')
    previewTitleTranslationBatch(scope, { signal: controller.signal, target })
      .then((result) => {
        if (!controller.signal.aborted) setPreview(result)
      })
      .catch((err) => {
        if (!controller.signal.aborted) setPreviewError(getErrorMessage(err))
      })
      .finally(() => {
        if (!controller.signal.aborted) setPreviewLoading(false)
      })
    return () => controller.abort()
  }, [expanded, scope, running, previewRevision, target])
  const runAction = async (action) => {
    setBusy(true)
    setError('')
    try {
      await action()
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }
  const buttonClass =
    'shrink-0 rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm disabled:opacity-50'
  return (
    <details
      data-translation-batch={target}
      open={expanded}
      onToggle={(e) => setExpanded(e.currentTarget.open)}
      className="group"
    >
      <summary className="flex cursor-pointer list-none items-center justify-between gap-4 px-5 py-4 [&::-webkit-details-marker]:hidden">
        <span>
          <span className="block text-sm font-medium text-zinc-700">
            {zh('批量翻译', 'Batch translation')}
          </span>
          <span className="mt-1 block text-xs leading-5 text-zinc-500">
            {running
              ? zh(
                  `翻译中 · ${batch.processed}/${batch.total}`,
                  `Translating · ${batch.processed}/${batch.total}`
                )
              : zh(
                  series ? '按需补齐或重新翻译全部系列名称。' : '按需补齐或重新翻译整个影片库。',
                  series
                    ? 'Fill missing series names or retranslate all series.'
                    : 'Fill missing titles or retranslate your library when needed.'
                )}
          </span>
        </span>
        <span
          className="text-zinc-400 transition-transform group-open:rotate-180"
          aria-hidden="true"
        >
          ⌄
        </span>
      </summary>
      <div className="mx-5 mb-5 space-y-4 rounded-xl border border-zinc-200 bg-zinc-50 p-4">
        {dirty && (
          <p className="text-xs text-amber-700">
            {zh(
              '请先保存或取消服务商配置的修改，再开始批量翻译。',
              'Save or cancel your provider changes before starting a batch.'
            )}
          </p>
        )}
        {!running && (
          <fieldset disabled={busy || dirty} className="space-y-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
              <label className="block min-w-0 flex-1 space-y-2">
                <span className="text-xs font-medium text-zinc-600">
                  {zh('翻译范围', 'Translation scope')}
                </span>
                <select
                  className="block w-full rounded-lg border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-800"
                  value={scope}
                  onChange={(e) => {
                    setScope(e.target.value)
                    setPreview(null)
                    setConfirmed(false)
                  }}
                >
                  <option value="missing">
                    {zh(
                      series
                        ? '仅翻译缺失系列名称（保留已有译名）'
                        : '仅翻译缺失标题（保留已有译名）',
                      series
                        ? 'Missing series names only (keep existing translations)'
                        : 'Missing titles only (keep existing translations)'
                    )}
                  </option>
                  <option value="all">
                    {zh(
                      series
                        ? '重新翻译全部系列名称（覆盖已有译名）'
                        : '重新翻译全部标题（覆盖已有译名）',
                      series
                        ? 'All series names (overwrite existing translations)'
                        : 'All titles (overwrite existing translations)'
                    )}
                  </option>
                </select>
              </label>
            </div>
            {previewLoading && (
              <p role="status" className="text-sm text-zinc-500">
                {zh('正在加载任务详情…', 'Loading batch details…')}
              </p>
            )}
            {previewError && (
              <div className="space-y-2">
                <p role="alert" className="break-words text-sm text-red-600">
                  {previewError}
                </p>
                <button
                  type="button"
                  className={buttonClass}
                  onClick={() => setPreviewRevision((revision) => revision + 1)}
                >
                  {zh('重试', 'Retry')}
                </button>
              </div>
            )}
            {!previewLoading && preview?.scope === scope && (
              <div className="space-y-3 rounded-lg border border-zinc-200 bg-white p-3">
                <p role="status" className="text-sm text-zinc-700">
                  {zh(
                    series
                      ? `将翻译 ${preview.total} 个系列，其中 ${preview.existing} 个已有中文名称。`
                      : `将翻译 ${preview.total} 部影片，其中 ${preview.existing} 部已有中文标题。`,
                    `Translate ${preview.total} ${series ? 'series names' : 'titles'}; ${preview.existing} already have translations.`
                  )}
                </p>
                <p className="text-xs text-zinc-500">
                  {zh(
                    '实际处理数量以启动时为准，服务商可能按用量收费。',
                    'The count is checked again when starting; provider usage charges may apply.'
                  )}
                </p>
                {scope === 'all' && (
                  <label className="flex items-start gap-2 text-sm text-amber-800">
                    <input
                      type="checkbox"
                      className="mt-1"
                      checked={confirmed}
                      onChange={(e) => setConfirmed(e.target.checked)}
                    />
                    {zh(
                      series
                        ? '我确认覆盖已有中文系列名称，包括人工校对的译名。'
                        : '我确认覆盖已有中文标题，包括人工校对的译名。',
                      'I confirm overwriting existing Chinese translations, including manual corrections.'
                    )}
                  </label>
                )}
                <button
                  type="button"
                  className={buttonClass}
                  disabled={!preview.total || (scope === 'all' && !confirmed)}
                  onClick={() =>
                    void runAction(async () => {
                      setBatch(await startTitleTranslationBatch(scope, confirmed, target))
                      setPreview(null)
                      setPreviewRevision((revision) => revision + 1)
                      setConfirmed(false)
                    })
                  }
                >
                  {zh('开始翻译', 'Start translation')}
                </button>
              </div>
            )}
          </fieldset>
        )}
        {batch?.total > 0 && (
          <p role="status" className="text-sm text-zinc-600">
            {zh(
              `${batch.running ? (batch.stopping ? '正在停止' : '翻译中') : batch.stopping ? '已停止' : batch.error_zh ? '翻译失败' : '已完成'}：${batch.processed}/${batch.total}，已翻译 ${batch.translated}，跳过 ${batch.skipped}`,
              `${batch.running ? (batch.stopping ? 'Stopping' : 'Translating') : batch.stopping ? 'Stopped' : batch.error_en ? 'Failed' : 'Finished'}: ${batch.processed}/${batch.total}; translated ${batch.translated}, skipped ${batch.skipped}`
            )}
          </p>
        )}
        {running && (
          <button
            type="button"
            className={buttonClass}
            disabled={busy || batch.stopping}
            onClick={() =>
              void runAction(async () => setBatch(await stopTitleTranslationBatch(target)))
            }
          >
            {batch.stopping
              ? zh('当前翻译完成后停止', 'Stopping after the current translation')
              : zh('停止任务', 'Stop batch')}
          </button>
        )}
        {(error || batch?.error_zh) && (
          <p role="alert" className="break-words text-sm text-red-600">
            {error || zh(batch.error_zh, batch.error_en)}
          </p>
        )}
      </div>
    </details>
  )
}
