import { useState } from 'react'
import { CircularProgress } from '@mui/material'
import { cleanDirectoryFiles, previewDirectoryCleanup } from '@/features/directories/api'
import AppModal from '@/shared/ui/AppModal'
import { formatBytes } from '@/utils/display'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

const formatCleanupBytes = (bytes) => formatBytes(bytes) || '0 B'

const cleanupCategories = () => [
  {
    reason: 'system',
    option: 'include_system',
    label: zh('系统垃圾', 'System junk'),
    description: '.DS_Store · Thumbs.db · ehthumbs.db · desktop.ini',
  },
  {
    reason: 'misc',
    option: 'include_misc',
    label: zh('文本、网址和网页存档', 'Text, links and web archives'),
    description: '.txt · .url · .html · .htm · .mht · .mhtml · .chm · .rtf',
  },
  {
    reason: 'torrent',
    option: 'include_torrents',
    label: zh('种子文件', 'Torrent files'),
    description: '.torrent',
  },
  {
    reason: 'shortcut',
    option: 'include_shortcuts',
    label: zh('Windows 快捷方式', 'Windows shortcuts'),
    description: '.lnk',
  },
  {
    reason: 'ad_image',
    option: 'include_ad_images',
    label: zh('疑似广告图片', 'Possible ad images'),
    description: zh(
      '文件名包含二维码、宣传、地址发布等关键词；保留标准封面命名',
      'Names containing QR code, promotion or address-publication keywords; standard artwork names are preserved'
    ),
  },
  {
    reason: 'download',
    option: 'include_downloads',
    label: zh('下载残留', 'Download leftovers'),
    description: zh(
      '.xltd、.aria2、.part、.crdownload；跳过已入库或可解析的媒体，请先停止下载',
      '.xltd, .aria2, .part, .crdownload; skip indexed or readable media. Stop downloads first'
    ),
  },
]

const issueLabel = (reason) => {
  switch (reason) {
    case 'changed':
      return zh(
        '文件已变化、不存在或为符号链接，请重新扫描',
        'File changed, missing, or a symbolic link; scan again'
      )
    case 'unreadable':
      return zh('无法读取，请检查权限', 'Cannot read; check permissions')
    case 'cancelled':
      return zh('操作已中止', 'Operation cancelled')
    case 'indexed_video':
      return zh('已入库视频，已保留', 'Indexed video; preserved')
    case 'media_file':
      return zh('可解析的媒体文件，已保留', 'Readable media file; preserved')
    case 'probe_failed':
      return zh(
        '无法确认媒体状态，已跳过；请检查 FFmpeg 工具、文件访问或超时情况',
        'Could not verify media; skipped. Check FFmpeg tools, file access or timeouts'
      )
    default:
      return zh(
        '删除失败，请检查权限或文件占用',
        'Deletion failed; check permissions or whether the file is in use'
      )
  }
}

