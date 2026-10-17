import { useEffect, useState } from 'react'
import { Switch } from '@mui/material'
import { useStore } from '@/store'
import {
  createTitleTranslationDraft,
  changeTranslationDraft,
  translationProfile,
} from '@/features/settings/titleTranslation'
import {
  saveTitleTranslationSettings,
  saveTitleTranslationEnabled,
} from '@/features/settings/actions'
import { testTitleTranslation } from '@/features/jav/api'
import JavTitleTranslationSettings from '@/features/settings/components/JavTitleTranslationSettings'
import TitleTranslationBatchSettings from '@/features/settings/components/TitleTranslationBatchSettings'
import { configFlag } from '@/utils/config'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

export default function TitleTranslationToolSettings() {
  const config = useStore((state) => state.config)
  const [draft, setDraft] = useState(() => createTitleTranslationDraft(config))
  const [editing, setEditing] = useState(false)
  const [toggling, setToggling] = useState(false)
  const [autoError, setAutoError] = useState('')
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [error, setError] = useState('')
  const [testError, setTestError] = useState('')
  const [result, setResult] = useState('')
  const [sample, setSample] = useState(
    '彼女へのプロポーズ前夜、「明日からはW不倫になっちゃうね…」と妖艶に笑う幼馴染と中出しセックスに溺れた僕。 明里つむぎ'
  )
  const [saved, setSaved] = useState(false)
  const [dirty, setDirty] = useState(false)

  useEffect(() => {
    if (!editing) {
      setDraft(createTitleTranslationDraft(config))
      setDirty(false)
    }
  }, [config, editing])

  const onChange = (patch) => {
    if (!editing || saving || testing || toggling) return
    setDraft((current) => changeTranslationDraft(current, patch))
    setError('')
    setTestError('')
    setResult('')
    setSaved(false)
    setDirty(true)
  }
  const handleSave = async (event) => {
    event.preventDefault()
    if (!editing || saving || testing || toggling) return
    setSaving(true)
    setError('')
    setSaved(false)
    try {
      await saveTitleTranslationSettings(draft)
      setSaved(true)
      setEditing(false)
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }
  const handleTest = async () => {
    if (!editing || saving || testing || toggling || !sample.trim()) return
    setTesting(true)
    setResult('')
    setTestError('')
    try {
      const response = await testTitleTranslation({
        provider: draft.provider,
        profile: translationProfile(draft),
        prompt: draft.prompt,
        title: sample.trim(),
      })
      setResult(response.zh_title)
    } catch (err) {
      setTestError(getErrorMessage(err))
    } finally {
      setTesting(false)
    }
  }

  const handleAutoChange = async (enabled, target = 'title') => {
    if (toggling || saving || testing) return
    setToggling(true)
    setAutoError('')
    try {
      await saveTitleTranslationEnabled(enabled, target)
    } catch (err) {
      setAutoError(getErrorMessage(err))
    } finally {
      setToggling(false)
    }
  }
  const resetEditor = () => {
    setDraft(createTitleTranslationDraft(useStore.getState().config))
    setDirty(false)
    setError('')
    setTestError('')
    setResult('')
    setSaved(false)
  }
  const busy = saving || testing || toggling

  if (!draft.catalog.length) {
    return <p role="status">{zh('正在加载翻译配置…', 'Loading translation settings…')}</p>
  }

  return (
    <div className="mx-auto w-full max-w-4xl space-y-4">
      <p
        data-translation-display-hint
        className="rounded-xl bg-blue-50 px-4 py-3 text-xs leading-5 text-blue-800"
      >
        {zh(
          '要展示翻译生成的中文标题和系列名称，请在「显示设置 → JAV → 卡片设置」中确保「优先展示中文标题」已开启（默认开启）。',
          'To display translated Chinese titles and series names, enable “Prefer Chinese titles” in Display settings → JAV → Card settings (enabled by default).'
        )}
      </p>

      <section
        aria-labelledby="translation-behavior-heading"
        className="overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-sm"
      >
        <header className="border-b border-zinc-100 px-5 py-4">
          <h4 id="translation-behavior-heading" className="text-sm font-semibold text-zinc-900">
            {zh('自动翻译', 'Automatic translation')}
          </h4>
          <p className="mt-1 text-xs leading-5 text-zinc-500">
            {zh(
              '自动补齐当前页缺失的中文译名。',
              'Automatically fill missing Chinese translations on the current page.'
            )}
          </p>
        </header>
        <div className="divide-y divide-zinc-100 px-5">
          {[
            {
              target: 'title',
              id: 'translation-auto-enabled',
              label: zh('自动翻译 JAV 标题', 'Automatically translate JAV titles'),
              enabled: config?.jav_title_translation_enabled,
            },
            {
              target: 'series',
              id: 'series-translation-auto-enabled',
              label: zh('自动翻译 JAV 系列', 'Automatically translate JAV series'),
              enabled: config?.jav_series_translation_enabled,
            },
          ].map(({ target, id, label, enabled }) => (
            <div key={target} className="flex items-center justify-between gap-6 py-3">
              <label htmlFor={id} className="cursor-pointer text-sm text-zinc-700">
                {label}
              </label>
              <Switch
                size="small"
                className="shrink-0"
                checked={configFlag(enabled)}
                disabled={busy}
                onChange={(_, checked) => void handleAutoChange(checked, target)}
                slotProps={{ input: { id, 'aria-label': label } }}
              />
            </div>
          ))}
        </div>
        {autoError && (
          <p role="alert" className="px-5 pb-4 text-sm text-red-600">
            {autoError}
          </p>
        )}
      </section>
      <section
        aria-label={zh('批量翻译', 'Batch translation')}
        className="overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-sm"
      >
        <div className="divide-y divide-zinc-100">
          <TitleTranslationBatchSettings dirty={dirty} />
          <TitleTranslationBatchSettings dirty={dirty} target="series" />
        </div>
      </section>
      <form onSubmit={handleSave}>
        <section
          aria-labelledby="translation-service-heading"
          className="overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-sm"
        >
          <header className="flex flex-wrap items-start justify-between gap-4 border-b border-zinc-100 px-5 py-4">
            <div className="min-w-0 flex-1">
              <h4 id="translation-service-heading" className="text-sm font-semibold text-zinc-900">
                {zh('服务商配置', 'Provider configuration')}
              </h4>
              <p className="mt-1 text-xs leading-5 text-zinc-500">
                {zh(
                  '配置服务商、密钥和模型，可先测试再保存。',
                  'Choose a provider and model, enter your key, then test before saving.'
                )}
              </p>
              {editing && (error || dirty) && (
                <p
                  role={error ? 'alert' : 'status'}
                  className={`mt-2 text-xs ${error ? 'text-red-600' : 'text-zinc-500'}`}
                >
                  {error || zh('有未保存的修改', 'Unsaved changes')}
                </p>
              )}
            </div>
            {editing ? (
              <div className="ml-auto flex shrink-0 items-center gap-2">
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => {
                    resetEditor()
                    setEditing(false)
                  }}
                  className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm disabled:opacity-50"
                >
                  {zh('取消', 'Cancel')}
                </button>
                <button
                  disabled={busy}
                  type="submit"
                  className="ml-auto shrink-0 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
                >
                  {saving ? zh('保存中…', 'Saving…') : zh('保存配置', 'Save configuration')}
                </button>
              </div>
            ) : (
              <button
                type="button"
                disabled={busy}
                onClick={() => {
                  resetEditor()
                  setEditing(true)
                }}
                className="ml-auto shrink-0 rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm disabled:opacity-50"
              >
                {zh('编辑', 'Edit')}
              </button>
            )}
          </header>
          <fieldset
            disabled={!editing || busy}
            className="min-w-0 space-y-5 p-5 [&_:disabled]:cursor-not-allowed [&_:disabled]:text-zinc-400 [&_button:disabled]:opacity-50 [&_input:disabled]:bg-zinc-100 [&_select:disabled]:bg-zinc-100 [&_textarea:disabled]:bg-zinc-100"
          >
            <JavTitleTranslationSettings
              value={draft}
              onChange={onChange}
              disabled={!editing || busy}
              readOnly={!editing}
              savedCapabilities={config?.jav_title_translation_capabilities}
            />
            <details data-translation-test className="group border-t border-zinc-100 pt-4">
              <summary className="flex cursor-pointer list-none items-center justify-between gap-3 text-sm font-medium text-zinc-700 [&::-webkit-details-marker]:hidden">
                <span>{zh('测试翻译', 'Test translation')}</span>
                <span
                  className="text-zinc-400 transition-transform group-open:rotate-180"
                  aria-hidden="true"
                >
                  ⌄
                </span>
              </summary>
              <div className="mt-4 space-y-3 rounded-xl bg-zinc-50 p-4">
                <label className="block space-y-2 text-sm">
                  <span className="font-medium text-zinc-700">
                    {zh('测试标题', 'Sample title')}
                  </span>
                  <input
                    className="w-full rounded-lg border border-zinc-200 bg-white px-3 py-2"
                    value={sample}
                    maxLength={8192}
                    onChange={(e) => {
                      setSample(e.target.value)
                      setResult('')
                      setTestError('')
                    }}
                  />
                </label>
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <p className="max-w-md text-xs leading-5 text-zinc-500">
                    {zh(
                      '使用当前配置试译，不写入影片库；按服务商规则计费。',
                      'Test the current settings without updating the library. Provider charges apply.'
                    )}
                  </p>
                  <button
                    type="button"
                    disabled={!sample.trim() || !draft.model.trim()}
                    onClick={() => void handleTest()}
                    className="shrink-0 rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm disabled:opacity-50"
                  >
                    {testing ? zh('测试中…', 'Testing…') : zh('测试翻译', 'Test translation')}
                  </button>
                </div>
                {result && (
                  <p
                    role="status"
                    className="break-words rounded-lg bg-emerald-50 p-3 text-sm text-emerald-700"
                  >
                    {result}
                  </p>
                )}
                {testError && (
                  <p role="alert" className="break-words text-sm text-red-600">
                    {testError}
                  </p>
                )}
              </div>
            </details>
          </fieldset>
          {!editing && saved && (
            <p role="status" className="px-5 pb-4 text-xs text-emerald-700">
              {zh('已保存', 'Saved')}
            </p>
          )}
        </section>
      </form>
    </div>
  )
}
