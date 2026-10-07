import { useEffect, useRef, useState } from 'react'
import { useStore } from '@/store'
import { configFlag } from '@/utils/config'
import { getErrorMessage } from '@/utils/errors'
import { translateJavSeriesName } from '@/features/jav/api'
import { needsSeriesNameTranslation } from '@/features/jav/titleTranslation'

// Paid requests finish once started; share them across rerenders, StrictMode and list remounts.
const inFlight = new Map()

function requestTranslation(item, key) {
  if (!inFlight.has(key)) {
    const request = translateJavSeriesName(item.id).finally(() => inFlight.delete(key))
    inFlight.set(key, request)
  }
  return inFlight.get(key)
}

export default function useJavSeriesNameTranslation(items) {
  const config = useStore((state) => state.config)
  const patchJavSeries = useStore((state) => state.patchJavSeries)
  const attempted = useRef(new Set())
  const [error, setError] = useState('')
  useEffect(() => {
    attempted.current.clear()
    setError('')
  }, [config])

  useEffect(() => {
    if (
      error ||
      !configFlag(config?.jav_title_translation_enabled) ||
      !configFlag(config?.jav_title_translation_api_key_set) ||
      !config?.jav_title_translation_model
    )
      return undefined
    const model = config.jav_title_translation_model
    const thinking = configFlag(config.jav_title_translation_thinking)
    let cancelled = false
    const run = async () => {
      for (const item of items || []) {
        if (cancelled) break
        const key = JSON.stringify([item.id, item.name, item.zh_name, model, thinking])
        if (!needsSeriesNameTranslation(item) || attempted.current.has(key)) continue
        try {
          const updated = await requestTranslation(item, key)
          if (cancelled) break
          attempted.current.add(key)
          patchJavSeries(updated)
        } catch (err) {
          if (cancelled) break
          attempted.current.add(key)
          setError(getErrorMessage(err))
          break
        }
      }
    }
    void run()
    return () => {
      cancelled = true
    }
  }, [items, config, error, patchJavSeries])

  return {
    error,
    retry: () => {
      attempted.current.clear()
      setError('')
    },
  }
}
