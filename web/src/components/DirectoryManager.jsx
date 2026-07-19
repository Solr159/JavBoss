import { useEffect, useState } from 'react'

import { pickDirectory } from '@/api'
import { apiHostPath, displayHostPath } from '@/utils/hostPath'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'

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
    }
  }, [open])

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

  return (
    <div className="space-y-3">
      {directories.length > 0 && (
        <div className="divide-y rounded border">
          {directories.map((d) => {
            const isEditing = editId === d.id
            const working = savingId === d.id || deletingId === d.id
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
                  {rowErrorId === d.id && rowErrorMsg && (
                    <div className="text-xs text-red-600">{rowErrorMsg}</div>
                  )}
                </div>
                <div className="flex flex-wrap items-center justify-end gap-2">
                  {!isEditing ? (
                    <>
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
    </div>
  )
}
