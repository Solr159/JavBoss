import { useEffect, useMemo, useState } from 'react'
import Pagination from '@/components/Pagination'
import { fetchWesternEntities } from '@/api'
import { zh } from '@/utils/i18n'

const labels = {
  performers: ['女优', 'Performers'],
  studios: ['片商', 'Studios'],
  series: ['系列', 'Series'],
}

function WesternEntityCard({ item, kind }) {
  const name = String(item?.name || '').trim() || zh('未知', 'Unknown')
  const count = Number(item?.work_count) || 0
  const cover = String(item?.cover_url || '').trim()
  const typeLabel = labels[kind]?.[1] || 'Works'

  return (
    <article className="group flex cursor-pointer flex-col overflow-hidden rounded-lg border bg-white shadow-sm transition hover:shadow-lg">
      <div className="relative aspect-[800/538] w-full overflow-hidden bg-gray-100">
        {cover ? (
          <img
            src={cover}
            alt={name}
            className="h-full w-full object-cover transition duration-200 group-hover:scale-[1.03]"
            loading="lazy"
          />
        ) : (
          <div className="absolute inset-0 flex h-full w-full items-center justify-center bg-gradient-to-br from-gray-100 to-gray-200 p-4 text-center text-lg font-semibold text-gray-600">
            {name}
          </div>
        )}
        <div className="absolute left-2 top-2 rounded bg-black/70 px-2 py-1 text-xs text-white">
          {zh(`作品 ${count}`, `${count} works`)}
        </div>
      </div>
      <div className="flex flex-1 flex-col gap-1 p-3">
        <div className="line-clamp-2 text-sm font-semibold leading-tight text-gray-950">{name}</div>
        <div className="flex min-w-0 items-center gap-2 text-xs text-gray-500">
          <span>{typeLabel}</span>
          <span aria-hidden="true">·</span>
          <span>{count} {zh('部作品', 'works')}</span>
        </div>
      </div>
    </article>
  )
}

export default function WesternLibrary({ kind }) {
  const [items, setItems] = useState([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const pageSize = 25
  const label = labels[kind] || labels.series
  const lastPage = Math.max(1, Math.ceil(total / pageSize))

  useEffect(() => {
    setPage(1)
  }, [kind])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError('')
    fetchWesternEntities(kind, { limit: pageSize, offset: (page - 1) * pageSize, search })
      .then((data) => {
        if (cancelled) return
        setItems(Array.isArray(data?.items) ? data.items : [])
        setTotal(Number(data?.total) || 0)
      })
      .catch((err) => !cancelled && setError(err.message || String(err)))
      .finally(() => !cancelled && setLoading(false))
    return () => {
      cancelled = true
    }
  }, [kind, page, search])

  const title = useMemo(() => zh(label[0], label[1]), [label])

  return (
    <>
      <div className="sticky-pagination pagination-toolbar-grid relative mb-4 grid md:grid-cols-[1fr_auto_1fr] md:items-center">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-gray-700">{title}</div>
        </div>
        <div className="flex justify-center overflow-x-auto">
          <Pagination
            page={page}
            lastPage={lastPage}
            totalItems={total}
            hasPrev={page > 1}
            hasNext={page < lastPage}
            loading={loading}
            onFirst={() => setPage(1)}
            onPrev={() => setPage((value) => Math.max(1, value - 1))}
            onNext={() => setPage((value) => Math.min(lastPage, value + 1))}
            onLast={() => setPage(lastPage)}
          />
        </div>
        <div className="flex justify-end">
          <div className="pagination-sort-group flex items-center gap-3">
            <span className="pagination-sort-label text-gray-500">{zh('排序', 'Sort')}</span>
            <span className="pagination-sort-button font-semibold text-gray-700">
              {zh('作品数', 'Works')}
            </span>
          </div>
        </div>
      </div>
      <div className="mb-4 flex justify-end">
        <input
          className="h-9 w-56 rounded border border-gray-300 bg-white px-3 py-2 text-sm outline-none focus:border-gray-900"
          value={search}
          onChange={(event) => {
            setPage(1)
            setSearch(event.target.value)
          }}
          placeholder={zh('搜索', 'Search')}
          aria-label={zh(`搜索${label[0]}`, `Search ${label[1]}`)}
        />
      </div>
      {error ? <div className="rounded border border-red-200 bg-red-50 p-3 text-red-700">{error}</div> : null}
      {loading ? (
        <div className="mt-4 flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
          {zh('加载中…', 'Loading...')}
        </div>
      ) : items.length === 0 ? (
        <div className="flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
          {zh('暂无数据', 'No data')}
        </div>
      ) : (
        <div className="grid gap-4 bg-white" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(16rem, 1fr))' }}>
          {items.map((item) => (
            <WesternEntityCard key={item.name} item={item} kind={kind} />
          ))}
        </div>
      )}
    </>
  )
}
