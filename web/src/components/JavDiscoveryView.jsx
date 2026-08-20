import { useCallback, useEffect, useMemo, useState } from 'react'
import FavoriteBorderRoundedIcon from '@mui/icons-material/FavoriteBorderRounded'
import FavoriteRoundedIcon from '@mui/icons-material/FavoriteRounded'
import {
  createJavDiscoverySubscription,
  createDiscoveryDownload,
  deleteJavDiscoverySubscription,
  fetchJavDiscoveryItems,
  fetchJavDiscoverySubscriptions,
  loadMoreJavDiscoveryHistory,
  triggerJavDiscoverySync,
  updateJavDiscoveryWanted,
} from '@/api'
import JavDiscoveryDetailModal from '@/components/JavDiscoveryDetailModal'
import JavDiscoveryDownloaderSettingsView from '@/components/JavDiscoveryDownloaderSettingsView'
import JavDiscoveryDownloadsView from '@/components/JavDiscoveryDownloadsView'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

const PAGE_SIZE = 49

function ShowOwnedOption({ checked, compact = false, onChange }) {
  return (
    <label
      className={`flex cursor-pointer items-center rounded-lg border border-gray-200 bg-white text-gray-600 ${
        compact ? 'h-8 gap-1.5 px-2.5 text-xs' : 'h-10 gap-2 px-3 text-sm'
      }`}
    >
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className={`${compact ? 'h-3.5 w-3.5' : 'h-4 w-4'} accent-blue-600`}
      />
      <span>{zh('显示已拥有的作品', 'Show owned works')}</span>
    </label>
  )
}

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
  const [selectedSubscriptionId, setSelectedSubscriptionId] = useState('')
  const [showOwned, setShowOwned] = useState(false)
  const [page, setPage] = useState(1)
  const [referenceCode, setReferenceCode] = useState('')
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [loadingHistory, setLoadingHistory] = useState(false)
  const [busyItemIds, setBusyItemIds] = useState(() => new Set())
  const [busyMagnetUrls, setBusyMagnetUrls] = useState(() => new Set())
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [selectedItem, setSelectedItem] = useState(null)

  const wantedOnly = activeTab === 'wanted'
  const lastPage = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const loadSubscriptions = useCallback(async () => {
    const result = await fetchJavDiscoverySubscriptions()
    const nextSubscriptions = Array.isArray(result) ? result : []
    setSubscriptions(nextSubscriptions)
    setSelectedSubscriptionId((current) =>
      current && !nextSubscriptions.some((subscription) => String(subscription.id) === current)
        ? ''
        : current
    )
  }, [])

  const loadItems = useCallback(async () => {
    const result = await fetchJavDiscoveryItems({
      wanted: wantedOnly,
      includeOwned: showOwned,
      subscriptionId: selectedSubscriptionId,
      limit: PAGE_SIZE,
      offset: (page - 1) * PAGE_SIZE,
    })
    setItems(Array.isArray(result?.items) ? result.items : [])
    setTotal(Number(result?.total) || 0)
  }, [page, selectedSubscriptionId, showOwned, wantedOnly])

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
    const cleanCode = referenceCode.trim()
    if (!cleanCode) {
      setError(zh('请输入一个单体作品番号', 'Enter a solo work code'))
      return
    }
    setSubmitting(true)
    setError('')
    setNotice('')
    try {
      await createJavDiscoverySubscription({ referenceCode: cleanCode })
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

  const handleDownload = async (item, magnet) => {
    const magnetUrl = String(magnet?.url || '')
    if (!magnetUrl) return
    setBusyMagnetUrls((current) => new Set(current).add(magnetUrl))
    setError('')
    setNotice('')
    try {
      await createDiscoveryDownload(item.id, magnetUrl)
      setNotice(
        zh(
          `${item.code} 已加入下载队列，可在“下载队列”中查看进度`,
          `${item.code} was added to the download queue; track it under Downloads`
        )
      )
      setSelectedItem(null)
      setActiveTab('downloads')
    } catch (downloadError) {
      setError(getErrorMessage(downloadError))
    } finally {
      setBusyMagnetUrls((current) => {
        const next = new Set(current)
        next.delete(magnetUrl)
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
  const handleSubscriptionFilterChange = (event) => {
    setSelectedSubscriptionId(event.target.value)
    setPage(1)
    setError('')
    setNotice('')
  }
  const handleShowOwnedChange = (checked) => {
    setShowOwned(checked)
    setPage(1)
    setError('')
    setNotice('')
  }
  const handleLoadMoreHistory = async () => {
    const subscriptionId = Number(selectedSubscriptionId)
    if (!Number.isFinite(subscriptionId) || subscriptionId <= 0) return
    setLoadingHistory(true)
    setError('')
    setNotice('')
    try {
      const result = await loadMoreJavDiscoveryHistory(subscriptionId)
      const loaded = Number(result?.loaded) || 0
      setNotice(
        loaded > 0
          ? zh(`已加载 ${loaded} 部历史作品`, `Loaded ${loaded} historical works`)
          : zh('没有更多历史作品', 'No more historical works')
      )
      await loadItems()
    } catch (historyError) {
      setError(getErrorMessage(historyError))
    } finally {
      setLoadingHistory(false)
    }
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
    {
      id: 'downloads',
      label: zh('下载队列', 'Downloads'),
    },
    {
      id: 'downloader_settings',
      label: zh('下载器设置', 'Downloader settings'),
    },
  ]

  return (
    <div className="w-full px-4 py-4 sm:px-6">
      <div className="lg:ml-[max(0rem,calc(11rem-var(--page-left-padding)))]">
        <aside className="mb-4 lg:fixed lg:bottom-0 lg:left-[var(--side-tabs-width)] lg:top-[var(--topbar-height)] lg:z-30 lg:mb-0 lg:w-44">
          <div className="rounded-xl border border-gray-200 bg-white p-1.5 shadow-sm lg:h-full lg:rounded-none lg:border-0 lg:border-r lg:p-0 lg:shadow-none">
            <div
              className="flex gap-1 overflow-x-auto lg:flex-col lg:gap-0 lg:pt-4"
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
                    className={`min-w-max whitespace-nowrap rounded-lg border-l-4 px-2.5 py-2 text-center text-sm font-semibold transition lg:w-full lg:rounded-none lg:px-5 lg:py-3 lg:text-left ${
                      selected
                        ? tab.id === 'wanted'
                          ? 'border-rose-500 bg-rose-50 text-rose-700'
                          : 'border-blue-500 bg-blue-50 text-blue-700'
                        : 'border-transparent text-gray-600 hover:bg-gray-50 hover:text-gray-900'
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
                      '输入一个单体作品番号，系统会从 JavBus 作品页识别女优并建立订阅。',
                      'Enter a solo work code; the actress will be identified from the JavBus detail page.'
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

              <form onSubmit={handleCreate} className="mt-5 grid gap-3 md:grid-cols-[1fr_auto]">
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
          ) : activeTab === 'downloads' ? (
            <JavDiscoveryDownloadsView />
          ) : activeTab === 'downloader_settings' ? (
            <JavDiscoveryDownloaderSettingsView />
          ) : (
            <section>
              <div className="sticky top-[var(--topbar-height)] z-30 ml-[calc(-1rem-var(--page-left-padding))] mr-[calc(-1rem-var(--page-right-padding))] bg-white px-4 py-2 sm:ml-[calc(-1.5rem-var(--page-left-padding))] sm:mr-[calc(-1.5rem-var(--page-right-padding))] sm:px-6 lg:-ml-6 lg:-mt-4 lg:pl-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <label className="block min-w-44">
                      <select
                        aria-label={zh('按订阅筛选', 'Filter by subscription')}
                        value={selectedSubscriptionId}
                        onChange={handleSubscriptionFilterChange}
                        className="h-8 w-full rounded-lg border border-gray-300 bg-white px-2.5 text-xs text-gray-700 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                      >
                        <option value="">{zh('全部订阅', 'All subscriptions')}</option>
                        {subscriptions.map((subscription) => (
                          <option key={subscription.id} value={String(subscription.id)}>
                            {subscription.name}
                          </option>
                        ))}
                      </select>
                    </label>
                    <ShowOwnedOption checked={showOwned} compact onChange={handleShowOwnedChange} />
                    {!wantedOnly ? (
                      <button
                        type="button"
                        onClick={handleLoadMoreHistory}
                        disabled={!selectedSubscriptionId || loadingHistory}
                        className="h-8 rounded-lg border border-blue-200 bg-blue-50 px-3 text-xs font-medium text-blue-700 hover:bg-blue-100 disabled:cursor-not-allowed disabled:opacity-50"
                        title={
                          selectedSubscriptionId
                            ? undefined
                            : zh('请先选择一个订阅', 'Select a subscription first')
                        }
                      >
                        {loadingHistory
                          ? zh('加载中…', 'Loading...')
                          : zh('再加载 10 部历史作品', 'Load 10 more historical works')}
                      </button>
                    ) : null}
                  </div>
                  <span className="text-xs text-gray-500">
                    {zh(`共 ${total} 部`, `${total} items`)}
                  </span>
                </div>
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
                          <button
                            type="button"
                            disabled={busyItemIds.has(item.id)}
                            onClick={(event) => {
                              event.stopPropagation()
                              handleWanted(item)
                            }}
                            aria-label={
                              item.wanted
                                ? zh(
                                    `将 ${item.code} 移出我想要的`,
                                    `Remove ${item.code} from wanted`
                                  )
                                : zh(`将 ${item.code} 加入我想要的`, `Add ${item.code} to wanted`)
                            }
                            title={
                              item.wanted
                                ? zh('移出我想要的', 'Remove from wanted')
                                : zh('加入我想要的', 'Add to wanted')
                            }
                            className="absolute right-2 top-2 z-20 flex h-7 w-7 items-center justify-center rounded-full bg-white/90 text-lg text-rose-600 shadow-md backdrop-blur transition hover:scale-105 hover:bg-white disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            {item.wanted ? (
                              <FavoriteRoundedIcon fontSize="inherit" aria-hidden="true" />
                            ) : (
                              <FavoriteBorderRoundedIcon fontSize="inherit" aria-hidden="true" />
                            )}
                          </button>
                        </div>
                        <div className="p-3">
                          <div className="font-mono text-sm font-bold text-gray-900">
                            {item.code}
                          </div>
                          <div className="mt-1 line-clamp-2 min-h-10 text-xs leading-5 text-gray-600">
                            {metadata.title || zh('暂无标题', 'No title')}
                          </div>
                          <div className="mt-2 text-xs font-semibold text-gray-600">
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
          onDownload={handleDownload}
          downloadBusy={(magnet) => busyMagnetUrls.has(String(magnet?.url || ''))}
        />
      ) : null}
    </div>
  )
}
