import SettingsRoundedIcon from '@mui/icons-material/SettingsRounded'
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded'
import { IconButton, Menu, MenuItem, Tooltip } from '@mui/material'
import { useState } from 'react'
import { zh } from '@/utils/i18n'

export default function BulkActionsMenu({
  label,
  hasItems,
  pageSelectable,
  busy,
  bulkPlaybackEnabled,
  onSelectAll,
  onSelectPage,
  onPlayPage,
  onPlayAll,
}) {
  const [anchorEl, setAnchorEl] = useState(null)
  const runAction = (action) => {
    setAnchorEl(null)
    action?.()
  }

  return (
    <>
      <Tooltip title={label} arrow>
        <span className="inline-flex">
          <IconButton
            size="small"
            onClick={(event) => setAnchorEl(event.currentTarget)}
            disabled={!hasItems || busy}
            aria-label={label}
            aria-haspopup="menu"
            aria-expanded={Boolean(anchorEl)}
            className="pagination-bulk-action"
          >
            <SettingsRoundedIcon fontSize="inherit" />
          </IconButton>
        </span>
      </Tooltip>
      <Menu
        open={Boolean(anchorEl)}
        anchorEl={anchorEl}
        onClose={() => setAnchorEl(null)}
        disableScrollLock
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'left' }}
        MenuListProps={{ dense: true, 'aria-label': label }}
      >
        <MenuItem disabled={!pageSelectable || busy} onClick={() => runAction(onSelectPage)}>
          {zh('选中本页', 'Select page')}
        </MenuItem>
        <MenuItem disabled={!hasItems || busy} onClick={() => runAction(onSelectAll)}>
          {zh('选中全部', 'Select all')}
        </MenuItem>
        <MenuItem
          disabled={!pageSelectable || !bulkPlaybackEnabled || busy}
          onClick={() => runAction(onPlayPage)}
        >
          <span className="flex-1">{zh('播放本页', 'Play page')}</span>
          <PlayArrowRoundedIcon fontSize="small" sx={{ ml: 2 }} />
        </MenuItem>
        <MenuItem
          disabled={!hasItems || !bulkPlaybackEnabled || busy}
          onClick={() => runAction(onPlayAll)}
        >
          <span className="flex-1">{zh('播放全部', 'Play all')}</span>
          <PlayArrowRoundedIcon fontSize="small" sx={{ ml: 2 }} />
        </MenuItem>
      </Menu>
    </>
  )
}
