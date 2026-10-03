import { useEffect, useState } from 'react'
import { fetchResources } from '@/features/settings/api'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

export default function useResources() {
  const [visible, setVisible] = useState(() => document.visibilityState !== 'hidden')
  const [snapshot, setSnapshot] = useState(null)
  const [error, setError] = useState('')

  useEffect(() => {
    const updateVisibility = () => setVisible(document.visibilityState !== 'hidden')
    document.addEventListener('visibilitychange', updateVisibility)
    return () => document.removeEventListener('visibilitychange', updateVisibility)
  }, [])

  useEffect(() => {
    if (!visible) return
    let stopped = false
    let timer
    let timeout
    let controller
    const poll = async () => {
      controller = new AbortController()
      timeout = setTimeout(() => controller.abort(), 10000)
      try {
        const next = await fetchResources({ signal: controller.signal })
        if (stopped) return
        setSnapshot(next)
        setError('')
      } catch (err) {
        if (!stopped) {
          setError(
            controller.signal.aborted
              ? zh('请求超时，稍后自动重试', 'Request timed out; retrying shortly')
              : getErrorMessage(err)
          )
        }
      } finally {
        clearTimeout(timeout)
        if (!stopped) timer = setTimeout(poll, 3000)
      }
    }
    poll()
    return () => {
      stopped = true
      clearTimeout(timer)
      clearTimeout(timeout)
      controller?.abort()
    }
  }, [visible])

  return { snapshot, error, refreshing: visible }
}
