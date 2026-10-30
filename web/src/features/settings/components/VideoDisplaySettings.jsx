import SwapVertIcon from '@mui/icons-material/SwapVert'
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

export default function VideoDisplaySettings({
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
  autoSubtitleEnabledInput = false,
  onAutoSubtitleEnabledChange,
  autoSubtitleLanguageInput = 'Chinese Simplified',
  onAutoSubtitleLanguageChange,
}) {
  return (
    <div className="divide-y divide-slate-100 rounded-xl border border-slate-200 bg-white p-4 shadow-sm shadow-slate-100/70">
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
      <div className={rowClassName}>
        <span>{zh('自动下载字幕', 'Auto-download subtitles')}</span>
        <SettingsSwitch
          label={zh('自动下载字幕', 'Auto-download subtitles')}
          checked={autoSubtitleEnabledInput}
          onChange={onAutoSubtitleEnabledChange}
        />
      </div>
      <label className={rowClassName}>
        <span>{zh('字幕语言', 'Subtitle language')}</span>
        <input
          type="text"
          maxLength={64}
          value={autoSubtitleLanguageInput}
          placeholder="Chinese Simplified"
          onChange={(event) => onAutoSubtitleLanguageChange?.(event.target.value)}
          className={controlClassName}
        />
      </label>
      <div className="space-y-2 pt-3">
        <div className="text-sm font-medium text-slate-700">{zh('默认排序', 'Default sort')}</div>
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
      </div>
    </div>
  )
}
