import { getIdolDisplayName } from '@/utils/javIdol'
import { getJavTagDisplayName } from '@/utils/javTag'
import { zh } from '@/utils/i18n'
import { useRef } from 'react'
import { useCloseOnOutsidePointer } from '@/shared/hooks/useCloseOnOutsidePointer'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'

export function formatDateInputFromUnix(value) {
  const unix = Number(value)
  if (!Number.isFinite(unix) || unix <= 0) return ''
  return new Date(unix * 1000).toISOString().slice(0, 10)
}

export const JAV_EDIT_FETCH_LIMIT = 500

export async function fetchAllJavEditOptions(fetcher) {
  const all = []
  let offset = 0
  let total = null
  while (total == null || offset < total) {
    const resp = await fetcher({
      limit: JAV_EDIT_FETCH_LIMIT,
      offset,
      search: '',
    })
    const items = Array.isArray(resp?.items) ? resp.items : []
    all.push(...items)
    total = Number.isFinite(Number(resp?.total)) ? Number(resp.total) : all.length
    if (items.length === 0) break
    offset += items.length
  }
  return all
}

export function mergeOptionsById(options, selectedOptions) {
  const map = new Map()
  for (const option of [...(selectedOptions || []), ...(options || [])]) {
    const id = Number(option?.id)
    if (Number.isFinite(id) && id > 0) {
      map.set(id, option)
    }
  }
  return Array.from(map.values())
}

export function filterOptionsByName(options, search, getSearchText = (option) => option?.name) {
  const q = String(search || '')
    .trim()
    .toLowerCase()
  if (!q) return options
  return (options || []).filter((option) =>
    String(getSearchText(option) || '')
      .toLowerCase()
      .includes(q)
  )
}

export function buildIdolSearchText(idol, preferChineseName) {
  const aliases = Array.isArray(idol?.aliases) ? idol.aliases : []
  return [
    getIdolDisplayName(idol, preferChineseName),
    idol?.name,
    idol?.roman_name,
    idol?.japanese_name,
    idol?.chinese_name,
    ...aliases,
  ]
    .filter(Boolean)
    .join(' ')
}

export function javEditIdolNames(idol, preferChineseName) {
  return [
    getIdolDisplayName(idol, preferChineseName),
    idol?.name,
    idol?.roman_name,
    idol?.japanese_name,
    idol?.chinese_name,
    ...(Array.isArray(idol?.aliases) ? idol.aliases : []),
  ]
}

export function javEditScrapedTagNames(tag, showSimplifiedTags) {
  return [
    tag?.original_name,
    tag?.name,
    tag?.simplified_name,
    getJavTagDisplayName(tag, showSimplifiedTags),
  ]
}

export function javEditWorkCountLabel(value) {
  const count = Math.max(0, Number(value) || 0)
  return zh(`${count} 部作品`, `${count} works`)
}

export function buildStudioSearchText(studio) {
  const aliases = Array.isArray(studio?.aliases) ? studio.aliases : []
  return [studio?.name, ...aliases].filter(Boolean).join(' ')
}

export function includeSelectedOptions(options, allOptions, selectedIds) {
  const selectedSet = new Set((selectedIds || []).map((id) => String(id)))
  if (selectedSet.size === 0) return options
  const map = new Map((options || []).map((option) => [String(option?.id), option]))
  for (const option of allOptions || []) {
    const id = String(option?.id)
    if (selectedSet.has(id) && !map.has(id)) {
      map.set(id, option)
    }
  }
  return Array.from(map.values())
}

export function optionById(options, id) {
  const key = String(id || '')
  if (!key) return null
  return (options || []).find((option) => String(option?.id) === key) || null
}

export function optionsByIds(options, ids) {
  const lookup = new Map((options || []).map((option) => [String(option?.id), option]))
  return (ids || []).map((id) => lookup.get(String(id))).filter(Boolean)
}

export function JavEditDropdown({
  label,
  selectedId,
  options,
  search,
  onSearchChange,
  onSelect,
  open,
  onOpenChange,
  emptyLabel,
  searchPlaceholder,
  disabled,
}) {
  const rootRef = useRef(null)
  const selected = optionById(options, selectedId)
  useCloseOnOutsidePointer(open, rootRef, onOpenChange)

  return (
    <div ref={rootRef} className="relative">
      <div className="block text-[13px] font-semibold text-black">{label}</div>
      <button
        type="button"
        className="mt-2 flex w-full items-center justify-between rounded-md border border-gray-300 bg-white px-3 py-2 text-left text-sm text-gray-900 outline-none hover:border-gray-400 focus:border-blue-500 focus:ring-2 focus:ring-blue-100 disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-500"
        onClick={() => onOpenChange?.(!open)}
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="min-w-0 truncate">{selected?.name || emptyLabel}</span>
        <span
          aria-hidden="true"
          className={`ml-2 h-1.5 w-1.5 shrink-0 rotate-45 border-b border-r border-gray-400 transition-transform ${
            open ? 'rotate-[225deg]' : ''
          }`}
        />
      </button>
      {open ? (
        <div className="absolute left-0 right-0 z-20 mt-1 rounded-md border border-gray-200 bg-white p-2 shadow-xl">
          <input
            type="search"
            value={search}
            onChange={(event) => onSearchChange?.(event.target.value)}
            placeholder={searchPlaceholder}
            className="mb-2 w-full rounded border border-gray-300 px-2 py-1.5 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
          />
          <div className="max-h-52 overflow-y-auto" role="listbox">
            <button
              type="button"
              className={`block w-full rounded px-2 py-1.5 text-left text-sm hover:bg-gray-50 ${
                selectedId ? 'text-gray-700' : 'bg-blue-50 text-blue-700'
              }`}
              onClick={() => {
                onSelect?.('')
                onOpenChange?.(false)
              }}
            >
              {emptyLabel}
            </button>
            {options.map((option) => {
              const active = String(option.id) === String(selectedId || '')
              return (
                <button
                  key={option.id}
                  type="button"
                  className={`block w-full rounded px-2 py-1.5 text-left text-sm hover:bg-gray-50 ${
                    active ? 'bg-blue-50 text-blue-700' : 'text-gray-800'
                  }`}
                  onClick={() => {
                    onSelect?.(String(option.id))
                    onOpenChange?.(false)
                  }}
                  role="option"
                  aria-selected={active}
                >
                  {option.name}
                </button>
              )
            })}
            {options.length === 0 ? (
              <div className="px-2 py-1.5 text-sm text-gray-500">
                {zh('没有匹配结果', 'No matches')}
              </div>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  )
}

export function SelectedChip({ label, onRemove, disabled, compact = false }) {
  return (
    <span
      className={`inline-flex min-w-0 items-center rounded-full bg-gray-100 text-gray-800 ${
        compact ? 'gap-0.5 px-1.5 py-0.5 text-xs' : 'gap-1 px-2 py-1 text-sm'
      }`}
    >
      <span className="truncate">{label}</span>
      <button
        type="button"
        className={`inline-flex shrink-0 items-center justify-center rounded-full text-gray-500 hover:bg-gray-200 hover:text-gray-900 disabled:cursor-not-allowed disabled:opacity-50 ${
          compact ? 'h-3.5 w-3.5' : 'h-4 w-4'
        }`}
        onClick={onRemove}
        disabled={disabled}
        aria-label={zh(`移除 ${label}`, `Remove ${label}`)}
      >
        <CloseOutlinedIcon sx={{ fontSize: compact ? 11 : 13 }} />
      </button>
    </span>
  )
}

export function editableJavTitle(item) {
  return String(item?.title || '')
}
