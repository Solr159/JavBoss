import { useEffect, useState } from 'react'
import { Switch } from '@mui/material'
import { useStore } from '@/store'
import { createTitleTranslationDraft } from '@/features/settings/model'
import {
  saveTitleTranslationSettings,
  setTitleTranslationEnabled,
} from '@/features/settings/actions'
import { fetchTitleTranslationBatch, startTitleTranslationBatch } from '@/features/jav/api'
import JavTitleTranslationSettings from '@/features/settings/components/JavTitleTranslationSettings'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

export default function TitleTranslationToolSettings() {
  const config = useStore((state) => state.config)
  const [draft, setDraft] = useState(() => createTitleTranslationDraft(config))
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  const [batch, setBatch] = useState(null)
  const batchRunning = Boolean(batch?.running)

  useEffect(() => {
    setDraft(createTitleTranslationDraft(config))
  }, [config])

  useEffect(() => {
    const controller = new AbortController()
    let timer
    const poll = async () => {
      try {
        const status = await fetchTitleTranslationBatch({ signal: controller.signal })
        if (controller.signal.aborted) return
        setError('')
        setBatch(status)
        if (status.running) {
          timer = setTimeout(poll, 1000)
        } else if (batchRunning) {
          await useStore.getState().loadJavs({ force: true, reconcile: true, background: true })
        }
      } catch (err) {
        if (!controller.signal.aborted) {
          setError(getErrorMessage(err))
          if (batchRunning) timer = setTimeout(poll, 2000)
        }
      }
    }
    void poll()
    return () => {
      controller.abort()
      clearTimeout(timer)
    }
  }, [batchRunning])

  const onChange = (patch) => {
    setDraft((current) => ({ ...current, ...patch }))
    setError('')
    setSaved(false)
  }
  const handleSave = async (event, retranslate = false) => {
    event.preventDefault()
    if (saving || !draft.enabled || batchRunning) return
    setSaving(true)
    setError('')
    setSaved(false)
    try {
      await saveTitleTranslationSettings(draft)
      if (retranslate) setBatch(await startTitleTranslationBatch())
      else setSaved(true)
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  const handleToggle = async (enabled) => {
    onChange({ enabled })
    if (enabled) return
    setSaving(true)
    try {
      await setTitleTranslationEnabled(false)
    } catch (err) {
      setDraft(draft)
      setError(getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
      <form onSubmit={handleSave}>
        <fieldset disabled={saving} className="space-y-4">
          <div className="flex items-center justify-between gap-4">
            <h4 className="text-sm font-semibold text-zinc-900">
              {zh('标题翻译', 'Title translation')}
            </h4>
            <Switch
              checked={draft.enabled}
              onChange={(_, enabled) => void handleToggle(enabled)}
              slotProps={{ input: { 'aria-label': zh('标题翻译', 'Title translation') } }}
            />
          </div>
          <p className="text-sm text-zinc-500">
            {zh(
              '与“显示 → JAV → 卡片设置”中的标题翻译开关同步。',
              'Synced with Display → JAV → Card settings.'
            )}
          </p>
          {draft.enabled && (
            <fieldset disabled={batchRunning}>
              <JavTitleTranslationSettings value={draft} onChange={onChange} />
            </fieldset>
          )}
          {error && (
            <p role="alert" className="text-sm text-red-600">
              {error}
            </p>
          )}
          {draft.enabled && (
            <>
              <div className="flex flex-wrap items-center gap-3">
                <button
                  type="submit"
                  disabled={batchRunning}
                  className="rounded-xl bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-60"
                >
                  {saving ? zh('保存中…', 'Saving…') : zh('保存', 'Save')}
                </button>
                <button
                  type="button"
                  disabled={batchRunning}
                  onClick={(event) => void handleSave(event, true)}
                  className="rounded-xl border border-zinc-200 px-4 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-60"
                >
                  {batchRunning
                    ? zh('重新翻译中…', 'Retranslating…')
                    : zh('重新翻译所有影片', 'Retranslate all titles')}
                </button>
                {saved && (
                  <span role="status" className="text-sm text-emerald-700">
                    {zh('已保存', 'Saved')}
                  </span>
                )}
              </div>
              <p className="text-xs text-zinc-500">
                {zh(
                  '重新翻译会先保存当前配置，覆盖已有中文标题并消耗 API 额度。',
                  'Retranslation saves these settings first, replaces existing Chinese titles and uses API credits.'
                )}
              </p>
            </>
          )}
          {batch?.total > 0 && (
            <p role="status" className="text-sm text-zinc-600">
              {zh(
                `${batch.running ? '翻译进度' : '已结束'}：${batch.processed}/${batch.total}，已翻译 ${batch.translated}，跳过 ${batch.skipped}`,
                `${batch.running ? 'Progress' : 'Finished'}: ${batch.processed}/${batch.total}; translated ${batch.translated}, skipped ${batch.skipped}`
              )}
            </p>
          )}
          {batch?.error_zh && (
            <p role="alert" className="text-sm text-red-600">
              {zh(batch.error_zh, batch.error_en)}
            </p>
          )}
        </fieldset>
      </form>
    </section>
  )
}
