import { useStore, videoSelectionKey } from '@/store'
import { useState, useCallback } from 'react'
import { zh } from '@/utils/i18n'
import { renameVideoLocation, deleteVideoLocation } from '@/features/video/api'
import { getErrorMessage } from '@/utils/errors'
import {
  updateVideoJavScrapeSettings,
  fetchVideoJavScrapePossibleCodes,
  manualVideoJavScrape,
  linkVideoToExistingJav,
} from '@/features/jav/api'
import { JAV_SCRAPE_OVERRIDE_SKIP, applyScrapeOverrideToVideo } from '@/features/video/scrapeModel'

export default function useVideoOperations({ showCenterToast, showToast }) {
  const [scrapeSettingsVideo, setScrapeSettingsVideo] = useState(null)

  const [scrapeSettingsSaving, setScrapeSettingsSaving] = useState(false)

  const handleRenameVideo = useCallback(
    async (video) => {
      const locationId = Number(video?.location_id)
      if (!video?.id || !Number.isFinite(locationId) || locationId <= 0) {
        showCenterToast(zh('无法重命名：缺少文件位置', 'Cannot rename: missing file location'))
        return
      }
      const currentName =
        String(video?.filename || '')
          .trim()
          .split(/[\\/]/)
          .pop() ||
        String(video?.path || '')
          .split(/[\\/]/)
          .pop()
      const nextName = window.prompt(zh('重命名视频文件', 'Rename video file'), currentName)
      if (nextName == null) return
      const filename = nextName.trim()
      if (!filename || filename === currentName) return
      try {
        const updated = await renameVideoLocation(video.id, locationId, filename)
        useStore.setState((state) => {
          const targetKey = videoSelectionKey(video)
          const nextVideos = Array.isArray(state.videos)
            ? state.videos.map((item) =>
                videoSelectionKey(item) === targetKey ? { ...item, ...updated } : item
              )
            : state.videos
          const nextMeta = { ...(state.selectedVideoMeta || {}) }
          if (targetKey && nextMeta[targetKey]) {
            nextMeta[targetKey] = {
              ...nextMeta[targetKey],
              label: updated.filename || updated.path || nextMeta[targetKey].label,
            }
          }
          return { videos: nextVideos, selectedVideoMeta: nextMeta }
        })
      } catch (err) {
        console.error(zh('重命名视频失败', 'Failed to rename video'), err)
        showCenterToast(getErrorMessage(err))
      }
    },
    [showCenterToast]
  )

  const handleDeleteVideo = useCallback(
    async (video) => {
      const locationId = Number(video?.location_id)
      if (!video?.id || !Number.isFinite(locationId) || locationId <= 0) {
        showCenterToast(zh('无法删除：缺少文件位置', 'Cannot delete: missing file location'))
        return
      }
      const label = String(video?.filename || video?.path || `#${video.id}`)
      if (!window.confirm(zh(`确定删除视频文件“${label}”吗？`, `Delete video file "${label}"?`))) {
        return
      }
      try {
        await deleteVideoLocation(video.id, locationId)
        useStore.getState().removeVideoLocations([videoSelectionKey(video)])
      } catch (err) {
        console.error(zh('删除视频失败', 'Failed to delete video'), err)
        showCenterToast(getErrorMessage(err))
      }
    },
    [showCenterToast]
  )

  const handleOpenScrapeSettings = useCallback((video) => {
    setScrapeSettingsVideo(video)
  }, [])

  const handleSaveScrapeSettings = useCallback(
    async ({ mode, code }) => {
      const video = scrapeSettingsVideo
      if (!video?.id) return
      setScrapeSettingsSaving(true)
      try {
        const updated = await updateVideoJavScrapeSettings(video.id, { mode, code })
        let override = ''
        if (typeof updated?.jav_scrape_override === 'string') {
          override = updated.jav_scrape_override
        } else if (mode === 'skip') {
          override = JAV_SCRAPE_OVERRIDE_SKIP
        } else if (mode === 'code') {
          override = String(code || '')
            .trim()
            .toUpperCase()
        }
        useStore.setState((state) => ({
          videos: Array.isArray(state.videos)
            ? state.videos.map((item) =>
                item?.id === video.id ? applyScrapeOverrideToVideo(item, override) : item
              )
            : state.videos,
        }))
        setScrapeSettingsVideo(null)
        showToast(zh('刮削设置已保存', 'Scrape settings saved'))
      } catch (err) {
        console.error(zh('保存刮削设置失败', 'Failed to save scrape settings'), err)
        showCenterToast(getErrorMessage(err))
      } finally {
        setScrapeSettingsSaving(false)
      }
    },
    [scrapeSettingsVideo, showCenterToast, showToast]
  )

  const handleFetchScrapePossibleCodes = useCallback(async () => {
    const video = scrapeSettingsVideo
    if (!video?.id) throw new Error(zh('缺少视频 ID', 'Missing video ID'))
    return fetchVideoJavScrapePossibleCodes(video.id)
  }, [scrapeSettingsVideo])

  const handleManualScrape = useCallback(
    async (info) => {
      const video = scrapeSettingsVideo
      if (!video?.id) return
      const locationId = Number(video?.location_id || video?.locations?.[0]?.id || 0)
      if (!Number.isFinite(locationId) || locationId <= 0) {
        showCenterToast(zh('缺少视频位置 ID', 'Missing video location ID'))
        return
      }
      setScrapeSettingsSaving(true)
      try {
        const updated = await manualVideoJavScrape(video.id, locationId, info)
        const override = String(updated?.jav_scrape_override || info?.code || '')
          .trim()
          .toUpperCase()
        const targetKey = videoSelectionKey(video)
        useStore.setState((state) => ({
          videos: Array.isArray(state.videos)
            ? state.videos.map((item) =>
                videoSelectionKey(item) === targetKey && updated
                  ? { ...updated, jav_scrape_override: override }
                  : item
              )
            : state.videos,
        }))
        setScrapeSettingsVideo(null)
        showToast(zh('手动刮削已保存', 'Manual scrape saved'))
      } catch (err) {
        console.error(zh('手动刮削失败', 'Manual scrape failed'), err)
        showCenterToast(getErrorMessage(err))
      } finally {
        setScrapeSettingsSaving(false)
      }
    },
    [scrapeSettingsVideo, showCenterToast, showToast]
  )

  const handleLinkExistingJav = useCallback(
    async (code) => {
      const video = scrapeSettingsVideo
      if (!video?.id) return
      const locationId = Number(video?.location_id || video?.locations?.[0]?.id || 0)
      if (!Number.isFinite(locationId) || locationId <= 0) {
        throw new Error(zh('缺少视频位置 ID', 'Missing video location ID'))
      }
      setScrapeSettingsSaving(true)
      try {
        const updated = await linkVideoToExistingJav(video.id, locationId, code)
        const override = String(updated?.jav_scrape_override || `:manual:${code}`)
          .trim()
          .toUpperCase()
        const targetKey = videoSelectionKey(video)
        useStore.setState((state) => ({
          videos: Array.isArray(state.videos)
            ? state.videos.map((item) =>
                videoSelectionKey(item) === targetKey && updated
                  ? { ...updated, jav_scrape_override: override }
                  : item
              )
            : state.videos,
        }))
        setScrapeSettingsVideo(null)
        showToast(zh('已关联已有番号', 'Linked to existing JAV'))
      } finally {
        setScrapeSettingsSaving(false)
      }
    },
    [scrapeSettingsVideo, showToast]
  )
  return {
    scrapeSettingsVideo,
    setScrapeSettingsVideo,
    scrapeSettingsSaving,
    handleRenameVideo,
    handleDeleteVideo,
    handleOpenScrapeSettings,
    handleSaveScrapeSettings,
    handleFetchScrapePossibleCodes,
    handleManualScrape,
    handleLinkExistingJav,
  }
}
