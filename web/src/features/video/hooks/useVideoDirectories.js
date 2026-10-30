import { useEffect, useState } from 'react'
import { fetchVideoDirectories } from '@/features/video/api'
import { directoryScopeKey } from '@/query/listQueries'
import { useStore } from '@/store'
import { getErrorMessage } from '@/utils/errors'

export default function useVideoDirectories(open) {
  const scopeKey = useStore(directoryScopeKey)
  const [options, setOptions] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    if (!open) return undefined
    const controller = new AbortController()
    setLoading(true)
    setError('')
    setOptions([])
    fetchVideoDirectories({ signal: controller.signal })
      .then((items) => {
        if (!controller.signal.aborted) setOptions(items)
      })
      .catch((err) => {
        if (!controller.signal.aborted) setError(getErrorMessage(err))
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    return () => controller.abort()
  }, [open, scopeKey, attempt])

  return { options, loading, error, retry: () => setAttempt((current) => current + 1) }
}
