import { useState } from 'react'
import AppModal from '@/shared/ui/AppModal'
import { useStore } from '@/store'
import { translateJavSeriesName, updateJavSeriesTranslation } from '@/features/jav/api'
import { parseTranslationConfig } from '@/features/jav/titleTranslation'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

export default function JavSeriesTranslationModal({ item: initialItem, onClose, onSaved }) {
  const [item] = useState(initialItem)
  const [name, setName] = useState(item.zh_name || '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const patchSeries = useStore((state) => state.patchJavSeries)
  const capabilities = useStore((state) => state.config?.jav_title_translation_capabilities)
  const reasoning = parseTranslationConfig(capabilities).reasoning
  const translate = async (thinking) => {
    setBusy(true)
    setError('')
    try {
      const result = await translateJavSeriesName(item.id, { name: item.name, thinking })
      setName(result.zh_name)
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }
  const save = async (event) => {
    event.preventDefault()
    if (busy) return
    setBusy(true)
    setError('')
    try {
      const updated = await updateJavSeriesTranslation(item.id, { name: item.name, zh_name: name })
      patchSeries(updated)
      onSaved?.(updated)
      onClose()
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <AppModal
      ariaLabel={zh('编辑系列译名', 'Edit series translation')}
      className="p-4"
      contentClassName="w-full max-w-xl rounded-lg bg-white p-5 shadow-xl"
      closeDisabled={busy}
      onClose={onClose}
    >
      <form onSubmit={save} className="space-y-4">
        <h3 className="text-base font-semibold">{zh('编辑系列译名', 'Edit series translation')}</h3>
        <div className="space-y-1 text-sm">
          <div className="text-gray-500">{zh('原系列名称', 'Original series name')}</div>
          <p className="break-words">{item.name}</p>
        </div>
        <label className="block space-y-2 text-sm">
          <span>{zh('中文系列名称', 'Chinese series name')}</span>
          <textarea
            className="w-full rounded border p-2"
            value={name}
            maxLength={8192}
            rows={3}
            disabled={busy}
            onChange={(event) => setName(event.target.value)}
          />
        </label>
        <div className="flex flex-wrap gap-2">
          {[false, true].map((thinking) => (
            <button
              key={String(thinking)}
              type="button"
              disabled={busy || !item.name?.trim() || (!thinking && reasoning === 'always')}
              onClick={() => void translate(thinking)}
              className="rounded border px-3 py-2 text-sm disabled:opacity-50"
            >
              {thinking
                ? zh('深度重新翻译', 'Reasoned retranslation')
                : zh('快速重新翻译', 'Quick retranslation')}
            </button>
          ))}
        </div>
        <p className="text-xs text-gray-500">
          {zh(
            '翻译结果可修改，点击保存后生效。',
            'Review or edit the translation, then save to apply it.'
          )}
        </p>
        {error && (
          <p role="alert" className="text-sm text-red-600">
            {error}
          </p>
        )}
        <div className="flex justify-end gap-2">
          <button
            type="button"
            disabled={busy}
            onClick={onClose}
            className="rounded border px-4 py-2 text-sm disabled:opacity-50"
          >
            {zh('取消', 'Cancel')}
          </button>
          <button
            type="submit"
            disabled={busy}
            className="rounded bg-blue-600 px-4 py-2 text-sm text-white disabled:opacity-50"
          >
            {busy ? zh('处理中…', 'Working…') : zh('保存', 'Save')}
          </button>
        </div>
      </form>
    </AppModal>
  )
}
