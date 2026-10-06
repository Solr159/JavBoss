import { useStore } from '@/store'
import { configFlag } from '@/utils/config'
import { useMemo } from 'react'
import { withJavTagDisplayName } from '@/utils/javTag'

export default function useJavPresentation(items) {
  const portraitMode = useStore((state) => configFlag(state.config?.jav_portrait_mode))
  const preferChineseName = useStore((state) =>
    configFlag(state.config?.jav_idol_prefer_chinese_name)
  )
  const hideSeries = useStore((state) => configFlag(state.config?.jav_hide_series))
  const hideIdols = useStore((state) => configFlag(state.config?.jav_hide_idols))
  const hideTags = useStore((state) => configFlag(state.config?.jav_hide_tags))
  const hideActions = useStore((state) => configFlag(state.config?.jav_hide_actions))
  const showFullFavoriteRating = useStore((state) =>
    configFlag(state.config?.jav_favorite_rating_show_full, false)
  )
  const showSimplifiedTags = useStore((state) => configFlag(state.config?.jav_tag_show_simplified))
  const displayItems = useMemo(() => {
    if (!showSimplifiedTags) return items
    return (items || []).map((item) => ({
      ...item,
      tags: Array.isArray(item?.tags)
        ? item.tags.map((tag) => withJavTagDisplayName(tag, true))
        : item?.tags,
    }))
  }, [items, showSimplifiedTags])
  return {
    portraitMode,
    preferChineseName,
    hideSeries,
    hideIdols,
    hideTags,
    hideActions,
    showFullFavoriteRating,
    displayItems,
  }
}
