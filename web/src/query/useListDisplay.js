import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { useEffect, useCallback } from 'react'
import { configFlag } from '@/utils/config'

export default function useListDisplay({ configLoaded, hydrated }) {
  const {
    config,
    loadVideos,
    loadJavIdols,
    loadJavFavoriteGroups,
    loadJavStudios,
    loadJavSeries,
    loadJavs,
    waterfallModes,
    setWaterfallModes,
  } = useStore(
    useShallow((state) => ({
      config: state.config,
      loadVideos: state.loadVideos,
      loadJavIdols: state.loadJavIdols,
      loadJavFavoriteGroups: state.loadJavFavoriteGroups,
      loadJavStudios: state.loadJavStudios,
      loadJavSeries: state.loadJavSeries,
      loadJavs: state.loadJavs,
      waterfallModes: state.waterfallModes,
      setWaterfallModes: state.setWaterfallModes,
    }))
  )

  useEffect(() => {
    if (!configLoaded) return
    setWaterfallModes((current) => ({
      ...current,
      video: configFlag(config?.video_waterfall_default),
      jav: configFlag(config?.jav_waterfall_default),
      idol: configFlag(config?.idol_waterfall_default),
      studio: configFlag(config?.studio_waterfall_default),
      series: configFlag(config?.series_waterfall_default),
    }))
  }, [
    setWaterfallModes,
    configLoaded,
    config?.video_waterfall_default,
    config?.jav_waterfall_default,
    config?.idol_waterfall_default,
    config?.studio_waterfall_default,
    config?.series_waterfall_default,
  ])

  const forceReloadVideos = useCallback(() => {
    if (!hydrated || !configLoaded) return
    loadVideos({ force: true })
  }, [configLoaded, hydrated, loadVideos])

  const forceReloadJavByTab = useCallback(
    (tab) => {
      if (!hydrated || !configLoaded) return
      if (tab === 'download') {
        return
      } else if (tab === 'idol') {
        loadJavIdols({ force: true })
        loadJavFavoriteGroups('idol', { force: true })
      } else if (tab === 'studio') {
        loadJavStudios({ force: true })
        loadJavFavoriteGroups('studio', { force: true })
      } else if (tab === 'series') {
        loadJavSeries({ force: true })
        loadJavFavoriteGroups('series', { force: true })
      } else {
        loadJavs({ force: true })
        loadJavFavoriteGroups('jav', { force: true })
      }
    },
    [
      configLoaded,
      hydrated,
      loadJavFavoriteGroups,
      loadJavIdols,
      loadJavSeries,
      loadJavStudios,
      loadJavs,
    ]
  )

  const setWaterfallMode = useCallback(
    (key, enabled) => {
      setWaterfallModes((current) => ({ ...current, [key]: enabled }))
      if (enabled || !hydrated || !configLoaded) return
      if (key === 'video') {
        loadVideos({ force: true })
      } else if (key === 'jav') {
        loadJavs({ force: true })
      } else if (key === 'idol') {
        loadJavIdols({ force: true })
      } else if (key === 'studio') {
        loadJavStudios({ force: true })
      } else if (key === 'series') {
        loadJavSeries({ force: true })
      }
    },
    [
      configLoaded,
      hydrated,
      loadJavIdols,
      loadJavSeries,
      loadJavStudios,
      loadJavs,
      loadVideos,
      setWaterfallModes,
    ]
  )
  return { waterfallModes, forceReloadVideos, forceReloadJavByTab, setWaterfallMode }
}
