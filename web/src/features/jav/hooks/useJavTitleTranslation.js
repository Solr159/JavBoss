import { useEffect, useRef, useState } from 'react'
import { useStore } from '@/store'
import { configFlag } from '@/utils/config'
import { getErrorMessage } from '@/utils/errors'
import { translateJavTitle } from '@/features/jav/api'
import { needsTitleTranslation } from '@/features/jav/titleTranslation'

// Paid requests finish once started; share them across rerenders, StrictMode and list remounts.
const inFlight = new Map()

function requestTranslation(item, key) {
  if (!inFlight.has(key)) {
    const request = translateJavTitle(item.id).finally(() => inFlight.delete(key))
    inFlight.set(key, request)
  }
  return inFlight.get(key)
}

export default function useJavTitleTranslation(items) {
  const config = useStore((state) => state.config)
  const patchJavItem = useStore((state) => state.patchJavItem)
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
    const prompt = config.jav_title_translation_prompt
    const thinking = configFlag(config.jav_title_translation_thinking)
    let cancelled = false
    const run = async () => {
      for (const item of items || []) {
        if (cancelled) break
        const key = JSON.stringify([item.id, item.title, item.zh_title, model, prompt, thinking])
        if (!needsTitleTranslation(item) || attempted.current.has(key)) continue
        try {
          const updated = await requestTranslation(item, key)
          if (cancelled) break
          attempted.current.add(key)
          patchJavItem(updated)
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
  }, [items, config, error, patchJavItem])

  return {
    error,
    retry: () => {
      attempted.current.clear()
      setError('')
    },
  }
}
