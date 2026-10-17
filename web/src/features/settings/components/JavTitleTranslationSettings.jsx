import RefreshOutlinedIcon from '@mui/icons-material/RefreshOutlined'
import Popper from '@mui/material/Popper'
import useAutocomplete from '@mui/material/useAutocomplete'
import { useEffect, useState } from 'react'
import { fetchTitleTranslationModels } from '@/features/jav/api'
import { translationPresets, protocolNames } from '@/features/settings/translationPresets'
import { translationProfile } from '@/features/settings/titleTranslation'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

const inputClass =
  'w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm focus:border-blue-500 focus:outline-none'
const emptyCaps = { reasoning: 'unknown', efforts: [] }

export default function JavTitleTranslationSettings({ value, onChange, disabled = false }) {
  const [models, setModels] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [caps, setCaps] = useState(emptyCaps)
  const [checkingCaps, setCheckingCaps] = useState(true)
  const [revision, setRevision] = useState(0)
  const [modelFilter, setModelFilter] = useState('')
  const { provider: providerID, baseUrl, protocol, model, reasoning } = value
  const provider = value.catalog.find((item) => item.id === providerID)
  const custom = providerID === 'custom'
  const apiKey = value.apiKey.trim()

  useEffect(() => {
    const controller = new AbortController()
    setCaps(emptyCaps)
    setCheckingCaps(true)
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
      } finally {
        if (!controller.signal.aborted) setCheckingCaps(false)
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
    if (disabled || (!apiKey && !value.hasApiKey && !custom) || (custom && !baseUrl))
      return undefined
    setLoading(true)
    const controller = new AbortController()
    const timer = setTimeout(async () => {
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
  }, [
    apiKey,
    value.hasApiKey,
    value.clearApiKey,
    providerID,
    baseUrl,
    protocol,
    custom,
    revision,
    disabled,
  ])

  const thinking = caps.reasoning === 'always' || (caps.reasoning === 'toggle' && value.thinking)
  const temperatureSupported = caps.temperature && (!thinking || caps.temperature_with_thinking)
  const field = (label, content) => (
    <label className="block space-y-1.5">
      <span className="font-medium text-slate-700">{label}</span>
      {content}
    </label>
  )
  const changeModel = (model) => onChange({ model, effort: '', temperature: '' })
  const {
    getRootProps,
    getInputLabelProps,
    getInputProps,
    getPopupIndicatorProps,
    getListboxProps,
    getOptionProps,
    groupedOptions,
    popupOpen,
    anchorEl,
    setAnchorEl,
  } = useAutocomplete({
    id: 'title-translation-model',
    disabled,
    options: models.map((model) => model.id),
    value: value.model || null,
    inputValue: value.model,
    freeSolo: true,
    openOnFocus: true,
    filterOptions: (options) => {
      const query = modelFilter.trim().toLowerCase()
      return options.filter((id) =>
        `${id} ${models.find((model) => model.id === id)?.name || ''}`.toLowerCase().includes(query)
      )
    },
    onClose: () => setModelFilter(''),
    onInputChange: (_, model, reason) => {
      if (reason === 'input' || reason === 'clear') {
        setModelFilter(model)
        changeModel(model)
      }
    },
    onChange: (_, model) => changeModel(model || ''),
  })
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

  const applyPreset = (protocol) => {
    if (!protocol) return
    onChange({
      protocol,
      request: JSON.stringify(translationPresets[protocol], null, 2),
      reasoning: 'none',
      thinking: false,
      effort: '',
      fastParams: '{}',
      thinkingParams: '{}',
    })
  }
  const effortNames = {
    low: zh('低', 'Low'),
    medium: zh('中', 'Medium'),
    high: zh('高', 'High'),
    xhigh: zh('很高', 'Very high'),
    max: zh('最高', 'Maximum'),
    minimal: zh('最低', 'Minimal'),
  }
  const endpointField = field(
    zh('API 地址', 'API base URL'),
    <input
      className={inputClass}
      value={value.baseUrl}
      placeholder={provider?.base_url || 'https://example.com/v1'}
      onChange={(e) => onChange({ baseUrl: e.target.value })}
    />
  )
  return (
    <div className="space-y-4 text-sm">
      <div className="grid gap-4 sm:grid-cols-2">
        {field(
          zh('服务商', 'Provider'),
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
        <div className="flex min-w-0 items-end gap-2">
          <div {...getRootProps()} className="min-w-0 flex-1 space-y-1.5">
            <label {...getInputLabelProps()} className="block font-medium text-slate-700">
              {zh('模型', 'Model')}
            </label>
            <div ref={setAnchorEl} className="relative">
              <input
                {...getInputProps()}
                className={inputClass + ' pr-9'}
                placeholder={zh(
                  '选择、搜索或直接输入模型名称',
                  'Choose, search or enter a model ID'
                )}
              />
              <button
                {...getPopupIndicatorProps()}
                type="button"
                aria-label={zh('选择模型', 'Choose model')}
                className="absolute inset-y-0 right-0 w-9 text-slate-500"
              >
                <span aria-hidden="true">{popupOpen ? '▴' : '▾'}</span>
              </button>
            </div>
            <Popper
              open={popupOpen && Boolean(anchorEl)}
              anchorEl={anchorEl}
              placement="bottom-start"
              style={{ width: anchorEl?.clientWidth, zIndex: 1500 }}
            >
              <div className="mt-1 overflow-hidden rounded-lg border border-slate-200 bg-white text-sm shadow-lg">
                {groupedOptions.length ? (
                  <ul {...getListboxProps()} className="max-h-60 overflow-auto py-1">
                    {groupedOptions.map((option, index) => {
                      const { key, ...props } = getOptionProps({ option, index })
                      return (
                        <li
                          key={key}
                          {...props}
                          className="cursor-pointer break-all px-3 py-2 aria-selected:bg-blue-50 [&.Mui-focused]:bg-slate-100"
                        >
                          {option}
                        </li>
                      )
                    })}
                  </ul>
                ) : (
                  <p role="status" className="px-3 py-2 text-slate-500">
                    {loading
                      ? zh('正在加载模型列表…', 'Loading models…')
                      : !apiKey && !value.hasApiKey && !custom
                        ? zh(
                            '请先填写 API Key，也可直接输入模型名称。',
                            'Enter an API key or type a model ID.'
                          )
                        : zh(
                            '暂无匹配模型，可直接输入模型名称。',
                            'No matching models. You can type a model ID.'
                          )}
                  </p>
                )}
              </div>
            </Popper>
          </div>
          <button
            type="button"
            disabled={loading || (!apiKey && !value.hasApiKey && !custom)}
            aria-label={zh('刷新模型列表', 'Refresh model list')}
            title={zh('刷新模型列表', 'Refresh model list')}
            className="flex h-[38px] w-[38px] shrink-0 items-center justify-center rounded-lg border border-slate-200 text-slate-500 hover:bg-slate-50 disabled:opacity-50"
            onClick={() => setRevision((current) => current + 1)}
          >
            <RefreshOutlinedIcon className={loading ? 'animate-spin' : ''} sx={{ fontSize: 18 }} />
          </button>
        </div>
      </div>
      <div className="relative">
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
                  : zh('填写服务商提供的密钥', 'Enter your provider API key')
            }
            onChange={(e) => onChange({ apiKey: e.target.value, clearApiKey: false })}
          />
        )}
        {value.hasApiKey && (
          <button
            type="button"
            aria-label={zh('清除密钥', 'Clear key')}
            className="absolute right-0 top-0 text-xs text-zinc-500 hover:text-red-600"
            onClick={() => onChange({ apiKey: '', hasApiKey: false, clearApiKey: true })}
          >
            {zh('清除密钥', 'Clear key')}
          </button>
        )}
      </div>
      {custom && endpointField}
      {error && (
        <p role="alert" className="break-words text-xs text-amber-700">
          {error}{' '}
          {zh(
            '也可以直接输入模型名称，再测试翻译。',
            'You can also enter a model ID and test translation.'
          )}
        </p>
      )}
      {!checkingCaps && caps.reasoning === 'unknown' && value.model && (
        <p className="text-xs text-amber-700">
          {zh(
            '尚未识别此模型的能力。可使用自定义服务商的接口预设，再测试翻译。',
            'Model capabilities are unknown. Try a custom provider preset, then test translation.'
          )}
        </p>
      )}
      {custom && (
        <div className="space-y-2 rounded-lg bg-slate-50 p-3">
          {field(
            zh('应用接口预设', 'Apply an API preset'),
            <select className={inputClass} value="" onChange={(e) => applyPreset(e.target.value)}>
              <option value="">
                {zh('选择预设以替换当前请求配置', 'Choose a preset to replace request settings')}
              </option>
              {Object.entries(protocolNames).map(([key, name]) => (
                <option key={key} value={key}>
                  {name}
                </option>
              ))}
            </select>
          )}
          <p className="text-xs text-slate-500">
            {zh(
              '新配置默认使用 OpenAI Chat Completions。预设不发送模型专用推理参数；特殊接口可在下方高级设置中调整。',
              'New configurations use OpenAI Chat Completions. Presets omit model-specific reasoning parameters; adjust special APIs in advanced settings below.'
            )}
          </p>
        </div>
      )}
      <details data-translation-advanced className="group border-t border-zinc-100 pt-4">
        <summary className="flex cursor-pointer list-none items-center justify-between gap-3 font-medium text-zinc-700 [&::-webkit-details-marker]:hidden">
          <span>{zh('高级设置', 'Advanced settings')}</span>
          <span
            className="text-zinc-400 transition-transform group-open:rotate-180"
            aria-hidden="true"
          >
            ⌄
          </span>
        </summary>
        <div className="mt-4 space-y-4 rounded-xl bg-zinc-50 p-4">
          {!custom && (
            <div className="space-y-2">
              <label className="flex items-center gap-2 text-slate-600">
                <input
                  type="checkbox"
                  checked={value.customAddress}
                  onChange={(e) =>
                    onChange({
                      customAddress: e.target.checked,
                      ...(!e.target.checked ? { baseUrl: provider?.base_url || '' } : {}),
                    })
                  }
                />
                {zh('使用自定义地址', 'Use a custom address')}
              </label>
              {value.customAddress && endpointField}
            </div>
          )}
          {custom &&
            field(
              zh('模型推理能力', 'Model reasoning capability'),
              <select
                className={inputClass}
                value={value.reasoning}
                onChange={(e) => onChange({ reasoning: e.target.value })}
              >
                <option value="none">{zh('不设置推理模式', 'Do not configure reasoning')}</option>
                <option value="toggle">{zh('支持开启和关闭', 'Can enable or disable')}</option>
                <option value="always">{zh('始终开启', 'Always on')}</option>
              </select>
            )}
          {caps.reasoning === 'toggle' && (
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                name="title-translation-thinking"
                checked={thinking}
                onChange={(e) => onChange({ thinking: e.target.checked })}
              />
              {zh(
                '启用推理（可能增加等待时间和费用）',
                'Enable reasoning (may increase time and cost)'
              )}
            </label>
          )}
          {caps.reasoning === 'always' && (
            <p className="text-xs text-slate-500">
              {zh('此模型始终开启推理。', 'This model always uses reasoning.')}
            </p>
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
                    {effortNames[effort] || effort}
                  </option>
                ))}
              </select>
            )}
          {temperatureSupported &&
            field(
              zh('温度', 'Temperature'),
              <input
                type="number"
                min="0"
                max={caps.temperature_max}
                step="0.1"
                className={inputClass}
                value={value.temperature}
                placeholder={zh('留空使用服务商默认值', 'Leave blank for provider default')}
                onChange={(e) => onChange({ temperature: e.target.value })}
              />
            )}
          {custom && (
            <details className="rounded-lg bg-slate-50 p-3">
              <summary className="cursor-pointer font-medium">
                {zh('高级自定义接口', 'Advanced custom API')}
              </summary>
              <div className="mt-3 space-y-3">
                {field(
                  zh('模型列表接口协议', 'Model discovery protocol'),
                  <select
                    className={inputClass}
                    value={value.protocol}
                    onChange={(e) => onChange({ protocol: e.target.value })}
                  >
                    {Object.entries(protocolNames).map(([key, name]) => (
                      <option key={key} value={key}>
                        {name}
                      </option>
                    ))}
                  </select>
                )}
                {value.reasoning !== 'none' &&
                  field(
                    zh('自定义推理强度值', 'Custom reasoning effort'),
                    <input
                      className={inputClass}
                      value={value.effort}
                      onChange={(e) => onChange({ effort: e.target.value })}
                      placeholder="high"
                    />
                  )}
                {[
                  [zh('完整请求模板（JSON）', 'Full request template (JSON)'), 'request'],
                  [
                    zh(
                      '关闭推理时的请求覆盖（JSON）',
                      'Request overrides without reasoning (JSON)'
                    ),
                    'fastParams',
                  ],
                  [
                    zh('开启推理时的请求覆盖（JSON）', 'Request overrides with reasoning (JSON)'),
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
                    '模板字段：url、method、headers、body、response_path（JSON Pointer）。占位符：{{base_url}}、{{api_key}}、{{model}}、{{prompt}}、{{title}}、{{temperature}}、{{thinking}}、{{effort}}。密钥请使用占位符。请求覆盖中的 null 删除字段。可切换推理时需分别填写两种模式的覆盖参数。',
                    'Template fields: url, method, headers, body, response_path (JSON Pointer). Placeholders: {{base_url}}, {{api_key}}, {{model}}, {{prompt}}, {{title}}, {{temperature}}, {{thinking}}, {{effort}}. Use placeholders for keys. null deletes a field in overrides. Toggleable reasoning requires overrides for both modes.'
                  )}
                </p>
                <p className="text-xs leading-5 text-slate-500">
                  {zh(
                    '标准响应会自动检查完整性。其他 JSON 格式需填写 completion_path 和 completion_value，例如 /done 和 true。',
                    'Standard responses are checked for completion. Other JSON formats require completion_path and completion_value, such as /done and true.'
                  )}
                </p>
              </div>
            </details>
          )}
          {field(
            zh('翻译提示词', 'Translation prompt'),
            <textarea
              className={inputClass}
              rows={5}
              maxLength={8192}
              value={value.prompt}
              onChange={(e) => onChange({ prompt: e.target.value })}
            />
          )}
          <button
            type="button"
            className="text-xs text-blue-600 underline"
            onClick={() => onChange({ prompt: value.defaultPrompt })}
          >
            {zh('恢复默认提示词', 'Restore default prompt')}
          </button>
        </div>
      </details>
      {paramsError && (
        <p role="alert" className="text-xs text-red-600">
          {paramsError}
        </p>
      )}
    </div>
  )
}
