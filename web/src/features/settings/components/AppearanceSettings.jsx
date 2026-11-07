import { useState } from 'react'
import { useAppTheme } from '@/app/theme/AppThemeProvider'
import { zh } from '@/utils/i18n'

export default function AppearanceSettings() {
  const { preference, setPreference } = useAppTheme()
  const [storageError, setStorageError] = useState(false)

  return (
    <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm dark:border-ui-border dark:bg-ui-surface">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <label
            htmlFor="appearance-theme"
            className="text-sm font-semibold text-zinc-800 dark:text-ui-text"
          >
            {zh('外观主题', 'Appearance theme')}
          </label>
        </div>
        <select
          id="appearance-theme"
          value={preference}
          disabled={!setPreference}
          onChange={(event) => setStorageError(!setPreference(event.target.value))}
          className="form-field rounded-xl border border-zinc-200 bg-white px-3 py-1.5 text-sm text-zinc-800 focus-visible:border-blue-500 focus-visible:ring-2 focus-visible:ring-blue-100"
        >
          <option value="light">{zh('浅色', 'Light')}</option>
          <option value="dark">{zh('深色', 'Dark')}</option>
          <option value="system">{zh('跟随系统', 'System')}</option>
        </select>
      </div>
      {storageError && (
        <p role="alert" className="mt-3 text-xs text-amber-700 dark:text-amber-300">
          {zh(
            '浏览器无法保存主题，本次选择仅在当前页面生效。',
            'The browser could not save your theme. This choice only applies to the current page.'
          )}
        </p>
      )}
    </section>
  )
}
