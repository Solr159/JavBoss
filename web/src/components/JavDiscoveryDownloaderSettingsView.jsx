import { useEffect, useState } from 'react'
import VisibilityOffOutlinedIcon from '@mui/icons-material/VisibilityOffOutlined'
import VisibilityOutlinedIcon from '@mui/icons-material/VisibilityOutlined'
import {
  fetchDirectories,
  fetchDownloaderProviderToken,
  fetchDownloaderSettings,
  testDownloaderProvider,
  updateDownloaderProvider,
  updateDownloaderSettings,
} from '@/api'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

const providers = [
  {
    id: 'clouddrive2',
    name: 'CloudDrive2',
    defaultAddress: 'http://127.0.0.1:19798',
    tokenLabel: ['API Token', 'API token'],
    folderLabel: ['云端离线目录', 'Remote offline folder'],
    folderPlaceholder: '/115/JavBoss',
  },
  {
    id: 'openlist',
    name: 'OpenList · 115 Open',
    defaultAddress: 'http://127.0.0.1:5244',
    tokenLabel: ['管理员 API Token', 'Admin API token'],
    folderLabel: ['115 Open 离线下载目标目录', '115 Open offline download target folder'],
    folderPlaceholder: '/115/JavBoss',
  },
]

function initialProviderForms() {
  return Object.fromEntries(
    providers.map((provider) => [
      provider.id,
      { address: provider.defaultAddress, apiToken: '', remoteFolder: '' },
    ])
  )
}

