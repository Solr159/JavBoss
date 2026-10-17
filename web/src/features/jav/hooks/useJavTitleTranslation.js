import { useEffect, useRef, useState } from 'react'
import { useStore } from '@/store'
import { configFlag } from '@/utils/config'
import { getErrorMessage } from '@/utils/errors'
import { translateJavTitles, translateJavSeriesNames } from '@/features/jav/api'
import { parseTranslationConfig } from '@/features/jav/titleTranslation'

// Paid requests finish once started; share them across rerenders, StrictMode and list remounts.
const inFlight = new Map()

const batchSize = 20
const domains = {
  title: {
    source: 'title',
    translated: 'zh_title',
    enabled: 'jav_title_translation_enabled',
    patch: 'patchJavItem',
    translate: translateJavTitles,
  },
  series: {
    source: 'name',
    translated: 'zh_name',
    enabled: 'jav_series_translation_enabled',
    patch: 'patchJavSeries',
    translate: translateJavSeriesNames,
  },
}

async function requestTranslations(entries, onResult, domain) {
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
    group.done = domain
      .translate(
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
  return useJavTextTranslation(items, 'title')
}

export function useJavSeriesTranslation(items, onTranslated) {
  return useJavTextTranslation(items, 'series', onTranslated)
}

function useJavTextTranslation(items, target, onTranslated) {
  const domain = domains[target]
  const config = useStore((state) => state.config)
  const patchItem = useStore((state) => state[domain.patch])
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
    const provider = config?.jav_title_translation_provider
    const profile = parseTranslationConfig(config?.jav_title_translation_profiles)[provider]
    if (
      stopped.current ||
      !configFlag(config?.[domain.enabled]) ||
      !configFlag(config?.[`jav_${target}_translation_auto_enabled`]) ||
      (!profile?.api_key_set && provider !== 'custom') ||
      !profile?.model
    )
      return undefined
    const prompt = config.jav_title_translation_prompt
    let cancelled = false
    const run = async () => {
      while (!cancelled && !stopped.current) {
        const seen = new Set()
        const entries = (items || []).flatMap((item) => {
          if (
            !(
              Number(item?.id) > 0 &&
              String(item[domain.source] || '').trim() &&
              !String(item[domain.translated] || '').trim()
            ) ||
            seen.has(Number(item.id))
          )
            return []
          seen.add(Number(item.id))
          const key = JSON.stringify([
            target,
            item.id,
            item[domain.source],
            item[domain.translated],
            provider,
            profile,
            prompt,
          ])
          return attempted.current.has(key) ? [] : [{ item, key }]
        })
        if (!entries.length) return
        await requestTranslations(
          entries,
          ({ key }, result) => {
            if (cancelled) return
            attempted.current.add(key)
            if (result.error) {
              // Stop before a store update can trigger another group.
              stopped.current = true
              setError(getErrorMessage(result.error))
            } else if (result.value) {
              patchItem(result.value)
              onTranslated?.(result.value)
            }
          },
          domain
        )
      }
    }
    void run()
    return () => {
      cancelled = true
    }
  }, [items, config, revision, patchItem, domain, target, onTranslated])

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
