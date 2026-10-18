import { useState } from 'react'
import { IconButton } from '@mui/material'
import CloseRoundedIcon from '@mui/icons-material/CloseRounded'
import TranslateRoundedIcon from '@mui/icons-material/TranslateRounded'
import RefreshOutlinedIcon from '@mui/icons-material/RefreshOutlined'
import AutoAwesomeOutlinedIcon from '@mui/icons-material/AutoAwesomeOutlined'
import AppModal from '@/shared/ui/AppModal'
import { useStore } from '@/store'
import { translateJavSeriesName, updateJavSeriesTranslation } from '@/features/jav/api'
import { parseTranslationConfig, preferChineseSeries } from '@/features/jav/titleTranslation'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

export default function JavSeriesTranslationModal({ item: initialItem, onClose, onSaved }) {
  const [item] = useState(initialItem)
  const [name, setName] = useState(item.zh_name || '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const patchSeries = useStore((state) => state.patchJavSeries)
  const capabilities = useStore((state) => state.config?.jav_title_translation_capabilities)
  const enabled = useStore((state) => preferChineseSeries(state.config))
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
      zIndex={1700}
      contentClassName="flex max-h-[90vh] w-full max-w-xl flex-col overflow-hidden rounded-xl bg-white shadow-2xl"
      closeDisabled={busy}
      onClose={onClose}
    >
      <form onSubmit={save} className="flex min-h-0 flex-col">
        <div className="flex items-center gap-3 border-b border-gray-100 px-6 py-5">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600">
            <TranslateRoundedIcon sx={{ fontSize: 22 }} />
          </div>
          <h3 className="flex-1 text-base font-semibold text-gray-900">
            {zh('编辑系列译名', 'Edit series translation')}
          </h3>
          <IconButton
            size="small"
            disabled={busy}
            onClick={onClose}
            aria-label={zh('关闭', 'Close')}
          >
            <CloseRoundedIcon sx={{ fontSize: 20 }} />
          </IconButton>
        </div>
        <div className="min-h-0 space-y-5 overflow-y-auto px-6 py-5">
          <div className="space-y-2 rounded-lg border border-gray-100 bg-gray-50 px-4 py-3">
            <div className="text-xs font-medium text-gray-500">
              {zh('原系列名称', 'Original series name')}
            </div>
            <p className="whitespace-pre-wrap break-words text-sm leading-relaxed text-gray-700">
              {item.name}
            </p>
          </div>
          <div className="space-y-3">
            <label className="block space-y-2 text-sm">
              <span className="font-semibold text-gray-900">
                {zh('中文系列名称', 'Chinese series name')}
              </span>
              <textarea
                className="block w-full resize-y rounded-lg border border-gray-300 px-3 py-3 text-sm leading-relaxed text-gray-900 outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100 disabled:bg-gray-50 disabled:text-gray-500"
                value={name}
                maxLength={8192}
                rows={3}
                disabled={busy}
                onChange={(event) => setName(event.target.value)}
              />
            </label>
            {enabled && (
              <div className="flex flex-wrap gap-2">
                {[false, true].map((thinking) => (
                  <button
                    key={String(thinking)}
                    type="button"
                    disabled={busy || !item.name?.trim() || (!thinking && reasoning === 'always')}
                    onClick={() => void translate(thinking)}
                    className="inline-flex items-center gap-1.5 rounded-md border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-600 transition hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 disabled:pointer-events-none disabled:opacity-50"
                  >
                    {thinking ? (
                      <AutoAwesomeOutlinedIcon sx={{ fontSize: 16 }} />
                    ) : (
                      <RefreshOutlinedIcon sx={{ fontSize: 16 }} />
                    )}
                    {thinking
                      ? zh('深度重新翻译', 'Reasoned retranslation')
                      : zh('快速重新翻译', 'Quick retranslation')}
                  </button>
                ))}
              </div>
            )}
            <p className="text-xs leading-relaxed text-gray-500">
              {zh(
                '翻译结果可修改，点击保存后生效。',
                'Review or edit the translation, then save to apply it.'
              )}
            </p>
          </div>
          {error && (
            <p
              role="alert"
              className="rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-sm leading-relaxed text-red-700"
            >
              {error}
            </p>
          )}
        </div>
        <div className="flex shrink-0 justify-end gap-3 border-t border-gray-100 bg-gray-50 px-6 py-4">
          <button
            type="button"
            disabled={busy}
            onClick={onClose}
            className="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 disabled:pointer-events-none disabled:opacity-50"
          >
            {zh('取消', 'Cancel')}
          </button>
          <button
            type="submit"
            disabled={busy}
            className="rounded-lg bg-blue-600 px-5 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-blue-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 disabled:pointer-events-none disabled:opacity-50"
          >
            {busy ? zh('处理中…', 'Working…') : zh('保存', 'Save')}
          </button>
        </div>
      </form>
    </AppModal>
  )
}
