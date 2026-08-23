import { Button, IconButton, Tooltip } from '@mui/material'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import RemoveCircleOutlineRoundedIcon from '@mui/icons-material/RemoveCircleOutlineRounded'
import AppModal from '@/components/AppModal'
import { zh } from '@/utils/i18n'

export default function SelectionOpsModal({
  open,
  onClose,
  selectedList,
  selectedCount,
  selectedJavCount = 0,
  onOpenTags,
  onOpenJavTags,
  onOpenRemoveTags,
  onRemoveSelected,
  onDeleteSelected,
  deleting = false,
}) {
  if (!open) return null

  const list = Array.isArray(selectedList) ? selectedList : []
  const count = Number.isFinite(selectedCount) ? selectedCount : 0

  return (
    <AppModal
      ariaLabel={zh('已选择文件', 'Selected Files')}
      className="px-4"
      closeDisabled={deleting}
      contentClassName="w-full max-w-lg rounded-lg bg-white p-4 shadow-xl"
      onClose={onClose}
    >
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-base font-semibold">{zh('已选择文件', 'Selected Files')}</h2>
        <button
          onClick={onClose}
          className="rounded px-2 py-1 text-gray-500 hover:bg-gray-100"
          aria-label={zh('关闭', 'Close')}
        >
          ✕
        </button>
      </div>
      <div className="max-h-[60vh] overflow-y-auto rounded border bg-gray-50 p-2 text-sm">
        {list.length === 0 ? (
          <div className="text-gray-500">{zh('暂无选择', 'No files selected')}</div>
        ) : (
          <ul className="space-y-0.5">
            {list.map((item) => (
              <li
                key={item.id}
                className="flex min-w-0 items-center gap-2 rounded px-2 py-0.5 text-gray-800"
              >
                <span className="min-w-0 flex-1 truncate">{item.label}</span>
                <Tooltip title={zh('移除所选', 'Remove from selection')} arrow>
                  <span>
                    <IconButton
                      size="small"
                      color="error"
                      onClick={() => onRemoveSelected?.(item.id)}
                      disabled={deleting}
                      aria-label={zh('移除所选', 'Remove from selection')}
                      className="!h-6 !w-6 !p-0"
                    >
                      <RemoveCircleOutlineRoundedIcon fontSize="inherit" />
                    </IconButton>
                  </span>
                </Tooltip>
              </li>
            ))}
          </ul>
        )}
      </div>
      <div className="mt-4 flex flex-wrap justify-end gap-2">
        <Button
          variant="outlined"
          size="small"
          onClick={onOpenTags}
          disabled={count === 0 || deleting}
        >
          {zh('添加标签', 'Add Tags')}
        </Button>
        <Button
          variant="outlined"
          size="small"
          onClick={onOpenJavTags}
          disabled={selectedJavCount === 0 || deleting}
        >
          {zh('添加 JAV 标签', 'Add JAV Tags')}
        </Button>
        <Button
          variant="outlined"
          color="error"
          size="small"
          onClick={onOpenRemoveTags}
          disabled={count === 0 || deleting}
        >
          {zh('移除标签', 'Remove Tags')}
        </Button>
        <Button
          variant="contained"
          color="error"
          size="small"
          onClick={onDeleteSelected}
          disabled={count === 0 || deleting}
          startIcon={<DeleteOutlineIcon fontSize="inherit" />}
        >
          {deleting ? zh('删除中…', 'Deleting...') : zh('删除所选视频', 'Delete Selected Videos')}
        </Button>
      </div>
    </AppModal>
  )
}
