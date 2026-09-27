import { useEffect, useRef, useState } from 'react'

import { checkProviderConnectivity, fetchConnectivityProviders } from '@/api'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'
import { runProviderChecks } from '@/utils/providerConnectivity'

const providerNames = {
  javbus: 'JavBus',
  javdatabase: 'JavDatabase',
  javdb: 'JavDB',
  'javdb-api': 'JavDB API',
  avmoo: 'Avmoo',
  avsox: 'Avsox',
  theporndb: 'ThePornDB',
  javmodel: 'JavModel',
  javmenu: 'JavMenu',
  minnanoav: 'MinnanoAV',
}

function resultLabel(result) {
  switch (result?.status) {
    case 'queued':
      return zh('等待检测', 'Queued')
    case 'checking':
      return zh('检测中…', 'Checking...')
    case 'ok':
      return zh('可连接', 'Reachable')
    case 'http_error':
      if (result.http_status === 401) return zh('需要认证', 'Authentication required')
      if (result.http_status === 403) return zh('访问被拒绝', 'Access denied')
      if (result.http_status === 407) return zh('代理需要认证', 'Proxy authentication required')
      if (result.http_status === 429) return zh('请求过于频繁', 'Rate limited')
      return zh('HTTP 响应异常', 'Unexpected HTTP response')
    case 'timeout':
      return zh('连接超时', 'Timed out')
    case 'dns_error':
      return zh('域名解析失败', 'DNS resolution failed')
    case 'tls_error':
      return zh('证书验证失败', 'Certificate verification failed')
    case 'network_error':
      return zh('网络或代理连接失败', 'Network or proxy connection failed')
    case 'request_error':
      return getErrorMessage(result.error)
    case 'canceled':
      return zh('已取消', 'Canceled')
    default:
      return zh('未检测', 'Not checked')
  }
}

export default function ProviderConnectivityPanel({ disabled = false }) {
  const [providers, setProviders] = useState([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [reload, setReload] = useState(0)
  const [results, setResults] = useState({})
  const [checking, setChecking] = useState(false)
  const controllerRef = useRef(null)

  useEffect(() => {
    const controller = new AbortController()
    setLoading(true)
    setLoadError('')
    fetchConnectivityProviders({ signal: controller.signal })
      .then((items) => {
        if (!controller.signal.aborted) setProviders(items)
      })
      .catch((error) => {
        if (!controller.signal.aborted) setLoadError(getErrorMessage(error))
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    return () => controller.abort()
  }, [reload])

  useEffect(() => {
    return () => {
      controllerRef.current?.abort()
      controllerRef.current = null
    }
  }, [])

  useEffect(() => {
    if (!disabled) return
    controllerRef.current?.abort()
    controllerRef.current = null
    setChecking(false)
    setResults({})
  }, [disabled])

  const startChecks = async (items) => {
    if (disabled || controllerRef.current) return
    const controller = new AbortController()
    controllerRef.current = controller
    setChecking(true)
    setResults((current) => ({
      ...current,
      ...Object.fromEntries(items.map((provider) => [provider.id, { status: 'queued' }])),
    }))
    try {
      await runProviderChecks(
        items,
        checkProviderConnectivity,
        (id, result) => setResults((current) => ({ ...current, [id]: result })),
        controller.signal
      )
    } finally {
      if (controllerRef.current === controller) {
        controllerRef.current = null
        setChecking(false)
      }
    }
  }

  const cancelChecks = () => {
    controllerRef.current?.abort()
    controllerRef.current = null
    setChecking(false)
    setResults((current) =>
      Object.fromEntries(
        Object.entries(current).map(([id, result]) => [
          id,
          ['checking', 'queued'].includes(result.status) ? { status: 'canceled' } : result,
        ])
      )
    )
  }

  return (
    <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h4 className="text-sm font-semibold text-zinc-800">
            {zh('数据源连通性检测', 'Provider Connectivity')}
          </h4>
          <p className="mt-1 text-sm text-zinc-500">
            {zh(
              '使用服务端已保存的代理设置检测 HTTP 响应，连通不代表刮削一定成功。',
              'Checks HTTP responses using the server’s saved proxy settings. Reachability does not guarantee successful scraping.'
            )}
          </p>
          {disabled && (
            <p className="mt-1 text-sm text-amber-700">
              {zh(
                '请先保存或取消代理编辑，再进行检测。',
                'Save or cancel proxy edits before checking.'
              )}
            </p>
          )}
        </div>
        {checking ? (
          <button
            type="button"
            onClick={cancelChecks}
            className="rounded-xl border border-zinc-200 px-3 py-1.5 text-sm text-zinc-700"
          >
            {zh('取消检测', 'Cancel checks')}
          </button>
        ) : (
          <button
            type="button"
            onClick={() => startChecks(providers)}
            disabled={disabled || loading || providers.length === 0 || Boolean(loadError)}
            className="rounded-xl bg-blue-600 px-3 py-1.5 text-sm text-white disabled:opacity-60"
          >
            {zh('检测全部', 'Check all')}
          </button>
        )}
      </div>
      {loading && <p className="mt-4 text-sm text-zinc-500">{zh('加载中…', 'Loading...')}</p>}
      {loadError && (
        <div className="mt-4 flex items-center gap-3 text-sm text-red-600" role="alert">
          <span>{loadError}</span>
          <button
            type="button"
            onClick={() => setReload((value) => value + 1)}
            className="underline"
          >
            {zh('重试', 'Retry')}
          </button>
        </div>
      )}
      <ul className="mt-4 divide-y divide-zinc-100" aria-live="polite">
        {providers.map((provider) => {
          const result = results[provider.id]
          const pending = !result || ['queued', 'checking', 'canceled'].includes(result.status)
          const color = pending
            ? 'text-zinc-500'
            : result.status === 'ok'
              ? 'text-emerald-700'
              : result.status === 'http_error'
                ? 'text-amber-700'
                : 'text-red-600'
          const name = providerNames[provider.name] || provider.name
          return (
            <li key={provider.id} className="flex flex-wrap items-center gap-3 py-3">
              <span className="w-28 text-sm font-medium text-zinc-800">{name}</span>
              <div className={`min-w-0 flex-1 text-sm ${color}`}>
                <span className="break-words">{resultLabel(result)}</span>
                {result?.http_status > 0 && <span> · HTTP {result.http_status}</span>}
                {Number.isFinite(result?.elapsed_ms) && <span> · {result.elapsed_ms} ms</span>}
              </div>
              <button
                type="button"
                aria-label={zh(`检测 ${name} 连通性`, `Check ${name} connectivity`)}
                onClick={() => startChecks([provider])}
                disabled={disabled || checking}
                className="rounded-xl border border-zinc-200 px-3 py-1.5 text-sm text-zinc-700 hover:bg-zinc-50 disabled:opacity-60"
              >
                {zh('检测', 'Check')}
              </button>
            </li>
          )
        })}
      </ul>
    </section>
  )
}
