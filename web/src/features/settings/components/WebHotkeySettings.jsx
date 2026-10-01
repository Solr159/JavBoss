import { useMemo, useState } from 'react'

import {
  defaultWebHotkeys,
  formatWebHotkeyKey,
  isAllowedWebHotkeyKey,
  normalizeWebHotkeyKey,
  parseWebHotkeys,
  WEB_HOTKEY_ACTIONS,
  webHotkeyFromKeyboardEvent,
  webHotkeyKeyId,
  webHotkeysEqual,
} from '@/utils/webHotkeys'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'

const ACTION_LABELS = {
  content_page_up: { zh: '内容向上滑动', en: 'Scroll content up' },
  content_page_down: { zh: '内容向下滑动', en: 'Scroll content down' },
  continuous_scroll_up: { zh: '内容持续缓慢上移', en: 'Continuously scroll content up' },
  continuous_scroll_down: { zh: '内容持续缓慢下移', en: 'Continuously scroll content down' },
  edit_jav_query: {
    zh: '显示/隐藏 编辑JAV查询条件弹窗',
    en: 'Show/hide JAV query editor dialog',
  },
  open_page_jump: { zh: '显示/隐藏 跳转页面下拉框', en: 'Show/hide page jump dropdown' },
  previous_page: { zh: '上一页/上一张图片', en: 'Previous page/image' },
  next_page: { zh: '下一页/下一张图片', en: 'Next page/image' },
  browser_back: { zh: '浏览器后退', en: 'Browser back' },
  browser_forward: { zh: '浏览器前进', en: 'Browser forward' },
}

export default function WebHotkeySettings({ hotkeys, onSave }) {
  const savedHotkeys = useMemo(() => parseWebHotkeys(hotkeys), [hotkeys])
  const [items, setItems] = useState(savedHotkeys)
  const [capturingAction, setCapturingAction] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [saving, setSaving] = useState(false)

  const beginCapture = (action) => {
    setCapturingAction(action)
    setError('')
    setSuccess('')
  }

  const setKey = (action, rawKey) => {
    const key = normalizeWebHotkeyKey(rawKey)
    if (!isAllowedWebHotkeyKey(key)) {
      setError(
        zh(
          '该按键不能用作快捷键，请选择非 Esc、Tab 或修饰键的其他按键。',
          'That key cannot be used. Choose a key other than Escape, Tab, or a modifier key.'
        )
      )
      return false
    }
    const duplicate = items.find(
      (item) => item.action !== action && webHotkeyKeyId(item.key) === webHotkeyKeyId(key)
    )
    if (duplicate) {
      const label = ACTION_LABELS[duplicate.action]
      setError(zh(`该按键已用于“${label.zh}”`, `That key is already assigned to “${label.en}”`))
      return false
    }
    setItems((current) => current.map((item) => (item.action === action ? { ...item, key } : item)))
    setCapturingAction('')
    setError('')
    setSuccess('')
    return true
  }

  const handleSave = async () => {
    setSaving(true)
    setError('')
    setSuccess('')
    try {
      await onSave?.(items)
      setSuccess(zh('快捷键已保存', 'Shortcuts saved'))
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
      <div>
        <h4 className="text-sm font-semibold text-zinc-900">{zh('网页快捷键', 'Web Shortcuts')}</h4>
        <p className="mt-1 text-sm text-zinc-500">
          {zh(
            '点击按键框后直接按下新按键，支持 Shift 组合键。',
            'Select a key field and press the new key. Shift combinations are supported.'
          )}
        </p>
      </div>

      <div className="mt-4 divide-y divide-zinc-100">
        {WEB_HOTKEY_ACTIONS.map(({ action }) => {
          const item = items.find((entry) => entry.action === action)
          const label = ACTION_LABELS[action]
          const capturing = capturingAction === action
          return (
            <div key={action} className="flex items-center justify-between gap-3 py-2">
              <span className="text-sm text-zinc-700">{zh(label.zh, label.en)}</span>
              <input
                id={`web-hotkey-${action}`}
                readOnly
                value={capturing ? zh('请按键…', 'Press a key…') : formatWebHotkeyKey(item?.key)}
                onFocus={() => beginCapture(action)}
                onClick={() => beginCapture(action)}
                onBlur={() => setCapturingAction('')}
                onKeyDown={(event) => {
                  if (event.key === 'Tab') return
                  if (event.key === 'Escape') {
                    event.preventDefault()
                    event.stopPropagation()
                    setCapturingAction('')
                    setError('')
                    event.currentTarget.blur()
                    return
                  }
                  event.preventDefault()
                  event.stopPropagation()
                  if (setKey(action, webHotkeyFromKeyboardEvent(event))) {
                    event.currentTarget.blur()
                  }
                }}
                className={`w-32 cursor-pointer rounded-lg border bg-white px-2.5 py-1.5 text-center text-sm font-medium outline-none ${
                  capturing
                    ? 'border-blue-500 ring-2 ring-blue-100'
                    : 'border-zinc-200 text-zinc-800'
                }`}
                aria-label={zh(`修改${label.zh}快捷键`, `Change ${label.en} shortcut`)}
              />
            </div>
          )
        })}
      </div>

      {error ? <div className="mt-3 text-sm text-red-600">{error}</div> : null}
      {success ? <div className="mt-3 text-sm text-emerald-600">{success}</div> : null}

      <div className="mt-5 flex justify-end gap-2">
        <button
          type="button"
          onClick={() => {
            setItems(defaultWebHotkeys())
            setCapturingAction('')
            setError('')
            setSuccess('')
          }}
          disabled={saving || webHotkeysEqual(items, defaultWebHotkeys())}
          className="rounded-xl border border-zinc-200 bg-white px-3 py-1.5 text-sm text-zinc-700 hover:bg-zinc-50 disabled:opacity-60"
        >
          {zh('恢复默认', 'Restore Defaults')}
        </button>
        <button
          type="button"
          onClick={handleSave}
          disabled={saving || webHotkeysEqual(items, savedHotkeys)}
          className="rounded-xl bg-blue-600 px-3 py-1.5 text-sm text-white disabled:opacity-60"
        >
          {saving ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
        </button>
      </div>
    </section>
  )
}
