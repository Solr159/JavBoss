import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { useState, useCallback } from 'react'
import {
  fetchJavFavoriteSelection,
  createJavFavoriteGroup,
  replaceJavFavoriteGroups,
  reorderJavFavoriteGroups,
  renameJavFavoriteGroup,
  deleteJavFavoriteGroup,
  fetchJavFavoriteGroupItems,
  reorderJavFavoriteGroupItems,
  removeJavFavoriteGroupItems,
} from '@/features/favorites/api'
import { getErrorMessage } from '@/utils/errors'
import { createDefaultIdolProfileFilters } from '@/constants/jav'

export default function useFavoriteGroups({
  isJavMode,
  saveScrollBeforeUrlStateChange,
  setJavSearchInput,
}) {
  const {
    loadJavFavoriteGroups,
    loadJavs,
    loadJavIdols,
    loadJavStudios,
    loadJavSeries,
    javTab,
    javFavoriteGroupId,
    studioFavoriteGroupId,
    seriesFavoriteGroupId,
    idolFavoriteGroupId,
    setJavFavoriteGroupId,
    setStudioFavoriteGroupId,
    setSeriesFavoriteGroupId,
    setIdolFavoriteGroupId,
    favoriteGroupsByType,
    favoriteGroupsLoadingByType,
    favoriteGroupsErrorByType,
  } = useStore(
    useShallow((state) => ({
      loadJavFavoriteGroups: state.loadJavFavoriteGroups,
      loadJavs: state.loadJavs,
      loadJavIdols: state.loadJavIdols,
      loadJavStudios: state.loadJavStudios,
      loadJavSeries: state.loadJavSeries,
      javTab: state.javTab,
      javFavoriteGroupId: state.javFavoriteGroupId,
      studioFavoriteGroupId: state.studioFavoriteGroupId,
      seriesFavoriteGroupId: state.seriesFavoriteGroupId,
      idolFavoriteGroupId: state.idolFavoriteGroupId,
      setJavFavoriteGroupId: state.setJavFavoriteGroupId,
      setStudioFavoriteGroupId: state.setStudioFavoriteGroupId,
      setSeriesFavoriteGroupId: state.setSeriesFavoriteGroupId,
      setIdolFavoriteGroupId: state.setIdolFavoriteGroupId,
      favoriteGroupsByType: state.favoriteGroupsByType,
      favoriteGroupsLoadingByType: state.favoriteGroupsLoadingByType,
      favoriteGroupsErrorByType: state.favoriteGroupsErrorByType,
    }))
  )
  const [idolFavoriteModalOpen, setIdolFavoriteModalOpen] = useState(false)

  const [idolFavoriteModalItem, setIdolFavoriteModalItem] = useState(null)

  const [favoriteModalEntityType, setFavoriteModalEntityType] = useState('idol')

  const [idolFavoriteSelectedIds, setIdolFavoriteSelectedIds] = useState([])

  const [idolFavoriteModalLoading, setIdolFavoriteModalLoading] = useState(false)

  const [idolFavoriteModalSaving, setIdolFavoriteModalSaving] = useState(false)

  const [idolFavoriteModalError, setIdolFavoriteModalError] = useState('')

  const [idolFavoriteManageOpen, setIdolFavoriteManageOpen] = useState(false)

  const [idolFavoriteManageEditGroupId, setIdolFavoriteManageEditGroupId] = useState(null)

  const [favoriteManageEntityType, setFavoriteManageEntityType] = useState('idol')

  const handleOpenFavoriteModal = useCallback(
    async (entityType, item) => {
      const type = ['jav', 'idol', 'studio', 'series'].includes(entityType) ? entityType : 'idol'
      const id = Number(item?.id)
      if (!Number.isFinite(id) || id <= 0) return
      setFavoriteModalEntityType(type)
      setIdolFavoriteModalItem(item)
      setIdolFavoriteSelectedIds([])
      setIdolFavoriteModalError('')
      setIdolFavoriteModalOpen(true)
      setIdolFavoriteModalLoading(true)
      try {
        const [selectedIds] = await Promise.all([
          fetchJavFavoriteSelection(type, id),
          loadJavFavoriteGroups(type, { force: true }),
        ])
        setIdolFavoriteSelectedIds(
          (selectedIds || []).map((value) => Number(value)).filter((value) => value > 0)
        )
      } catch (err) {
        setIdolFavoriteModalError(getErrorMessage(err))
      } finally {
        setIdolFavoriteModalLoading(false)
      }
    },
    [loadJavFavoriteGroups]
  )

  const handleOpenIdolFavoriteModal = useCallback(
    (idol) => handleOpenFavoriteModal('idol', idol),
    [handleOpenFavoriteModal]
  )

  const handleCloseIdolFavoriteModal = useCallback(() => {
    if (idolFavoriteModalSaving) return
    setIdolFavoriteModalOpen(false)
    setIdolFavoriteModalItem(null)
    setFavoriteModalEntityType('idol')
    setIdolFavoriteSelectedIds([])
    setIdolFavoriteModalError('')
    setIdolFavoriteModalLoading(false)
  }, [idolFavoriteModalSaving])

  const reloadFavoriteData = useCallback(
    async (entityType) => {
      const type = ['jav', 'idol', 'studio', 'series'].includes(entityType) ? entityType : 'idol'
      useStore.getState().invalidateJavFavoriteCounts(type)
      const tabByType = { jav: 'list', idol: 'idol', studio: 'studio', series: 'series' }
      const reloadByType = {
        jav: loadJavs,
        idol: loadJavIdols,
        studio: loadJavStudios,
        series: loadJavSeries,
      }
      const shouldReloadCurrentList = isJavMode && javTab === tabByType[type]
      const reloadCurrentList = shouldReloadCurrentList ? reloadByType[type] : null
      await Promise.all([
        loadJavFavoriteGroups(type, { force: true }),
        reloadCurrentList ? reloadCurrentList({ force: true }) : Promise.resolve(),
      ])
    },
    [
      isJavMode,
      javTab,
      loadJavFavoriteGroups,
      loadJavIdols,
      loadJavSeries,
      loadJavStudios,
      loadJavs,
    ]
  )

  const activeFavoriteGroupId = useCallback(
    (entityType) => {
      switch (entityType) {
        case 'jav':
          return javFavoriteGroupId
        case 'studio':
          return studioFavoriteGroupId
        case 'series':
          return seriesFavoriteGroupId
        case 'idol':
        default:
          return idolFavoriteGroupId
      }
    },
    [idolFavoriteGroupId, javFavoriteGroupId, seriesFavoriteGroupId, studioFavoriteGroupId]
  )

  const setActiveFavoriteGroupId = useCallback(
    (entityType, groupId) => {
      switch (entityType) {
        case 'jav':
          setJavFavoriteGroupId(groupId)
          break
        case 'studio':
          setStudioFavoriteGroupId(groupId)
          break
        case 'series':
          setSeriesFavoriteGroupId(groupId)
          break
        case 'idol':
        default:
          setIdolFavoriteGroupId(groupId)
          break
      }
    },
    [
      setIdolFavoriteGroupId,
      setJavFavoriteGroupId,
      setSeriesFavoriteGroupId,
      setStudioFavoriteGroupId,
    ]
  )

  const handleFavoriteGroupSelect = useCallback(
    (entityType, groupId) => {
      const type = ['jav', 'idol', 'studio', 'series'].includes(entityType) ? entityType : 'idol'
      const parsedGroupId = Number(groupId)
      const nextGroupId = Number.isFinite(parsedGroupId) && parsedGroupId > 0 ? parsedGroupId : null
      const parsedCurrentGroupId = Number(activeFavoriteGroupId(type))
      const currentGroupId =
        Number.isFinite(parsedCurrentGroupId) && parsedCurrentGroupId > 0
          ? parsedCurrentGroupId
          : null
      if (currentGroupId === nextGroupId) return

      saveScrollBeforeUrlStateChange()
      setJavSearchInput('')
      useStore.setState({
        javSearchTerm: '',
        javIdolIds: [],
        javTags: [],
        javStudioId: null,
        javStudioName: '',
        javSeriesId: null,
        javSeriesName: '',
        javPrefix: '',
        javSoloOnly: false,
        javFavoriteRatingEnabled: false,
        javFavoriteRatingMin: 0.5,
        javFavoriteRatingMax: 5,
        idolProfileFilters: createDefaultIdolProfileFilters(),
        javRandomMode: false,
        javRandomSeed: null,
      })
      setActiveFavoriteGroupId(type, nextGroupId)
    },
    [
      activeFavoriteGroupId,
      saveScrollBeforeUrlStateChange,
      setActiveFavoriteGroupId,
      setJavSearchInput,
    ]
  )

  const patchFavoriteCountInCurrentList = useCallback(
    (entityType, entityID, groupIds) => {
      const type = ['jav', 'idol', 'studio', 'series'].includes(entityType) ? entityType : 'idol'
      useStore
        .getState()
        .patchJavFavoriteCount(type, entityID, groupIds, activeFavoriteGroupId(type))
    },
    [activeFavoriteGroupId]
  )

  const handleCreateFavoriteGroup = useCallback(
    async (name, entityType = favoriteModalEntityType) => {
      const type = ['jav', 'idol', 'studio', 'series'].includes(entityType) ? entityType : 'idol'
      const group = await createJavFavoriteGroup(type, name)
      useStore.setState((state) => {
        const current = Array.isArray(state.favoriteGroupsByType?.[type])
          ? state.favoriteGroupsByType[type]
          : []
        const exists = current.some((item) => Number(item?.id) === Number(group?.id))
        const next = exists ? current : [...current, { ...group, count: group?.count || 0 }]
        next.sort((a, b) => {
          const orderA = Number(a?.sort_order) || 0
          const orderB = Number(b?.sort_order) || 0
          if (orderA !== orderB) return orderA - orderB
          return String(a?.name || '').localeCompare(String(b?.name || ''))
        })
        return {
          favoriteGroupsByType: { ...(state.favoriteGroupsByType || {}), [type]: next },
        }
      })
      return group
    },
    [favoriteModalEntityType]
  )

  const handleSaveFavoriteGroups = useCallback(
    async (groupIds) => {
      const entityID = Number(idolFavoriteModalItem?.id)
      const type = favoriteModalEntityType
      if (!Number.isFinite(entityID) || entityID <= 0) return
      setIdolFavoriteModalSaving(true)
      setIdolFavoriteModalError('')
      try {
        await replaceJavFavoriteGroups(type, entityID, groupIds)
        patchFavoriteCountInCurrentList(type, entityID, groupIds)
        setIdolFavoriteModalOpen(false)
        setIdolFavoriteModalItem(null)
        setFavoriteModalEntityType('idol')
        setIdolFavoriteSelectedIds([])
        await loadJavFavoriteGroups(type, { force: true })
      } catch (err) {
        setIdolFavoriteModalError(getErrorMessage(err))
      } finally {
        setIdolFavoriteModalSaving(false)
      }
    },
    [
      favoriteModalEntityType,
      idolFavoriteModalItem,
      loadJavFavoriteGroups,
      patchFavoriteCountInCurrentList,
    ]
  )

  const handleSaveIdolFavoriteGroups = handleSaveFavoriteGroups

  const handleReorderIdolFavoriteGroups = useCallback(
    async (groupIds) => {
      const type = favoriteManageEntityType || 'idol'
      await reorderJavFavoriteGroups(type, groupIds)
      await loadJavFavoriteGroups(type, { force: true })
    },
    [favoriteManageEntityType, loadJavFavoriteGroups]
  )

  const handleRenameIdolFavoriteGroup = useCallback(
    async (groupId, name) => {
      const type = favoriteManageEntityType || 'idol'
      await renameJavFavoriteGroup(type, groupId, name)
      useStore.setState((state) => ({
        favoriteGroupsByType: {
          ...(state.favoriteGroupsByType || {}),
          [type]: (state.favoriteGroupsByType?.[type] || []).map((group) =>
            Number(group.id) === Number(groupId) ? { ...group, name } : group
          ),
        },
      }))
      await loadJavFavoriteGroups(type, { force: true })
    },
    [favoriteManageEntityType, loadJavFavoriteGroups]
  )

  const handleDeleteIdolFavoriteGroup = useCallback(
    async (groupId) => {
      const type = favoriteManageEntityType || 'idol'
      await deleteJavFavoriteGroup(type, groupId)
      if (Number(activeFavoriteGroupId(type)) === Number(groupId)) {
        setActiveFavoriteGroupId(type, null)
      }
      await reloadFavoriteData(type)
    },
    [favoriteManageEntityType, activeFavoriteGroupId, reloadFavoriteData, setActiveFavoriteGroupId]
  )

  const handleLoadIdolFavoriteGroupIdols = useCallback(
    (groupId) => {
      const type = favoriteManageEntityType || 'idol'
      return fetchJavFavoriteGroupItems(type, groupId)
    },
    [favoriteManageEntityType]
  )

  const handleReorderIdolFavoriteGroupIdols = useCallback(
    async (groupId, idolIds) => {
      const type = favoriteManageEntityType || 'idol'
      await reorderJavFavoriteGroupItems(type, groupId, idolIds)
      if (Number(activeFavoriteGroupId(type)) === Number(groupId)) await reloadFavoriteData(type)
    },
    [favoriteManageEntityType, activeFavoriteGroupId, reloadFavoriteData]
  )

  const handleRemoveIdolFavoriteGroupIdols = useCallback(
    async (groupId, idolIds) => {
      const type = favoriteManageEntityType || 'idol'
      await removeJavFavoriteGroupItems(type, groupId, idolIds)
      await reloadFavoriteData(type)
    },
    [favoriteManageEntityType, reloadFavoriteData]
  )

  const activeFavoriteEntityType =
    javTab === 'studio'
      ? 'studio'
      : javTab === 'series'
        ? 'series'
        : javTab === 'idol'
          ? 'idol'
          : 'jav'

  const activeFavoriteGroups = favoriteGroupsByType?.[activeFavoriteEntityType] || []

  const activeFavoriteGroupsLoading = Boolean(
    favoriteGroupsLoadingByType?.[activeFavoriteEntityType]
  )

  const activeFavoriteGroupsError = favoriteGroupsErrorByType?.[activeFavoriteEntityType] || null

  const activeSelectedFavoriteGroupId = activeFavoriteGroupId(activeFavoriteEntityType)
  return {
    idolFavoriteModalOpen,
    idolFavoriteModalItem,
    favoriteModalEntityType,
    idolFavoriteSelectedIds,
    idolFavoriteModalLoading,
    idolFavoriteModalSaving,
    idolFavoriteModalError,
    idolFavoriteManageOpen,
    setIdolFavoriteManageOpen,
    idolFavoriteManageEditGroupId,
    setIdolFavoriteManageEditGroupId,
    favoriteManageEntityType,
    setFavoriteManageEntityType,
    handleOpenFavoriteModal,
    handleOpenIdolFavoriteModal,
    handleCloseIdolFavoriteModal,
    activeFavoriteGroupId,
    handleFavoriteGroupSelect,
    handleCreateFavoriteGroup,
    handleSaveIdolFavoriteGroups,
    handleReorderIdolFavoriteGroups,
    handleRenameIdolFavoriteGroup,
    handleDeleteIdolFavoriteGroup,
    handleLoadIdolFavoriteGroupIdols,
    handleReorderIdolFavoriteGroupIdols,
    handleRemoveIdolFavoriteGroupIdols,
    activeFavoriteEntityType,
    activeFavoriteGroups,
    activeFavoriteGroupsLoading,
    activeFavoriteGroupsError,
    activeSelectedFavoriteGroupId,
  }
}
