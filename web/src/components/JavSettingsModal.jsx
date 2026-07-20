import { useState } from 'react'
import CloseRoundedIcon from '@mui/icons-material/CloseRounded'
import SwapVertIcon from '@mui/icons-material/SwapVert'
import {
  IDOL_SORT_OPTIONS,
  JAV_SORT_OPTIONS,
  findSortOption,
  reverseSortValue,
  sortLabelParts,
} from '@/constants/jav'
import { zh } from '@/utils/i18n'

function SortText({ option, value }) {
  const parts = sortLabelParts(option, value, zh)

  return (
    <span className="truncate text-xs font-semibold sm:text-sm">
      <span>{parts.label}</span>
      <span className="font-normal text-gray-500">{parts.separator}</span>
      <span className="font-normal text-gray-500">{parts.direction}</span>
    </span>
  )
}

function SortOptionRow({ option, name, inputValue, onChange }) {
  const active = findSortOption([option], inputValue)
  const displayValue = active ? inputValue : option.defaultValue
  const id = `${name}-${option.base}`

  return (
    <div
      className={`flex items-center gap-2 rounded-lg border bg-white px-3 py-2 transition ${
        active
          ? 'border-blue-500 bg-blue-50/40 ring-1 ring-blue-100'
          : 'border-slate-200 hover:border-blue-300'
      }`}
    >
      <label htmlFor={id} className="flex min-w-0 flex-1 cursor-pointer items-center gap-3">
        <input
          id={id}
          type="radio"
          name={name}
          value={displayValue}
          checked={Boolean(active)}
          onChange={() => onChange?.(displayValue)}
          className="h-4 w-4 accent-blue-600"
        />
        <SortText option={option} value={displayValue} />
      </label>
      <button
        type="button"
        onClick={() => onChange?.(reverseSortValue([option], displayValue, option.defaultValue))}
        className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-slate-200 bg-white text-slate-500 transition hover:border-blue-300 hover:bg-blue-50 hover:text-blue-700"
        title={zh('反转排序', 'Reverse sort')}
        aria-label={zh(`反转${option.label[0]}排序`, `Reverse ${option.label[1]} sort`)}
      >
        <SwapVertIcon fontSize="inherit" />
      </button>
    </div>
  )
}

function SectionTitle({ children }) {
  return (
    <div className="mb-2 flex items-center gap-2 text-sm font-semibold text-slate-800">
      <span className="h-4 w-1 rounded-full bg-blue-600" aria-hidden="true" />
      <span>{children}</span>
    </div>
  )
}

function SettingsSection({ title, children }) {
  return (
    <section className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm shadow-slate-100/70">
      <SectionTitle>{title}</SectionTitle>
      {children}
    </section>
  )
}

function SettingsRow({ label, children }) {
  return (
    <div className="flex min-h-11 items-center justify-between gap-4 py-1.5 text-sm text-slate-700">
      <span className="font-medium">{label}</span>
      {children}
    </div>
  )
}

function SettingsSwitch({ label, checked, onChange }) {
  return (
    <button
      type="button"
      role="switch"
      aria-label={label}
      aria-checked={Boolean(checked)}
      onClick={() => onChange?.(!checked)}
      className={`relative inline-flex h-6 w-11 shrink-0 rounded-full transition focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 ${
        checked ? 'bg-blue-600' : 'bg-slate-200'
      }`}
    >
      <span
        className={`mt-1 h-4 w-4 rounded-full bg-white shadow-sm transition-transform ${
          checked ? 'translate-x-6' : 'translate-x-1'
        }`}
      />
    </button>
  )
}

const controlClassName =
  'h-9 w-32 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-700 outline-none transition hover:border-slate-300 focus:border-blue-500 focus:ring-2 focus:ring-blue-100'

function normalizeSettingsTab(tab) {
  return ['jav', 'idol', 'studio', 'series'].includes(tab) ? tab : 'jav'
}