export default function DirectoryCleanupModal({ directory, displayPath, onClose }) {
  const [options, setOptions] = useState({
    include_system: true,
    include_misc: true,
    include_torrents: true,
    include_shortcuts: true,
    include_ad_images: true,
    include_downloads: false,
  })
  const [preview, setPreview] = useState(null)
  const [selected, setSelected] = useState(new Set())
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [result, setResult] = useState(null)
  const files = preview?.files || []
  const chosen = files.filter((file) => selected.has(file.path))
  const chosenBytes = chosen.reduce((total, file) => total + file.size, 0)
  const categories = cleanupCategories()

  const scan = async () => {
    setBusy('scan')
    setError('')
    setResult(null)
    setPreview(null)
    setSelected(new Set())
    try {
      setPreview(await previewDirectoryCleanup(directory.id, options))
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setBusy('')
    }
  }

  const clean = async () => {
    if (!chosen.length) return
    if (
      !window.confirm(
        zh(
          `将永久删除所选的 ${chosen.length} 个文件（${formatCleanupBytes(chosenBytes)}），不会移入回收站。确认清理？`,
          `Permanently delete ${chosen.length} selected files (${formatCleanupBytes(chosenBytes)})? They will not be moved to the recycle bin.`
        )
      )
    )
      return
    setBusy('clean')
    setError('')
    setResult(null)
    try {
      const next = await cleanDirectoryFiles(directory.id, preview.root_path, chosen)
      setResult(next)
      const deleted = new Set(next.deleted)
      setPreview((current) => ({
        ...current,
        files: current.files.filter((file) => !deleted.has(file.path)),
      }))
      setSelected(new Set())
    } catch (err) {
      setError(getErrorMessage(err))
      // A lost response can follow a partial deletion. Require a fresh preview.
      setPreview(null)
      setSelected(new Set())
    } finally {
      setBusy('')
    }
  }

  return (
    <AppModal
      ariaLabelledby="directory-cleanup-title"
      className="p-4"
      closeDisabled={Boolean(busy)}
      contentClassName="flex max-h-[90vh] w-full max-w-2xl flex-col overflow-y-auto rounded-2xl bg-white p-5 shadow-xl"
      onClose={onClose}
      zIndex={1400}
    >
      <h2 id="directory-cleanup-title" className="text-base font-semibold text-zinc-900">
        {zh('垃圾文件清理', 'Clean junk files')}
      </h2>
      <div className="mt-1 break-all text-xs text-zinc-500">{displayPath}</div>
      <p className="mt-3 text-sm leading-6 text-zinc-600">
        {zh(
          '递归扫描当前目录及子目录，跳过符号链接。默认保留视频、字幕、NFO、普通图片和整理报告。广告图片仅按文件名推断，请检查候选后再勾选。',
          'Scan this folder and its subfolders, skipping symbolic links. Preserve videos, subtitles, NFO files, ordinary images and organization reports by default. Ad images are inferred from filenames; review candidates before selecting.'
        )}
      </p>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        {categories
          .filter((category) => category.option)
          .map((category) => (
            <label key={category.option} className="flex items-start gap-2 text-sm text-zinc-800">
              <input
                type="checkbox"
                aria-label={category.label}
                checked={options[category.option]}
                disabled={Boolean(busy)}
                className="mt-1"
                onChange={(event) => {
                  setOptions((current) => ({ ...current, [category.option]: event.target.checked }))
                  setPreview(null)
                  setSelected(new Set())
                  setResult(null)
                  setError('')
                }}
              />
              <span>
                <span className="block">{category.label}</span>
                <span className="mt-0.5 block text-xs leading-5 text-zinc-500">
                  {category.description}
                </span>
              </span>
            </label>
          ))}
      </div>
      {preview && (
        <div className="mt-4">
          <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                aria-label={zh('全选', 'Select all')}
                disabled={Boolean(busy) || !files.length}
                checked={files.length > 0 && files.every((file) => selected.has(file.path))}
                onChange={(event) =>
                  setSelected(new Set(event.target.checked ? files.map((file) => file.path) : []))
                }
              />
              {zh('全选', 'Select all')}
            </label>
            <span className="text-zinc-500">
              {zh(`已选 ${chosen.length} 个`, `${chosen.length} selected`)} ·{' '}
              {formatCleanupBytes(chosenBytes)}
            </span>
          </div>
          <p className="mt-2 text-xs text-zinc-500">
            {zh(`找到 ${files.length} 个文件`, `${files.length} files found`)}
          </p>
          {files.length ? (
            <div className="mt-2 max-h-64 overflow-auto rounded-lg border border-zinc-200">
              {categories.map((category) => {
                const group = files.filter((file) => file.reason === category.reason)
                if (!group.length) return null
                return (
                  <div key={category.reason}>
                    <div className="bg-zinc-50 px-3 py-2 text-xs font-medium text-zinc-600">
                      {category.label} · {group.length} ·{' '}
                      {formatCleanupBytes(group.reduce((sum, file) => sum + file.size, 0))}
                    </div>
                    {group.map((file) => (
                      <label
                        key={file.path}
                        className="flex items-start gap-3 border-b border-zinc-100 px-3 py-2 last:border-0 hover:bg-zinc-50"
                      >
                        <input
                          type="checkbox"
                          aria-label={file.path}
                          checked={selected.has(file.path)}
                          disabled={Boolean(busy)}
                          className="mt-1"
                          onChange={(event) =>
                            setSelected((current) => {
                              const next = new Set(current)
                              if (event.target.checked) next.add(file.path)
                              else next.delete(file.path)
                              return next
                            })
                          }
                        />
                        <span className="min-w-0 flex-1">
                          <span className="block break-all text-sm text-zinc-800">{file.path}</span>
                          <span className="text-xs text-zinc-500">
                            {formatCleanupBytes(file.size)} · {category.label}
                          </span>
                        </span>
                      </label>
                    ))}
                  </div>
                )
              })}
            </div>
          ) : (
            <p className="mt-3 text-sm text-zinc-500">
              {zh('没有待清理的匹配文件。', 'No matching files to clean.')}
            </p>
          )}
          {preview.truncated && (
            <p className="mt-2 text-xs text-amber-700">
              {zh(
                '本次最多显示 10000 个文件，清理后请再次扫描。',
                'Showing up to 10,000 files. Scan again after cleaning.'
              )}
            </p>
          )}
        </div>
      )}
      {result && (
        <p role="status" className="mt-3 text-sm text-zinc-700">
          {zh(
            `已删除 ${result.deleted.length} 个文件，释放 ${formatCleanupBytes(result.deleted_bytes)}；未删除 ${result.issues.length} 个。`,
            `Deleted ${result.deleted.length} files, freed ${formatCleanupBytes(result.deleted_bytes)}; ${result.issues.length} files were not deleted.`
          )}
        </p>
      )}
      {(result?.issues.length > 0 || preview?.issues.length > 0) && (
        <details className="mt-3 text-sm text-amber-800">
          <summary className="cursor-pointer">
            {zh('查看跳过或失败的项目', 'View skipped or failed items')}
          </summary>
          <ul className="mt-2 max-h-32 space-y-1 overflow-auto text-xs">
            {[...(preview?.issues || []), ...(result?.issues || [])].map((issue, index) => (
              <li key={`${issue.path}-${index}`} className="break-all">
                {issue.path}：{issueLabel(issue.reason)}
              </li>
            ))}
          </ul>
        </details>
      )}
      {error && (
        <p role="alert" className="mt-3 text-sm text-red-600">
          {error}
        </p>
      )}
      <div className="mt-5 flex flex-wrap items-center justify-end gap-2">
        {busy && (
          <span role="status" className="mr-auto flex items-center gap-2 text-sm text-zinc-500">
            <CircularProgress size={16} />
            {busy === 'scan' ? zh('扫描中…', 'Scanning…') : zh('清理中…', 'Cleaning…')}
          </span>
        )}
        <button
          type="button"
          onClick={onClose}
          disabled={Boolean(busy)}
          className="rounded border px-3 py-1.5 text-sm text-zinc-700 hover:bg-zinc-50 disabled:opacity-60"
        >
          {zh('关闭', 'Close')}
        </button>
        <button
          type="button"
          onClick={scan}
          disabled={Boolean(busy)}
          className="rounded border border-blue-600 px-3 py-1.5 text-sm text-blue-700 hover:bg-blue-50 disabled:opacity-60"
        >
          {preview ? zh('重新扫描', 'Scan again') : zh('扫描垃圾文件', 'Scan junk files')}
        </button>
        <button
          type="button"
          onClick={clean}
          disabled={Boolean(busy) || !chosen.length}
          className="rounded bg-red-600 px-3 py-1.5 text-sm text-white hover:bg-red-700 disabled:opacity-60"
        >
          {zh('清理所选文件', 'Clean selected files')}
        </button>
      </div>
    </AppModal>
  )
}
