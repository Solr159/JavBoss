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
    <span className="truncate text-sm font-semibold">
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
    <div className="flex items-center gap-2 rounded border px-3 py-1.5 hover:border-blue-500">
      <label htmlFor={id} className="flex min-w-0 flex-1 cursor-pointer items-center gap-3">
        <input
          id={id}
          type="radio"
          name="sort"
          value={displayValue}
          checked={active}
          onChange={() => onChange?.(displayValue)}
        />
        <SortText option={option} value={displayValue} />
      </label>
      <button
        type="button"
        onClick={() => onChange?.(reverseVideoSortValue(displayValue, option.defaultValue))}
        className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded border border-gray-200 text-gray-500 hover:border-blue-400 hover:bg-blue-50 hover:text-blue-700"
        title={zh('反转排序', 'Reverse sort')}
        aria-label={zh(`反转${option.label[0]}排序`, `Reverse ${option.label[1]} sort`)}
      >
        <SwapVertIcon fontSize="inherit" />
      </button>
    </div>
  )
}

export default function VideoSettingsModal({
  open,
  onClose,
  pageSizeInput,
  onPageSizeChange,
  sortInput,
  onSortChange,
  hideJavInput = true,
  onHideJavChange,
  onSave,
}) {
  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 px-4">
      <div className="w-full max-w-sm rounded-lg bg-white p-3 shadow-xl">
        <div className="mb-2 flex items-center justify-between">
          <h2 className="text-base font-semibold">{zh('视频设置', 'Video Settings')}</h2>
          <button
            onClick={onClose}
            className="rounded px-2 py-1 text-gray-500 hover:bg-gray-100"
            aria-label={zh('关闭设置', 'Close settings')}
          >
            ✕
          </button>
        </div>
        <div className="space-y-2">
          <label className="flex cursor-pointer items-center gap-2 rounded border px-3 py-2 text-sm font-medium text-gray-700 hover:border-blue-500">
            <input
              type="checkbox"
              checked={Boolean(hideJavInput)}
              onChange={(event) => onHideJavChange?.(event.target.checked)}
              className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
            />
            <span>{zh('不显示已识别为 JAV 的视频', 'Hide videos recognized as JAV')}</span>
          </label>
          <label className="flex items-center justify-between gap-3 text-sm font-medium text-gray-700">
            <span>{zh('每页视频数量', 'Videos per page')}</span>
            <input
              type="number"
              min="1"
              value={pageSizeInput}
              onChange={(e) => onPageSizeChange?.(e.target.value)}
              className="w-24 rounded border px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </label>
          <div className="text-sm font-medium text-gray-700">{zh('分页排序', 'Sort order')}</div>
          {VIDEO_SORT_OPTIONS.map((option) => (
            <SortOptionRow
              key={option.base}
              option={option}
              inputValue={sortInput}
              onChange={onSortChange}
            />
          ))}
        </div>
        <div className="mt-3 flex justify-end">
          <button onClick={onClose} className="rounded border px-3 py-1 text-sm hover:bg-gray-50">
            {zh('取消', 'Cancel')}
          </button>
          <button
            onClick={onSave}
            className="ml-2 rounded bg-blue-600 px-3 py-1 text-sm text-white hover:bg-blue-700"
          >
            {zh('保存', 'Save')}
          </button>
        </div>
      </div>
    </div>
  )
}
