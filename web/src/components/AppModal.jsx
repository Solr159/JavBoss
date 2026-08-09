import { useEffect } from 'react'
import { Modal } from '@mui/material'

let openModalCount = 0

export default function AppModal({
  ariaLabel,
  ariaLabelledby,
  backdropBlur,
  backdropColor = 'rgba(0, 0, 0, 0.5)',
  children,
  className = '',
  closeDisabled = false,
  contentClassName = '',
  contentComponent: ContentComponent = 'div',
  contentProps,
  onClose,
  open = true,
  zIndex,
}) {
  useEffect(() => {
    if (!open) return undefined

    openModalCount += 1
    document.documentElement.classList.add('app-modal-open')

    return () => {
      openModalCount = Math.max(0, openModalCount - 1)
      if (openModalCount === 0) {
        document.documentElement.classList.remove('app-modal-open')
      }
    }
  }, [open])

  const handleClose = (event, reason) => {
    if (closeDisabled && (reason === 'backdropClick' || reason === 'escapeKeyDown')) return
    onClose?.(event, reason)
  }

  return (
    <Modal
      open={open}
      onClose={handleClose}
      disableEscapeKeyDown={closeDisabled}
      disableScrollLock
      className={`flex items-center justify-center ${className}`}
      sx={zIndex ? { zIndex } : undefined}
      slotProps={{
        backdrop: {
          sx: {
            backgroundColor: backdropColor,
            backdropFilter: backdropBlur ? `blur(${backdropBlur})` : undefined,
          },
        },
      }}
    >
      <ContentComponent
        {...contentProps}
        role="dialog"
        aria-modal="true"
        aria-label={ariaLabelledby ? undefined : ariaLabel}
        aria-labelledby={ariaLabelledby}
        className={`outline-none ${contentClassName}`}
      >
        {children}
      </ContentComponent>
    </Modal>
  )
}
