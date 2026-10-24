import { useState } from 'react'
import CloseRoundedIcon from '@mui/icons-material/CloseRounded'
import SwapVertIcon from '@mui/icons-material/SwapVert'
import AppModal from '@/shared/ui/AppModal'
import {
  VIDEO_SORT_OPTIONS,
  findVideoSortOption,
  reverseVideoSortValue,
  videoSortLabelParts,
} from '@/constants/video'
import { zh } from '@/utils/i18n'

function SortText({ option, value }) {
  const parts = videoSortLabelParts(option, value, zh)

  return (
    <span className="truncate text-xs font-semibold sm:text-sm">
      <span>{parts.label}</span>
      <span className="font-normal text-gray-500">{parts.separator}</span>
      <span className="font-normal text-gray-500">{parts.direction}</span>
    </span>
  )
}

function SortOptionRow({ option, inputValue, onChange }) {
  const active = findVideoSortOption(inputValue)?.base === option.base
  const displayValue = active ? inputValue : option.defaultValue
  const id = `sort-${option.base}`

  return (
    <div
      className={`flex min-w-0 items-center gap-2 rounded-lg border px-3 py-2 transition ${
        active
          ? 'border-blue-500 bg-blue-50/40 ring-1 ring-blue-100'
          : 'border-slate-200 bg-white hover:border-blue-300'
      }`}
    >
      <label htmlFor={id} className="flex min-w-0 flex-1 cursor-pointer items-center gap-3">
        <input
          id={id}
          type="radio"
          name="sort"
          value={displayValue}
          checked={active}
          onChange={() => onChange?.(displayValue)}
          className="h-4 w-4 shrink-0 accent-blue-600"
        />
        <SortText option={option} value={displayValue} />
      </label>
      <button
        type="button"
        onClick={() => onChange?.(reverseVideoSortValue(displayValue, option.defaultValue))}
        className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-slate-200 bg-white text-slate-500 transition hover:border-blue-300 hover:bg-blue-50 hover:text-blue-700"
        title={zh('反转排序', 'Reverse sort')}
        aria-label={zh(`反转${option.label[0]}排序`, `Reverse ${option.label[1]} sort`)}
      >
        <SwapVertIcon fontSize="inherit" />
      </button>
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
      className={`relative inline-flex h-6 w-11 shrink-0 rounded-full transition focus:outline-none ${
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

const rowClassName =
  'flex min-h-11 items-center justify-between gap-4 py-1.5 text-sm font-medium text-slate-700'
const controlClassName =
  'h-9 w-24 shrink-0 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-700 outline-none transition hover:border-slate-300 focus:border-blue-500 focus:ring-2 focus:ring-blue-100 sm:w-32'

export default function VideoSettingsModal({
  open,
  onClose,
  onCancel,
  pageSizeInput,
  onPageSizeChange,
  sortInput,
  onSortChange,
  hideJavInput = false,
  onHideJavChange,
  waterfallDefaultInput = false,
  onWaterfallDefaultChange,
  watchTimeIconMinutesInput = 30,
  onWatchTimeIconMinutesChange,
  onSave,
  saving = false,
}) {
  const [editing, setEditing] = useState(false)

  if (!open) return null

  const resetDefaults = () => {
    onPageSizeChange?.(25)
    onSortChange?.(VIDEO_SORT_OPTIONS[0].defaultValue)
    onHideJavChange?.(false)
    onWaterfallDefaultChange?.(false)
    onWatchTimeIconMinutesChange?.(30)
  }

  return (
    <AppModal
      ariaLabel={zh('视频设置', 'Video Settings')}
      className="px-4 py-4"
      contentClassName="flex max-h-[94vh] w-full max-w-lg flex-col overflow-hidden rounded-2xl border border-white/80 bg-slate-50 shadow-2xl shadow-slate-900/20"
      onClose={onClose}
    >
      <header className="flex shrink-0 items-center justify-between px-5 pb-4 pt-5">
        <h2 className="text-xl font-bold tracking-tight text-slate-900">
          {zh('视频设置', 'Video Settings')}
        </h2>
        <button
          type="button"
          onClick={onClose}
          className="inline-flex h-8 w-8 items-center justify-center rounded-full text-slate-500 transition hover:bg-slate-200/70 hover:text-slate-800 focus:outline-none"
          aria-label={zh('关闭设置', 'Close settings')}
        >
          <CloseRoundedIcon sx={{ fontSize: 22 }} />
        </button>
      </header>
      <div className="min-h-0 min-w-0 flex-1 overflow-y-auto px-4 pb-4 sm:px-5">
        <fieldset
          disabled={!editing || saving}
          className="min-w-0 space-y-2 rounded-xl border border-slate-200 bg-white p-4 shadow-sm shadow-slate-100/70 [&_:disabled]:cursor-default [&_button:disabled]:opacity-50 [&_input:disabled]:bg-slate-100"
        >
          <label className={rowClassName}>
            <span>{zh('每页视频数量', 'Videos per page')}</span>
            <input
              type="number"
              min="1"
              value={pageSizeInput}
              onChange={(e) => onPageSizeChange?.(e.target.value)}
              className={controlClassName}
            />
          </label>
          <div className={rowClassName}>
            <span>{zh('默认开启瀑布流', 'Enable waterfall by default')}</span>
            <SettingsSwitch
              label={zh('默认开启瀑布流', 'Enable waterfall by default')}
              checked={waterfallDefaultInput}
              onChange={onWaterfallDefaultChange}
            />
          </div>
          <label className={rowClassName}>
            <span>{zh('每个观看图标代表的分钟数', 'Minutes per watch icon')}</span>
            <input
              type="number"
              min="1"
              step="1"
              value={watchTimeIconMinutesInput}
              onChange={(event) => onWatchTimeIconMinutesChange?.(event.target.value)}
              className={controlClassName}
            />
          </label>
          <div className={rowClassName}>
            <span>{zh('隐藏已刮削的视频', 'Hide scraped videos')}</span>
            <SettingsSwitch
              label={zh('隐藏已刮削的视频', 'Hide scraped videos')}
              checked={hideJavInput}
              onChange={onHideJavChange}
            />
          </div>
          <div className="pt-2 text-sm font-medium text-slate-700">
            {zh('默认排序', 'Default sort')}
          </div>
          <div className="space-y-2">
            {VIDEO_SORT_OPTIONS.map((option) => (
              <SortOptionRow
                key={option.base}
                option={option}
                inputValue={sortInput}
                onChange={onSortChange}
              />
            ))}
          </div>
        </fieldset>
      </div>
      <footer className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-t border-slate-200 bg-white px-4 py-4 sm:px-5">
        {editing && (
          <button
            type="button"
            onClick={resetDefaults}
            disabled={saving}
            className="rounded-lg px-2 py-2 text-sm font-medium text-slate-500 transition hover:bg-slate-100 hover:text-slate-800 focus:outline-none disabled:opacity-50 sm:px-3"
          >
            {zh('恢复默认', 'Restore defaults')}
          </button>
        )}
        <div className="ml-auto flex items-center gap-2 sm:gap-3">
          {editing && (
            <button
              key="cancel"
              type="button"
              onClick={() => {
                onCancel?.()
                setEditing(false)
              }}
              disabled={saving}
              className="min-w-20 rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:bg-slate-50 focus:outline-none disabled:opacity-50 sm:min-w-24 sm:px-5"
            >
              {zh('取消', 'Cancel')}
            </button>
          )}
          <button
            key="primary"
            type="button"
            onClick={editing ? onSave : () => setEditing(true)}
            disabled={saving}
            className="min-w-20 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-md shadow-blue-200 transition hover:bg-blue-700 focus:outline-none disabled:opacity-50 sm:min-w-24 sm:px-5"
          >
            {editing ? zh('保存', 'Save') : zh('编辑', 'Edit')}
          </button>
        </div>
      </footer>
    </AppModal>
  )
}
