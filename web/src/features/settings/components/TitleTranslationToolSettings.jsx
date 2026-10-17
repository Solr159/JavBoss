import { useEffect, useState } from 'react'
import { Switch } from '@mui/material'
import { useStore } from '@/store'
import {
  createTitleTranslationDraft,
  changeTranslationDraft,
  translationProfile,
} from '@/features/settings/titleTranslation'
import { saveTitleTranslationSettings } from '@/features/settings/actions'
import { testTitleTranslation } from '@/features/jav/api'
import JavTitleTranslationSettings from '@/features/settings/components/JavTitleTranslationSettings'
import TitleTranslationBatchSettings from '@/features/settings/components/TitleTranslationBatchSettings'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

export default function TitleTranslationToolSettings() {
  const config = useStore((state) => state.config)
  const [draft, setDraft] = useState(() => createTitleTranslationDraft(config))
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [error, setError] = useState('')
  const [testError, setTestError] = useState('')
  const [result, setResult] = useState('')
  const [sample, setSample] = useState('雨の日、駅で再会した二人')
  const [saved, setSaved] = useState(false)
  const [dirty, setDirty] = useState(false)

  useEffect(() => {
    setDraft(createTitleTranslationDraft(config))
    setDirty(false)
  }, [config])

  const onChange = (patch) => {
    setDraft((current) => changeTranslationDraft(current, patch))
    setError('')
    setTestError('')
    setResult('')
    setSaved(false)
    setDirty(true)
  }
  const handleSave = async (event) => {
    event.preventDefault()
    if (saving || testing) return
    setSaving(true)
    setError('')
    setSaved(false)
    try {
      await saveTitleTranslationSettings(draft)
      setSaved(true)
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }
  const handleTest = async () => {
    if (testing || !sample.trim()) return
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

  return (
    <div className="mx-auto w-full max-w-4xl space-y-4">
      <form onSubmit={handleSave} className="space-y-4">
        <fieldset disabled={saving || testing} className="min-w-0 space-y-4">
          <section
            aria-labelledby="translation-behavior-heading"
            className="overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-sm"
          >
            <header className="flex items-center justify-between gap-6 px-5 pb-1 pt-4">
              <h4 id="translation-behavior-heading" className="text-sm font-semibold text-zinc-900">
                <label htmlFor="translation-auto-enabled" className="cursor-pointer">
                  {zh('自动翻译缺失标题', 'Automatically translate missing titles')}
                </label>
              </h4>
              <Switch
                size="small"
                className="shrink-0"
                checked={draft.enabled}
                onChange={(_, checked) => onChange({ enabled: checked })}
                slotProps={{
                  input: {
                    id: 'translation-auto-enabled',
                    'aria-label': zh('自动翻译缺失标题', 'Automatically translate missing titles'),
                  },
                }}
              />
            </header>
            <p className="px-5 pb-4 text-xs leading-5 text-zinc-500">
              {zh(
                '开启后优先显示中文标题，并自动翻译当前页缺失的标题；关闭后显示原标题。',
                'When enabled, prefer Chinese titles and translate missing titles on the page. When disabled, show original titles.'
              )}
            </p>
          </section>
          <section
            aria-labelledby="translation-service-heading"
            className="overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-sm"
          >
            <header className="border-b border-zinc-100 px-5 py-4">
              <h4 id="translation-service-heading" className="text-sm font-semibold text-zinc-900">
                {zh('服务商配置', 'Provider configuration')}
              </h4>
              <p className="mt-1 text-xs leading-5 text-zinc-500">
                {zh(
                  '配置服务商、密钥和模型，可先测试再保存。',
                  'Choose a provider and model, enter your key, then test before saving.'
                )}
              </p>
            </header>
            <div className="space-y-5 p-5">
              <JavTitleTranslationSettings value={draft} onChange={onChange} />
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
            </div>
            <footer className="flex flex-wrap items-center justify-between gap-3 border-t border-zinc-100 bg-zinc-50/70 px-5 py-4">
              <div className="min-w-0 text-xs text-zinc-500">
                {error ? (
                  <p role="alert" className="break-words text-red-600">
                    {error}
                  </p>
                ) : saved ? (
                  <span role="status" className="text-emerald-700">
                    {zh('已保存', 'Saved')}
                  </span>
                ) : dirty ? (
                  zh('有未保存的修改', 'Unsaved changes')
                ) : (
                  zh(
                    '自动翻译与服务配置在保存后生效。',
                    'Automatic translation and service settings take effect after saving.'
                  )
                )}
              </div>
              <button
                type="submit"
                className="ml-auto shrink-0 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
              >
                {saving ? zh('保存中…', 'Saving…') : zh('保存设置', 'Save settings')}
              </button>
            </footer>
          </section>
        </fieldset>
      </form>
      <TitleTranslationBatchSettings dirty={dirty} />
    </div>
  )
}
