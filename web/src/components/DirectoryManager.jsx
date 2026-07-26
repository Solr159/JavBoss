import { useEffect, useState } from 'react'

import { pickDirectory } from '@/api'
import { apiHostPath, displayHostPath } from '@/utils/hostPath'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'

const DIRECTORY_PROCESS_SIDECAR = 'sidecar'
const DIRECTORY_PROCESS_ORGANIZE = 'organize'
const DIRECTORY_PROCESS_ORGANIZE_WITH_SIDECAR = 'organize_with_sidecar'
const DIRECTORY_PROCESS_LAYOUT_PREFIX = 'prefix'
const DIRECTORY_PROCESS_LAYOUT_IDOL = 'idol'

const directoryProcessOptions = () => [
  {
    mode: DIRECTORY_PROCESS_SIDECAR,
    title: zh('仅生成 NFO 和封面', 'Generate NFO and covers only'),
    description: zh(
      '在视频旁生成媒体库可用的 NFO 和封面，不移动视频。',
      'Generate media-library-compatible NFO files and covers beside each video without moving it.'
    ),
  },
  {
    mode: DIRECTORY_PROCESS_ORGANIZE,
    title: zh('仅整理目录', 'Organize only'),
    description: zh(
      '按照选择的整理方式移动视频，保留原文件名，不生成 NFO 和封面。',
      'Move videos using the selected layout, preserve filenames, and do not generate NFO files or covers.'
    ),
  },
  {
    mode: DIRECTORY_PROCESS_ORGANIZE_WITH_SIDECAR,
    title: zh('整理并生成 NFO 和封面', 'Organize and generate NFO and covers'),
    description: zh(
      '移动视频及同名附属文件，保留原文件名，然后生成媒体库可用的 NFO 和封面。',
      'Move videos and matching companion files while preserving filenames, then generate media-library-compatible NFO files and covers.'
    ),
  },
]

const directoryProcessLayoutOptions = () => [
  {
    layout: DIRECTORY_PROCESS_LAYOUT_PREFIX,
    title: zh('按番号前缀', 'By code prefix'),
    example: 'JAV/IPX/IPX-001/...',
  },
  {
    layout: DIRECTORY_PROCESS_LAYOUT_IDOL,
    title: zh('按女优', 'By idol'),
    example: zh('JAV/女优名/IPX-001/...', 'JAV/IDOL NAME/IPX-001/...'),
  },
]

