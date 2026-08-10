import { useEffect, useState } from 'react'
import SearchIcon from '@mui/icons-material/Search'
import AppModal from '@/components/AppModal'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

function names(values) {
  return Array.isArray(values) ? values.filter(Boolean).join(', ') : ''
}

export default function WesternScrapeModal({ open, video, onClose, onSearch, onSave, onDelete }) {
  const [query, setQuery] = useState('')
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(false)
  const [savingId, setSavingId] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return
    setQuery(String(video?.filename || video?.path || '').trim())
    setItems([])
    setLoading(false)
    setSavingId('')
    setError('')
  }, [open, video])

  if (!open) return null

  const current = video?.western_metadata

  const search = async () => {
    setLoading(true)
    setError('')
    try {
      const result = await onSearch(query)
      setItems(Array.isArray(result?.items) ? result.items : [])
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }

  const save = async (item) => {
    setSavingId(String(item?.source_id || 'saving'))
    setError('')
    try {
      await onSave(item)
    } catch (err) {
      setError(getErrorMessage(err))
      setSavingId('')
    }
  }

  const remove = async () => {
    setSavingId('delete')
    setError('')
    try {
      await onDelete()
    } catch (err) {
      setError(getErrorMessage(err))
      setSavingId('')
    }
  }

  return (
    <AppModal
      ariaLabel={zh('欧美元数据刮削', 'Western metadata scrape')}
      className="px-4"
      contentClassName="flex max-h-[88vh] w-full max-w-4xl flex-col overflow-hidden rounded-xl bg-white shadow-xl"
      onClose={onClose}
    >
      <div className="border-b px-5 py-4">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h2 className="text-lg font-semibold text-gray-900">
              {zh('欧美元数据', 'Western Metadata')}
            </h2>
            <p className="mt-1 max-w-2xl truncate text-xs text-gray-500">
              {video?.filename || video?.path}
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded px-2 py-1 text-gray-500 hover:bg-gray-100"
          >
            {zh('关闭', 'Close')}
          </button>
        </div>
      </div>

      <div className="overflow-y-auto p-5">
        {current ? (
          <div className="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <span className="font-semibold text-emerald-900">{current.title}</span>
                <span className="ml-2 text-xs uppercase text-emerald-700">
                  {current.source} / {current.content_type}
                </span>
              </div>
              <button
                type="button"
                onClick={remove}
                disabled={Boolean(savingId)}
                className="rounded border border-red-300 bg-white px-2 py-1 text-xs text-red-700 hover:bg-red-50 disabled:opacity-50"
              >
                {savingId === 'delete'
                  ? zh('删除中...', 'Deleting...')
                  : zh('删除元数据', 'Remove metadata')}
              </button>
            </div>
            <div className="mt-1 text-xs text-emerald-800">
              {[current.studio, names(current.performers), names(current.labels)]
                .filter(Boolean)
                .join(' · ')}
            </div>
          </div>
        ) : null}

        <form
          className="flex gap-2"
          onSubmit={(event) => {
            event.preventDefault()
            search()
          }}
        >
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            className="min-w-0 flex-1 rounded-lg border px-3 py-2 text-sm focus:border-emerald-600 focus:outline-none focus:ring-1 focus:ring-emerald-600"
            placeholder={zh('文件名、站点、演员或标题', 'Filename, site, performer, or title')}
          />
          <button
            type="submit"
            disabled={loading || !query.trim()}
            className="inline-flex items-center gap-1 rounded-lg bg-emerald-700 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-800 disabled:bg-gray-300"
          >
            <SearchIcon fontSize="small" />
            {loading ? zh('搜索中...', 'Searching...') : zh('搜索', 'Search')}
          </button>
        </form>

        {error ? (
          <div className="mt-3 rounded bg-red-50 px-3 py-2 text-sm text-red-700">{error}</div>
        ) : null}

        <div className="mt-4 grid gap-3 md:grid-cols-2">
          {items.map((item) => (
            <article
              key={item.source_id}
              className="overflow-hidden rounded-lg border border-gray-200 bg-white"
            >
              {item.cover_url ? (
                <img
                  src={item.cover_url}
                  alt=""
                  className="aspect-video w-full bg-gray-100 object-cover"
                  loading="lazy"
                />
              ) : null}
              <div className="p-3">
                <div className="line-clamp-2 text-sm font-semibold text-gray-900">{item.title}</div>
                <div className="mt-1 text-xs font-medium text-emerald-700">
                  {[item.studio, item.release_date, item.content_type].filter(Boolean).join(' · ')}
                </div>
                {item.performers?.length ? (
                  <div className="mt-2 line-clamp-2 text-xs text-gray-600">
                    {names(item.performers)}
                  </div>
                ) : null}
                <div className="mt-2 flex flex-wrap gap-1">
                  {(item.labels || []).slice(0, 8).map((label) => (
                    <span
                      key={label}
                      className="rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-900"
                    >
                      {label}
                    </span>
                  ))}
                </div>
                <button
                  type="button"
                  onClick={() => save(item)}
                  disabled={Boolean(savingId)}
                  className="mt-3 w-full rounded bg-gray-900 px-3 py-1.5 text-xs font-semibold text-white hover:bg-black disabled:bg-gray-300"
                >
                  {savingId === String(item.source_id)
                    ? zh('保存中...', 'Saving...')
                    : zh('使用此结果', 'Use this result')}
                </button>
              </div>
            </article>
          ))}
        </div>
        {!loading && items.length === 0 ? (
          <div className="mt-6 rounded-lg border border-dashed py-8 text-center text-sm text-gray-500">
            {zh('输入文件名后搜索 ThePornDB', 'Search ThePornDB using the filename')}
          </div>
        ) : null}
      </div>
    </AppModal>
  )
}
