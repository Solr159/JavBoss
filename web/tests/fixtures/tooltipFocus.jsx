import Tooltip from '@/shared/ui/Tooltip'
import { StrictMode, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { Button, Menu, MenuItem } from '@mui/material'
import AppModal from '@/shared/ui/AppModal'
import '@/index.css'

function Fixture() {
  const [dialog, setDialog] = useState(false)
  const [anchor, setAnchor] = useState(null)
  const [tip, setTip] = useState(false)
  return (
    <main className="flex gap-6 p-20">
      <button id="before">Before</button>
      <Tooltip title="Open editor" arrow>
        <Button id="editor-trigger" onClick={() => setDialog(true)}>
          Edit
        </Button>
      </Tooltip>
      <Tooltip
        title="Open actions"
        open={tip}
        onOpen={() => setTip(true)}
        onClose={() => setTip(false)}
      >
        <span>
          <Button id="menu-trigger" onClick={(event) => setAnchor(event.currentTarget)}>
            Actions
          </Button>
        </span>
      </Tooltip>
      <button id="after">After</button>
      {dialog && (
        <AppModal
          ariaLabel="Editor"
          onClose={() => setDialog(false)}
          contentClassName="bg-white p-10"
        >
          <input aria-label="Name" />
          <Button onClick={() => setDialog(false)}>Save</Button>
        </AppModal>
      )}
      <Menu open={Boolean(anchor)} anchorEl={anchor} onClose={() => setAnchor(null)}>
        <MenuItem onClick={() => setAnchor(null)}>Action</MenuItem>
      </Menu>
    </main>
  )
}
createRoot(document.getElementById('root')).render(
  <StrictMode>
    <Fixture />
  </StrictMode>
)