export default function JavDiscoveryDownloaderSettingsView() {
  const [settings, setSettings] = useState(null)
  const [directories, setDirectories] = useState([])
  const [commonForm, setCommonForm] = useState({ directoryId: '', localConcurrency: 2 })
  const [providerForms, setProviderForms] = useState(initialProviderForms)
  const [visibleTokens, setVisibleTokens] = useState({ clouddrive2: false, openlist: false })
  const [loading, setLoading] = useState(true)
  const [savingCommon, setSavingCommon] = useState(false)
  const [providerAction, setProviderAction] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  useEffect(() => {
    let cancelled = false
    Promise.all([fetchDownloaderSettings(), fetchDirectories()])
      .then(([nextSettings, directoryItems]) => {
        if (cancelled) return
        setSettings(nextSettings)
        setDirectories(
          (Array.isArray(directoryItems) ? directoryItems : []).filter(
            (directory) => !directory.is_delete
          )
        )
        setCommonForm({
          directoryId: nextSettings?.directory_id ? String(nextSettings.directory_id) : '',
          localConcurrency: Number(nextSettings?.local_concurrency) || 2,
        })
        setProviderForms(
          Object.fromEntries(
            providers.map((provider) => {
              const saved = nextSettings?.providers?.[provider.id]
              return [
                provider.id,
                {
                  address: saved?.address || provider.defaultAddress,
                  apiToken: '',
                  remoteFolder: saved?.remote_folder || '',
                },
              ]
            })
          )
        )
      })
      .catch((loadError) => {
        if (!cancelled) setError(getErrorMessage(loadError))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const updateProviderForm = (provider, field, value) => {
    setProviderForms((current) => ({
      ...current,
      [provider]: { ...current[provider], [field]: value },
    }))
  }

  const commonPayload = (activeProvider = settings?.active_provider || '') => ({
    active_provider: activeProvider,
    directory_id: commonForm.directoryId ? Number(commonForm.directoryId) : null,
    local_concurrency: Number(commonForm.localConcurrency),
  })

  const saveProvider = async (provider) => {
    const form = providerForms[provider]
    const payload = {
      address: form.address.trim(),
      remote_folder: form.remoteFolder.trim(),
    }
    if (form.apiToken.trim()) payload.api_token = form.apiToken.trim()
    const saved = await updateDownloaderProvider(provider, payload)
    setSettings((current) => ({
      ...current,
      providers: { ...current?.providers, [provider]: saved },
    }))
    setProviderForms((current) => ({
      ...current,
      [provider]: { ...current[provider], apiToken: '' },
    }))
    setVisibleTokens((current) => ({ ...current, [provider]: false }))
    return saved
  }

  const handleCommonSave = async (event) => {
    event.preventDefault()
    setSavingCommon(true)
    setError('')
    setNotice('')
    try {
      const saved = await updateDownloaderSettings(commonPayload())
      setSettings(saved)
      setNotice(zh('下载行为设置已保存', 'Download behavior settings saved'))
    } catch (saveError) {
      setError(getErrorMessage(saveError))
    } finally {
      setSavingCommon(false)
    }
  }

  const handleProviderSave = async (provider) => {
    setProviderAction(`${provider}:save`)
    setError('')
    setNotice('')
    try {
      await saveProvider(provider)
      setNotice(
        zh(
          `${providers.find((item) => item.id === provider)?.name} 设置已保存`,
          `${providers.find((item) => item.id === provider)?.name} settings saved`
        )
      )
    } catch (saveError) {
      setError(getErrorMessage(saveError))
    } finally {
      setProviderAction('')
    }
  }

  const handleProviderTest = async (provider) => {
    setProviderAction(`${provider}:test`)
    setError('')
    setNotice('')
    try {
      await saveProvider(provider)
      const result = await testDownloaderProvider(provider)
      const fallbackName = providers.find((item) => item.id === provider)?.name
      setNotice(
        zh(
          `连接成功：${result?.user_name || fallbackName}，目标目录可用`,
          `Connected as ${result?.user_name || fallbackName}; the target folder is ready`
        )
      )
    } catch (testError) {
      setError(getErrorMessage(testError))
    } finally {
      setProviderAction('')
    }
  }

  const handleProviderToggle = async (provider) => {
    const enabled = settings?.active_provider === provider
    setProviderAction(`${provider}:toggle`)
    setError('')
    setNotice('')
    try {
      if (!enabled) {
        if (!commonForm.directoryId) {
          throw new Error(zh('请先选择本地下载目录', 'Select a local download directory first'))
        }
        await saveProvider(provider)
      }
      const saved = await updateDownloaderSettings(commonPayload(enabled ? '' : provider))
      setSettings(saved)
      const providerName = providers.find((item) => item.id === provider)?.name
      setNotice(
        enabled
          ? zh(`${providerName} 已停用`, `${providerName} disabled`)
          : zh(`${providerName} 已启用`, `${providerName} enabled`)
      )
    } catch (toggleError) {
      setError(getErrorMessage(toggleError))
    } finally {
      setProviderAction('')
    }
  }

  const handleTokenVisibility = async (provider) => {
    if (visibleTokens[provider]) {
      setVisibleTokens((current) => ({ ...current, [provider]: false }))
      return
    }
    if (providerForms[provider].apiToken || !settings?.providers?.[provider]?.token_configured) {
      setVisibleTokens((current) => ({ ...current, [provider]: true }))
      return
    }

    setProviderAction(`${provider}:token`)
    setError('')
    try {
      const result = await fetchDownloaderProviderToken(provider)
      setProviderForms((current) => ({
        ...current,
        [provider]: { ...current[provider], apiToken: result?.api_token || '' },
      }))
      setVisibleTokens((current) => ({ ...current, [provider]: true }))
    } catch (loadError) {
      setError(getErrorMessage(loadError))
    } finally {
      setProviderAction('')
    }
  }

  const busy = savingCommon || Boolean(providerAction)

  if (loading) {
    return (
      <div className="rounded-xl border border-dashed border-gray-200 bg-white p-10 text-center text-sm text-gray-400">
        {zh('加载下载器设置…', 'Loading downloader settings...')}
      </div>
    )
  }

  return (
    <div className="space-y-5">
      {error ? (
        <div role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
          {error}
        </div>
      ) : null}
      {notice ? (
        <div role="status" className="rounded-lg bg-green-50 px-3 py-2 text-sm text-green-700">
          {notice}
        </div>
      ) : null}

      <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm">
        <div>
          <h2 className="text-2xl font-bold text-gray-900">
            {zh('下载器设置', 'Downloader settings')}
          </h2>
          <p className="mt-1 text-sm text-gray-500">
            {zh(
              '选择本地保存位置和并发数，再在下方启用一个下载器。',
              'Choose the local destination and concurrency, then enable one downloader below.'
            )}
          </p>
        </div>

        <form onSubmit={handleCommonSave} className="mt-5 grid gap-3 lg:grid-cols-2">
          <label className="text-xs font-medium text-gray-600">
            {zh('本地下载目录', 'Local download directory')}
            <select
              value={commonForm.directoryId}
              onChange={(event) =>
                setCommonForm((current) => ({ ...current, directoryId: event.target.value }))
              }
              className="mt-1 h-10 w-full rounded-lg border border-gray-300 bg-white px-3 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
            >
              <option value="">{zh('请选择目录', 'Select a directory')}</option>
              {directories.map((directory) => (
                <option key={directory.id} value={String(directory.id)}>
                  {directory.path}
                </option>
              ))}
            </select>
          </label>
          <label className="text-xs font-medium text-gray-600">
            {zh('本地下载并发数', 'Concurrent local downloads')}
            <select
              value={commonForm.localConcurrency}
              onChange={(event) =>
                setCommonForm((current) => ({
                  ...current,
                  localConcurrency: Number(event.target.value),
                }))
              }
              className="mt-1 h-10 w-full rounded-lg border border-gray-300 bg-white px-3 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
            >
              {[1, 2, 3, 4, 5].map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </select>
            <span className="mt-1 block font-normal text-gray-400">
              {zh(
                '云端离线任务可并行；此项仅限制同时写入本地的任务数。',
                'Cloud offline jobs run in parallel; this only limits simultaneous local transfers.'
              )}
            </span>
          </label>
          <div className="flex justify-end lg:col-span-2">
            <button
              type="submit"
              disabled={busy}
              className="h-10 rounded-lg bg-blue-600 px-4 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50"
            >
              {savingCommon ? zh('保存中…', 'Saving...') : zh('保存下载行为', 'Save behavior')}
            </button>
          </div>
        </form>
      </section>

      <div className="grid items-start gap-5 xl:grid-cols-2">
        {providers.map((provider) => {
          const form = providerForms[provider.id]
          const saved = settings?.providers?.[provider.id]
          const enabled = settings?.active_provider === provider.id
          const action = providerAction.startsWith(`${provider.id}:`) ? providerAction : ''
          return (
            <section
              key={provider.id}
              className={`rounded-2xl border bg-white p-5 shadow-sm ${
                enabled ? 'border-blue-300 ring-1 ring-blue-100' : 'border-gray-200'
              }`}
            >
              <div className="flex items-center justify-between gap-4">
                <div>
                  <h3 className="text-xl font-bold text-gray-900">{provider.name}</h3>
                  <p
                    className={`mt-1 text-xs font-semibold ${enabled ? 'text-blue-600' : 'text-gray-400'}`}
                  >
                    {enabled
                      ? zh('当前已启用', 'Currently enabled')
                      : zh('当前未启用', 'Currently disabled')}
                  </p>
                </div>
                <button
                  type="button"
                  role="switch"
                  aria-checked={enabled}
                  aria-label={zh(`启用 ${provider.name}`, `Enable ${provider.name}`)}
                  disabled={busy}
                  onClick={() => handleProviderToggle(provider.id)}
                  className="flex items-center gap-2 text-sm font-medium text-gray-600 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <span
                    className={`relative inline-flex h-6 w-11 shrink-0 rounded-full transition ${
                      enabled ? 'bg-blue-600' : 'bg-gray-300'
                    }`}
                  >
                    <span
                      className={`absolute top-0.5 h-5 w-5 rounded-full bg-white shadow transition ${
                        enabled ? 'translate-x-5' : 'translate-x-0.5'
                      }`}
                    />
                  </span>
                  {enabled ? zh('已启用', 'Enabled') : zh('启用', 'Enable')}
                </button>
              </div>

              <div className="mt-5 space-y-3">
                <label className="block text-xs font-medium text-gray-600">
                  {zh(`${provider.name} 地址`, `${provider.name} address`)}
                  <input
                    value={form.address}
                    onChange={(event) =>
                      updateProviderForm(provider.id, 'address', event.target.value)
                    }
                    placeholder={provider.defaultAddress}
                    className="mt-1 h-10 w-full rounded-lg border border-gray-300 bg-white px-3 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                  />
                </label>
                <label className="block text-xs font-medium text-gray-600">
                  {zh(provider.tokenLabel[0], provider.tokenLabel[1])}
                  <div className="relative mt-1">
                    <input
                      type={visibleTokens[provider.id] ? 'text' : 'password'}
                      value={form.apiToken}
                      onChange={(event) =>
                        updateProviderForm(provider.id, 'apiToken', event.target.value)
                      }
                      placeholder={
                        saved?.token_configured
                          ? zh('已配置；留空保持不变', 'Configured; leave blank to keep it')
                          : zh('请输入 API Token', 'Enter an API token')
                      }
                      autoComplete="new-password"
                      className="h-10 w-full rounded-lg border border-gray-300 bg-white py-2 pl-3 pr-11 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                    />
                    <button
                      type="button"
                      disabled={busy}
                      onClick={() => handleTokenVisibility(provider.id)}
                      className="absolute right-2 top-1/2 flex -translate-y-1/2 items-center justify-center rounded-md p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-50"
                      aria-label={
                        visibleTokens[provider.id]
                          ? zh('隐藏 API Token', 'Hide API token')
                          : zh('显示 API Token', 'Show API token')
                      }
                    >
                      {visibleTokens[provider.id] ? (
                        <VisibilityOutlinedIcon fontSize="small" aria-hidden="true" />
                      ) : (
                        <VisibilityOffOutlinedIcon fontSize="small" aria-hidden="true" />
                      )}
                    </button>
                  </div>
                </label>
                <label className="block text-xs font-medium text-gray-600">
                  {zh(provider.folderLabel[0], provider.folderLabel[1])}
                  <input
                    value={form.remoteFolder}
                    onChange={(event) =>
                      updateProviderForm(provider.id, 'remoteFolder', event.target.value)
                    }
                    placeholder={provider.folderPlaceholder}
                    className="mt-1 h-10 w-full rounded-lg border border-gray-300 bg-white px-3 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                  />
                </label>
              </div>

              <div className="mt-5 flex justify-end gap-2">
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => handleProviderTest(provider.id)}
                  className="h-10 rounded-lg border border-blue-200 bg-blue-50 px-4 text-sm font-semibold text-blue-700 hover:bg-blue-100 disabled:opacity-50"
                >
                  {action.endsWith(':test')
                    ? zh('测试中…', 'Testing...')
                    : zh('保存并测试', 'Save and test')}
                </button>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => handleProviderSave(provider.id)}
                  className="h-10 rounded-lg bg-blue-600 px-4 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50"
                >
                  {action.endsWith(':save')
                    ? zh('保存中…', 'Saving...')
                    : zh('保存设置', 'Save settings')}
                </button>
              </div>
            </section>
          )
        })}
      </div>

      <p className="text-xs text-gray-400">
        {zh(
          '一次只能启用一个下载器；队列存在未结束任务时不能切换。',
          'Only one downloader can be enabled; switching is blocked while jobs are unfinished.'
        )}
      </p>
    </div>
  )
}
