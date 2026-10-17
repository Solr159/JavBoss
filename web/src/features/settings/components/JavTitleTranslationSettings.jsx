import { useEffect, useState } from 'react'
import { fetchTitleTranslationModels } from '@/features/jav/api'
import { translationProfile } from '@/features/settings/titleTranslation'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

const inputClass =
  'w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm focus:border-blue-500 focus:outline-none'
const emptyCaps = { reasoning: 'unknown', efforts: [] }

export default function JavTitleTranslationSettings({ value, onChange }) {
  const [models, setModels] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [caps, setCaps] = useState(emptyCaps)
  const [revision, setRevision] = useState(0)
  const { provider: providerID, baseUrl, protocol, model, reasoning } = value
  const provider = value.catalog.find((item) => item.id === providerID)
  const custom = providerID === 'custom'
  const apiKey = value.apiKey.trim()

  useEffect(() => {
    const controller = new AbortController()
    setCaps(emptyCaps)
    const timer = setTimeout(async () => {
      try {
        const result = await fetchTitleTranslationModels({
          provider: providerID,
          profile: {
            base_url: baseUrl,
            protocol,
            model,
            reasoning: providerID === 'custom' ? reasoning : undefined,
          },
          discover: false,
          signal: controller.signal,
        })
        if (!controller.signal.aborted) setCaps(result.capabilities || emptyCaps)
      } catch {
        // Unknown capability stays explicit; discovery errors are shown separately.
      }
    }, 250)
    return () => {
      clearTimeout(timer)
      controller.abort()
    }
  }, [providerID, baseUrl, protocol, model, reasoning])

  useEffect(() => {
    setModels([])
    setError('')
    setLoading(false)
    if ((!apiKey && !value.hasApiKey && !custom) || (custom && !baseUrl)) return undefined
    const controller = new AbortController()
    const timer = setTimeout(async () => {
      setLoading(true)
      try {
        const result = await fetchTitleTranslationModels({
          apiKey: value.clearApiKey ? '' : apiKey || undefined,
          provider: providerID,
          profile: { base_url: baseUrl, protocol },
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
  }, [apiKey, value.hasApiKey, value.clearApiKey, providerID, baseUrl, protocol, custom, revision])

  const thinking = caps.reasoning === 'always' || (caps.reasoning === 'toggle' && value.thinking)
  const temperatureSupported = caps.temperature && (!thinking || caps.temperature_with_thinking)
  const field = (label, content) => (
    <label className="block space-y-1.5">
      <span className="font-medium text-slate-700">{label}</span>
      {content}
    </label>
  )
  const changeModel = (model) => onChange({ model, effort: '', temperature: '' })
  let paramsError = ''
  if (custom) {
    try {
      translationProfile(value)
    } catch {
      paramsError = zh(
        '自定义参数必须填写有效的 JSON 对象。',
        'Enter valid JSON objects for custom parameters.'
      )
    }
  }

  return (
    <div className="space-y-3 rounded-lg bg-slate-50 p-3 text-sm">
      {field(
        zh('模型提供商', 'Model provider'),
        <select
          className={inputClass}
          value={value.provider}
          onChange={(e) => onChange({ provider: e.target.value })}
        >
          {value.catalog.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name}
            </option>
          ))}
        </select>
      )}
      {field(
        zh('API 地址', 'API base URL'),
        <input
          className={inputClass}
          value={value.baseUrl}
          placeholder={provider?.base_url || 'https://example.com/v1'}
          onChange={(e) => onChange({ baseUrl: e.target.value })}
        />
      )}
      {custom &&
        field(
          zh('模型列表接口协议（可选）', 'Model discovery protocol (optional)'),
          <select
            className={inputClass}
            value={value.protocol}
            onChange={(e) => onChange({ protocol: e.target.value })}
          >
            <option value="chat">OpenAI Chat Completions</option>
            <option value="responses">OpenAI Responses</option>
            <option value="anthropic">Anthropic Messages</option>
            <option value="gemini">Gemini GenerateContent</option>
          </select>
        )}
      {field(
        'API Key',
        <input
          type="password"
          autoComplete="new-password"
          className={inputClass}
          value={value.apiKey}
          placeholder={
            value.hasApiKey
              ? zh('已保存，留空沿用', 'Saved; leave blank to keep')
              : custom
                ? zh('本地服务可留空', 'Optional for local services')
                : 'sk-…'
          }
          onChange={(e) => onChange({ apiKey: e.target.value, clearApiKey: false })}
        />
      )}
      {value.hasApiKey && (
        <button
          type="button"
          className="text-xs text-slate-500 underline"
          onClick={() => onChange({ apiKey: '', hasApiKey: false, clearApiKey: true })}
        >
          {zh('清除已保存的 API Key', 'Clear saved API key')}
        </button>
      )}
      <div className="flex items-end gap-2">
        <div className="min-w-0 flex-1">
          {field(
            zh('模型名称', 'Model name'),
            value.manualModel ? (
              <input
                className={inputClass}
                value={value.model}
                placeholder={zh('输入模型名称', 'Enter a model ID')}
                onChange={(e) => changeModel(e.target.value)}
              />
            ) : (
              <select
                aria-label={zh('服务商模型列表', 'Provider models')}
                className={inputClass}
                value={value.model}
                onChange={(e) => changeModel(e.target.value)}
              >
                <option value="">
                  {loading ? zh('同步中…', 'Syncing…') : zh('请选择模型', 'Select a model')}
                </option>
                {value.model && !models.some((model) => model.id === value.model) && (
                  <option value={value.model}>{value.model}</option>
                )}
                {models.map((model) => (
                  <option key={model.id} value={model.id}>
                    {model.name === model.id ? model.id : model.name + ' (' + model.id + ')'}
                  </option>
                ))}
              </select>
            )
          )}
        </div>
        <button
          type="button"
          disabled={loading || (!apiKey && !value.hasApiKey && !custom)}
          className="shrink-0 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm disabled:opacity-50"
          onClick={() => setRevision((current) => current + 1)}
        >
          {loading ? zh('同步中…', 'Syncing…') : zh('同步模型', 'Sync models')}
        </button>
      </div>
      <label className="flex items-center gap-2 text-slate-700">
        <input
          type="checkbox"
          name="title-translation-manual-model"
          checked={value.manualModel}
          onChange={(e) => onChange({ manualModel: e.target.checked })}
          className="h-4 w-4 rounded border-slate-300 accent-blue-600"
        />
        {zh('允许手动填写模型名称', 'Allow manual model name')}
      </label>
      {error && (
        <p role="alert" className="text-xs text-amber-700">
          {error}{' '}
          {zh(
            '模型列表不可用时，可勾选“允许手动填写模型名称”。',
            'If discovery is unavailable, enable manual model entry.'
          )}
        </p>
      )}
      {custom &&
        field(
          zh('模型推理能力', 'Model reasoning capability'),
          <select
            className={inputClass}
            value={value.reasoning}
            onChange={(e) => onChange({ reasoning: e.target.value })}
          >
            <option value="none">{zh('不支持推理', 'No reasoning')}</option>
            <option value="toggle">{zh('可开启或关闭', 'Can enable or disable')}</option>
            <option value="always">{zh('强制推理', 'Always reasons')}</option>
          </select>
        )}
      <label className="flex items-center gap-2 text-slate-700">
        <input
          type="checkbox"
          name="title-translation-thinking"
          checked={thinking}
          disabled={caps.reasoning !== 'toggle'}
          onChange={(e) => onChange({ thinking: e.target.checked })}
          className="h-4 w-4 rounded border-slate-300 accent-blue-600"
        />
        {zh('启用推理', 'Enable reasoning')}
      </label>
      {caps.reasoning === 'always' && (
        <p className="text-xs text-slate-500">
          {zh(
            '此模型无法关闭推理，JAV 编辑中的快速重新翻译将不可用。',
            'This model always reasons; quick retranslation will be unavailable in the JAV editor.'
          )}
        </p>
      )}
      {caps.reasoning === 'none' && (
        <p className="text-xs text-slate-500">
          {zh(
            '此模型不支持推理，推理翻译会使用快速翻译。',
            'This model does not reason; reasoning translation falls back to quick translation.'
          )}
        </p>
      )}
      {caps.reasoning === 'unknown' && value.model && (
        <p className="text-xs text-amber-700">
          {zh(
            '尚未识别此模型的参数能力，请改用自定义服务商配置。',
            'Unrecognized model capabilities; configure it using the custom provider.'
          )}
        </p>
      )}
      {temperatureSupported &&
        field(
          zh('温度', 'Temperature'),
          <input
            type="number"
            min="0"
            max={caps.temperature_max}
            step="0.1"
            value={value.temperature}
            placeholder={zh('留空使用服务商默认值', 'Leave blank for provider default')}
            className={inputClass}
            onChange={(e) => onChange({ temperature: e.target.value })}
          />
        )}
      {thinking &&
        caps.efforts?.length > 0 &&
        field(
          zh('推理强度', 'Reasoning effort'),
          <select
            className={inputClass}
            value={value.effort}
            onChange={(e) => onChange({ effort: e.target.value })}
          >
            <option value="">{zh('默认', 'Default')}</option>
            {caps.efforts.map((effort) => (
              <option key={effort} value={effort}>
                {effort}
              </option>
            ))}
          </select>
        )}
      {custom && (
        <>
          {value.reasoning !== 'none' &&
            field(
              zh('自定义推理强度值', 'Custom reasoning effort value'),
              <input
                className={inputClass}
                value={value.effort}
                onChange={(e) => onChange({ effort: e.target.value })}
                placeholder="high"
              />
            )}
          {[
            [zh('完整请求模板（JSON）', 'Full request template (JSON)'), 'request'],
            [zh('快速翻译请求覆盖（JSON）', 'Quick request overrides (JSON)'), 'fastParams'],
            [
              zh('推理翻译请求覆盖（JSON）', 'Reasoning request overrides (JSON)'),
              'thinkingParams',
            ],
          ].map(([label, key]) => (
            <div key={key}>
              {field(
                label,
                <textarea
                  className={inputClass + ' font-mono'}
                  rows={key === 'request' ? 12 : 3}
                  value={value[key]}
                  onChange={(e) => onChange({ [key]: e.target.value })}
                />
              )}
            </div>
          ))}
          <p className="text-xs leading-5 text-slate-500">
            {zh(
              '模板包含 url、method（GET/POST）、headers、body 和 response_path（JSON Pointer，如 /choices/0/message/content）。支持 JSON 响应。占位符：{{base_url}}、{{api_key}}、{{model}}、{{prompt}}、{{title}}、{{temperature}}、{{thinking}}、{{effort}}。整值占位符保留数值或布尔类型；密钥请使用占位符。模式覆盖可替换上述任意字段，null 删除字段。可切换推理时，必须分别配置快速和推理请求覆盖，例如快速覆盖 {"body":{"thinking":{"type":"disabled"}}}。',
              'Template fields: url, method (GET/POST), headers, body, response_path (JSON Pointer, e.g. /choices/0/message/content). JSON responses only. Placeholders: {{base_url}}, {{api_key}}, {{model}}, {{prompt}}, {{title}}, {{temperature}}, {{thinking}}, {{effort}}. Exact placeholders preserve number/boolean types. Use the API key placeholder instead of a literal secret. Mode overrides may replace any field; null removes it. Toggleable models require both quick and reasoning overrides, e.g. {"body":{"thinking":{"type":"disabled"}}} for quick mode.'
            )}
          </p>
          <p className="text-xs leading-5 text-slate-500">
            {zh(
              '常见模型响应会自动检查是否完整。其他 JSON 格式需指定 completion_path 和 completion_value，例如 "completion_path":"/done","completion_value":true；状态缺失或不匹配时不会保存译文。',
              'Standard model responses are checked for completion automatically. Other JSON formats require completion_path and completion_value, e.g. "completion_path":"/done","completion_value":true. Missing or mismatched status prevents saving.'
            )}
          </p>
          {paramsError && (
            <p role="alert" className="text-xs text-red-600">
              {paramsError}
            </p>
          )}
        </>
      )}
      {field(
        zh('翻译提示词', 'Translation prompt'),
        <textarea
          className={inputClass}
          rows={4}
          maxLength={8192}
          value={value.prompt}
          onChange={(e) => onChange({ prompt: e.target.value })}
        />
      )}
    </div>
  )
}
