import { useStore, videoSelectionKey } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { useState, useMemo, useEffect, useCallback } from 'react'
import { zh } from '@/utils/i18n'
import { confirmLargeMPVPlaylist } from '@/features/playback/model'
import { playVideoPlaylist, deleteVideoLocation, fetchVideos } from '@/features/video/api'
import { getErrorMessage } from '@/utils/errors'
import { removeTagFromVideos, addTagToVideos } from '@/features/tags/api'
import { addJavTagToJavs } from '@/features/jav/api'

export default function useVideoSelection({
  mpvEnabled,
  ensureMPVPlaylistAvailable,
  showCenterToast,
  showToast,
  playVideosWithMPV,
}) {
  const {
    selectedVideoIds,
    videos,
    selectedVideoMeta,
    loadVideos,
    tags,
    clearSelection,
    loadJavTags,
    randomMode,
    total,
    videoTempSort,
    sortOrder,
    selectedTags,
    searchTerm,
    videoHideJav,
  } = useStore(
    useShallow((state) => ({
      selectedVideoIds: state.selectedVideoIds,
      videos: state.videos,
      selectedVideoMeta: state.selectedVideoMeta,
      loadVideos: state.loadVideos,
      tags: state.tags,
      clearSelection: state.clearSelection,
      loadJavTags: state.loadJavTags,
      randomMode: state.randomMode,
      total: state.total,
      videoTempSort: state.videoTempSort,
      sortOrder: state.sortOrder,
      selectedTags: state.selectedTags,
      searchTerm: state.searchTerm,
      videoHideJav: state.videoHideJav,
    }))
  )
  const [selectionOpsOpen, setSelectionOpsOpen] = useState(false)

  const [selectionTagsOpen, setSelectionTagsOpen] = useState(false)

  const [selectionTagAction, setSelectionTagAction] = useState('add')

  const [selectionTagChoices, setSelectionTagChoices] = useState([])

  const [selectionJavTagsOpen, setSelectionJavTagsOpen] = useState(false)

  const [selectionJavTagChoices, setSelectionJavTagChoices] = useState([])

  const [selectionJavTagSaving, setSelectionJavTagSaving] = useState(false)

  const [selectionPlaying, setSelectionPlaying] = useState(false)

  const [selectionDeleting, setSelectionDeleting] = useState(false)

  const [videoBulkActionBusy, setVideoBulkActionBusy] = useState(false)

  const selectedCount = useMemo(() => selectedVideoIds.size, [selectedVideoIds])

  const selectedList = useMemo(() => {
    const keys = Array.from(selectedVideoIds)
    return keys.map((key) => {
      const v = videos.find((item) => videoSelectionKey(item) === String(key))
      const meta = selectedVideoMeta?.[key]
      const labelFromMeta = meta && typeof meta === 'object' ? meta.label : meta
      const videoId = Number(meta && typeof meta === 'object' ? meta.video_id : v?.id)
      const locationId = Number(
        meta && typeof meta === 'object' ? meta.location_id : v?.location_id
      )
      const javId = Number(
        meta && typeof meta === 'object' ? (meta.jav_id ?? v?.jav_id) : v?.jav_id
      )
      const javCode = String(
        (meta && typeof meta === 'object' ? meta.jav_code : '') ||
          v?.jav?.code ||
          v?.locations?.[0]?.jav?.code ||
          ''
      ).trim()
      return {
        id: key,
        label: labelFromMeta || v?.filename || v?.path || `#${key}`,
        video: v,
        video_id: Number.isFinite(videoId) && videoId > 0 ? videoId : null,
        location_id: Number.isFinite(locationId) && locationId > 0 ? locationId : null,
        jav_id: Number.isFinite(javId) && javId > 0 ? javId : null,
        jav_code: javCode,
      }
    })
  }, [selectedVideoIds, videos, selectedVideoMeta])

  const selectedJavIds = useMemo(
    () =>
      Array.from(
        new Set(
          selectedList
            .map((item) => Number(item?.jav_id || item?.video?.jav_id))
            .filter((id) => Number.isFinite(id) && id > 0)
        )
      ),
    [selectedList]
  )

  const selectedJavVideoCount = useMemo(
    () => selectedList.filter((item) => Number(item?.jav_id) > 0).length,
    [selectedList]
  )

  useEffect(() => {
    if (selectedCount !== 0) return
    setSelectionOpsOpen(false)
    setSelectionTagsOpen(false)
    setSelectionJavTagsOpen(false)
    setSelectionTagAction('add')
    setSelectionTagChoices([])
    setSelectionJavTagChoices([])
    setSelectionJavTagSaving(false)
  }, [selectedCount])

  const handleRemoveSelectedVideo = useCallback((key) => {
    if (!key) return
    useStore.setState((state) => {
      const normalizedKey = String(key)
      const nextIds = new Set(state.selectedVideoIds || [])
      const nextMeta = { ...(state.selectedVideoMeta || {}) }
      nextIds.delete(normalizedKey)
      delete nextMeta[normalizedKey]
      return {
        selectedVideoIds: nextIds,
        selectedVideoMeta: nextMeta,
      }
    })
  }, [])

  const handlePlaySelection = useCallback(async () => {
    if (selectionPlaying || !mpvEnabled) return
    if (!ensureMPVPlaylistAvailable()) return
    const targets = selectedList
      .map((item) => {
        const videoId = Number(item?.video_id || item?.video?.id)
        const locationId = Number(item?.location_id || item?.video?.location_id || 0)
        if (!Number.isFinite(videoId) || videoId <= 0) return null
        return {
          video_id: videoId,
          location_id: Number.isFinite(locationId) && locationId > 0 ? locationId : 0,
          title: item?.label || item?.video?.filename || `Video #${videoId}`,
        }
      })
      .filter(Boolean)
    if (targets.length !== selectedList.length || targets.length === 0) {
      showCenterToast(
        zh(
          '无法播放：部分所选视频缺少文件信息',
          'Cannot play: some selected videos are missing file information'
        )
      )
      return
    }
    if (!confirmLargeMPVPlaylist(targets.length)) return

    setSelectionPlaying(true)
    try {
      const result = await playVideoPlaylist(targets)
      const count = Number(result?.count) || targets.length
      setSelectionOpsOpen(false)
      showToast(
        zh(`已将 ${count} 个视频加入 MPV 播放列表`, `Added ${count} videos to the MPV playlist`)
      )
    } catch (err) {
      console.error(zh('加入 MPV 播放列表失败', 'Failed to add to MPV playlist'), err)
      showCenterToast(getErrorMessage(err))
    } finally {
      setSelectionPlaying(false)
    }
  }, [
    ensureMPVPlaylistAvailable,
    mpvEnabled,
    selectedList,
    selectionPlaying,
    showCenterToast,
    showToast,
  ])

  const handleDeleteSelection = useCallback(async () => {
    if (selectionDeleting) return
    const targets = selectedList
      .map((item) => {
        const videoId = Number(item?.video_id || item?.video?.id)
        const locationId = Number(item?.location_id || item?.video?.location_id)
        if (
          !Number.isFinite(videoId) ||
          videoId <= 0 ||
          !Number.isFinite(locationId) ||
          locationId <= 0
        ) {
          return null
        }
        return {
          key: item.id,
          label: item.label || `#${videoId}`,
          videoId,
          locationId,
        }
      })
      .filter(Boolean)
    const skipped = Math.max(0, selectedList.length - targets.length)
    if (targets.length === 0) {
      showCenterToast(
        zh('无法删除：所选视频缺少文件位置', 'Cannot delete: selected videos have no file location')
      )
      return
    }
    const confirmMessage =
      skipped > 0
        ? zh(
            `确定删除 ${targets.length} 个可删除视频文件吗？${skipped} 项缺少文件位置，将跳过。`,
            `Delete ${targets.length} deletable video files? ${skipped} selected items have no file location and will be skipped.`
          )
        : zh(
            `确定删除所选 ${targets.length} 个视频文件吗？`,
            `Delete the selected ${targets.length} video files?`
          )
    if (!window.confirm(confirmMessage)) return

    setSelectionDeleting(true)
    const deletedKeys = []
    const failed = []
    try {
      for (const target of targets) {
        try {
          await deleteVideoLocation(target.videoId, target.locationId)
          deletedKeys.push(target.key)
        } catch (err) {
          failed.push({ target, err })
        }
      }

      if (deletedKeys.length > 0) {
        const deletedSet = new Set(deletedKeys)
        useStore.setState((state) => {
          const nextIds = new Set(state.selectedVideoIds || [])
          const nextMeta = { ...(state.selectedVideoMeta || {}) }
          deletedSet.forEach((key) => {
            nextIds.delete(key)
            delete nextMeta[key]
          })
          const nextVideos = Array.isArray(state.videos)
            ? state.videos.filter((item) => !deletedSet.has(videoSelectionKey(item)))
            : state.videos
          return {
            videos: nextVideos,
            selectedVideoIds: nextIds,
            selectedVideoMeta: nextMeta,
            total: Math.max(0, Number(state.total || 0) - deletedKeys.length),
          }
        })
        await loadVideos({ force: true })
      }

      if (failed.length > 0) {
        console.error('batch delete videos failed', failed)
        showCenterToast(getErrorMessage(failed[0]?.err))
      } else if (skipped > 0) {
        showCenterToast(
          zh(
            `已删除 ${deletedKeys.length} 个视频，跳过 ${skipped} 项`,
            `Deleted ${deletedKeys.length} videos, skipped ${skipped} items`
          )
        )
      } else {
        setSelectionOpsOpen(false)
        showToast(zh(`已删除 ${deletedKeys.length} 个视频`, `Deleted ${deletedKeys.length} videos`))
      }
    } finally {
      setSelectionDeleting(false)
    }
  }, [loadVideos, selectedList, selectionDeleting, showCenterToast, showToast])

  const handleSelectionTagsClose = () => {
    setSelectionTagsOpen(false)
    setSelectionTagAction('add')
    setSelectionTagChoices([])
  }

  const handleSelectionTagChoiceToggle = (tagId, checked) => {
    setSelectionTagChoices((prev) => {
      const set = new Set(prev)
      if (checked) set.add(String(tagId))
      else set.delete(String(tagId))
      return Array.from(set)
    })
  }

  const handleApplySelectionTags = async () => {
    const ids = selectionTagChoices.map((t) => Number(t)).filter(Boolean)
    const selectedKeys = Array.from(selectedVideoIds)
    const vidIds = Array.from(
      new Set(
        selectedKeys
          .map((key) => {
            const meta = selectedVideoMeta?.[key]
            const raw = meta && typeof meta === 'object' ? meta.video_id : key
            const parsed = Number(raw)
            return Number.isFinite(parsed) && parsed > 0 ? parsed : null
          })
          .filter(Boolean)
      )
    )
    try {
      if (selectionTagAction === 'remove') {
        await Promise.all(ids.map((tid) => removeTagFromVideos(tid, vidIds)))
        const removedIds = new Set(ids)
        useStore.setState(({ videos }) => {
          const next = videos.map((v) => {
            if (!vidIds.includes(v.id)) return v
            const existing = Array.isArray(v.tags) ? v.tags : []
            const nextTags = existing.filter((tag) => !removedIds.has(tag.id))
            return nextTags.length === existing.length ? v : { ...v, tags: nextTags }
          })
          return { videos: next }
        })
      } else {
        await Promise.all(ids.map((tid) => addTagToVideos(tid, vidIds)))
        const addedTags = tags.filter((t) => ids.includes(t.id))
        useStore.setState(({ videos }) => {
          const next = videos.map((v) => {
            if (!vidIds.includes(v.id)) return v
            const existing = Array.isArray(v.tags) ? v.tags : []
            const mergedById = new Map()
            for (const tag of existing) mergedById.set(tag.id, tag)
            for (const tag of addedTags) mergedById.set(tag.id, tag)
            return { ...v, tags: Array.from(mergedById.values()) }
          })
          return { videos: next }
        })
      }
    } catch (err) {
      console.error(`${selectionTagAction} tags for selection failed`, err)
      showCenterToast(getErrorMessage(err))
    } finally {
      setSelectionTagsOpen(false)
      setSelectionTagAction('add')
      setSelectionTagChoices([])
      setSelectionOpsOpen(false)
      clearSelection()
    }
  }

  const handleSelectionJavTagsClose = () => {
    if (selectionJavTagSaving) return
    setSelectionJavTagsOpen(false)
    setSelectionJavTagChoices([])
  }

  const handleSelectionJavTagChoiceToggle = (tagId, checked) => {
    setSelectionJavTagChoices((current) => {
      const next = new Set(current)
      if (checked) next.add(String(tagId))
      else next.delete(String(tagId))
      return Array.from(next)
    })
  }

  const handleApplySelectionJavTags = async () => {
    const tagIds = selectionJavTagChoices
      .map((tagId) => Number(tagId))
      .filter((tagId) => Number.isFinite(tagId) && tagId > 0)
    if (tagIds.length === 0 || selectedJavIds.length === 0) return

    setSelectionJavTagSaving(true)
    try {
      await Promise.all(tagIds.map((tagId) => addJavTagToJavs(tagId, selectedJavIds)))
      await loadJavTags({ force: true })
      const skipped = Math.max(0, selectedCount - selectedJavVideoCount)
      showToast(
        skipped > 0
          ? zh(
              `已给 ${selectedJavIds.length} 个 JAV 添加标签，跳过 ${skipped} 个未关联 JAV 的视频`,
              `Added tags to ${selectedJavIds.length} JAV items; skipped ${skipped} videos without JAV links`
            )
          : zh(
              `已给 ${selectedJavIds.length} 个 JAV 添加标签`,
              `Added tags to ${selectedJavIds.length} JAV items`
            )
      )
      setSelectionJavTagsOpen(false)
      setSelectionJavTagChoices([])
      setSelectionOpsOpen(false)
      clearSelection()
    } catch (err) {
      console.error('add JAV tags for selection failed', err)
      showCenterToast(getErrorMessage(err))
    } finally {
      setSelectionJavTagSaving(false)
    }
  }

  const addVideosToSelection = useCallback((items) => {
    const entries = (Array.isArray(items) ? items : [])
      .map((video) => {
        const key = videoSelectionKey(video)
        const videoId = Number(video?.id)
        if (!key || !Number.isFinite(videoId) || videoId <= 0) return null
        return {
          key,
          meta: {
            label: video.filename || video.path || `#${videoId}`,
            video_id: videoId,
            location_id: video.location_id || null,
            jav_id: video.jav_id || null,
            jav_code: video.jav?.code || video.locations?.[0]?.jav?.code || '',
          },
        }
      })
      .filter(Boolean)

    if (entries.length === 0) return 0
    useStore.setState((state) => {
      const nextIds = new Set(state.selectedVideoIds)
      const nextMeta = { ...state.selectedVideoMeta }
      entries.forEach(({ key, meta }) => {
        nextIds.add(key)
        nextMeta[key] = meta
      })
      return { selectedVideoIds: nextIds, selectedVideoMeta: nextMeta }
    })
    return entries.length
  }, [])

  const fetchAllMatchingVideos = useCallback(async () => {
    if (randomMode) {
      return Array.isArray(videos) ? videos : []
    }

    const batchSize = 500
    let expectedTotal = Math.max(0, Number(total) || 0)
    let offset = 0
    const items = []
    const effectiveSort = videoTempSort || sortOrder

    while (offset < expectedTotal) {
      const limit = Math.min(batchSize, expectedTotal - offset)
      const response = await fetchVideos({
        limit,
        offset,
        tags: selectedTags,
        search: searchTerm || '',
        sort: effectiveSort,
        hideJav: videoHideJav,
      })
      const batch = Array.isArray(response?.items) ? response.items : []
      items.push(...batch)
      const responseTotal = Number(response?.total)
      if (Number.isFinite(responseTotal) && responseTotal >= 0) {
        expectedTotal = responseTotal
      }
      if (batch.length === 0) break
      offset += limit
    }

    return items
  }, [randomMode, searchTerm, selectedTags, sortOrder, total, videoHideJav, videoTempSort, videos])

  const handleSelectVideoPage = useCallback(() => {
    const count = addVideosToSelection(videos)
    if (count > 0) {
      showToast(zh(`已选择本页 ${count} 个视频`, `Selected ${count} videos on this page`))
    }
  }, [addVideosToSelection, showToast, videos])

  const handleSelectAllVideos = useCallback(async () => {
    if (videoBulkActionBusy) return
    setVideoBulkActionBusy(true)
    try {
      const items = await fetchAllMatchingVideos()
      const count = addVideosToSelection(items)
      if (count > 0) {
        showToast(zh(`已选择全部 ${count} 个视频`, `Selected all ${count} videos`))
      }
    } catch (err) {
      showCenterToast(getErrorMessage(err))
    } finally {
      setVideoBulkActionBusy(false)
    }
  }, [
    addVideosToSelection,
    fetchAllMatchingVideos,
    showCenterToast,
    showToast,
    videoBulkActionBusy,
  ])

  const handlePlayVideoPage = useCallback(async () => {
    if (selectionPlaying || videoBulkActionBusy || !mpvEnabled) return
    setSelectionPlaying(true)
    try {
      await playVideosWithMPV(videos)
    } catch (err) {
      showCenterToast(getErrorMessage(err))
    } finally {
      setSelectionPlaying(false)
    }
  }, [
    mpvEnabled,
    playVideosWithMPV,
    selectionPlaying,
    showCenterToast,
    videoBulkActionBusy,
    videos,
  ])

  const handlePlayAllVideos = useCallback(async () => {
    if (selectionPlaying || videoBulkActionBusy || !mpvEnabled) return
    if (!ensureMPVPlaylistAvailable()) return
    setSelectionPlaying(true)
    setVideoBulkActionBusy(true)
    try {
      const items = await fetchAllMatchingVideos()
      await playVideosWithMPV(items)
    } catch (err) {
      showCenterToast(getErrorMessage(err))
    } finally {
      setVideoBulkActionBusy(false)
      setSelectionPlaying(false)
    }
  }, [
    ensureMPVPlaylistAvailable,
    fetchAllMatchingVideos,
    mpvEnabled,
    playVideosWithMPV,
    selectionPlaying,
    showCenterToast,
    videoBulkActionBusy,
  ])
  return {
    selectionOpsOpen,
    setSelectionOpsOpen,
    selectionTagsOpen,
    setSelectionTagsOpen,
    selectionTagAction,
    setSelectionTagAction,
    selectionTagChoices,
    setSelectionTagChoices,
    selectionJavTagsOpen,
    setSelectionJavTagsOpen,
    selectionJavTagChoices,
    setSelectionJavTagChoices,
    selectionJavTagSaving,
    selectionPlaying,
    selectionDeleting,
    videoBulkActionBusy,
    selectedCount,
    selectedList,
    selectedJavIds,
    handleRemoveSelectedVideo,
    handlePlaySelection,
    handleDeleteSelection,
    handleSelectionTagsClose,
    handleSelectionTagChoiceToggle,
    handleApplySelectionTags,
    handleSelectionJavTagsClose,
    handleSelectionJavTagChoiceToggle,
    handleApplySelectionJavTags,
    handleSelectVideoPage,
    handleSelectAllVideos,
    handlePlayVideoPage,
    handlePlayAllVideos,
  }
}