export default function JavSettingsModal({
  open,
  initialTab = 'jav',
  onClose,
  javPageSizeInput,
  onJavPageSizeChange,
  javGridColumnsInput,
  onJavGridColumnsChange,
  javTitleMaxRowsInput,
  onJavTitleMaxRowsChange,
  javIdolTagMaxRowsInput,
  onJavIdolTagMaxRowsChange,
  javTagMaxRowsInput,
  onJavTagMaxRowsChange,
  javHideSeriesInput = false,
  onJavHideSeriesChange,
  javHideIdolsInput = false,
  onJavHideIdolsChange,
  javHideTagsInput = false,
  onJavHideTagsChange,
  javHideActionsInput = false,
  onJavHideActionsChange,
  javWaterfallDefaultInput = false,
  onJavWaterfallDefaultChange,
  idolPageSizeInput,
  onIdolPageSizeChange,
  idolWaterfallDefaultInput = false,
  onIdolWaterfallDefaultChange,
  studioPageSizeInput,
  onStudioPageSizeChange,
  studioWaterfallDefaultInput = false,
  onStudioWaterfallDefaultChange,
  seriesPageSizeInput,
  onSeriesPageSizeChange,
  seriesWaterfallDefaultInput = false,
  onSeriesWaterfallDefaultChange,
  javSortInput,
  onJavSortChange,
  idolSortInput,
  onIdolSortChange,
  javIdolPreferChineseNameInput = false,
  onJavIdolPreferChineseNameChange,
  onSave,
}) {
  const [activeTab, setActiveTab] = useState(() => normalizeSettingsTab(initialTab))

  if (!open) return null

  const tabs = [
    { key: 'jav', label: zh('JAV', 'JAV') },
    { key: 'idol', label: zh('女优', 'Idols') },
    { key: 'studio', label: zh('片商', 'Studios') },
    { key: 'series', label: zh('系列', 'Series') },
  ]

  const resetActiveTab = () => {
    switch (activeTab) {
      case 'idol':
        onIdolPageSizeChange?.(24)
        onIdolWaterfallDefaultChange?.(false)
        onIdolSortChange?.(IDOL_SORT_OPTIONS[0]?.defaultValue || 'recent')
        onJavIdolPreferChineseNameChange?.(false)
        break
      case 'studio':
        onStudioPageSizeChange?.(25)
        onStudioWaterfallDefaultChange?.(false)
        break
      case 'series':
        onSeriesPageSizeChange?.(25)
        onSeriesWaterfallDefaultChange?.(false)
        break
      default:
        onJavPageSizeChange?.(24)
        onJavWaterfallDefaultChange?.(false)
        onJavGridColumnsChange?.(0)
        onJavTitleMaxRowsChange?.(2)
        onJavIdolTagMaxRowsChange?.(2)
        onJavTagMaxRowsChange?.(2)
        onJavHideSeriesChange?.(false)
        onJavHideIdolsChange?.(false)
        onJavHideTagsChange?.(false)
        onJavHideActionsChange?.(false)
        onJavSortChange?.(JAV_SORT_OPTIONS[0]?.defaultValue || 'recent')
        break
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/20 px-4 py-4 backdrop-blur-sm">
      <div
        className="flex h-[860px] max-h-[94vh] w-full max-w-2xl flex-col overflow-hidden rounded-2xl border border-white/80 bg-slate-50 shadow-2xl shadow-slate-900/20"
        role="dialog"
        aria-modal="true"
        aria-labelledby="jav-display-settings-title"
      >
        <header className="flex shrink-0 items-center justify-between px-5 pb-3 pt-5">
          <h2
            id="jav-display-settings-title"
            className="text-xl font-bold tracking-tight text-slate-900"
          >
            {zh('显示设置', 'Display settings')}
          </h2>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-8 w-8 items-center justify-center rounded-full text-slate-500 transition hover:bg-slate-200/70 hover:text-slate-800 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
            aria-label={zh('关闭设置', 'Close settings')}
          >
            <CloseRoundedIcon sx={{ fontSize: 22 }} />
          </button>
        </header>

        <div className="flex shrink-0 flex-wrap gap-2 px-5 pb-4" role="tablist">
          {tabs.map((tab) => {
            const active = activeTab === tab.key
            return (
              <button
                key={tab.key}
                type="button"
                role="tab"
                aria-selected={active}
                onClick={() => setActiveTab(tab.key)}
                className={`rounded-full border px-4 py-1.5 text-sm font-medium transition focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 ${
                  active
                    ? 'border-blue-600 bg-blue-600 text-white shadow-md shadow-blue-200'
                    : 'border-slate-200 bg-white text-slate-600 hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700'
                }`}
              >
                {tab.label}
              </button>
            )
          })}
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 pb-4">
          {activeTab === 'jav' ? (
            <div className="space-y-3">
              <SettingsSection title={zh('布局设置', 'Layout')}>
                <div className="divide-y divide-slate-100 px-1">
                  <SettingsRow label={zh('每页 JAV 数量', 'JAVs per page')}>
                    <input
                      type="number"
                      min="1"
                      value={javPageSizeInput}
                      onChange={(e) => onJavPageSizeChange?.(e.target.value)}
                      className={controlClassName}
                    />
                  </SettingsRow>
                  <SettingsRow label={zh('每行 JAV 数量', 'JAVs per row')}>
                    <select
                      value={String(javGridColumnsInput ?? 0)}
                      onChange={(e) => onJavGridColumnsChange?.(e.target.value)}
                      className={controlClassName}
                    >
                      <option value="0">{zh('自适应', 'Auto')}</option>
                      {Array.from({ length: 12 }, (_, index) => index + 1).map((count) => (
                        <option key={count} value={String(count)}>
                          {count}
                        </option>
                      ))}
                    </select>
                  </SettingsRow>
                  <SettingsRow label={zh('标题最多行数', 'Title max rows')}>
                    <select
                      value={String(javTitleMaxRowsInput ?? 2)}
                      onChange={(e) => onJavTitleMaxRowsChange?.(e.target.value)}
                      className={controlClassName}
                    >
                      <option value="0">{zh('完全展开', 'All')}</option>
                      {Array.from({ length: 12 }, (_, index) => index + 1).map((count) => (
                        <option key={count} value={String(count)}>
                          {count}
                        </option>
                      ))}
                    </select>
                  </SettingsRow>
                  <SettingsRow label={zh('标签最多行数', 'Tag max rows')}>
                    <select
                      value={String(javTagMaxRowsInput ?? 2)}
                      onChange={(e) => onJavTagMaxRowsChange?.(e.target.value)}
                      className={controlClassName}
                    >
                      <option value="0">{zh('完全展开', 'All')}</option>
                      {Array.from({ length: 12 }, (_, index) => index + 1).map((count) => (
                        <option key={count} value={String(count)}>
                          {count}
                        </option>
                      ))}
                    </select>
                  </SettingsRow>
                  <SettingsRow label={zh('演员标签最多行数', 'Actor tag max rows')}>
                    <select
                      value={String(javIdolTagMaxRowsInput ?? 0)}
                      onChange={(e) => onJavIdolTagMaxRowsChange?.(e.target.value)}
                      className={controlClassName}
                    >
                      <option value="0">{zh('完全展开', 'All')}</option>
                      {Array.from({ length: 12 }, (_, index) => index + 1).map((count) => (
                        <option key={count} value={String(count)}>
                          {count}
                        </option>
                      ))}
                    </select>
                  </SettingsRow>
                  <SettingsRow label={zh('默认开启瀑布流', 'Enable waterfall by default')}>
                    <SettingsSwitch
                      label={zh('默认开启瀑布流', 'Enable waterfall by default')}
                      checked={javWaterfallDefaultInput}
                      onChange={onJavWaterfallDefaultChange}
                    />
                  </SettingsRow>
                </div>
              </SettingsSection>

              <SettingsSection title={zh('卡片内容', 'Card content')}>
                <div className="divide-y divide-slate-100 px-1">
                  <SettingsRow label={zh('不显示系列', 'Hide series')}>
                    <SettingsSwitch
                      label={zh('不显示系列', 'Hide series')}
                      checked={javHideSeriesInput}
                      onChange={onJavHideSeriesChange}
                    />
                  </SettingsRow>
                  <SettingsRow label={zh('不显示演员', 'Hide actors')}>
                    <SettingsSwitch
                      label={zh('不显示演员', 'Hide actors')}
                      checked={javHideIdolsInput}
                      onChange={onJavHideIdolsChange}
                    />
                  </SettingsRow>
                  <SettingsRow label={zh('不显示标签', 'Hide tags')}>
                    <SettingsSwitch
                      label={zh('不显示标签', 'Hide tags')}
                      checked={javHideTagsInput}
                      onChange={onJavHideTagsChange}
                    />
                  </SettingsRow>
                  <SettingsRow label={zh('不显示操作按钮', 'Hide action buttons')}>
                    <SettingsSwitch
                      label={zh('不显示操作按钮', 'Hide action buttons')}
                      checked={javHideActionsInput}
                      onChange={onJavHideActionsChange}
                    />
                  </SettingsRow>
                </div>
              </SettingsSection>

              <SettingsSection title={zh('默认排序', 'Default sort')}>
                <div className="space-y-2">
                  {JAV_SORT_OPTIONS.map((option) => (
                    <SortOptionRow
                      key={option.base}
                      option={option}
                      name="jav-sort"
                      inputValue={javSortInput}
                      onChange={onJavSortChange}
                    />
                  ))}
                </div>
              </SettingsSection>
            </div>
          ) : null}

          {activeTab === 'idol' ? (
            <div className="space-y-3">
              <SettingsSection title={zh('布局设置', 'Layout')}>
                <div className="divide-y divide-slate-100 px-1">
                  <SettingsRow label={zh('每页 女优 数量', 'Idols per page')}>
                    <input
                      type="number"
                      min="1"
                      value={idolPageSizeInput}
                      onChange={(e) => onIdolPageSizeChange?.(e.target.value)}
                      className={controlClassName}
                    />
                  </SettingsRow>
                  <SettingsRow label={zh('默认开启瀑布流', 'Enable waterfall by default')}>
                    <SettingsSwitch
                      label={zh('默认开启瀑布流', 'Enable waterfall by default')}
                      checked={idolWaterfallDefaultInput}
                      onChange={onIdolWaterfallDefaultChange}
                    />
                  </SettingsRow>
                </div>
              </SettingsSection>
              <SettingsSection title={zh('卡片内容', 'Card content')}>
                <SettingsRow label={zh('优先显示中文名', 'Prefer Chinese name')}>
                  <SettingsSwitch
                    label={zh('优先显示中文名', 'Prefer Chinese name')}
                    checked={javIdolPreferChineseNameInput}
                    onChange={onJavIdolPreferChineseNameChange}
                  />
                </SettingsRow>
              </SettingsSection>
              <SettingsSection title={zh('默认排序', 'Default sort')}>
                <div className="space-y-2">
                  {IDOL_SORT_OPTIONS.map((option) => (
                    <SortOptionRow
                      key={option.base}
                      option={option}
                      name="idol-sort"
                      inputValue={idolSortInput}
                      onChange={onIdolSortChange}
                    />
                  ))}
                </div>
              </SettingsSection>
            </div>
          ) : null}

          {activeTab === 'studio' ? (
            <SettingsSection title={zh('布局设置', 'Layout')}>
              <div className="divide-y divide-slate-100 px-1">
                <SettingsRow label={zh('每页 片商 数量', 'Studios per page')}>
                  <input
                    type="number"
                    min="1"
                    value={studioPageSizeInput}
                    onChange={(e) => onStudioPageSizeChange?.(e.target.value)}
                    className={controlClassName}
                  />
                </SettingsRow>
                <SettingsRow label={zh('默认开启瀑布流', 'Enable waterfall by default')}>
                  <SettingsSwitch
                    label={zh('默认开启瀑布流', 'Enable waterfall by default')}
                    checked={studioWaterfallDefaultInput}
                    onChange={onStudioWaterfallDefaultChange}
                  />
                </SettingsRow>
              </div>
            </SettingsSection>
          ) : null}

          {activeTab === 'series' ? (
            <SettingsSection title={zh('布局设置', 'Layout')}>
              <div className="divide-y divide-slate-100 px-1">
                <SettingsRow label={zh('每页 系列 数量', 'Series per page')}>
                  <input
                    type="number"
                    min="1"
                    value={seriesPageSizeInput}
                    onChange={(e) => onSeriesPageSizeChange?.(e.target.value)}
                    className={controlClassName}
                  />
                </SettingsRow>
                <SettingsRow label={zh('默认开启瀑布流', 'Enable waterfall by default')}>
                  <SettingsSwitch
                    label={zh('默认开启瀑布流', 'Enable waterfall by default')}
                    checked={seriesWaterfallDefaultInput}
                    onChange={onSeriesWaterfallDefaultChange}
                  />
                </SettingsRow>
              </div>
            </SettingsSection>
          ) : null}
        </div>

        <footer className="flex shrink-0 items-center justify-between gap-3 border-t border-slate-200 bg-white px-5 py-4">
          <button
            type="button"
            onClick={resetActiveTab}
            className="rounded-lg px-3 py-2 text-sm font-medium text-slate-500 transition hover:bg-slate-100 hover:text-slate-800 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          >
            {zh('恢复默认', 'Restore defaults')}
          </button>
          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={onClose}
              className="min-w-24 rounded-lg border border-slate-300 bg-white px-5 py-2 text-sm font-medium text-slate-700 transition hover:bg-slate-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
            >
              {zh('取消', 'Cancel')}
            </button>
            <button
              type="button"
              onClick={onSave}
              className="min-w-24 rounded-lg bg-blue-600 px-5 py-2 text-sm font-medium text-white shadow-md shadow-blue-200 transition hover:bg-blue-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2"
            >
              {zh('保存', 'Save')}
            </button>
          </div>
        </footer>
      </div>
    </div>
  )
}
