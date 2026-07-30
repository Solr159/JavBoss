import { useEffect, useRef } from 'react'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'
import { zh } from '@/utils/i18n'

function metadataList(value) {
  if (!Array.isArray(value)) return []
  return value.map((item) => String(item || '').trim()).filter(Boolean)
}

export default function JavDiscoveryDetailModal({
  item,
  releaseText,
  wantedBusy = false,
  onClose,
  onToggleWanted,
}) {
  const dialogRef = useRef(null)
  const metadata = item?.metadata || {}
  const titleId = `jav-discovery-detail-title-${item?.id || 'item'}`
  const actresses = metadataList(metadata.actresses)
  const tags = metadataList(metadata.tags)
  const subscriptions = metadataList(item?.subscriptions)
  const coverPath = `/jav/discovery/items/${encodeURIComponent(item?.id)}/cover?v=${encodeURIComponent(
    item?.updated_at || ''
  )}`

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
    [zh('番号', 'Code'), item?.code || zh('未知', 'Unknown')],
    [zh('发布日期', 'Release date'), releaseText],
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
        className="relative z-10 flex max-h-[92vh] w-full max-w-4xl flex-col overflow-hidden rounded-xl bg-white shadow-2xl outline-none"
      >
        <div className="flex items-center justify-between gap-3 border-b border-gray-200 px-4 py-2 sm:px-5">
          <h2
            id={titleId}
            className="min-w-0 truncate text-sm font-semibold text-gray-900 sm:text-base"
          >
            {metadata.title || item?.code || zh('发现作品详情', 'Discovery item details')}
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
          <div className="grid gap-6 md:grid-cols-[minmax(13rem,18rem)_minmax(0,1fr)]">
            <div className="relative mx-auto aspect-[2/3] w-full max-w-72 overflow-hidden rounded-lg border border-gray-200 bg-gray-100 shadow-sm">
              {metadata.cover_url ? (
                <img
                  src={coverPath}
                  alt={item?.code || zh('JAV 封面', 'JAV cover')}
                  className="h-full w-full object-cover"
                />
              ) : (
                <div className="flex h-full items-center justify-center text-sm text-gray-400">
                  {zh('暂无封面', 'No cover')}
                </div>
              )}
              <div className="absolute left-2 top-2 flex flex-wrap gap-1.5">
                {item?.owned ? (
                  <span className="rounded bg-emerald-600/95 px-2 py-1 text-xs font-semibold text-white shadow">
                    {zh('已拥有', 'Owned')}
                  </span>
                ) : null}
                {item?.wanted ? (
                  <span className="rounded bg-rose-600/95 px-2 py-1 text-xs font-semibold text-white shadow">
                    {zh('我想要', 'Wanted')}
                  </span>
                ) : null}
              </div>
            </div>

            <div className="flex min-w-0 flex-col gap-5">
              <dl className="overflow-hidden rounded-lg border border-gray-200">
                {rows.map(([label, value], index) => (
                  <div
                    key={label}
                    className={`grid grid-cols-[5.5rem_minmax(0,1fr)] gap-3 px-4 py-2.5 text-sm ${
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
                  <h3 className="mb-2 text-sm font-semibold text-gray-800">
                    {zh('女优', 'Actresses')}
                  </h3>
                  <div className="flex flex-wrap gap-2">
                    {actresses.map((actress) => (
                      <span
                        key={actress}
                        className="rounded-full border border-purple-200 bg-purple-50 px-3 py-1 text-xs font-medium text-purple-700"
                      >
                        {actress}
                      </span>
                    ))}
                  </div>
                </section>
              ) : null}

              {tags.length > 0 ? (
                <section>
                  <h3 className="mb-2 text-sm font-semibold text-gray-800">{zh('标签', 'Tags')}</h3>
                  <div className="flex flex-wrap gap-2">
                    {tags.map((tag) => (
                      <span
                        key={tag}
                        className="rounded bg-orange-100 px-2.5 py-1 text-xs font-medium text-orange-700"
                      >
                        {tag}
                      </span>
                    ))}
                  </div>
                </section>
              ) : null}

              <section className="mt-auto">
                <h3 className="mb-2 text-sm font-semibold text-gray-800">
                  {zh('操作', 'Actions')}
                </h3>
                <div className="flex flex-wrap gap-2">
                  <button
                    type="button"
                    disabled={wantedBusy}
                    onClick={() => onToggleWanted?.(item)}
                    className={`rounded-md px-3 py-2 text-xs font-semibold ${
                      item?.wanted
                        ? 'border border-rose-200 bg-rose-50 text-rose-700 hover:bg-rose-100'
                        : 'bg-rose-600 text-white hover:bg-rose-700'
                    } disabled:opacity-50`}
                  >
                    {item?.wanted
                      ? zh('移出我想要的', 'Remove from wanted')
                      : zh('加入我想要的', 'Add to wanted')}
                  </button>
                  {metadata.detail_url ? (
                    <a
                      href={metadata.detail_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="rounded-md border border-gray-300 bg-white px-3 py-2 text-xs font-medium text-gray-700 hover:bg-gray-50"
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
