import { useCallback } from 'react'

import TagManagementModal from '@/components/TagManagementModal'

export default function VideoTagModal({ tags, onToggleFilter, onApplyTagFilter, ...props }) {
  const handleApplyTagFilter = useCallback(
    (tagIds) => {
      const selectedIds = new Set((tagIds || []).map(Number))
      const names = (tags || [])
        .filter((tag) => selectedIds.has(Number(tag.id)))
        .map((tag) => tag.name)
      if (typeof onApplyTagFilter === 'function') {
        onApplyTagFilter(names)
        return
      }
      if (names.length === 1) onToggleFilter?.(names[0])
    },
    [onApplyTagFilter, onToggleFilter, tags]
  )

  return <TagManagementModal {...props} tags={tags} onApplyTagFilter={handleApplyTagFilter} />
}