const formatScanFinishedAt = (summary) => {
  const timestamp = Number(summary?.finished_at_unix_ms)
  if (!Number.isFinite(timestamp) || timestamp <= 0) return ''
  const date = new Date(timestamp)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(zh('zh-CN', 'en-US'), {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(date)
}

const formatScanDuration = (summary) => {
  const durationMS = Number(summary?.duration_ms)
  if (!Number.isFinite(durationMS) || durationMS < 1000) {
    return zh('不足 1 秒', 'Less than 1 second')
  }
  const totalSeconds = Math.max(1, Math.round(durationMS / 1000))
  if (totalSeconds < 60) {
    return zh(`${totalSeconds} 秒`, `${totalSeconds} sec`)
  }
  const totalMinutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (totalMinutes < 60) {
    return zh(`${totalMinutes} 分 ${seconds} 秒`, `${totalMinutes} min ${seconds} sec`)
  }
  const hours = Math.floor(totalMinutes / 60)
  const minutes = totalMinutes % 60
  return zh(`${hours} 小时 ${minutes} 分`, `${hours} hr ${minutes} min`)
}

const directoryWorkStatus = (directory) =>
  directory?.work_status || (directory?.is_scanning ? 'scanning' : 'idle')

const directoryWorkStatusDisplay = (status) => {
  switch (status) {
    case 'scanning':
      return {
        label: zh('当前状态：扫描中', 'Status: Scanning'),
        badge: 'bg-blue-50 text-blue-700',
        dot: 'animate-pulse bg-blue-500',
      }
    case 'organizing':
      return {
        label: zh('当前状态：整理中', 'Status: Organizing'),
        badge: 'bg-amber-50 text-amber-700',
        dot: 'animate-pulse bg-amber-500',
      }
    case 'generating_sidecar':
      return {
        label: zh('当前状态：生成 NFO 和封面中', 'Status: Generating NFO and covers'),
        badge: 'bg-violet-50 text-violet-700',
        dot: 'animate-pulse bg-violet-500',
      }
    case 'organizing_with_sidecar':
      return {
        label: zh(
          '当前状态：整理并生成 NFO 和封面中',
          'Status: Organizing and generating NFO and covers'
        ),
        badge: 'bg-amber-50 text-amber-700',
        dot: 'animate-pulse bg-amber-500',
      }
    case 'rescanning':
      return {
        label: zh('当前状态：重新扫描中', 'Status: Rescanning'),
        badge: 'bg-blue-50 text-blue-700',
        dot: 'animate-pulse bg-blue-500',
      }
    default:
      return {
        label: zh('当前状态：空闲', 'Status: Idle'),
        badge: 'bg-zinc-100 text-zinc-600',
        dot: 'bg-zinc-400',
      }
  }
}

function isWindowsPlatform() {
  if (typeof navigator === 'undefined') return false

  const platform =
    navigator.userAgentData?.platform || navigator.platform || navigator.userAgent || ''

  return /windows/i.test(String(platform))
}

export default function DirectoryManager({
  open,
  directories,
  onCreate,
  onUpdate,
  onDelete,
  onProcess,
  onRefresh,
  directoryPickerEnabled = true,
  useHostPaths = false,
}) {
  const [path, setPath] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [picking, setPicking] = useState(false)
  const [error, setError] = useState('')
  const [adding, setAdding] = useState(false)

  const [editId, setEditId] = useState(null)
  const [editPath, setEditPath] = useState('')
  const [rowErrorId, setRowErrorId] = useState(null)
  const [rowErrorMsg, setRowErrorMsg] = useState('')
  const [savingId, setSavingId] = useState(null)
  const [deletingId, setDeletingId] = useState(null)
  const [processingId, setProcessingId] = useState(null)
  const [toolDirectory, setToolDirectory] = useState(null)
  const [toolMode, setToolMode] = useState(DIRECTORY_PROCESS_SIDECAR)
  const [toolLayout, setToolLayout] = useState(DIRECTORY_PROCESS_LAYOUT_PREFIX)
  const windowsPlatform = isWindowsPlatform()
  const pathPlaceholder = useHostPaths
    ? zh(
        '输入宿主机目录路径，例如 /mnt/disk1/videos',
        'Enter a host folder path, e.g. /mnt/disk1/videos'
      )
    : windowsPlatform
      ? zh('输入目录路径，例如 D:\\Videos', 'Enter a folder path, e.g. D:\\Videos')
      : zh('输入目录路径，例如 /Volumes/Videos', 'Enter a folder path, e.g. /Volumes/Videos')
  const pathHelperText = zh(
    directoryPickerEnabled
      ? '建议优先使用“选择目录”，也可以手动输入完整目录路径。'
      : useHostPaths
        ? '请输入宿主机上的完整目录路径，Docker 部署会自动映射到容器内路径。'
        : '请输入容器内可访问的完整目录路径，例如 /media。',
    directoryPickerEnabled
      ? 'Use "Choose directory" when possible, or enter the full folder path manually.'
      : useHostPaths
        ? 'Enter the full host path. Docker deployments map it to the container path automatically.'
        : 'Enter a full path that is accessible inside the container, for example /media.'
  )
  const displayPath = (value) => displayHostPath(value, useHostPaths)
  const apiPath = (value) => apiHostPath(value, useHostPaths)

  useEffect(() => {
    if (open) {
      setPath('')
      setError('')
      setAdding(false)
      setEditId(null)
      setEditPath('')
      setRowErrorId(null)
      setRowErrorMsg('')
      setToolDirectory(null)
      setToolMode(DIRECTORY_PROCESS_SIDECAR)
      setToolLayout(DIRECTORY_PROCESS_LAYOUT_PREFIX)
    }
  }, [open])

  useEffect(() => {
    if (!toolDirectory) return undefined
    const handleKeyDown = (event) => {
      if (event.key === 'Escape') {
        setToolDirectory(null)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [toolDirectory])

  useEffect(() => {
    if (!open || !onRefresh) return undefined

    let refreshing = false
    const refresh = async () => {
      if (refreshing) return
      refreshing = true
      try {
        await onRefresh()
      } catch {
        // Keep the last successful counts visible and retry on the next interval.
      } finally {
        refreshing = false
      }
    }

    refresh()
    const timer = window.setInterval(refresh, 1000)
    return () => window.clearInterval(timer)
  }, [onRefresh, open])

  const handlePick = async ({ setValue, setErr, setRowId }) => {
    setError('')
    setPicking(true)
    try {
      const data = await pickDirectory()
      const picked = data?.path?.trim()
      if (!picked) {
        throw new Error(zh('未获取到目录路径', 'No directory path returned'))
      }
      setValue?.(displayPath(picked))
    } catch (err) {
      if (setErr) {
        setErr(getErrorMessage(err))
      } else {
        setError(getErrorMessage(err))
      }
      if (setRowId) {
        setRowId()
      }
    } finally {
      setPicking(false)
    }
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    if (!path.trim()) {
      setError(zh('路径不能为空', 'Path cannot be empty'))
      return
    }
    setSubmitting(true)
    try {
      await onCreate?.({ path: apiPath(path) })
      setPath('')
      setAdding(false)
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setSubmitting(false)
    }
  }

  const startEdit = (dir) => {
    setEditId(dir.id)
    setEditPath(displayPath(dir.path))
    setRowErrorId(null)
    setRowErrorMsg('')
  }

  const cancelEdit = () => {
    setEditId(null)
    setEditPath('')
    setRowErrorId(null)
    setRowErrorMsg('')
  }

  const handleEditSubmit = async (e) => {
    if (e?.preventDefault) e.preventDefault()
    if (!editId) return
    const trimmed = editPath.trim()
    if (!trimmed) {
      setRowErrorId(editId)
      setRowErrorMsg(zh('路径不能为空', 'Path cannot be empty'))
      return
    }
    setSavingId(editId)
    setRowErrorId(null)
    setRowErrorMsg('')
    try {
      await onUpdate?.(editId, { path: apiPath(trimmed) })
      cancelEdit()
    } catch (err) {
      setRowErrorId(editId)
      setRowErrorMsg(getErrorMessage(err))
    } finally {
      setSavingId(null)
    }
  }

  const handleDelete = async (dir) => {
    if (!dir?.id || dir.is_delete) return
    const ok = window.confirm(
      zh(
        '删除后将不再扫描该目录，该目录下的文件位置会不可用。确认删除？',
        'This directory will no longer be scanned and file locations under it will become unavailable. Delete it?'
      )
    )
    if (!ok) return
    setRowErrorId(null)
    setRowErrorMsg('')
    setDeletingId(dir.id)
    try {
      await onDelete?.(dir.id)
      if (editId === dir.id) {
        cancelEdit()
      }
    } catch (err) {
      setRowErrorId(dir.id)
      setRowErrorMsg(getErrorMessage(err))
    } finally {
      setDeletingId(null)
    }
  }

  const handleProcess = async (dir, mode, layout) => {
    if (!dir?.id || directoryWorkStatus(dir) !== 'idle') return

    setToolDirectory(null)
    setProcessingId(dir.id)
    setRowErrorId(null)
    setRowErrorMsg('')
    try {
      await onProcess?.(dir.id, mode, layout)
    } catch (err) {
      setRowErrorId(dir.id)
      setRowErrorMsg(getErrorMessage(err))
    } finally {
      setProcessingId(null)
    }
  }

  const currentToolDirectory = directories.find((directory) => directory.id === toolDirectory?.id)
  const toolDirectoryWorking =
    processingId === toolDirectory?.id ||
    (currentToolDirectory != null && directoryWorkStatus(currentToolDirectory) !== 'idle')

  return (
    <div className="space-y-3">
      {directories.length > 0 && (
        <div className="divide-y rounded border">
          {directories.map((d) => {
            const isEditing = editId === d.id
            const status = directoryWorkStatus(d)
            const statusDisplay = directoryWorkStatusDisplay(status)
            const lastScanFinishedAt = formatScanFinishedAt(d.last_scan_summary)
            const working =
              savingId === d.id || deletingId === d.id || processingId === d.id || status !== 'idle'
            return (
              <div
                key={d.id}
                className={`flex flex-col gap-2 p-3 sm:flex-row sm:items-center sm:justify-between ${
                  isEditing ? 'rounded border bg-gray-50' : ''
                }`}
              >
                <div className="min-w-0 space-y-1">
                  {!isEditing ? (
                    <div className="truncate text-sm font-medium">{displayPath(d.path)}</div>
                  ) : (
                    <form onSubmit={handleEditSubmit} className="space-y-2">
                      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
                        <input
                          value={editPath}
                          onChange={(e) => setEditPath(e.target.value)}
                          className="w-full rounded border px-3 py-2 text-sm sm:min-w-[420px] sm:flex-1"
                          placeholder={pathPlaceholder}
                        />
                        {directoryPickerEnabled ? (
                          <button
                            type="button"
                            onClick={() => {
                              setRowErrorId(null)
                              setRowErrorMsg('')
                              handlePick({
                                setValue: setEditPath,
                                setErr: setRowErrorMsg,
                                setRowId: () => setRowErrorId(editId),
                              })
                            }}
                            disabled={picking || working}
                            className="rounded border px-3 py-2 text-sm hover:bg-gray-100 disabled:opacity-60"
                          >
                            {picking
                              ? zh('选择中…', 'Picking...')
                              : zh('选择目录', 'Choose directory')}
                          </button>
                        ) : null}
                      </div>
                      <div className="text-xs text-blue-700">{pathHelperText}</div>
                    </form>
                  )}
                  <div className="flex flex-wrap items-center gap-2">
                    {!isEditing && (
                      <div className="flex items-center divide-x divide-zinc-200 text-xs text-zinc-500">
                        <span className="pr-3">
                          {zh('已扫描视频', 'Scanned videos')}{' '}
                          <strong className="font-semibold tabular-nums text-zinc-800">
                            {Number(d.scanned_video_count) || 0}
                          </strong>
                        </span>
                        <span className="pl-3">
                          {zh('已刮削视频', 'Scraped videos')}{' '}
                          <strong className="font-semibold tabular-nums text-zinc-800">
                            {Number(d.scraped_video_count) || 0}
                          </strong>
                        </span>
                      </div>
                    )}
                    {!isEditing && (
                      <span
                        className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${statusDisplay.badge}`}
                      >
                        <span className={`mr-1.5 h-1.5 w-1.5 rounded-full ${statusDisplay.dot}`} />
                        {statusDisplay.label}
                      </span>
                    )}
                    {d.missing && (
                      <span className="inline-flex items-center rounded-full bg-red-50 px-2 py-0.5 text-xs font-medium text-red-700">
                        {zh('目录缺失', 'Missing')}
                      </span>
                    )}
                    {d.is_delete && (
                      <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700">
                        {zh('已删除', 'Deleted')}
                      </span>
                    )}
                  </div>
                  {!isEditing && (
                    <>
                      {lastScanFinishedAt ? (
                        <div className="text-xs text-zinc-500">
                          <span>{zh('上次扫描：结束时间 ', 'Last scan: Finished at ')}</span>
                          <span className="font-semibold tabular-nums text-zinc-900">
                            {lastScanFinishedAt}
                          </span>
                          <span aria-hidden="true" className="mx-2 text-zinc-300">
                            ·
                          </span>
                          <span>{zh('耗时 ', 'Duration ')}</span>
                          <span className="font-semibold tabular-nums text-zinc-900">
                            {formatScanDuration(d.last_scan_summary)}
                          </span>
                        </div>
                      ) : (
                        <div className="text-xs text-zinc-500">
                          {zh('上次扫描：暂无记录', 'Last scan: No record')}
                        </div>
                      )}
                    </>
                  )}
                  {rowErrorId === d.id && rowErrorMsg && (
                    <div className="text-xs text-red-600">{rowErrorMsg}</div>
                  )}
                </div>
                <div className="flex flex-wrap items-center justify-end gap-2">
                  {!isEditing ? (
                    <>
                      <button
                        type="button"
                        onClick={() => {
                          setToolDirectory(d)
                          setToolMode(DIRECTORY_PROCESS_SIDECAR)
                          setToolLayout(DIRECTORY_PROCESS_LAYOUT_PREFIX)
                          setRowErrorId(null)
                          setRowErrorMsg('')
                        }}
                        disabled={d.is_delete || working}
                        className="rounded border border-blue-200 px-3 py-1.5 text-xs text-blue-700 hover:bg-blue-50 disabled:opacity-60"
                      >
                        {processingId === d.id ? zh('启动中…', 'Starting...') : zh('工具', 'Tools')}
                      </button>
                      <button
                        type="button"
                        onClick={() => startEdit(d)}
                        disabled={d.is_delete || working}
                        className="rounded border px-3 py-1.5 text-xs text-gray-700 hover:bg-gray-50 disabled:opacity-60"
                      >
                        {zh('编辑', 'Edit')}
                      </button>
                      <button
                        type="button"
                        onClick={() => handleDelete(d)}
                        disabled={d.is_delete || working}
                        className="rounded border px-3 py-1.5 text-xs text-red-700 hover:bg-red-50 disabled:opacity-60"
                      >
                        {deletingId === d.id ? zh('删除中…', 'Deleting...') : zh('删除', 'Delete')}
                      </button>
                    </>
                  ) : (
                    <>
                      <button
                        type="button"
                        onClick={handleEditSubmit}
                        disabled={working}
                        className="rounded bg-blue-600 px-3 py-1.5 text-xs text-white disabled:opacity-60"
                      >
                        {savingId === d.id ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
                      </button>
                      <button
                        type="button"
                        onClick={cancelEdit}
                        disabled={working}
                        className="rounded border px-3 py-1.5 text-xs text-gray-700 hover:bg-gray-50 disabled:opacity-60"
                      >
                        {zh('取消', 'Cancel')}
                      </button>
                    </>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      )}
      {adding && (
        <form onSubmit={handleSubmit} className="flex flex-col gap-2 rounded border bg-gray-50 p-3">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
            <input
              id="dir-path-input"
              value={path}
              onChange={(e) => setPath(e.target.value)}
              placeholder={pathPlaceholder}
              className="flex-1 rounded border px-3 py-2"
            />
            {directoryPickerEnabled ? (
              <button
                type="button"
                onClick={() => handlePick({ setValue: setPath, setErr: setError })}
                disabled={picking || submitting}
                className="rounded border px-3 py-2 text-sm hover:bg-gray-100 disabled:opacity-60"
              >
                {picking ? zh('选择中…', 'Picking...') : zh('选择目录', 'Choose directory')}
              </button>
            ) : null}
          </div>
          <div className="text-xs text-blue-700">{pathHelperText}</div>
          {error && <div className="text-sm text-red-600">{error}</div>}
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={() => {
                setAdding(false)
                setPath('')
                setError('')
              }}
              className="rounded border px-3 py-1.5 text-sm hover:bg-gray-50"
            >
              {zh('取消', 'Cancel')}
            </button>
            <button
              type="submit"
              disabled={submitting || picking}
              className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white disabled:opacity-60"
            >
              {submitting ? zh('创建中…', 'Creating...') : zh('保存', 'Save')}
            </button>
          </div>
        </form>
      )}
      {!adding && (
        <div className="flex justify-end">
          <button
            type="button"
            onClick={() => {
              setAdding(true)
              setError('')
            }}
            className="rounded border px-3 py-1.5 text-sm hover:bg-gray-50"
          >
            {zh('添加目录', 'Add Directory')}
          </button>
        </div>
      )}
      {toolDirectory && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/40 p-4">
          <div
            role="dialog"
            aria-modal="true"
            aria-labelledby="directory-tools-title"
            className="w-full max-w-lg rounded-2xl bg-white p-5 shadow-xl"
          >
            <div id="directory-tools-title" className="text-base font-semibold text-zinc-900">
              {zh('目录工具', 'Directory Tools')}
            </div>
            <div className="mt-1 truncate text-xs text-zinc-500">
              {displayPath(toolDirectory.path)}
            </div>
            <div className="mt-4 space-y-2">
              {directoryProcessOptions().map((option) => (
                <label
                  key={option.mode}
                  htmlFor={`directory-process-${option.mode}`}
                  aria-label={option.title}
                  className={`flex cursor-pointer gap-3 rounded-xl border p-3 transition ${
                    toolMode === option.mode
                      ? 'border-blue-400 bg-blue-50'
                      : 'border-zinc-200 hover:bg-zinc-50'
                  }`}
                >
                  <input
                    id={`directory-process-${option.mode}`}
                    type="radio"
                    name="directory-process-mode"
                    value={option.mode}
                    checked={toolMode === option.mode}
                    onChange={() => setToolMode(option.mode)}
                    className="mt-0.5"
                  />
                  <span>
                    <span className="block text-sm font-medium text-zinc-900">{option.title}</span>
                    <span className="mt-0.5 block text-xs leading-5 text-zinc-600">
                      {option.description}
                    </span>
                  </span>
                </label>
              ))}
            </div>
            {toolMode !== DIRECTORY_PROCESS_SIDECAR && (
              <div className="mt-4">
                <div className="text-sm font-medium text-zinc-900">
                  {zh('整理方式', 'Organization layout')}
                </div>
                <div className="mt-2 grid gap-2 sm:grid-cols-2">
                  {directoryProcessLayoutOptions().map((option) => (
                    <label
                      key={option.layout}
                      htmlFor={`directory-process-layout-${option.layout}`}
                      className={`cursor-pointer rounded-xl border p-3 transition ${
                        toolLayout === option.layout
                          ? 'border-blue-400 bg-blue-50'
                          : 'border-zinc-200 hover:bg-zinc-50'
                      }`}
                    >
                      <span className="flex items-center gap-2">
                        <input
                          id={`directory-process-layout-${option.layout}`}
                          type="radio"
                          name="directory-process-layout"
                          value={option.layout}
                          checked={toolLayout === option.layout}
                          onChange={() => setToolLayout(option.layout)}
                        />
                        <span className="text-sm font-medium text-zinc-900">{option.title}</span>
                      </span>
                      <span className="mt-1 block pl-6 text-xs text-zinc-500">
                        {option.example}
                      </span>
                    </label>
                  ))}
                </div>
                {toolLayout === DIRECTORY_PROCESS_LAYOUT_IDOL && (
                  <div className="mt-2 text-xs leading-5 text-zinc-500">
                    {zh(
                      '最多拼接 3 位女优名；超过 3 位时统一归入“多女优”，没有女优信息时归入“未知女优”。',
                      'Up to 3 sorted idol names are joined with "，". Works with more than 3 idols go under "多女优", and works without idol metadata go under "未知女优".'
                    )}
                  </div>
                )}
              </div>
            )}
            {toolMode !== DIRECTORY_PROCESS_SIDECAR && (
              <div className="mt-3 rounded-lg bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">
                {zh(
                  '整理后的文件统一位于所选目录中的 JAV 文件夹下。',
                  'Organized files are stored under the JAV folder inside the selected directory.'
                )}
              </div>
            )}
            <div className="mt-5 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setToolDirectory(null)}
                className="rounded border px-3 py-1.5 text-sm text-zinc-700 hover:bg-zinc-50"
              >
                {zh('取消', 'Cancel')}
              </button>
              <button
                type="button"
                onClick={() => handleProcess(toolDirectory, toolMode, toolLayout)}
                disabled={toolDirectoryWorking}
                className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-60"
              >
                {zh('执行', 'Run')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
