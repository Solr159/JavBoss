import { useEffect, useState } from 'react'
import { fetchTitleTranslationModels } from '@/features/jav/api'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

export default function JavTitleTranslationSettings({ value, onChange }) {
  const [models, setModels] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [revision, setRevision] = useState(0)
  const apiKey = value.apiKey.trim()

  useEffect(() => {
    setModels([])
    setError('')
    setLoading(false)
    if (!apiKey && !value.hasApiKey) return undefined
    const controller = new AbortController()
    const timer = setTimeout(async () => {
      setLoading(true)
      try {
        const result = await fetchTitleTranslationModels(apiKey || undefined, {
          signal: controller.signal,
        })
        if (!controller.signal.aborted) setModels(result.models)
      } catch (err) {
        if (!controller.signal.aborted) setError(getErrorMessage(err))
      } finally {
        if (!controller.signal.aborted) setLoading(false)
      }
    }, 500)
    return () => {
      clearTimeout(timer)
      controller.abort()
    }
  }, [apiKey, value.hasApiKey, revision])

  useEffect(() => {
    if (models.length && !models.some((model) => model.id === value.model)) {
      onChange({ model: models[0].id })
    }
  }, [models, value.model, onChange])

  const inputClass =
    'w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm focus:border-blue-500 focus:outline-none'
  return (
    <div className="space-y-3 rounded-lg bg-slate-50 p-3 text-sm">
      <label className="block space-y-1.5">
        <span className="font-medium text-slate-700">DeepSeek API Key</span>
        <input
          type="password"
          autoComplete="new-password"
          value={value.apiKey}
          placeholder={
            value.hasApiKey ? zh('已保存，留空沿用', 'Saved; leave blank to keep') : 'sk-…'
          }
          onChange={(event) => onChange({ apiKey: event.target.value, clearApiKey: false })}
          className={inputClass}
        />
      </label>
      {value.hasApiKey && (
        <button
          type="button"
          className="text-xs text-slate-500 underline"
          onClick={() => onChange({ apiKey: '', hasApiKey: false, clearApiKey: true, model: '' })}
        >
          {zh('清除已保存的 API Key', 'Clear saved API key')}
        </button>
      )}
      <div className="flex items-end gap-2">
        <label className="min-w-0 flex-1 space-y-1.5">
          <span className="block font-medium text-slate-700">
            {zh('翻译模型', 'Translation model')}
          </span>
          <select
            className={inputClass}
            value={value.model}
            disabled={loading || models.length === 0}
            onChange={(event) => onChange({ model: event.target.value })}
          >
            <option value="">
              {loading ? zh('同步中…', 'Syncing…') : zh('请选择模型', 'Select a model')}
            </option>
            {value.model && !models.some((model) => model.id === value.model) && (
              <option value={value.model}>{value.model}</option>
            )}
            {models.map((model) => (
              <option key={model.id} value={model.id}>
                {model.name || model.id}
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          disabled={loading || (!apiKey && !value.hasApiKey)}
          className="shrink-0 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm disabled:opacity-50"
          onClick={() => setRevision((current) => current + 1)}
        >
          {zh('刷新模型', 'Refresh models')}
        </button>
      </div>
      <label className="flex items-center gap-2 text-slate-700">
        <input
          type="checkbox"
          name="title-translation-thinking"
          checked={value.thinking}
          onChange={(event) => onChange({ thinking: event.target.checked })}
          className="h-4 w-4 rounded border-slate-300 accent-blue-600"
        />
        {zh('启用推理', 'Enable reasoning')}
      </label>
      {error && (
        <p role="alert" className="text-xs text-red-600">
          {error}
        </p>
      )}
      <label className="block space-y-1.5">
        <span className="font-medium text-slate-700">{zh('翻译提示词', 'Translation prompt')}</span>
        <textarea
          className={inputClass}
          rows={4}
          maxLength={8192}
          value={value.prompt}
          onChange={(event) => onChange({ prompt: event.target.value })}
        />
      </label>
      <p className="text-xs leading-5 text-slate-500">
        {zh(
          '配置后自动翻译当前页缺少中文标题的影片和系列名称。译文单独保存，开启时显示中文标题，关闭时显示原标题。',
          'Once configured, missing Chinese titles and series names on the current page are translated and saved separately. Enable to show Chinese titles; disable to show original titles.'
        )}
      </p>
    </div>
  )
}
