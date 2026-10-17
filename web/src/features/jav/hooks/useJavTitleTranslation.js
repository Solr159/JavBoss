import { useEffect, useRef, useState } from 'react'
import { useStore } from '@/store'
import { configFlag } from '@/utils/config'
import { getErrorMessage } from '@/utils/errors'
import { translateJavTitles } from '@/features/jav/api'
import { needsTitleTranslation } from '@/features/jav/titleTranslation'

// Paid requests finish once started; share them across rerenders, StrictMode and list remounts.
const inFlight = new Map()

const batchSize = 20

async function requestTranslations(entries, onResult) {
  // Finish the current group before filling a new one after incremental store updates.
  let selected = entries.filter(({ key }) => inFlight.has(key))
  if (!selected.length) {
    selected = entries.slice(0, batchSize)
    const pending = new Map()
    const group = {}
    for (const entry of selected) {
      const promise = new Promise((resolve, reject) => {
        pending.set(Number(entry.item.id), { resolve, reject })
      })
      inFlight.set(entry.key, { promise, group })
    }
    group.done = translateJavTitles(
      selected.map(({ item }) => item.id),
      {
        onItem: (item) => {
          pending.get(Number(item.id))?.resolve(item)
          pending.delete(Number(item.id))
        },
      }
    )
      .then(({ error }) => {
        for (const task of pending.values()) {
          if (error) task.reject(error)
          else task.resolve(undefined)
        }
      })
      .catch((error) => {
        for (const task of pending.values()) task.reject(error)
      })
      .finally(() => {
        for (const { key } of selected) inFlight.delete(key)
      })
  }
  const tasks = selected.map((entry) => ({ entry, ...inFlight.get(entry.key) }))
  await Promise.all(
    tasks.map(async ({ entry, promise }) => {
      const result = await promise.then(
        (value) => ({ value }),
        (error) => ({ error })
      )
      onResult(entry, result)
    })
  )
  await Promise.all(tasks.map(({ group }) => group.done))
}

export default function useJavTitleTranslation(items) {
  const config = useStore((state) => state.config)
  const patchJavItem = useStore((state) => state.patchJavItem)
  const attempted = useRef(new Set())
  const stopped = useRef(false)
  const [error, setError] = useState('')
  const [revision, setRevision] = useState(0)
  useEffect(() => {
    attempted.current.clear()
    stopped.current = false
    setError('')
  }, [config])

  useEffect(() => {
    if (
      stopped.current ||
      !configFlag(config?.jav_title_translation_enabled) ||
      (!configFlag(config?.jav_title_translation_api_key_set) &&
        config?.jav_title_translation_provider !== 'custom') ||
      !config?.jav_title_translation_model
    )
      return undefined
    const model = config.jav_title_translation_model
    const prompt = config.jav_title_translation_prompt
    const thinking = configFlag(config.jav_title_translation_thinking)
    let cancelled = false
    const run = async () => {
      while (!cancelled && !stopped.current) {
        const seen = new Set()
        const entries = (items || []).flatMap((item) => {
          if (!needsTitleTranslation(item) || seen.has(Number(item.id))) return []
          seen.add(Number(item.id))
          const key = JSON.stringify([
            item.id,
            item.title,
            item.zh_title,
            model,
            prompt,
            thinking,
            config.jav_title_translation_provider,
            config.jav_title_translation_profiles,
          ])
          return attempted.current.has(key) ? [] : [{ item, key }]
        })
        if (!entries.length) return
        await requestTranslations(entries, ({ key }, result) => {
          if (cancelled) return
          attempted.current.add(key)
          if (result.error) {
            // Stop before a store update can trigger another group.
            stopped.current = true
            setError(getErrorMessage(result.error))
          } else if (result.value) {
            patchJavItem(result.value)
          }
        })
      }
    }
    void run()
    return () => {
      cancelled = true
    }
  }, [items, config, revision, patchJavItem])

  return {
    error,
    retry: () => {
      attempted.current.clear()
      stopped.current = false
      setError('')
      setRevision((value) => value + 1)
    },
  }
}
