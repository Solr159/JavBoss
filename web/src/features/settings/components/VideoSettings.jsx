import { useState } from 'react'
import { createVideoSettingsDraft } from '@/features/settings/model'
import { useStore } from '@/store'
import { saveVideoSettings } from '@/features/settings/actions'
import { getErrorMessage } from '@/utils/errors'
import VideoSettingsModal from '@/features/settings/components/VideoSettingsModal'

export default function VideoSettings({ onClose, onError, onWaterfallChange }) {
  const [initialDraft] = useState(() => createVideoSettingsDraft(useStore.getState()))
  const [draft, setDraft] = useState(initialDraft)
  const [saving, setSaving] = useState(false)
  const dirty = Object.keys(draft).some((key) => draft[key] !== initialDraft[key])
  const handleSave = async () => {
    if (saving) return
    setSaving(true)
    try {
      await saveVideoSettings(draft, onWaterfallChange)
      onClose()
    } catch (error) {
      onError(getErrorMessage(error))
    } finally {
      setSaving(false)
    }
  }
  return (
    <VideoSettingsModal
      open={true}
      onClose={() => onClose()}
      pageSizeInput={draft.videoPageSizeInput}
      onPageSizeChange={(value) =>
        setDraft((current) => ({ ...current, videoPageSizeInput: value }))
      }
      sortInput={draft.videoSortInput}
      onSortChange={(value) => setDraft((current) => ({ ...current, videoSortInput: value }))}
      hideJavInput={draft.videoHideJavInput}
      onHideJavChange={(value) => setDraft((current) => ({ ...current, videoHideJavInput: value }))}
      waterfallDefaultInput={draft.videoWaterfallDefaultInput}
      onWaterfallDefaultChange={(value) =>
        setDraft((current) => ({ ...current, videoWaterfallDefaultInput: value }))
      }
      watchTimeIconMinutesInput={draft.videoWatchTimeIconMinutesInput}
      onWatchTimeIconMinutesChange={(value) =>
        setDraft((current) => ({ ...current, videoWatchTimeIconMinutesInput: value }))
      }
      autoSubtitleEnabledInput={draft.autoSubtitleEnabledInput}
      onAutoSubtitleEnabledChange={(value) =>
        setDraft((current) => ({ ...current, autoSubtitleEnabledInput: value }))
      }
      autoSubtitleLanguageInput={draft.autoSubtitleLanguageInput}
      onAutoSubtitleLanguageChange={(value) =>
        setDraft((current) => ({ ...current, autoSubtitleLanguageInput: value }))
      }
      dirty={dirty}
      onSave={handleSave}
      saving={saving}
    />
  )
}
