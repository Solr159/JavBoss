import { useEffect, useMemo, useState } from 'react'
import Button from '@mui/material/Button'
import AppModal from '@/components/AppModal'
import { fetchWesternEntities } from '@/api'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

const compactButtonSx = {
  minWidth: 0,
  textTransform: 'none',
}

export default function WesternTagModal({ open, onClose, selectedTags = [], onApplyTagFilter }) {
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')

  useEffect(() => {
    if (!open) return undefined
    let cancelled = false
    setLoading(true)
    setError('')
    fetchWesternEntities('tags', { limit: 5000 })
      .then((result) => {
        if (!cancelled) setItems(Array.isArray(result?.items) ? result.items : [])
      })
      .catch((err) => {
        if (!cancelled) setError(getErrorMessage(err))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [open])

  const tags = useMemo(() => {
    const needle = search.trim().toLowerCase()
    return items
      .map((item) => ({
        id: item.name,
        name: item.name,
        count: Number(item.work_count) || 0,
      }))
      .filter((tag) => !needle || tag.name.toLowerCase().includes(needle))
      .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name))
  }, [items, search])

  if (!open) return null
  return (
    <AppModal
      ariaLabel={zh('标签管理', 'Tag Management')}
      contentClassName="mx-4 flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden rounded-3xl bg-white shadow-2xl ring-1 ring-slate-200/70"
      onClose={onClose}
    >
      <div className="flex shrink-0 items-center justify-between border-b border-slate-200/70 bg-slate-50/80 px-6 py-4">
        <h2 className="text-lg font-semibold text-slate-900">{zh('标签管理', 'Tag Management')}</h2>
        <Button size="small" variant="text" onClick={onClose} sx={compactButtonSx}>
          {zh('关闭', 'Close')}
        </Button>
      </div>
      <div className="tag-management-modal-list min-h-0 flex-1 overflow-y-auto px-6 py-5">
        <div className="mb-5 flex flex-wrap items-center justify-between gap-3 text-xs text-slate-500">
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full border border-blue-300 bg-blue-50" />
            {zh('ThePornDB 抓取标签', 'ThePornDB scraped tags')}
          </span>
          <input
            type="search"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={zh('搜索标签', 'Search tags')}
            className="h-8 w-48 rounded border border-slate-200 px-2 text-sm text-slate-700 outline-none focus:border-blue-400"
          />
        </div>
        {loading ? (
          <div className="text-sm text-slate-400">{zh('正在加载标签…', 'Loading tags...')}</div>
        ) : error ? (
          <div className="text-sm text-rose-600">{error}</div>
        ) : tags.length === 0 ? (
          <div className="text-sm text-slate-400">{zh('暂无标签', 'No tags')}</div>
        ) : (
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-base font-semibold text-slate-800">
              <span>{zh('默认分类', 'Default')}</span>
              <span className="text-sm font-normal text-slate-400">{tags.length}</span>
            </div>
            <div className="flex flex-wrap gap-2">
              {tags.map((tag) => (
                <button
                  type="button"
                  key={tag.id}
                  className={`skeuo-tag skeuo-tag--main-button skeuo-tag--scraped skeuo-tag--button ${selectedTags.includes(tag.name) ? 'skeuo-tag--selected' : ''}`}
                  title={tag.name}
                  onClick={() => {
                    onApplyTagFilter?.(tag.name)
                    onClose()
                  }}
                >
                  <span className="skeuo-tag-main flex min-w-0 items-center gap-2 text-left">
                    <span className="skeuo-tag-label">{tag.name}</span>
                    <span className="skeuo-tag-count">{tag.count}</span>
                  </span>
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
      <div className="shrink-0 border-t border-slate-200/70 bg-slate-50/80 px-6 py-4">
        <Button size="small" variant="outlined" onClick={onClose} sx={compactButtonSx}>
          {zh('关闭', 'Close')}
        </Button>
      </div>
    </AppModal>
  )
}
