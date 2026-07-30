import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  createJavDiscoverySubscription,
  deleteJavDiscoverySubscription,
  fetchJavDiscoveryItems,
  fetchJavDiscoverySubscriptions,
  triggerJavDiscoverySync,
  updateJavDiscoveryWanted,
} from '@/api'
import JavDiscoveryDetailModal from '@/components/JavDiscoveryDetailModal'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

const PAGE_SIZE = 48

function formatReleaseDate(releaseUnix) {
  const value = Number(releaseUnix)
  if (!Number.isFinite(value) || value <= 0) return zh('日期未知', 'Unknown date')
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(new Date(value * 1000))
}

function formatSyncTime(value) {
  if (!value) return zh('等待首次同步', 'Waiting for first sync')
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return zh('同步时间未知', 'Unknown sync time')
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

export default function JavDiscoveryView() {
  const [subscriptions, setSubscriptions] = useState([])
  const [items, setItems] = useState([])
  const [total, setTotal] = useState(0)
  const [activeTab, setActiveTab] = useState('subscriptions')
  const [page, setPage] = useState(1)
  const [name, setName] = useState('')
  const [referenceCode, setReferenceCode] = useState('')
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [busyItemIds, setBusyItemIds] = useState(() => new Set())
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [selectedItem, setSelectedItem] = useState(null)

  const wantedOnly = activeTab === 'wanted'
  const lastPage = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const loadSubscriptions = useCallback(async () => {
    const result = await fetchJavDiscoverySubscriptions()
    setSubscriptions(Array.isArray(result) ? result : [])
  }, [])

  const loadItems = useCallback(async () => {
    const result = await fetchJavDiscoveryItems({
      wanted: wantedOnly,
      limit: PAGE_SIZE,
      offset: (page - 1) * PAGE_SIZE,
    })
    setItems(Array.isArray(result?.items) ? result.items : [])
    setTotal(Number(result?.total) || 0)
  }, [page, wantedOnly])

  const refresh = useCallback(
    async ({ quiet = false } = {}) => {
      if (!quiet) setLoading(true)
      try {
        await Promise.all([loadSubscriptions(), loadItems()])
        if (!quiet) setError('')
      } catch (loadError) {
        if (!quiet) setError(getErrorMessage(loadError))
      } finally {
        if (!quiet) setLoading(false)
      }
    },
    [loadItems, loadSubscriptions]
  )

  useEffect(() => {
    refresh()
  }, [refresh])

  useEffect(() => {
    const timer = window.setInterval(() => refresh({ quiet: true }), 20000)
    return () => window.clearInterval(timer)
  }, [refresh])

  useEffect(() => {
    if (page > lastPage) setPage(lastPage)
  }, [lastPage, page])

  const handleCreate = async (event) => {
    event.preventDefault()
    const cleanName = name.trim()
    const cleanCode = referenceCode.trim()
    if (!cleanName || !cleanCode) {
      setError(zh('请输入女优名和一个单体作品番号', 'Enter an idol name and one solo work code'))
      return
    }
    setSubmitting(true)
    setError('')
    setNotice('')
    try {
      await createJavDiscoverySubscription({ name: cleanName, referenceCode: cleanCode })
      setName('')
      setReferenceCode('')
      setNotice(zh('订阅已添加，后台同步已安排', 'Subscription added; background sync scheduled'))
      await loadSubscriptions()
    } catch (createError) {
      setError(getErrorMessage(createError))
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (subscription) => {
    const confirmed = window.confirm(
      zh(
        `确定取消订阅“${subscription.name}”吗？已发现的作品会保留。`,
        `Unsubscribe from “${subscription.name}”? Discovered items will be kept.`
      )
    )
    if (!confirmed) return
    setError('')
    try {
      await deleteJavDiscoverySubscription(subscription.id)
      await loadSubscriptions()
    } catch (deleteError) {
      setError(getErrorMessage(deleteError))
    }
  }

  const handleSync = async () => {
    setSyncing(true)
    setError('')
    setNotice('')
    try {
      await triggerJavDiscoverySync()
      setNotice(
        zh('同步已安排，页面会自动刷新结果', 'Sync scheduled; results will refresh automatically')
      )
    } catch (syncError) {
      setError(getErrorMessage(syncError))
    } finally {
      setSyncing(false)
    }
  }

  const handleWanted = async (item) => {
    const nextWanted = !item.wanted
    setBusyItemIds((current) => new Set(current).add(item.id))
    setItems((current) =>
      current.map((entry) => (entry.id === item.id ? { ...entry, wanted: nextWanted } : entry))
    )
    setSelectedItem((current) =>
      current?.id === item.id ? { ...current, wanted: nextWanted } : current
    )
    try {
      await updateJavDiscoveryWanted(item.id, nextWanted)
      if (wantedOnly && !nextWanted) {
        setItems((current) => current.filter((entry) => entry.id !== item.id))
        setTotal((current) => Math.max(0, current - 1))
      }
    } catch (updateError) {
      setItems((current) =>
        current.map((entry) => (entry.id === item.id ? { ...entry, wanted: item.wanted } : entry))
      )
      setSelectedItem((current) =>
        current?.id === item.id ? { ...current, wanted: item.wanted } : current
      )
      setError(getErrorMessage(updateError))
    } finally {
      setBusyItemIds((current) => {
        const next = new Set(current)
        next.delete(item.id)
        return next
      })
    }
  }

  const subscriptionCountLabel = useMemo(
    () => zh(`${subscriptions.length} 个女优订阅`, `${subscriptions.length} idol subscriptions`),
    [subscriptions.length]
  )
  const handleDetailsResolved = useCallback((resolved) => {
    setItems((current) =>
      current.map((item) => (item.id === resolved.id ? { ...item, ...resolved } : item))
    )
    setSelectedItem((current) =>
      current?.id === resolved.id ? { ...current, ...resolved } : current
    )
  }, [])
  const handleTabChange = (tab) => {
    if (tab === activeTab) return
    setActiveTab(tab)
    setPage(1)
    setError('')
    setNotice('')
  }
  const tabs = [
    {
      id: 'subscriptions',
      label: zh('订阅规则', 'Subscription rules'),
    },
    {
      id: 'discovered',
      label: zh('已发现', 'Discovered'),
    },
    {
      id: 'wanted',
      label: zh('我想要', 'Wanted'),
    },
  ]

  return (
    <div className="mx-auto w-full max-w-[1600px] px-4 py-4 sm:px-6">
      <div className="lg:ml-10">
        <aside className="mb-4 lg:fixed lg:left-0 lg:top-[calc(var(--topbar-height)+1rem)] lg:z-30 lg:mb-0 lg:w-24">
          <div className="rounded-xl border border-gray-200 bg-white p-1.5 shadow-sm lg:rounded-l-none lg:border-l-0">
            <div
              className="flex gap-1 overflow-x-auto lg:flex-col"
              role="tablist"
              aria-label={zh('发现页面导航', 'Discovery navigation')}
            >
              {tabs.map((tab) => {
                const selected = activeTab === tab.id
                return (
                  <button
                    key={tab.id}
                    type="button"
                    role="tab"
                    aria-selected={selected}
                    aria-controls="jav-discovery-tab-panel"
                    onClick={() => handleTabChange(tab.id)}
                    className={`min-w-max whitespace-nowrap rounded-lg px-2.5 py-2 text-center text-sm font-semibold transition lg:w-full ${
                      selected
                        ? tab.id === 'wanted'
                          ? 'bg-rose-50 text-rose-700 ring-1 ring-inset ring-rose-200'
                          : 'bg-blue-50 text-blue-700 ring-1 ring-inset ring-blue-200'
                        : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'
                    }`}
                  >
                    {tab.label}
                  </button>
                )
              })}
            </div>
          </div>
        </aside>

        <main id="jav-discovery-tab-panel" role="tabpanel" className="min-w-0">
          {error ? (
            <div role="alert" className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
              {error}
            </div>
          ) : null}
          {notice ? (
            <div
              role="status"
              className="mb-4 rounded-lg bg-green-50 px-3 py-2 text-sm text-green-700"
            >
              {notice}
            </div>
          ) : null}

          {activeTab === 'subscriptions' ? (
            <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm">
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <h2 className="text-2xl font-bold text-gray-900">
                    {zh('订阅规则', 'Subscription rules')}
                  </h2>
                  <p className="mt-1 max-w-3xl text-sm text-gray-500">
                    {zh(
                      '输入女优名和一个对应的单体作品番号，通过 JavBus 校验后建立订阅。',
                      'Enter an idol name and one matching solo work code to validate and subscribe through JavBus.'
                    )}
                  </p>
                </div>
                <button
                  type="button"
                  onClick={handleSync}
                  disabled={syncing || subscriptions.length === 0}
                  className="rounded-lg border border-blue-200 bg-blue-50 px-4 py-2 text-sm font-medium text-blue-700 hover:bg-blue-100 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {syncing ? zh('安排中…', 'Scheduling...') : zh('立即同步', 'Sync now')}
                </button>
              </div>

              <form onSubmit={handleCreate} className="mt-5 grid gap-3 md:grid-cols-[1fr_1fr_auto]">
                <label className="block">
                  <span className="mb-1 block text-xs font-medium text-gray-600">
                    {zh('女优名', 'Idol name')}
                  </span>
                  <input
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    placeholder={zh('需与 JavBus 作品页一致', 'Must match the JavBus detail page')}
                    className="h-10 w-full rounded-lg border border-gray-300 px-3 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                  />
                </label>
                <label className="block">
                  <span className="mb-1 block text-xs font-medium text-gray-600">
                    {zh('单体作品番号', 'Solo work code')}
                  </span>
                  <input
                    value={referenceCode}
                    onChange={(event) => setReferenceCode(event.target.value)}
                    placeholder="ABP-123"
                    className="h-10 w-full rounded-lg border border-gray-300 px-3 text-sm uppercase outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                  />
                </label>
                <button
                  type="submit"
                  disabled={submitting}
                  className="mt-auto h-10 rounded-lg bg-blue-600 px-5 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {submitting
                    ? zh('JavBus 校验中…', 'Validating with JavBus...')
                    : zh('添加订阅', 'Add')}
                </button>
              </form>

              <div className="mt-5 border-t border-gray-100 pt-4">
                <div className="mb-3 text-xs font-semibold uppercase tracking-wide text-gray-500">
                  {subscriptionCountLabel}
                </div>
                {subscriptions.length === 0 ? (
                  <p className="text-sm text-gray-400">
                    {zh(
                      '还没有订阅，添加后会自动开始首次同步。',
                      'No subscriptions yet. The first sync starts automatically after adding one.'
                    )}
                  </p>
                ) : (
                  <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
                    {subscriptions.map((subscription) => (
                      <div
                        key={subscription.id}
                        className="flex items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2"
                      >
                        <div className="min-w-0">
                          <div className="truncate text-sm font-semibold text-gray-800">
                            {subscription.name}
                          </div>
                          <div className="truncate text-xs text-gray-500">
                            {subscription.reference_code} ·{' '}
                            {formatSyncTime(subscription.last_synced_at)}
                          </div>
                          {subscription.last_error ? (
                            <div
                              className="truncate text-xs text-red-600"
                              title={subscription.last_error}
                            >
                              {zh('上次同步失败', 'Last sync failed')}
                            </div>
                          ) : null}
                        </div>
                        <button
                          type="button"
                          onClick={() => handleDelete(subscription)}
                          className="shrink-0 rounded px-2 py-1 text-xs text-gray-500 hover:bg-red-50 hover:text-red-600"
                        >
                          {zh('取消', 'Remove')}
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </section>
          ) : (
            <section>
              <div className="flex flex-wrap items-start justify-between gap-4 rounded-2xl border border-gray-200 bg-white px-5 py-4 shadow-sm">
                <div>
                  <h2 className="text-2xl font-bold text-gray-900">
                    {wantedOnly ? zh('我想要', 'Wanted') : zh('已发现', 'Discovered')}
                  </h2>
                  <p className="mt-1 text-sm text-gray-500">
                    {wantedOnly
                      ? zh(
                          '“我想要”是已发现作品的子集。',
                          'Wanted works are a subset of discovered works.'
                        )
                      : zh(
                          '后台根据订阅规则从 JavBus 自动收集的作品。',
                          'Works automatically collected from JavBus using your subscription rules.'
                        )}
                  </p>
                </div>
                <span className="pt-1 text-sm text-gray-500">
                  {zh(`共 ${total} 部`, `${total} items`)}
                </span>
              </div>

              {loading ? (
                <div className="mt-4 flex min-h-48 items-center justify-center rounded-xl border border-dashed border-gray-200 text-sm text-gray-400">
                  {zh('加载发现作品…', 'Loading discovered items...')}
                </div>
              ) : items.length === 0 ? (
                <div className="mt-4 flex min-h-48 items-center justify-center rounded-xl border border-dashed border-gray-200 bg-white text-sm text-gray-400">
                  {wantedOnly
                    ? zh('还没有标记想要的作品', 'No wanted items yet')
                    : zh('等待后台完成首次同步', 'Waiting for the first background sync')}
                </div>
              ) : (
                <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-7">
                  {items.map((item) => {
                    const metadata = item.metadata || {}
                    return (
                      <article
                        key={item.id}
                        className="group relative overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm transition hover:border-blue-300 hover:shadow-md"
                      >
                        <button
                          type="button"
                          onClick={() => setSelectedItem(item)}
                          aria-label={zh(`查看 ${item.code} 详情`, `View details for ${item.code}`)}
                          className="absolute inset-0 z-10 cursor-pointer rounded-xl focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500"
                        />
                        <div className="relative block aspect-[2/3] overflow-hidden bg-gray-100">
                          {metadata.cover_url ? (
                            <img
                              src={`/jav/discovery/items/${encodeURIComponent(item.id)}/thumbnail?v=${encodeURIComponent(
                                item.updated_at || ''
                              )}`}
                              alt=""
                              loading="lazy"
                              referrerPolicy="no-referrer"
                              onError={(event) => {
                                event.currentTarget.style.display = 'none'
                              }}
                              className="h-full w-full object-cover"
                            />
                          ) : null}
                          {item.owned ? (
                            <span className="absolute left-2 top-2 rounded bg-emerald-600/95 px-2 py-1 text-[11px] font-semibold text-white shadow">
                              {zh('已拥有', 'Owned')}
                            </span>
                          ) : null}
                        </div>
                        <div className="p-3">
                          <div className="flex items-start justify-between gap-2">
                            <div className="font-mono text-sm font-bold text-gray-900">
                              {item.code}
                            </div>
                            <button
                              type="button"
                              disabled={busyItemIds.has(item.id)}
                              onClick={(event) => {
                                event.stopPropagation()
                                handleWanted(item)
                              }}
                              className={`relative z-20 shrink-0 rounded-full px-2 py-1 text-xs font-medium ${
                                item.wanted
                                  ? 'bg-rose-100 text-rose-700 hover:bg-rose-200'
                                  : 'bg-gray-100 text-gray-500 hover:bg-gray-200'
                              } disabled:opacity-50`}
                            >
                              {item.wanted ? zh('已想要', 'Wanted') : zh('想要', 'Want')}
                            </button>
                          </div>
                          <div className="mt-1 line-clamp-2 min-h-10 text-xs leading-5 text-gray-600">
                            {metadata.title || zh('暂无标题', 'No title')}
                          </div>
                          <div className="mt-2 text-xs text-gray-400">
                            {formatReleaseDate(item.release_unix)}
                          </div>
                          {Array.isArray(item.subscriptions) && item.subscriptions.length > 0 ? (
                            <div className="mt-1 truncate text-xs text-blue-500">
                              {item.subscriptions.join('、')}
                            </div>
                          ) : null}
                        </div>
                      </article>
                    )
                  })}
                </div>
              )}

              {lastPage > 1 ? (
                <div className="mt-5 flex items-center justify-center gap-3">
                  <button
                    type="button"
                    disabled={page <= 1}
                    onClick={() => setPage((current) => Math.max(1, current - 1))}
                    className="rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-600 disabled:opacity-40"
                  >
                    {zh('上一页', 'Previous')}
                  </button>
                  <span className="text-sm text-gray-500">
                    {page} / {lastPage}
                  </span>
                  <button
                    type="button"
                    disabled={page >= lastPage}
                    onClick={() => setPage((current) => Math.min(lastPage, current + 1))}
                    className="rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-600 disabled:opacity-40"
                  >
                    {zh('下一页', 'Next')}
                  </button>
                </div>
              ) : null}
            </section>
          )}
        </main>
      </div>
      {selectedItem ? (
        <JavDiscoveryDetailModal
          item={selectedItem}
          wantedBusy={busyItemIds.has(selectedItem.id)}
          onClose={() => setSelectedItem(null)}
          onResolved={handleDetailsResolved}
          onToggleWanted={handleWanted}
        />
      ) : null}
    </div>
  )
}
