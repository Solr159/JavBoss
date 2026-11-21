import Tooltip from '@/shared/ui/Tooltip'
import { useEffect, useRef, useState } from 'react'
import { IconButton } from '@mui/material'
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded'
import AccessTimeRoundedIcon from '@mui/icons-material/AccessTimeRounded'

import {
  checkProviderAvailability,
  fetchAvailabilityProviders,
  updateProviderBaseURL,
} from '@/features/settings/api'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'
import { runProviderChecks } from '@/utils/providerAvailability'

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
  avwiki: 'AV Wiki',
}

function resultLabel(result) {
  switch (result?.status) {
    case 'queued':
      return zh('等待检测', 'Queued')
    case 'checking':
      return zh('检测中…', 'Checking...')
    case 'ok':
      return zh('可用', 'Available')
    case 'not_found':
      return zh('未获取到测试数据', 'Test data not found')
    case 'invalid_response':
      return zh('数据解析失败或不完整', 'Invalid or incomplete data')
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

function ProviderBaseURLField({ provider, name, disabled, onSaved }) {
  const [draft, setDraft] = useState(provider.base_url || '')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    setDraft(provider.base_url || '')
  }, [provider.base_url])

  const save = async (value) => {
    if (saving || disabled) return
    const next = String(value || '').trim()
    if (next === (provider.base_url || '')) {
      setDraft(provider.base_url || '')
      return
    }
    setSaving(true)
    setError('')
    try {
      const updated = await updateProviderBaseURL(provider.id, next)
      setDraft(updated.base_url || '')
      onSaved?.(updated)
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form
      className="space-y-1"
      onSubmit={(event) => {
        event.preventDefault()
        save(draft)
      }}
    >
      <div className="flex flex-wrap items-center gap-1.5">
        <input
          type="text"
          inputMode="url"
          autoComplete="off"
          spellCheck={false}
          value={draft}
          placeholder={provider.default_base_url || ''}
          aria-label={zh(`自定义 ${name} 站点地址`, `Custom ${name} URL`)}
          disabled={disabled || saving}
          onChange={(event) => setDraft(event.target.value)}
          className="min-w-0 flex-1 rounded-lg border border-zinc-200 bg-white px-2 py-1 text-[11px] leading-4 text-zinc-800 placeholder:text-zinc-400 focus:border-ui-primary focus:outline-none disabled:opacity-60 dark:border-ui-border dark:bg-ui-surface dark:text-ui-text dark:placeholder:text-ui-faint"
        />
        <button
          type="submit"
          disabled={disabled || saving}
          className="rounded-lg border border-zinc-200 px-2 py-1 text-[11px] text-zinc-700 transition hover:border-zinc-300 disabled:opacity-60 dark:border-ui-border dark:text-ui-secondary"
        >
          {saving ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
        </button>
        {provider.base_url ? (
          <button
            type="button"
            onClick={() => save('')}
            disabled={disabled || saving}
            className="rounded-lg border border-zinc-200 px-2 py-1 text-[11px] text-zinc-700 transition hover:border-zinc-300 disabled:opacity-60 dark:border-ui-border dark:text-ui-secondary"
          >
            {zh('默认地址', 'Default')}
          </button>
        ) : null}
      </div>
      {error ? (
        <p role="alert" className="text-[11px] leading-4 text-red-600 dark:text-red-300">
          {error}
        </p>
      ) : (
        <p className="text-[10px] leading-4 text-zinc-400 dark:text-ui-faint">
          {zh(
            '留空使用默认地址，仅支持站点地址（不含路径）',
            'Leave empty for the default site; origin only'
          )}
        </p>
      )}
    </form>
  )
}

export default function ProviderAvailabilityPanel({ disabled = false }) {
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
    fetchAvailabilityProviders({ signal: controller.signal })
      .then((items) => {
        if (!controller.signal.aborted) {
          setProviders(items)
          setResults(
            Object.fromEntries(
              items
                .filter((provider) => provider.last_result)
                .map((provider) => [provider.id, provider.last_result])
            )
          )
        }
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
        checkProviderAvailability,
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

  const applyProviderUpdate = (updated) => {
    setProviders((current) =>
      current.map((provider) =>
        provider.id === updated.provider
          ? {
              ...provider,
              base_url: updated.base_url,
              default_base_url: updated.default_base_url,
              domain: updated.domain,
            }
          : provider
      )
    )
    setResults((current) => {
      if (!(updated.provider in current)) return current
      const next = { ...current }
      delete next[updated.provider]
      return next
    })
  }

  return (
    <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm dark:border-ui-border dark:bg-ui-surface">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h4 className="text-[13px] font-semibold text-zinc-800 dark:text-ui-text">
            {zh('可用性检测', 'Availability Check')}
          </h4>
          <p className="mt-1 text-xs leading-5 text-zinc-500 dark:text-ui-faint">
            {zh(
              '使用当前代理设置执行真实查询，成功获取并解析数据后视为可用',
              'Run a real lookup using the current proxy settings; success requires parsed data'
            )}
          </p>
          <p className="mt-1 text-xs leading-5 text-zinc-500 dark:text-ui-faint">
            {zh(
              '可为每个数据源填写自定义站点地址替代默认地址，修改后需重新检测',
              'Each source accepts a custom site origin instead of the default; re-check after saving'
            )}
          </p>
          {disabled && (
            <p className="mt-1 text-sm text-amber-700 dark:text-amber-300">
              {zh(
                '请先在“网络与代理”中保存或取消代理编辑，再进行检测。',
                'Save or cancel proxy edits in Network & Proxy before checking.'
              )}
            </p>
          )}
        </div>
        {checking ? (
          <button
            type="button"
            onClick={cancelChecks}
            className="rounded-lg border border-zinc-200 px-3 py-1.5 text-xs text-zinc-700 dark:border-ui-border dark:text-ui-secondary"
          >
            {zh('取消检测', 'Cancel checks')}
          </button>
        ) : (
          <button
            type="button"
            onClick={() => startChecks(providers)}
            disabled={disabled || loading || providers.length === 0 || Boolean(loadError)}
            className="rounded-lg bg-ui-primary px-3 py-1.5 text-xs font-medium text-white transition hover:bg-ui-primary-hover disabled:opacity-60"
          >
            {zh('检测全部', 'Check all')}
          </button>
        )}
      </div>
      {loading && (
        <p className="mt-4 text-sm text-zinc-500 dark:text-ui-faint">
          {zh('加载中…', 'Loading...')}
        </p>
      )}
      {loadError && (
        <div
          className="mt-4 flex items-center gap-3 text-sm text-red-600 dark:text-red-300"
          role="alert"
        >
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
      <ul
        className="mt-4 grid grid-cols-[repeat(auto-fit,minmax(min(100%,240px),1fr))] gap-3"
        aria-live="polite"
      >
        {providers.map((provider) => {
          const result = results[provider.id]
          const pending = !result || ['queued', 'checking', 'canceled'].includes(result.status)
          const color = pending
            ? 'bg-zinc-100 dark:bg-ui-muted text-zinc-500 dark:text-ui-faint'
            : result.status === 'ok'
              ? 'bg-emerald-50 dark:bg-emerald-950 text-emerald-700 dark:text-emerald-300'
              : ['http_error', 'not_found', 'invalid_response'].includes(result.status)
                ? 'bg-amber-50 dark:bg-amber-950 text-amber-700 dark:text-amber-300'
                : 'bg-red-50 dark:bg-red-950 text-red-600 dark:text-red-300'
          const name = providerNames[provider.name] || provider.name
          const checkLabel =
            result?.status === 'checking'
              ? zh(`正在检测 ${name}`, `Checking ${name}`)
              : zh(`检测 ${name} 可用性`, `Check ${name} availability`)
          const checkedAt = result?.checked_at ? new Date(result.checked_at) : null
          return (
            <li
              key={provider.id}
              className="flex min-w-0 flex-col gap-3 rounded-xl border border-zinc-200/80 bg-white p-3.5 transition hover:border-zinc-300 hover:shadow-sm dark:border-ui-border/80 dark:bg-ui-surface dark:hover:border-ui-faint"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="text-[13px] font-semibold leading-5 text-zinc-800 dark:text-ui-text">
                    {name}
                  </div>
                  <div className="mt-0.5 flex flex-wrap items-center gap-1.5 text-[11px] leading-4 text-zinc-500 dark:text-ui-faint">
                    <span className="break-all">{provider.domain}</span>
                    {provider.base_url ? (
                      <span className="rounded bg-blue-50 px-1.5 py-0.5 text-[10px] leading-4 text-blue-700 dark:bg-blue-950 dark:text-blue-300">
                        {zh('自定义', 'Custom')}
                      </span>
                    ) : null}
                  </div>
                  <div className="mt-1 text-[11px] text-zinc-500 dark:text-ui-faint">
                    {zh('测试查询：', 'Test lookup: ')}
                    {provider.sample}
                  </div>
                </div>
                <Tooltip title={checkLabel} arrow>
                  <span className="inline-flex shrink-0">
                    <IconButton
                      type="button"
                      size="small"
                      aria-label={checkLabel}
                      onClick={() => startChecks([provider])}
                      disabled={disabled || checking}
                      sx={{
                        width: 28,
                        height: 28,
                        borderRadius: '8px',
                        color: 'var(--theme-faint, #71717a)',
                        '&:hover': {
                          color: 'var(--theme-blue-text, #2563eb)',
                          backgroundColor: 'var(--theme-blue-surface, #eff6ff)',
                        },
                      }}
                    >
                      <RefreshRoundedIcon
                        sx={{ fontSize: 17 }}
                        className={result?.status === 'checking' ? 'motion-safe:animate-spin' : ''}
                      />
                    </IconButton>
                  </span>
                </Tooltip>
              </div>
              {provider.configurable ? (
                <ProviderBaseURLField
                  provider={provider}
                  name={name}
                  disabled={disabled || checking}
                  onSaved={applyProviderUpdate}
                />
              ) : null}
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span
                  className={`inline-flex min-w-0 items-center gap-1.5 rounded-md px-2 py-1 text-[11px] font-medium leading-4 ${color}`}
                >
                  <span
                    className="h-1.5 w-1.5 shrink-0 rounded-full bg-current"
                    aria-hidden="true"
                  />
                  <span className="break-words">{resultLabel(result)}</span>
                </span>
                {Number.isFinite(result?.elapsed_ms) && (
                  <span className="shrink-0 text-[11px] tabular-nums text-zinc-500 dark:text-ui-faint">
                    {result.elapsed_ms} ms
                  </span>
                )}
              </div>
              {(result?.http_status > 0 || checkedAt) && (
                <div className="mt-auto flex flex-wrap items-center justify-between gap-2 border-t border-zinc-100 pt-2.5 text-[10px] tabular-nums leading-4 text-zinc-500 dark:border-ui-border dark:text-ui-faint">
                  {result?.http_status > 0 && <span>HTTP {result.http_status}</span>}
                  {checkedAt && (
                    <span
                      className="ml-auto inline-flex items-center gap-1"
                      title={zh('上次检测：', 'Last checked: ') + checkedAt.toLocaleString()}
                    >
                      <AccessTimeRoundedIcon sx={{ fontSize: 12 }} />
                      <time
                        dateTime={result.checked_at}
                        aria-label={zh('上次检测：', 'Last checked: ') + checkedAt.toLocaleString()}
                      >
                        {checkedAt.toLocaleString(undefined, {
                          month: '2-digit',
                          day: '2-digit',
                          hour: '2-digit',
                          minute: '2-digit',
                          hour12: false,
                        })}
                      </time>
                    </span>
                  )}
                </div>
              )}
            </li>
          )
        })}
      </ul>
    </section>
  )
}
