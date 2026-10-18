import { cloneElement, forwardRef, useRef, useState } from 'react'
import { Tooltip as MuiTooltip } from '@mui/material'

const Tooltip = forwardRef(function Tooltip({ children, open, onOpen, onClose, ...props }, ref) {
  const [visible, setVisible] = useState(false)
  const interaction = useRef({ dismissed: false, pending: null })

  const handleOpen = (event) => {
    if (interaction.current.dismissed) {
      interaction.current.pending = event
      return
    }
    setVisible(true)
    onOpen?.(event)
  }
  const handleClose = (event) => {
    interaction.current.pending = null
    setVisible(false)
    onClose?.(event)
  }
  const resume = (showPending = false) => {
    const { pending } = interaction.current
    interaction.current = { dismissed: false, pending: null }
    if (showPending && pending) handleOpen(pending)
  }
  const withChildHandler = (name, handler) => (event) => {
    handler(event)
    children.props[name]?.(event)
  }

  return (
    <MuiTooltip
      {...props}
      ref={ref}
      open={open ?? visible}
      onOpen={handleOpen}
      onClose={handleClose}
    >
      {cloneElement(children, {
        // Keep restored focus and overlay-generated hover silent after activation.
        onClickCapture: withChildHandler('onClickCapture', (event) => {
          interaction.current.dismissed = true
          handleClose(event)
        }),
        // Only fresh input resumes hints; MUI still owns hover/focus/touch timing.
        onMouseMoveCapture: withChildHandler('onMouseMoveCapture', () => resume(true)),
        onKeyDownCapture: withChildHandler('onKeyDownCapture', () => resume()),
        onTouchStartCapture: withChildHandler('onTouchStartCapture', () => resume()),
        onBlurCapture: withChildHandler('onBlurCapture', () => {
          if (interaction.current.pending) resume()
        }),
      })}
    </MuiTooltip>
  )
})

export default Tooltip
