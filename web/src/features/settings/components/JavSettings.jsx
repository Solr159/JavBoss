import { useState } from 'react'
import { createJavSettingsDraft } from '@/features/settings/model'
import { useStore } from '@/store'
import { saveJavSettings } from '@/features/settings/actions'
import { getErrorMessage } from '@/utils/errors'
import JavSettingsModal from '@/features/settings/components/JavSettingsModal'

export default function JavSettings({ onClose, onError, onWaterfallChange, initialTab }) {
  const [draft, setDraft] = useState(() => createJavSettingsDraft(useStore.getState()))
  const [saving, setSaving] = useState(false)
  const handleSave = async () => {
    if (saving) return
    setSaving(true)
    try {
      await saveJavSettings(draft, onWaterfallChange)
      onClose()
    } catch (error) {
      onError(getErrorMessage(error))
    } finally {
      setSaving(false)
    }
  }
  return (
    <JavSettingsModal
      open={true}
      initialTab={initialTab}
      onClose={() => onClose()}
      titleTranslationEnabled={draft.titleTranslationEnabled}
      onTitleTranslationChange={(enabled) =>
        setDraft((current) => ({
          ...current,
          titleTranslationEnabled: enabled,
        }))
      }
      javPageSizeInput={draft.javPageSizeInput}
      onJavPageSizeChange={(value) =>
        setDraft((current) => ({ ...current, javPageSizeInput: value }))
      }
      javGridColumnsInput={draft.javGridColumnsInput}
      onJavGridColumnsChange={(value) =>
        setDraft((current) => ({ ...current, javGridColumnsInput: value }))
      }
      javTitleMaxRowsInput={draft.javTitleMaxRowsInput}
      onJavTitleMaxRowsChange={(value) =>
        setDraft((current) => ({ ...current, javTitleMaxRowsInput: value }))
      }
      javIdolTagMaxRowsInput={draft.javIdolTagMaxRowsInput}
      onJavIdolTagMaxRowsChange={(value) =>
        setDraft((current) => ({ ...current, javIdolTagMaxRowsInput: value }))
      }
      javTagMaxRowsInput={draft.javTagMaxRowsInput}
      onJavTagMaxRowsChange={(value) =>
        setDraft((current) => ({ ...current, javTagMaxRowsInput: value }))
      }
      javHideSeriesInput={draft.javHideSeriesInput}
      onJavHideSeriesChange={(value) =>
        setDraft((current) => ({ ...current, javHideSeriesInput: value }))
      }
      javHideIdolsInput={draft.javHideIdolsInput}
      onJavHideIdolsChange={(value) =>
        setDraft((current) => ({ ...current, javHideIdolsInput: value }))
      }
      javHideTagsInput={draft.javHideTagsInput}
      onJavHideTagsChange={(value) =>
        setDraft((current) => ({ ...current, javHideTagsInput: value }))
      }
      javHideActionsInput={draft.javHideActionsInput}
      onJavHideActionsChange={(value) =>
        setDraft((current) => ({ ...current, javHideActionsInput: value }))
      }
      javFavoriteRatingShowFullInput={draft.javFavoriteRatingShowFullInput}
      onJavFavoriteRatingShowFullChange={(value) =>
        setDraft((current) => ({ ...current, javFavoriteRatingShowFullInput: value }))
      }
      javWaterfallDefaultInput={draft.javWaterfallDefaultInput}
      onJavWaterfallDefaultChange={(value) =>
        setDraft((current) => ({ ...current, javWaterfallDefaultInput: value }))
      }
      idolPageSizeInput={draft.idolPageSizeInput}
      onIdolPageSizeChange={(value) =>
        setDraft((current) => ({ ...current, idolPageSizeInput: value }))
      }
      idolWaterfallDefaultInput={draft.idolWaterfallDefaultInput}
      onIdolWaterfallDefaultChange={(value) =>
        setDraft((current) => ({ ...current, idolWaterfallDefaultInput: value }))
      }
      studioPageSizeInput={draft.studioPageSizeInput}
      onStudioPageSizeChange={(value) =>
        setDraft((current) => ({ ...current, studioPageSizeInput: value }))
      }
      studioWaterfallDefaultInput={draft.studioWaterfallDefaultInput}
      onStudioWaterfallDefaultChange={(value) =>
        setDraft((current) => ({ ...current, studioWaterfallDefaultInput: value }))
      }
      seriesPageSizeInput={draft.seriesPageSizeInput}
      onSeriesPageSizeChange={(value) =>
        setDraft((current) => ({ ...current, seriesPageSizeInput: value }))
      }
      seriesWaterfallDefaultInput={draft.seriesWaterfallDefaultInput}
      onSeriesWaterfallDefaultChange={(value) =>
        setDraft((current) => ({ ...current, seriesWaterfallDefaultInput: value }))
      }
      javSortInput={draft.javSortInput}
      onJavSortChange={(value) => setDraft((current) => ({ ...current, javSortInput: value }))}
      javSortRulesInput={draft.javSortRulesInput}
      onJavSortRulesChange={(value) =>
        setDraft((current) => ({ ...current, javSortRulesInput: value }))
      }
      idolSortInput={draft.idolSortInput}
      onIdolSortChange={(value) => setDraft((current) => ({ ...current, idolSortInput: value }))}
      javIdolPreferChineseNameInput={draft.javIdolPreferChineseNameInput}
      onJavIdolPreferChineseNameChange={(value) =>
        setDraft((current) => ({ ...current, javIdolPreferChineseNameInput: value }))
      }
      javTagShowSimplifiedInput={draft.javTagShowSimplifiedInput}
      onJavTagShowSimplifiedChange={(value) =>
        setDraft((current) => ({ ...current, javTagShowSimplifiedInput: value }))
      }
      onSave={handleSave}
      saving={saving}
    />
  )
}
