import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { useState, useMemo, useEffect } from 'react'

export default function useApplicationData() {
  const { directories, loadConfig, loadDirectories, loadTags, loadJavTags, videoHideJav } =
    useStore(
      useShallow((state) => ({
        directories: state.directories,
        loadConfig: state.loadConfig,
        loadDirectories: state.loadDirectories,
        loadTags: state.loadTags,
        loadJavTags: state.loadJavTags,
        videoHideJav: state.videoHideJav,
      }))
    )
  const [configLoaded, setConfigLoaded] = useState(false)

  const directoryStateKey = useMemo(
    () =>
      directories
        .filter((directory) => !directory?.is_delete)
        .map((directory) => `${directory.id}:${directory.enabled !== false ? '1' : '0'}`)
        .join(','),
    [directories]
  )

  useEffect(() => {
    let mounted = true
    loadConfig().finally(() => {
      if (mounted) setConfigLoaded(true)
    })
    return () => {
      mounted = false
    }
  }, [loadConfig])

  useEffect(() => {
    loadDirectories()
  }, [loadDirectories])

  useEffect(() => {
    loadTags({ skipUnchanged: true })
    loadJavTags({ skipUnchanged: true })
  }, [loadTags, loadJavTags, directoryStateKey, videoHideJav])
  return { configLoaded, directoryStateKey }
}
