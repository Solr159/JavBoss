import { useState } from 'react'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'

export default function SubtitleSettings({ enabled = false, language = 'zh-CN', onSave }) {
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')

  return (
    <form
      key={`${enabled}:${language}`}
      className="space-y-3 rounded-2xl border border-zinc-200 bg-white p-5 dark:border-ui-border dark:bg-ui-surface"
      onSubmit={async (event) => {
        event.preventDefault()
        const data = new FormData(event.currentTarget)
        setSaving(true)
        setMessage('')
        try {
          await onSave({
            subtitle_auto_download: data.has('enabled'),
            subtitle_language: data.get('language'),
          })
          setMessage(zh('字幕设置已保存', 'Subtitle settings saved'))
        } catch (error) {
          setMessage(getErrorMessage(error))
        } finally {
          setSaving(false)
        }
      }}
    >
      <label className="flex items-center gap-3 text-sm font-semibold">
        <input name="enabled" type="checkbox" defaultChecked={enabled} disabled={saving} />
        {zh('播放时自动下载字幕', 'Automatically download subtitles during playback')}
      </label>
      <p className="text-xs text-zinc-500 dark:text-ui-faint">
        {zh(
          '从 SubtitleCat 查找字幕，支持 MPV 和网页播放器，也可在播放器字幕菜单中手动触发加载。',
          'Find subtitles on SubtitleCat for MPV and the web player. You can also load subtitles manually from the player subtitle menu.'
        )}
      </p>
      <label className="flex items-center gap-3 text-sm">
        {zh('优先语言', 'Preferred language')}
        <select
          name="language"
          defaultValue={language}
          disabled={saving}
          className="rounded border border-zinc-200 bg-white px-2 py-1 dark:border-ui-border dark:bg-ui-surface"
        >
          <option value="zh-CN">{zh('简体中文', 'Simplified Chinese')}</option>
          <option value="zh-TW">{zh('繁体中文', 'Traditional Chinese')}</option>
        </select>
      </label>
      <p className="text-xs text-zinc-500 dark:text-ui-faint">
        {zh(
          '缺少首选语言时尝试另一种中文，字幕可能为机器翻译。',
          'Falls back to the other Chinese variant when the preferred language is unavailable. Subtitles may be machine translated.'
        )}
      </p>
      <div className="flex items-center justify-end gap-3">
        <span role="status" className="text-sm">
          {message}
        </span>
        <button
          type="submit"
          disabled={saving}
          className="rounded-xl bg-ui-primary px-3 py-1.5 text-sm text-white disabled:opacity-60"
        >
          {saving ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
        </button>
      </div>
    </form>
  )
}
