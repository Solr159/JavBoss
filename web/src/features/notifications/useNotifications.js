import { useState, useCallback } from 'react'

export default function useNotifications() {
  const [toastMessage, setToastMessage] = useState('')

  const [toastDuration, setToastDuration] = useState(1800)

  const [toastId, setToastId] = useState(0)

  const [centerToastMessage, setCenterToastMessage] = useState('')

  const showToast = useCallback((message, duration = 1800) => {
    setToastMessage(String(message || '').trim())
    setToastDuration(duration)
    setToastId((id) => id + 1)
  }, [])

  const closeToast = useCallback(() => {
    setToastMessage('')
  }, [])

  const showCenterToast = useCallback((message) => {
    setCenterToastMessage(String(message || '').trim())
  }, [])

  const closeCenterToast = useCallback(() => {
    setCenterToastMessage('')
  }, [])
  return {
    toastMessage,
    toastDuration,
    toastId,
    centerToastMessage,
    showToast,
    closeToast,
    showCenterToast,
    closeCenterToast,
  }
}
