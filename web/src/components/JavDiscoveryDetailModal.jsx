import { useEffect, useRef, useState } from 'react'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'
import { resolveJavDiscoveryDetails } from '@/api'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

function metadataList(value) {
  if (!Array.isArray(value)) return []
  return value.map((item) => String(item || '').trim()).filter(Boolean)
}

function formatReleaseDate(releaseUnix) {
  const value = Number(releaseUnix)
  if (!Number.isFinite(value) || value <= 0) return zh('未知', 'Unknown')
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(new Date(value * 1000))
}

export default function JavDiscoveryDetailModal({
  item,
  wantedBusy = false,
  onClose,
  onResolved,
  onToggleWanted,
}) {
  const dialogRef = useRef(null)
  const itemRef = useRef(item)
  itemRef.current = item
  const itemId = item.id
  const [resolvedItem, setResolvedItem] = useState(item)
  const [detailsLoading, setDetailsLoading] = useState(true)
  const [detailsError, setDetailsError] = useState('')
  const [detailsResolved, setDetailsResolved] = useState(false)
  const [coverLoaded, setCoverLoaded] = useState(false)
  const [coverError, setCoverError] = useState(false)
  const [reloadToken, setReloadToken] = useState(0)
  const displayItem = resolvedItem || item
  const metadata = displayItem?.metadata || {}
  const titleId = `jav-discovery-detail-title-${displayItem?.id || 'item'}`
  const actresses = metadataList(metadata.actresses)
  const tags = metadataList(metadata.tags)
  const subscriptions = metadataList(displayItem?.subscriptions)
  const coverPath = `/jav/discovery/items/${encodeURIComponent(displayItem?.id)}/cover?v=${encodeURIComponent(
    displayItem?.updated_at || ''
  )}`

  useEffect(() => {
    let cancelled = false
    const currentItem = itemRef.current
    setResolvedItem(currentItem)
    setDetailsLoading(true)
    setDetailsError('')
    setDetailsResolved(false)
    setCoverLoaded(false)
    setCoverError(false)
    void resolveJavDiscoveryDetails(itemId)
      .then((payload) => {
        if (cancelled) return
        const next = {
          ...currentItem,
          metadata: payload?.metadata || currentItem.metadata || {},
          release_unix: Number(payload?.release_unix) || currentItem.release_unix,
          updated_at: payload?.updated_at || currentItem.updated_at,
        }
        setResolvedItem(next)
        setDetailsResolved(true)
        onResolved?.(next)
      })
      .catch((error) => {
        if (!cancelled) setDetailsError(getErrorMessage(error))
      })
      .finally(() => {
        if (!cancelled) setDetailsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [itemId, onResolved, reloadToken])

  useEffect(() => {
    setResolvedItem((current) =>
      current?.id === item.id ? { ...current, owned: item.owned, wanted: item.wanted } : current
    )
  }, [item.id, item.owned, item.wanted])

  useEffect(() => {
    const previousBodyOverflow = document.body.style.overflow
    const previousHtmlOverflow = document.documentElement.style.overflow
    const handleKeyDown = (event) => {
      if (event.key === 'Escape') onClose?.()
    }
    document.body.style.overflow = 'hidden'
    document.documentElement.style.overflow = 'hidden'
    document.addEventListener('keydown', handleKeyDown)
    dialogRef.current?.focus()
    return () => {
      document.body.style.overflow = previousBodyOverflow
      document.documentElement.style.overflow = previousHtmlOverflow
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [onClose])

  const rows = [
    [zh('番号', 'Code'), displayItem?.code || zh('未知', 'Unknown')],
    [zh('发布日期', 'Release date'), formatReleaseDate(displayItem?.release_unix)],
    [
      zh('时长', 'Runtime'),
      Number(metadata.duration_min) > 0
        ? zh(`${metadata.duration_min} 分钟`, `${metadata.duration_min} min`)
        : zh('未知', 'Unknown'),
    ],
    [zh('厂商', 'Studio'), metadata.studio || zh('未知', 'Unknown')],
    [zh('系列', 'Series'), metadata.series || zh('未知', 'Unknown')],
    [
      zh('类型', 'Type'),
      typeof metadata.is_uncensored === 'boolean'
        ? metadata.is_uncensored
          ? zh('无码', 'Uncensored')
          : zh('有码', 'Censored')
        : zh('未知', 'Unknown'),
    ],
    [
      zh('订阅来源', 'Subscriptions'),
      subscriptions.length > 0 ? subscriptions.join('、') : zh('未知', 'Unknown'),
    ],
    [zh('数据来源', 'Source'), metadata.source || 'JavBus'],
  ]

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/70 p-3 backdrop-blur-[2px] sm:p-6"
      role="presentation"
    >
      <button
        type="button"
        className="absolute inset-0 cursor-default"
        aria-label={zh('关闭发现作品详情', 'Close discovery item details')}
        onClick={onClose}
      />
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className="relative z-10 flex max-h-[95vh] w-full max-w-[77rem] flex-col overflow-hidden rounded-xl bg-white shadow-2xl outline-none"
      >
        <div className="flex items-start justify-between gap-3 border-b border-gray-200 px-4 py-3 sm:px-5">
          <h2
            id={titleId}
            className="min-w-0 break-words text-sm font-semibold leading-5 text-gray-900 sm:text-base sm:leading-6"
          >
            {metadata.title || displayItem?.code || zh('发现作品详情', 'Discovery item details')}
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label={zh('关闭', 'Close')}
            className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-gray-500 hover:bg-gray-100 hover:text-gray-900"
          >
            <CloseOutlinedIcon sx={{ fontSize: 18 }} />
          </button>
        </div>

        <div className="min-h-0 overflow-y-auto p-4 sm:p-6">
          {detailsLoading ? (
            <div
              className="mb-4 rounded-lg border border-blue-100 bg-blue-50 px-3 py-2 text-sm text-blue-700"
              role="status"
            >
              {zh('正在从 JavBus 加载完整详情…', 'Loading full details from JavBus...')}
            </div>
          ) : null}
          {detailsError ? (
            <div
              className="mb-4 flex flex-wrap items-center justify-between gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700"
              role="alert"
            >
              <span>{detailsError}</span>
              <button
                type="button"
                onClick={() => setReloadToken((current) => current + 1)}
                className="rounded border border-red-200 bg-white px-2.5 py-1 text-xs font-medium hover:bg-red-100"
              >
                {zh('重试', 'Retry')}
              </button>
            </div>
          ) : null}
          <div className="grid gap-6 md:grid-cols-[minmax(20rem,28rem)_minmax(0,1fr)] xl:grid-cols-[minmax(26rem,40rem)_minmax(0,32rem)] xl:justify-center">
            <div className="relative mx-auto h-[min(48vh,33.333rem)] w-full max-w-[28rem] overflow-hidden rounded-lg border border-gray-200 bg-gray-100 shadow-sm xl:max-w-[40rem]">
              {detailsResolved && metadata.cover_url ? (
                <>
                  {!coverLoaded && !coverError ? (
                    <div
                      className="absolute inset-0 flex items-center justify-center px-4 text-center text-sm text-gray-400"
                      role="status"
                    >
                      {zh('正在加载完整封面…', 'Loading full-size cover...')}
                    </div>
                  ) : null}
                  <img
                    src={coverPath}
                    alt={displayItem?.code || zh('JAV 封面', 'JAV cover')}
                    onLoad={() => setCoverLoaded(true)}
                    onError={() => setCoverError(true)}
                    className={`block h-full w-full object-contain transition-opacity ${
                      coverLoaded ? 'opacity-100' : 'opacity-0'
                    }`}
                  />
                  {coverError ? (
                    <div className="absolute inset-0 flex items-center justify-center px-4 text-center text-sm text-gray-400">
                      {zh('完整封面加载失败', 'Failed to load full-size cover')}
                    </div>
                  ) : null}
                </>
              ) : (
                <div className="flex h-full items-center justify-center px-4 text-center text-sm text-gray-400">
                  {detailsLoading
                    ? zh('正在获取完整封面…', 'Fetching full-size cover...')
                    : zh('暂无完整封面', 'No full-size cover')}
                </div>
              )}
              <div className="absolute left-2 top-2 flex flex-wrap gap-1.5">
                {displayItem?.owned ? (
                  <span className="rounded bg-emerald-600/95 px-2 py-1 text-xs font-semibold text-white shadow">
                    {zh('已拥有', 'Owned')}
                  </span>
                ) : null}
                {displayItem?.wanted ? (
                  <span className="rounded bg-rose-600/95 px-2 py-1 text-xs font-semibold text-white shadow">
                    {zh('我想要', 'Wanted')}
                  </span>
                ) : null}
              </div>
            </div>

            <div className="flex min-w-0 flex-col gap-3">
              <dl className="overflow-hidden rounded-lg border border-gray-200">
                {rows.map(([label, value], index) => (
                  <div
                    key={label}
                    className={`grid grid-cols-[5rem_minmax(0,1fr)] gap-2.5 px-3 py-2 text-[13px] ${
                      index > 0 ? 'border-t border-gray-100' : ''
                    }`}
                  >
                    <dt className="font-medium text-gray-500">{label}</dt>
                    <dd className="min-w-0 break-words text-gray-800">{value}</dd>
                  </div>
                ))}
              </dl>

              {actresses.length > 0 ? (
                <section>
                  <h3 className="mb-1.5 text-xs font-semibold text-gray-800">
                    {zh('女优', 'Actresses')}
                  </h3>
                  <div className="flex flex-wrap gap-1.5">
                    {actresses.map((actress) => (
                      <span
                        key={actress}
                        className="rounded-full border border-purple-200 bg-purple-50 px-2.5 py-0.5 text-[11px] font-medium text-purple-700"
                      >
                        {actress}
                      </span>
                    ))}
                  </div>
                </section>
              ) : null}

              {tags.length > 0 ? (
                <section>
                  <h3 className="mb-1.5 text-xs font-semibold text-gray-800">
                    {zh('标签', 'Tags')}
                  </h3>
                  <div className="flex flex-wrap gap-1.5">
                    {tags.map((tag) => (
                      <span
                        key={tag}
                        className="rounded bg-orange-100 px-2 py-0.5 text-[11px] font-medium text-orange-700"
                      >
                        {tag}
                      </span>
                    ))}
                  </div>
                </section>
              ) : null}

              <section className="mt-auto">
                <h3 className="mb-1.5 text-xs font-semibold text-gray-800">
                  {zh('操作', 'Actions')}
                </h3>
                <div className="flex flex-wrap gap-2">
                  <button
                    type="button"
                    disabled={wantedBusy}
                    onClick={() => onToggleWanted?.(displayItem)}
                    className={`rounded-md px-3 py-1.5 text-xs font-semibold ${
                      displayItem?.wanted
                        ? 'border border-rose-200 bg-rose-50 text-rose-700 hover:bg-rose-100'
                        : 'bg-rose-600 text-white hover:bg-rose-700'
                    } disabled:opacity-50`}
                  >
                    {displayItem?.wanted
                      ? zh('移出我想要的', 'Remove from wanted')
                      : zh('加入我想要的', 'Add to wanted')}
                  </button>
                  {metadata.detail_url ? (
                    <a
                      href={metadata.detail_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="rounded-md border border-gray-300 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-50"
                    >
                      {zh('在 JavBus 查看', 'View on JavBus')}
                    </a>
                  ) : null}
                </div>
              </section>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
